# nutrition-api: Cookidoo importer (Chrome extension)

Saves Cookidoo recipes to nutrition-api as `source=recipe` products you can
log meals against from the mobile app or the LLM agent.

> **Status:** v0.1 — the extension exists but depends on a backend that
> accepts the `source` and `external_url` fields on `POST /products`. That
> backend extension is the second half of the `add-cookidoo-importer`
> change; until it ships, the popup will receive a 400 from the API. See
> `openspec/changes/add-cookidoo-importer/` for the in-flight plan.

## Limits

- **Chrome only.** Manifest V3, no Firefox manifest variant in v0.1.
- **Cookidoo JSON-LD dependent.** The extension reads the page's
  `application/ld+json` Recipe block. If Cookidoo changes their page
  structure (or you're on a recipe that doesn't have JSON-LD for some
  reason), the popup shows "no recipe detected" and you can fill the form
  by hand.
- **No backend scraping.** The extension reads the page in the browser you
  are already logged into. Nothing on the server fetches Cookidoo.
- **No auto-sync.** Re-importing the same recipe creates a second product
  row; clean up by hand.

## Install (Load unpacked)

1. Go to `chrome://extensions/` and toggle **Developer mode** on.
2. Click **Load unpacked** and select the
   `extensions/cookidoo/` directory in this repo.
3. The extension shows up in the toolbar.

## Configure

Open the options page (right-click the toolbar icon → **Options**, or via
`chrome://extensions/` → Details → Extension options) and fill in:

| Field        | Value                                                              |
|--------------|--------------------------------------------------------------------|
| API base URL | The URL of your running nutrition-api. Default: `http://localhost:8080`. |
| Token        | Either `MOBILE_API_TOKEN` or `AGENT_API_TOKEN` from your `.env.local`. |
| Token type   | Pick whichever token you pasted. Affects `client_id` in API logs only. |

Hit **Save**. The values are stored in `chrome.storage.sync` and follow your
Chrome profile across machines.

## Use

1. With `task dev` running (`http://localhost:8080/healthz` returns OK),
   open a Cookidoo recipe page in Chrome.
2. Click the **nutrition-api: Cookidoo importer** toolbar button.
3. The popup pre-fills the recipe name, servings, serving size (when
   Cookidoo's JSON-LD gives grams), and per-100g macros. Edit anything
   that looks wrong.
4. Click **Save**. The popup either confirms with the new product id or
   shows the backend's error verbatim so you can fix and retry.
5. From the mobile app or the LLM agent, search for the recipe by name
   and log meals against it as you eat servings.

## Files

```
extensions/cookidoo/
├── manifest.json        MV3 manifest
├── content_script.js    JSON-LD extraction; runs on cookidoo.*/recipes/recipe/*
├── popup.html / popup.js   preview + Save form (toolbar button)
├── options.html / options.js   API URL + token + token type
├── icons/               placeholder PNGs (16/48/128)
└── README.md            (this file)
```

No bundler, no npm. The whole extension is plain ES + HTML/CSS, so reading
it is the same as reading the code in this repo.

## Troubleshooting

**Popup says "no recipe detected on this page".**
Either the page isn't a Cookidoo recipe URL (the content script only runs
on `*/recipes/recipe/*`), or Cookidoo dropped the JSON-LD on that recipe.
Fill the form manually and Save.

**Save returns `400 source_invalid` or `400 external_url_too_long`.**
The backend half of `add-cookidoo-importer` isn't merged yet. Apply the
backend tasks (sections 1–3 of `openspec/changes/add-cookidoo-importer/tasks.md`)
or work around it by switching the popup's `source` field to `manual`
(remove the recipe-source assumption).

**Save returns `401 auth_required` or `401 auth_invalid`.**
Token in the options doesn't match `MOBILE_API_TOKEN` or `AGENT_API_TOKEN`
in your `.env.local`. Re-paste and Save options again.

**Save returns `could not reach http://localhost:8080`.**
The REST API isn't running, or the URL doesn't match. Start it with
`task dev` and verify `curl http://localhost:8080/healthz` from your shell.

**The per-100g values look way too high.**
Cookidoo's JSON-LD nutrition is typically per serving. The popup divides
by the `serving_size_g` field to produce per-100g. If `serving_size_g` is
the wrong value, the math is off — re-check it before Save.
