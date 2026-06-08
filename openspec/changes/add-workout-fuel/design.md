## Context

The capture surfaces today are: `meals` (food, by product or freeform name + per-100g nutriments), `hydration` (volume-only sips), `workouts` (sessions), `body_weight_entries` (measurements). What's missing is the *in-session* fueling layer — gels, electrolyte drinks, salt tabs, caffeine pre-race — that carries its own unit shape (carbs in g per item, sodium in mg per item, caffeine in mg per item, optional ml when it's a drink). Trying to fit this into meals (per-100g nutriments) misrepresents the data (a gel is 22g total, not "per 100g"). Trying to fit it into hydration would force ml into a struct that needs to carry mg, which is the exact unit-mixing footgun we explicitly avoided when shipping hydration.

A sibling capability is the cheap, honest answer. It mirrors the hydration shape (one table, CRUD + window-list, no daily summary in v1), borrows the `workout_id` link semantics from `add-meal-workout-link`, and cashes in the promise both that change and `add-workouts-capability` made: when workout_fuel exists, the workout-anchored fueling endpoint composes it in for free.

## Goals / Non-Goals

**Goals:**

- Capture: log a fueling event with its actual numbers — what was it (free-text `name`), how much volume (optional), how much of each measurable nutriment (carbs, sodium, potassium, caffeine; all optional but at least one required).
- Persist the optional link to a workout (same FK semantics as meals/hydration after `add-meal-workout-link`).
- Compose into `/workouts/{id}/fueling` so the workout-anchored fueling read is complete.
- Stay unit-isolated: hydration daily summary does NOT include workout_fuel ml; nutrition daily summary does NOT include workout_fuel carbs. Each capability owns its own totals; the workout-anchored summary is the only cross-capability composer.

**Non-Goals:**

- Daily summary endpoint for workout fuel.
- Product catalog (`workout_fuel_products`) and ID-based references.
- Per-hour-rate computation (carbs/hr, sodium/hr) as a stored or computed field.
- Garmin / smart-bottle ingestion.
- Bulk endpoint.
- Cross-aggregation into daily hydration or daily nutrition summaries.
- Snapshot semantics.
- New capability for "supplements" (B12, D3, iron pills) — that's a different data shape (typically taken outside workout context); `priorities.md` T2 #9 is a separate change.

## Decisions

### 1. New table, NOT a column extension on `hydration_entries`

```sql
CREATE TABLE workout_fuel_entries (
    id            UUID PRIMARY KEY,
    logged_at     TIMESTAMPTZ NOT NULL,
    name          TEXT NOT NULL CHECK (length(name) > 0),
    quantity_ml   NUMERIC(10, 1) NULL CHECK (quantity_ml IS NULL OR quantity_ml > 0),
    carbs_g       NUMERIC(10, 1) NULL CHECK (carbs_g       IS NULL OR carbs_g       >= 0),
    sodium_mg     NUMERIC(10, 1) NULL CHECK (sodium_mg     IS NULL OR sodium_mg     >= 0),
    potassium_mg  NUMERIC(10, 1) NULL CHECK (potassium_mg  IS NULL OR potassium_mg  >= 0),
    caffeine_mg  NUMERIC(10, 1) NULL CHECK (caffeine_mg   IS NULL OR caffeine_mg   >= 0),
    note          TEXT NULL,
    workout_id    UUID NULL REFERENCES workouts(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX workout_fuel_entries_logged_at_idx ON workout_fuel_entries (logged_at);
CREATE INDEX workout_fuel_entries_workout_id_idx ON workout_fuel_entries (workout_id)
    WHERE workout_id IS NOT NULL;
```

The case for a sibling table over column extension on hydration:

- **Unit isolation.** Hydration's Totals struct carries `total_ml`. Extending it with `sodium_mg` / `carbs_g` reintroduces the exact unit-mixing footgun we explicitly avoided in `add-hydration-tracking` (per its "Why" — *"mixing g and ml in one Totals struct is a footgun"*).
- **Semantic separation.** "Did I drink enough water today" and "did I take the right gels during my ride" are different mental models. Conflating them forces callers to filter every read by intent.
- **Honest cardinality.** Most hydration entries are nutriment-free (water). Most workout-fuel entries carry at least one nutriment. Keeping the tables separate keeps both schemas honest about what they actually hold.

**Alternatives considered:**

