## Context

The goals capability evolved organically: kcal landed first as a single number, macros came in with `{min, max}`, then min-only and max-only sub-shapes were carved out for nutrients where only one bound makes sense. Each step was locally reasonable; the cumulative shape is now four different polymorphic forms across one capability. Adherence rows inherit the polymorphism via a `Target: any` field, which means every consumer special-cases.

The summary capability owns `GET /summary/daily` and `GET /summary/range`; nutrition-goals owns the `adherence` block inside both responses. When the range endpoint shipped, its adherence-building code drifted from the daily endpoint's "omit when no contribution" rule. The result: the same goals + same data produce different adherence shapes depending on which endpoint you call.

Float precision is the third paper-cut. Component-scaled recipe nutriments compound float arithmetic; the artefacts surface in the response as `70.44969999999999`. Nothing breaks; the API just looks unfinished.

The user's read on the timing: this change is "API-aesthetics, nice-to-ship" and explicitly fine to break. It does not have to interlock with `harden-write-paths`; it can land independently. The goal here is one coherent shape across goals input, goals output, adherence output, and summary numeric serialization.

## Goals / Non-Goals

**Goals:**

- One target shape across every goal field: `{min?, max?}`. No special-case for kcal.
- One adherence row shape across every nutrient and across both summary endpoints. Always present per configured goal.
- One serialization precision rule for nutrient values: 1 decimal place.
- Storage stays at full precision; rounding is presentation.
- Migration is straightforward, single-pass, no shim.

**Non-Goals:**

- Backward compatibility for `kcal_target`. Hard break.
- Per-nutrient precision rules. Uniform 1 dp.
- ETag / If-Match.
- Recipe quality indicators, list_products / delete_product, or any other adjacent finding from the MCP test report. Those are explicitly other changes.

## Decisions

### 1. Collapse `MinOnly` / `MaxOnly` / `Range` / scalar into one `Range{Min, Max *float64}`

```go
type Range struct {
    Min *float64 `json:"min,omitempty"`
    Max *float64 `json:"max,omitempty"`
}
```

`MinOnly` and `MaxOnly` are valid `Range` instances where one side is nil. The semantics fall out: a nutrient with `{min: 30}` means "at least 30, no upper bound"; `{max: 50}` means "at most 50, no lower bound"; `{min: 150, max: 190}` is a two-sided window. `Range{Min: nil, Max: nil}` is meaningless — the goal isn't set; the parent pointer is nil.

`kcal` becomes a `*Range` like every other macro field, with both `Min` and `Max` typically set (the user expresses tolerance explicitly).

This pushes complexity from the *shape* layer (multiple types) to the *behaviour* layer (status computation reads min/max nullability and produces under/on/over). The behaviour rules already exist in the current spec; they just get written once instead of three times.

**Alternatives considered:**

- *Keep MinOnly / MaxOnly as distinct types, just lift kcal into Range.* Rejected — the polymorphism leaks into adherence via `Target: any`, which is the bug we're fixing. Three types means three serialisations means three special-cases at every consumer.
- *Add a `target: number` form alongside the Range form (scalar + range polymorphism).* Rejected — exactly the asymmetry we're removing.

### 2. Drop the `±5%` magic for kcal; the user sets explicit min/max

The current behaviour expands a single `kcal_target` into `{target × 0.95, target × 1.05}` internally. We migrate the data once (backfill `kcal_min = kcal_target * 0.95`, `kcal_max = kcal_target * 1.05`) and then it's just numbers.

The 5% number was never user-tunable; some users want 3%, some want 10%. By dropping the magic constant, we make the tolerance a first-class property the user controls. The cost: the user has to type `{min: 2090, max: 2310}` instead of `{kcal_target: 2200}`. The benefit: the same kind of input as every other macro, and the tolerance is visible in the goals row instead of hidden in a service-layer constant.

The MCP `set_goals` tool description should call this out so the agent learns to construct the range when the user just says "I want 2200 kcal a day."

**Alternatives considered:**

- *Accept either `{target: X}` (shorthand) or `{min, max}` (explicit) on every goal.* Rejected — convenience that buys polymorphism back. We just removed it.
- *Make the 5% configurable via env.* Rejected — wrong scope; this is per-user, not per-deployment.

