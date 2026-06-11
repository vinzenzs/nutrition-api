// popup.js — runs when the user clicks the toolbar button.
//
// Reads the recipe stored by content_script.js (per-tab URL key), pre-fills
// the form, computes per-100g values whenever serving_size_g is known, and
// POSTs to <API_URL>/products on submit.

"use strict";

const NUTRIMENT_FIELDS = [
  "kcal",
  "protein_g",
  "carbs_g",
  "fat_g",
  "fiber_g",
  "sugar_g",
  "salt_g",
  "iron_mg",
  "calcium_mg",
  "vitamin_d_mcg",
  "vitamin_b12_mcg",
  "vitamin_c_mg",
  "magnesium_mg",
  "potassium_mg",
  "zinc_mg",
];

// ---- UI helpers ----

const $ = (id) => document.getElementById(id);

function showBanner(kind, text) {
  for (const k of ["info", "warn", "error", "success"]) {
    const el = $(`banner-${k}`);
    if (k === kind) {
      el.textContent = text;
      el.classList.remove("hidden");
    } else {
      el.textContent = "";
      el.classList.add("hidden");
    }
  }
}

function hideAllBanners() {
  for (const k of ["info", "warn", "error", "success"]) {
    $(`banner-${k}`).classList.add("hidden");
  }
}

function readNum(id) {
  const el = $(id);
  if (!el || el.value === "") return null;
  const n = Number(el.value);
  return Number.isFinite(n) ? n : null;
}

function setNum(id, n) {
  if (n == null || !Number.isFinite(n)) return;
  $(id).value = String(n);
}

function round1(n) { return Math.round(n * 10) / 10; }
function round3(n) { return Math.round(n * 1000) / 1000; }

function nutrimentStep(field) {
  if (field === "kcal" || field === "calcium_mg" || field === "vitamin_c_mg" || field === "magnesium_mg" || field === "potassium_mg") return 1;
  if (field === "salt_g" || field === "vitamin_d_mcg" || field === "vitamin_b12_mcg") return 1000;
  return 100;
}

// ---- per-serving → per-100g auto-fill ----

function autoFillPer100g(perServing, servingSizeG) {
  if (!perServing || !servingSizeG || !(servingSizeG > 0)) return;
  const factor = 100 / servingSizeG;
  for (const f of NUTRIMENT_FIELDS) {
    const v = perServing[f];
    if (v == null) continue;
    // Don't clobber user-entered values that are different from the empty state.
    const current = $(f);
    if (current && current.dataset.userEdited === "true") continue;
    let val = v * factor;
    val = Math.round(val * 1000) / 1000;
    if (current) {
      current.value = String(val);
    }
  }
}

// ---- options + storage ----

async function getOptions() {
  return new Promise((resolve) => {
    chrome.storage.sync.get(
      { api_url: "http://localhost:8080", token: "", token_type: "mobile" },
      (items) => resolve(items)
    );
  });
}

function recipeStorageKey(url) {
  return "recipe:" + url;
}

async function getStoredRecipe(tabUrl) {
  const key = recipeStorageKey(tabUrl);
  try {
    const items = await chrome.storage.session.get(key);
    if (items && items[key]) return items[key];
  } catch (_e) {}
  // Fallback to local in case session storage wasn't available.
  const items = await new Promise((resolve) => chrome.storage.local.get(key, resolve));
  return items && items[key] ? items[key] : null;
}

async function activeTab() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  return tab;
}

// ---- main ----

let recipeContext = { tabUrl: "", recipe: null, warnings: [] };

