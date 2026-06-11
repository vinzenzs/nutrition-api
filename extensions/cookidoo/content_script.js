// content_script.js — runs on Cookidoo recipe pages.
//
// Walks every <script type="application/ld+json"> looking for a Recipe block
// (Schema.org), normalises it into a shape the popup expects, and stores it
// in chrome.storage.session keyed by tab id. The popup queries this storage
// when the user clicks the toolbar button.
//
// No bundler, no dependencies. ES modules-style globals via the function
// scope below.

(function () {
  "use strict";

  // ---- field parsers ----

  // Returns a positive number parsed from the leading numeric portion of s,
  // or null. Handles decimal commas and ignores trailing text.
  function parseLeadingNumber(s) {
    if (s == null) return null;
    const str = String(s).trim().replace(",", ".");
    const m = str.match(/^[-+]?\d+(\.\d+)?/);
    if (!m) return null;
    const n = parseFloat(m[0]);
    return Number.isFinite(n) ? n : null;
  }

  // Extract a gram quantity from a free-text string like "350 g", "350g",
  // "  350 grams ". Returns null if the trailing unit is not grams.
  function parseGrams(s) {
    if (s == null) return null;
    const str = String(s).trim().toLowerCase();
    const m = str.match(/^([\d.,]+)\s*(g|grams?)\b/);
    if (!m) return null;
    return parseLeadingNumber(m[1]);
  }

  // Schema.org NutritionInformation strings carry a unit suffix: "580 kcal",
  // "2425 kJ", "33 g", "560 mg". Returns { value, unit } or null.
  function parseUnitValue(s) {
    if (s == null) return null;
    const str = String(s).trim();
    const m = str.match(/^([\d.,]+)\s*([a-zA-Zµ]+)?/);
    if (!m) return null;
    const value = parseLeadingNumber(m[1]);
    if (value == null) return null;
    const unit = (m[2] || "").toLowerCase();
    return { value, unit };
  }

  // ---- recipe normaliser ----

  function flattenGraph(node) {
    if (!node || typeof node !== "object") return [];
    if (Array.isArray(node)) return node.flatMap(flattenGraph);
    if (node["@graph"] && Array.isArray(node["@graph"])) return node["@graph"];
    return [node];
  }

  function isRecipeNode(n) {
    if (!n || typeof n !== "object") return false;
    const t = n["@type"];
    if (t === "Recipe") return true;
    if (Array.isArray(t) && t.includes("Recipe")) return true;
    return false;
  }

  // Convert a Schema.org NutritionInformation block (per serving) into a
  // best-effort per-serving nutriment object using the fields nutrition-api
  // understands. Anything we can't parse stays absent.
  function parsePerServingNutriments(nutrition, warnings) {
    if (!nutrition || typeof nutrition !== "object") return {};
    const out = {};

    // Energy: prefer kcal; fall back to kJ → kcal.
    const kcalPart = parseUnitValue(nutrition.calories);
    if (kcalPart) {
      if (kcalPart.unit === "kcal" || kcalPart.unit === "cal" || kcalPart.unit === "") {
        out.kcal = kcalPart.value;
      } else if (kcalPart.unit === "kj") {
        out.kcal = round1(kcalPart.value / 4.184);
      } else {
        warnings.push(`unknown energy unit: ${kcalPart.unit}`);
      }
    }
    // Some sites use a separate "kilojoulesContent" field.
    if (out.kcal == null && nutrition.kilojoulesContent != null) {
      const kj = parseUnitValue(nutrition.kilojoulesContent);
      if (kj && (kj.unit === "kj" || kj.unit === "")) {
        out.kcal = round1(kj.value / 4.184);
      }
    }

    // Grams-valued macros.
    addGramField(out, "protein_g", nutrition.proteinContent, warnings, "protein");
    addGramField(out, "carbs_g", nutrition.carbohydrateContent, warnings, "carbs");
    addGramField(out, "fat_g", nutrition.fatContent, warnings, "fat");
    addGramField(out, "fiber_g", nutrition.fiberContent, warnings, "fiber");
    addGramField(out, "sugar_g", nutrition.sugarContent, warnings, "sugar");

    // Sodium → salt: salt(g) ≈ sodium(mg) × 2.5 / 1000.
    if (nutrition.sodiumContent != null) {
      const sodium = parseUnitValue(nutrition.sodiumContent);
      if (sodium != null) {
        if (sodium.unit === "mg") {
          out.salt_g = round3((sodium.value * 2.5) / 1000);
        } else if (sodium.unit === "g") {
          out.salt_g = round3(sodium.value * 2.5);
        } else {
          warnings.push(`unknown sodium unit: ${sodium.unit}`);
        }
      }
    }

    return out;
  }

  function addGramField(out, key, raw, warnings, label) {
    if (raw == null) return;
    const v = parseUnitValue(raw);
    if (v == null) return;
    if (v.unit === "g" || v.unit === "") {
      out[key] = v.value;
    } else if (v.unit === "mg") {
      out[key] = round3(v.value / 1000);
    } else {
      warnings.push(`unknown ${label} unit: ${v.unit}`);
    }
  }

  function round1(n) {
    return Math.round(n * 10) / 10;
  }
  function round3(n) {
    return Math.round(n * 1000) / 1000;
  }

  // ---- main extraction ----

  function extractRecipe() {
    const warnings = [];
    const scripts = document.querySelectorAll('script[type="application/ld+json"]');
    let recipe = null;
    for (const el of scripts) {
      let data;
      try {
        data = JSON.parse(el.textContent || "");
      } catch (_e) {
        continue;
      }
      const candidates = flattenGraph(data);
      const found = candidates.find(isRecipeNode);
      if (found) {
        recipe = found;
        break;
      }
    }
    if (!recipe) return { recipe: null, warnings: ["no Recipe JSON-LD on page"] };

    const servings = parseLeadingNumber(
      typeof recipe.recipeYield === "string"
        ? recipe.recipeYield
        : Array.isArray(recipe.recipeYield)
        ? recipe.recipeYield[0]
        : recipe.recipeYield
    );

    let servingSizeG = null;
    if (recipe.nutrition && recipe.nutrition.servingSize) {
      servingSizeG = parseGrams(recipe.nutrition.servingSize);
    }

    const perServing = parsePerServingNutriments(recipe.nutrition, warnings);

    // recipeIngredient is an ordered array of verbatim free-text strings
    // (e.g. "100 g Staudensellerie"). Kept exactly as the page provides them —
    // nutrition-api stores them unparsed for the shopping-list flow.
    let ingredients = [];
    if (Array.isArray(recipe.recipeIngredient)) {
      ingredients = recipe.recipeIngredient.filter((x) => typeof x === "string");
    } else if (typeof recipe.recipeIngredient === "string" && recipe.recipeIngredient !== "") {
      ingredients = [recipe.recipeIngredient];
    }

    return {
      recipe: {
        name: typeof recipe.name === "string" ? recipe.name : "",
        image: typeof recipe.image === "string" ? recipe.image : null,
        url: typeof recipe.url === "string" ? recipe.url : location.href,
        servings: servings,
        servingSizeG: servingSizeG,
        perServingNutriments: perServing,
        ingredients: ingredients,
      },
      warnings,
    };
  }

  // ---- store + reply to popup ----

  function store(payload) {
    try {
      chrome.storage.session
        .set({ [keyForTab()]: payload })
        .catch(() => {});
    } catch (_e) {
      // chrome.storage.session may not be available in very old MV3 builds;
      // fall back to local.
      try {
        chrome.storage.local
          .set({ [keyForTab()]: payload })
          .catch(() => {});
      } catch (_e2) {}
    }
  }

  function keyForTab() {
    // The tab id isn't available inside the content script directly. We use
    // the page URL as a stable key; popup.js does the same lookup.
    return "recipe:" + location.href;
  }

  // Diagnostic dump on every fresh extraction. Helps when Cookidoo (or another
  // site) uses a JSON-LD shape this parser does not yet understand — open
  // DevTools console on the recipe page to see what was found.
  function logExtraction(label, result) {
    const r = result && result.recipe;
    // eslint-disable-next-line no-console
    console.log(
      "[nutrition-api importer] " + label,
      {
        hasRecipe: !!r,
        name: r ? r.name : null,
        servings: r ? r.servings : null,
        servingSizeG: r ? r.servingSizeG : null,
        perServing: r ? r.perServingNutriments : null,
        warnings: result ? result.warnings : null,
      }
    );
  }

  // Run once at document_idle and store the result.
  const initial = extractRecipe();
  store(initial);
  logExtraction("initial scan", initial);

  // Re-extract on demand when the popup opens. SPAs (Cookidoo included) often
  // inject nutrition into the DOM after document_idle, so the popup's view
  // should always reflect the current DOM rather than the stale cache.
  chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
    if (msg && msg.type === "get_recipe") {
      const fresh = extractRecipe();
      store(fresh);
      logExtraction("popup re-scan", fresh);
      sendResponse(fresh);
    }
  });
})();