### 3. Adherence rows are always present per configured goal; new `no_data` status

```go
type AdherenceEntry struct {
    Actual   *float64 `json:"actual"`            // null when no_data
    Target   Range    `json:"target"`            // always the unified shape
    DeltaPct *float64 `json:"delta_pct,omitempty"`
    Status   string   `json:"status"`            // under | on | over | no_data
}
type Adherence map[string]AdherenceEntry         // keyed by goal field name
```

Rules:

- For every goal the user has set (i.e. the parent pointer in `Goals` is non-nil), there is exactly one entry in `adherence`. No omission.
- `Actual` is the day's effective total for that nutrient. For macros, this is always a number (a meal contributes 0 if it has no carbs; the total stays numeric). For micros, the total is `null` when every contributing meal had `null` for that micro (the column was never populated). The 0-vs-null distinction is preserved from the existing `Totals` type.
- `Status` is computed from `Actual` and `Target`:
  - `Actual == nil` → `no_data`
  - `Target.Min != nil && *Actual < *Target.Min` → `under`
  - `Target.Max != nil && *Actual > *Target.Max` → `over`
  - otherwise → `on`
- `DeltaPct` is computed when both `Actual != nil` and the target has a reference point (mid of min/max, or the single bound if only one is set). When the status is `no_data`, `DeltaPct` is absent (omitempty drops it).

This rule fires identically in `DailyFor` and in each per-day `RangeFor` entry. The "drifted between daily and range" bug disappears because there's one code path: build adherence from `(totals, goals)` once.

**Alternatives considered:**