async function init() {
  $("open-options").addEventListener("click", () => chrome.runtime.openOptionsPage());

  // Track user edits so the auto-fill doesn't overwrite them.
  for (const f of NUTRIMENT_FIELDS) {
    const el = $(f);
    if (el) el.addEventListener("input", () => { el.dataset.userEdited = "true"; });
  }
  $("serving_size_g").addEventListener("input", () => {
    const sz = readNum("serving_size_g");
    autoFillPer100g(recipeContext.recipe ? recipeContext.recipe.perServingNutriments : null, sz);
    updateSaveEnabled();
  });
  $("name").addEventListener("input", updateSaveEnabled);

  const tab = await activeTab();
  recipeContext.tabUrl = tab && tab.url ? tab.url : "";
  $("external_url").textContent = recipeContext.tabUrl;

  // Always ask the content script to re-extract — Cookidoo and other SPAs
  // inject nutrition data after document_idle, so the stored snapshot from the
  // initial scan may be stale. Fall back to the snapshot only if messaging
  // fails (e.g. content script not on this page).
  let payload = null;
  if (tab && tab.id) {
    try {
      payload = await new Promise((resolve) => {
        chrome.tabs.sendMessage(tab.id, { type: "get_recipe" }, (resp) => {
          if (chrome.runtime.lastError) resolve(null);
          else resolve(resp);
        });
      });
    } catch (_e) {
      payload = null;
    }
  }
  if ((!payload || !payload.recipe) && recipeContext.tabUrl) {
    payload = await getStoredRecipe(recipeContext.tabUrl);
  }

  if (payload && payload.recipe) {
    recipeContext.recipe = payload.recipe;
    recipeContext.warnings = payload.warnings || [];
    populateFromRecipe(payload.recipe);

    const r = payload.recipe;
    const ps = r.perServingNutriments || {};
    const hasPerServing = Object.keys(ps).length > 0;

    if (r.servingSizeG == null && hasPerServing) {
      // The most common Cookidoo case: per-serving values are present but no
      // gram weight for one serving. We pre-fill 100 g so the per-100 g grid
      // populates immediately; the user adjusts to the real serving weight
      // before Save if they want accurate per-100 g math.
      const parts = [];
      if (ps.kcal != null) parts.push(`${ps.kcal} kcal`);
      if (ps.protein_g != null) parts.push(`${ps.protein_g} g protein`);
      if (ps.carbs_g != null) parts.push(`${ps.carbs_g} g carbs`);
      if (ps.fat_g != null) parts.push(`${ps.fat_g} g fat`);
      const summary = parts.join(", ");
      showBanner(
        "info",
        `Per serving: ${summary}. Defaulted to ${DEFAULT_SERVING_SIZE_G} g per serving — ` +
          `change the serving size below to match the real portion weight and the per-100 g values will recompute.`
      );
    } else if (recipeContext.warnings.length) {
      showBanner("warn", "Parsed with warnings: " + recipeContext.warnings.join("; "));
    }
  } else {
    // No JSON-LD recipe on the page. Pre-fill the serving size so the user
    // doesn't have to type one before Save becomes enabled.
    $("serving_size_g").value = String(DEFAULT_SERVING_SIZE_G);
    showBanner("info", `No recipe detected on this page — fill the form manually. Serving size defaulted to ${DEFAULT_SERVING_SIZE_G} g; change it before Save if needed.`);
  }

  const opts = await getOptions();
  if (!opts.token) {
    showBanner("warn", "Configure the extension options before saving.");
  }
  updateSaveEnabled();
}

// DEFAULT_SERVING_SIZE_G is what we pre-fill when neither the JSON-LD nor
// the user has supplied a serving weight. 100 makes the per-100g grid
// populate immediately (under the identity transform `× 100/100 = ×1`) and
// enables Save out of the gate; the user adjusts before saving if the
// actual serving weight is different.
const DEFAULT_SERVING_SIZE_G = 100;

function populateFromRecipe(r) {
  if (r.name) $("name").value = r.name;
  if (r.servings != null) $("servings").value = String(r.servings);
  const sz = r.servingSizeG != null ? r.servingSizeG : DEFAULT_SERVING_SIZE_G;
  $("serving_size_g").value = String(sz);
  autoFillPer100g(r.perServingNutriments, sz);
  // Ingredients are captured verbatim and sent on Save, but not editable here —
  // show a read-only count so the user knows they came along.
  const el = $("ingredients-summary");
  if (el) {
    const n = Array.isArray(r.ingredients) ? r.ingredients.length : 0;
    if (n > 0) {
      el.textContent = `${n} ingredient${n === 1 ? "" : "s"} captured`;
      el.classList.remove("hidden");
    } else {
      el.textContent = "";
      el.classList.add("hidden");
    }
  }
}

function updateSaveEnabled() {
  const name = $("name").value.trim();
  const size = readNum("serving_size_g");
  const ok = name.length > 0 && size != null && size > 0;
  $("save").disabled = !ok;
}

async function onSubmit(ev) {
  ev.preventDefault();
  hideAllBanners();

  const opts = await getOptions();
  if (!opts.token) {
    showBanner("error", "No token configured — open the options first.");
    return;
  }

  const nutriments = {};
  for (const f of NUTRIMENT_FIELDS) {
    const v = readNum(f);
    if (v != null) nutriments[f] = v;
  }

  const body = {
    name: $("name").value.trim(),
    source: "recipe",
    external_url: recipeContext.tabUrl || null,
    serving_size_g: readNum("serving_size_g"),
    nutriments_per_100g: nutriments,
  };

  // Attach the verbatim ingredient list when the page provided one. nutrition-api
  // stores it unparsed; only recipe-source products may carry it.
  const ingredients =
    recipeContext.recipe && Array.isArray(recipeContext.recipe.ingredients)
      ? recipeContext.recipe.ingredients
      : [];
  if (ingredients.length > 0) {
    body.ingredients = ingredients;
  }

  const url = opts.api_url.replace(/\/+$/, "") + "/products";
  $("save").disabled = true;
  showBanner("info", "Saving…");

  try {
    const resp = await fetch(url, {
      method: "POST",
      headers: {
        "Authorization": "Bearer " + opts.token,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });
    let respText = await resp.text();
    if (resp.ok) {
      let id = "";
      try {
        const parsed = JSON.parse(respText);
        if (parsed && parsed.id) id = parsed.id;
      } catch (_e) {}
      showBanner(
        "success",
        id
          ? `Saved. id=${id}`
          : `Saved (${resp.status}).`
      );
      $("save").textContent = "Close";
      $("save").disabled = false;
      $("save").addEventListener("click", () => window.close(), { once: true });
      return;
    }
    showBanner("error", `${resp.status} ${respText}`);
    $("save").disabled = false;
  } catch (e) {
    showBanner("error", `Could not reach ${opts.api_url}: ${e.message || e}`);
    $("save").disabled = false;
  }
}

document.getElementById("form").addEventListener("submit", onSubmit);
init();