- *Add `sodium_mg`, `carbs_g`, `caffeine_mg` (all nullable) to `hydration_entries`.* Rejected — re-introduces the unit-mixing footgun.
- *Add the new fields to `meal_entries`.* Rejected — meals carry per-100g nutriments tied to a `quantity_g`; gels are total-amount data, not per-100g.
- *Make this a new `intake_events` table with a `kind` discriminator (water / fuel / supplement).* Considered. Rejected — discriminator-based tables are notoriously brittle; the unit shapes are too different to share a row.

### 2. At least one quantitative field required

An entry that has only `name` and `note` is just narrative — there's no number for any tool to aggregate. The service rejects with `400 empty_entry` unless at least one of `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` is non-null.

This catches the common mistake "I logged a gel and forgot to enter the carbs." Loud-over-silent.

**Alternatives considered:**

- *Allow empty entries.* Rejected — these would pollute the fueling summary's `entry_count` without contributing any total.
- *Require `name` AND at least one quantitative field.* That's the chosen rule. `name` is always required (the rehearsal-data value depends on knowing *what* you took); quantitative is at-least-one.

### 3. `quantity_ml > 0`; nutriment fields `>= 0`

For `quantity_ml`, zero is meaningless (you didn't drink anything; just omit). For nutriment fields, zero is a meaningful signal ("yes I measured, this gel had no caffeine") that distinguishes from "didn't measure / N/A" (null).

```
quantity_ml      > 0  when supplied  (else omit)
carbs_g         >= 0  when supplied
sodium_mg       >= 0  when supplied
potassium_mg    >= 0  when supplied
caffeine_mg     >= 0  when supplied
```

**Alternatives considered:**

- *All numeric fields `> 0`.* Rejected — wipes out the "explicitly zero caffeine" signal; agents would have to omit, losing the "I confirmed this" semantic.

### 4. Optional `workout_id` with the empty-string clear semantic

Same shape as `add-meal-workout-link`'s extension to meals and hydration:

- POST: `workout_id` optional; validated to reference an existing workout; `400 workout_not_found` on miss.
- PATCH: `"<uuid>"` sets, `""` clears, missing leaves unchanged.
- `ON DELETE SET NULL`: deleting a workout clears the link on its workout_fuel rows.

Identical tri-state pattern; identical sentinel. No new idioms to remember.

### 5. `/workouts/{id}/fueling` extension: third sub-object per window

The endpoint added by `add-meal-workout-link` returns each window with `nutrition` and `hydration` sub-objects. This change adds `workout_fuel`:

```json
"intra_window": {
  "start": "...",
  "end":   "...",
  "minutes": 90,
  "nutrition": {
    "totals":      { "kcal": 420, "carbs_g": 75, ... },
    "entry_count": 1
  },
  "hydration": {
    "total_ml":    750,
    "entry_count": 2
  },
  "workout_fuel": {
    "totals": {
      "quantity_ml":  500,
      "carbs_g":      80,
      "sodium_mg":    600,
      "potassium_mg": 150,
      "caffeine_mg":  100
    },
    "entry_count": 3
  }
}
```

Same matching rule as for nutrition + hydration: time-window-by-`logged_at`, regardless of the `workout_id` tag value. The tag is metadata; the window is the aggregation key.

`workout_fuel.totals` carries this capability's own field shape — separate from `nutrition.totals` (kcal + macros + micros). No mixing.

**Alternatives considered:**

- *Merge workout_fuel into the existing `nutrition.totals` (since gels have carbs).* Rejected — would lose sodium/caffeine (which `nutrition` doesn't carry), would commingle "what I ate" with "what I took during the ride" (different rehearsal-data semantics), and would make the post-shipping daily-macro adherence subtly include in-session fuel. Three different problems.
- *Add workout_fuel ml to `hydration.total_ml`.* Rejected — same unit-isolation argument as #1 + would make hydration's daily-summary contract leak into workout_fuel.

### 6. No daily summary endpoint in v1

Reasoning: the workout-anchored summary (`/workouts/{id}/fueling`) covers the primary question ("did this session get the right fueling"). For a day-wide rollup the agent can compose from `GET /workout-fuel?from=&to=` and sum. Adding `/summary/workout-fuel/daily` is YAGNI today.

If real use shows the agent constantly composes the daily total, a one-endpoint follow-up adds it. The pattern is already proven across hydration / weight.

### 7. MCP tool count: 4, not 5

Hydration / weight / workouts each ship with 5 tools (CRUD + a summary tool). Workout-fuel ships with 4 (CRUD only). The "summary" position is filled by the existing `workout_fueling_summary` tool from `add-meal-workout-link` — which now naturally surfaces workout_fuel contributions for the workout-anchored question.

No `daily_workout_fuel_summary` tool in v1, for the same YAGNI reason. Agent composes if needed.

**Alternatives considered:**

- *Ship 5 tools for symmetry.* Rejected — symmetry isn't enough of a reason to add an empty MCP surface. The agent's "did I take enough sodium today" question is rare relative to the per-workout question; defer the rollup tool.

### 8. Hydration vs workout-fuel routing: simple rule

The agent decides where to log based on the data shape:

```
Plain water / juice (volume only)           → log_hydration
Anything with electrolytes / carbs / caffeine → log_workout_fuel
```

The MCP description for both tools should call this rule out explicitly so the agent doesn't second-guess. The fueling summary picks up both; the daily hydration summary picks up only hydration_entries.

**Alternatives considered:**

- *"During a workout" goes to workout_fuel, otherwise hydration.* Rejected — too context-driven; the agent has to know whether the moment was "during a workout" without certainty about workout timing.
- *Always offer both; let the agent and user disambiguate.* That's effectively what happens — but the simple "does it carry nutriments" rule keeps tooling decisions predictable.

## Risks / Trade-offs

- **Daily hydration total understates total fluid intake for users who track electrolyte drinks as workout_fuel.** A user who tracks every ride bottle as workout_fuel will see a lower "daily ml" in the hydration summary than their actual fluid consumption. Mitigation: the rule is documented; the `/workouts/{id}/fueling` summary covers the in-session view; if the friction is real, a single `?include=workout_fuel_ml` flag on the hydration daily summary is a small follow-up.
- **At-least-one-quantitative validation may catch legitimate "I forgot to enter the numbers" cases.** Better than the silent alternative — an empty row pollutes summaries. Agent users will see `400 empty_entry` and can re-prompt for the missing field.
- **`name` is free-text, not a product.** Rehearsal data ("Maurten 320 worked, SiS Beta Fuel didn't") will require fuzzy matching across spellings. Mitigation: agents are good at normalising; if structured rehearsal becomes a real workflow, `workout_fuel_products` is a clean follow-up that doesn't require migrating existing entries (the `name` text stays alongside an optional `product_id`).
- **Cross-capability dependency for the fueling extension.** The workouts package's `/workouts/{id}/fueling` handler now reads from three repos (meals, hydration, workout_fuel). One more constructor argument; the aggregation logic gets a third loop. Surface, not behaviour, change.
- **Five nullable nutriment columns invite "should magnesium be here too?" requests.** The field set is deliberately tight — sodium / potassium / caffeine / carbs cover ~95% of real fueling decisions. Magnesium, calcium, vitamin C in workout fuel exist but are second-order. Adding columns later is cheap; over-fitting the schema upfront isn't.
- **The `workout_id` validation cost on every POST/PATCH.** One extra DB read per write when the field is supplied. Negligible at single-user write rates; matches the cost added by `add-meal-workout-link` to meals/hydration writes.

## Migration Plan

- Forward: create `workout_fuel_entries` + the two indexes. No backfill.
- Rollback: drop the table.
- The migration is numbered `015_add_workout_fuel` (next after `014_add_workout_link_to_intake`).
- If applied in a different order — e.g. before `add-meal-workout-link` — renumber at apply time. The pattern was established by `add-hydration-tracking` ↔ `add-date-varying-goals`.

## Open Questions

- Whether to add `protein_g` as a sixth nullable column. Some recovery drinks include protein. Tentative answer: no for v1 — protein in workout fuel is rare; recovery nutrition is better captured as a post-workout meal. Add later if real use shows it matters.
- Whether `name` should have a max length (e.g. 200 chars) like product names elsewhere. Tentative answer: yes, 200 chars (matches `products.name` convention); document in spec.
- Whether `/workouts/{id}/fueling` should expose computed rates (e.g. `intra_window.workout_fuel.carbs_per_hour`). Tentative answer: no for v1 — derivable; if the agent needs it consistently, `?include=rates` adds it without a contract change.
- Whether to add a `quantity_servings` field (e.g. "0.5 gel") for partial servings. Tentative answer: no — record the actual amounts (carbs/sodium directly). Partial-serving math is the agent's job.