- *Keep "omit when no contribution" but enforce it consistently in both endpoints.* Less informative for clients — a missing key requires you to remember which goals you set in order to ask "why didn't I see iron today?" Always-present with `no_data` is the friendlier shape and the same amount of code to enforce.
- *Use `actual: 0` instead of `actual: null` for the no-data case.* Rejected — conflates "logged a meal with 0 iron" (legitimately `under`) with "no data at all this day" (you can't infer adherence). Null is the honest representation.

### 4. Float rounding at serialization, 1 decimal place, uniform

A single helper in the serialisation path:

```go
func round1(f float64) float64 { return math.Round(f*10) / 10 }
```

Applied to:

- `Totals.{Kcal, ProteinG, CarbsG, FatG, FiberG, SugarG, SaltG}` and the eight pointer-valued micro fields, when building the summary response.
- `AdherenceEntry.Actual`, `.Target.Min`, `.Target.Max`, `.DeltaPct`, when building adherence rows.
- Goals row on read: `Range.Min`, `Range.Max`.
- Product / meal responses, when exposing stored nutriment values to clients.

The application point is the response-building layer — the service constructs the typed response, calls `round1` on each numeric field, and hands the rounded struct to the JSON encoder. No `MarshalJSON` magic, no global JSON encoder override. The function is trivial; the discipline is to call it everywhere a nutrient float reaches a response.

Stored values stay at full precision. Recipe nutriment computation stays at full precision (so re-rounding on every read is the only place precision is shed). The 70.44969999999999 → 70.4 transformation happens once, at the JSON boundary.

**Alternatives considered:**

- *Round at storage time.* Rejected — N rounded inputs summed produce different totals than (sum of unrounded) rounded once. The drift compounds.
- *Custom MarshalJSON on Range / Totals / AdherenceEntry.* Rejected — too clever, harder to debug, and silently affects every consumer of the type (including internal logging). Plain helper at the response boundary is honest.
- *Per-nutrient precision rules (e.g. kcal at 0 dp, mcg at 2 dp).* Rejected — saves one digit of typing, costs a per-field rule that every change has to remember. Uniform 1 dp is good enough.

### 5. Goals row response uses the canonical Range shape

After this change, `GET /goals` always returns goals in the unified shape:

```json
{
  "goals": {
    "kcal":          {"min": 2090, "max": 2310},
    "protein_g":     {"min": 150,  "max": 190},
    "fiber_g":       {"min": 30},
    "sugar_g":       {"max": 50},
    "iron_mg":       {"min": 14},
    "...": "..."
  }
}
```

`{"goals": null}` still means "no goals row yet." A configured goal with both bounds null (shouldn't happen via the API but might appear in directly-edited rows) serializes as `{}` for that field — a future spec scenario could mandate that we treat that as "not set" but for v1 we trust the data path.

**Alternatives considered:**

- *Always emit both `min` and `max` keys with explicit `null` for the unset bound.* Rejected — verbose without payoff; consumers handle missing keys naturally.

### 6. Migration is single-pass; no compat shim

```sql
-- Up
ALTER TABLE nutrition_goals
  ADD COLUMN kcal_min NUMERIC(10,3),
  ADD COLUMN kcal_max NUMERIC(10,3);

UPDATE nutrition_goals
SET kcal_min = kcal_target * 0.95,
    kcal_max = kcal_target * 1.05
WHERE kcal_target IS NOT NULL;

ALTER TABLE nutrition_goals DROP COLUMN kcal_target;
```

```sql
-- Down (best effort: recover kcal_target as the midpoint)
ALTER TABLE nutrition_goals ADD COLUMN kcal_target NUMERIC(10,3);

UPDATE nutrition_goals
SET kcal_target = (kcal_min + kcal_max) / 2
WHERE kcal_min IS NOT NULL AND kcal_max IS NOT NULL;

ALTER TABLE nutrition_goals
  DROP COLUMN kcal_min,
  DROP COLUMN kcal_max;
```

The `down` migration is lossy (it cannot recover an asymmetric user-set range), but it leaves the row in a usable state with the prior shape. Rollback is "OK but not great" — acceptable for a pre-1.0 personal app.

## Risks / Trade-offs

- **Breaking change for any external caller of `PUT /goals`.** *Mitigation:* the change ships in lockstep with the MCP wrapper update and the swag regeneration; the user's own scripts are the only known external callers. The proposal documents the migration explicitly.
- **Adherence rows always present means larger response bodies.** A user with all 15 goals set gets 15 adherence rows on every empty day instead of zero. *Mitigation:* JSON shape is tiny per row; the consumer benefits from a stable shape are worth a few hundred bytes per response.
- **`no_data` introduces a fourth status the client needs to handle.** *Mitigation:* documented prominently in the spec; clients that ignore it land in the default "neither under nor on nor over" branch, which is honest behaviour.
- **Float rounding at serialization means the JSON value differs from the stored value.** *Mitigation:* explicit and documented. Clients that need full precision can use the internal pipeline (there isn't one for v1, but the data is there if we ever need it).
- **Migration `down` is lossy.** *Mitigation:* the rollback target is "approximately what it was" via the midpoint; this is acceptable for a personal app and documented in design.md so a future operator isn't surprised.

## Migration Plan

- Backend, MCP wrapper, and swag regeneration ship in one commit so the API surface is consistent at the moment of deploy.
- `RUN_LOCAL.md` and `README.md` get updated curl examples that use the new `kcal: {min, max}` shape; the old examples are replaced (no compat note since there's no compat layer).
- Rollback: revert the commit + run the `down` migration. Any goals row that was set under the new shape with an asymmetric kcal range gets its kcal target collapsed to the midpoint. Users notified out-of-band if rollback is needed (one user).

## Open Questions

- Whether `Actual` should be rounded BEFORE the status comparison (i.e. `0.04 vs min:0.05` → on, because `actual` rounds to 0.0 which is "under"). I'm leaning toward "compute status on the unrounded value; round only on the way out" — preserves correctness against fractional thresholds. Capture as a scenario.
- Whether the goals spec should require both `min` and `max` to be present for any range (no one-sided ranges except for the documented `fiber_g` / `sugar_g` style). Current proposal is "any combination of min/max is valid"; if the user wants stricter validation per nutrient, that's a follow-up.
- Whether the eventual `unify-adherence-shape` change should subsume the rounding rule into a cross-cutting "response number precision" capability (sibling of the planned `http-error-shape` from `harden-write-paths`). For now the rule lives in `nutrition-goals/spec.md` since that's where it's first documented; we can promote it later if a third surface adopts the same rule.
