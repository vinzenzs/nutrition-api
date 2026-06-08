## Context

The repo now has six event-shaped tables — `meal_entries`, `hydration_entries`, `workouts`, `body_weight_entries`, plus the older `idempotency_records` and the singleton-flavoured `nutrition_goals` / `daily_goal_overrides`. The intake events (`meal_entries`, `hydration_entries`) and the output event (`workouts`) coexist without any structural link. The agent has been bridging them in conversation: "I had a banana before my ride at 8am" requires the agent to remember which ride, mentally compute pre/intra/post windows, and ask separate tool calls to get the totals.

This change closes that gap with the smallest sufficient shape: an FK column on the two intake tables, an aggregation endpoint that does the windowing math server-side, and the minimum tool-surface change to make it usable. It deliberately does *not* introduce `workout_fuel_entries` (in-session carbs / sodium / caffeine) — that's a separate sibling capability tracked as T1 #2 in priorities. v1 composes meals + hydration only; the new endpoint's response shape is designed so workout-fuel contributions slot in naturally when they exist.

## Goals / Non-Goals

**Goals:**

- Persist the link from a meal or hydration event to the workout it was logged against.
- Enable a one-call answer to "how did I fuel workout X?" — pre/intra/post bucketed totals.
- Make the link discoverable on every meal / hydration response so the agent doesn't need to remember it across turns.
- Keep the unit-isolation rule intact: the fueling response separates nutrition Totals (kcal/g) from hydration totals (ml) per window.
- Backward-compatible: every existing call site that omits `workout_id` continues to work unchanged.

**Non-Goals:**

- `workout_fuel_entries` capability and tools.
- EA computation, weekly energy summaries, training-load math.
- Snapshot of workout metadata onto meal_entries (the link gracefully degrades via `ON DELETE SET NULL`).
- Filter parameters on `GET /meals?workout_id=` (defer until requested).
- Auto-classification / agent-side inference of which workout an untagged meal belongs to.
- Including products' nutriment-snapshot logic in this change — meal_entries already snapshot product nutriments; that path is unchanged.

## Decisions

### 1. Nullable FK column on both intake tables; `ON DELETE SET NULL`

```sql
ALTER TABLE meal_entries
    ADD COLUMN workout_id UUID NULL REFERENCES workouts(id) ON DELETE SET NULL;

ALTER TABLE hydration_entries
    ADD COLUMN workout_id UUID NULL REFERENCES workouts(id) ON DELETE SET NULL;

CREATE INDEX meal_entries_workout_id_idx      ON meal_entries (workout_id)      WHERE workout_id IS NOT NULL;
CREATE INDEX hydration_entries_workout_id_idx ON hydration_entries (workout_id) WHERE workout_id IS NOT NULL;
```

`ON DELETE SET NULL` matches the existing `meal_entries.product_id` semantics: intake events outlive their references. If you delete a workout, the meals that were tagged with it lose the link but stay in the log (and still have their own `logged_at`, nutriments, etc.).

Partial indexes (`WHERE workout_id IS NOT NULL`) keep the index size minimal — most rows will have NULL here in v1, especially historical entries.

**Alternatives considered:**

- *`ON DELETE CASCADE`.* Rejected — deleting a workout shouldn't delete the food the user ate. Intake events are independent records.
- *`ON DELETE RESTRICT`.* Rejected — would prevent deleting workouts that have any tagged intake. Annoying for the common case of "I logged the wrong workout, delete it and re-import from Garmin."
- *Snapshot the workout's (started_at, ended_at, sport) onto the intake row so the link survives deletion.* Rejected — workouts are events, not definitions; there's no nutriment-style "this thing might be edited and break my historical totals" concern. Garmin re-syncs are already silent updates per the workouts proposal; adding snapshot semantics here is asymmetric.

### 2. PATCH supports three states via the `""` sentinel for clear

Go's JSON decoder collapses missing fields and `null` into the same `*string == nil`, so the standard pattern can't natively distinguish "don't touch" from "explicitly clear." Two practical paths:

- **Wrapper type with explicit "is-set" flag.** More code; non-idiomatic in this codebase.
- **Empty-string sentinel for clear.** Document it: `{"workout_id": "<uuid>"}` sets, `{"workout_id": ""}` clears, omitting the field doesn't touch. Matches the way several real-world APIs handle this; minimal change to existing patch handler shape.

Going with the sentinel. The spec documents it explicitly; tool descriptions mention it; tests cover all three states.

**Alternatives considered:**

- *Dedicated endpoint `DELETE /meals/{id}/workout-link`.* Rejected as verbose for a tri-state on one field.
- *Defer clear support to a follow-up; PATCH only sets in v1.* Considered. Rejected — "I tagged the wrong workout" is a real and common need; making the user delete-and-recreate the meal is poor ergonomics.

### 3. Validation: 400 `workout_not_found` when workout_id doesn't exist

Same pattern as `product_not_found` on `POST /meals`: validate the FK before insert and return a structured 400. The DB constraint catches it as a fallback (5xx) but the explicit pre-check gives the better error code.

Handler order on POST: parse body → validate basic fields → if `workout_id` set, fetch workout (existence-only query) → on miss, return 400 with `{"error":"workout_not_found"}`. PATCH does the same when `workout_id` is non-empty.

### 4. `/workouts/{id}/fueling` URL, not under `/summary/*`

Existing `/summary/*` endpoints are date-anchored aggregations: `daily`, `range`, `hydration/daily`. Workout fueling is workout-anchored — anchored on a row, not a date. Different URL family communicates the different concept.

```
GET /workouts/{id}/fueling?pre_window_min=240&post_window_min=60
```

The choice has a small downstream cost (the new handler lives in the workouts package, not summary, so the aggregation code is duplicated from `summary.sumEntries` or imported across packages). Either of those is fine — implementation can decide.

**Alternatives considered:**

- *`GET /summary/workout-fueling/{workout_id}`.* Considered for consistency with the summary URL family. Rejected — it reads less naturally than the resource-nested form, and the summary family's "date-anchored" identity is worth keeping clean.

### 5. Response shape preserves the unit-isolation rule

The hydration capability was explicitly designed to *not* mix ml into the nutrition Totals struct (mixing g and ml in one Totals is a footgun). The fueling response respects the same boundary by giving each window two separately-typed sub-objects:

```json
{
  "workout_id": "...",
  "started_at": "2026-06-07T08:00:00Z",
  "ended_at": "2026-06-07T09:30:00Z",
  "pre_window": {
    "start": "2026-06-07T04:00:00Z",
    "end":   "2026-06-07T08:00:00Z",
    "minutes": 240,
    "nutrition": {
      "totals":      { "kcal": 420, "carbs_g": 75, "protein_g": 12, "fat_g": 8, ... },
      "entry_count": 1
    },
    "hydration": {
      "total_ml":    750,
      "entry_count": 2
    }
  },
  "intra_window": { ... },
  "post_window":  { ... }
}
```

Each window: half-open interval `[start, end)`. A meal logged at exactly `workout.started_at` lands in `intra_window` (intra wins the boundary; this is the more useful default for "did I time my last gel"). Documented.

The nutrition Totals shape exactly matches `/summary/daily.totals` (macros + nullable micros) so the agent can compare without remembering two schemas. Re-using `summary.Totals` directly is one option; defining a parallel type in the workouts package is another. Either preserves the contract.

**Alternatives considered:**

- *One Totals struct per window with both kcal and ml mixed.* Rejected — explicit violation of the hydration-vs-summary unit isolation we established. Cost of asymmetry forever.
- *Tag-based aggregation (only entries with this `workout_id`)*. Considered. Rejected as the *only* aggregation logic — would miss entries the user forgot to tag. But the tag is still useful: see Decision 6.
- *Union of tag-matched + time-window-matched, with per-entry `match_reason`*. Considered. Rejected for v1 as too complex; time-window is the more common case and simpler to reason about.

### 6. Time-window matching is the primary aggregation; tag is metadata

The fueling response includes any meal/hydration whose `logged_at` falls in the relevant window — regardless of whether it has `workout_id` set. The `workout_id` tag is metadata for grouping/listing (the agent can use it on `GET /meals?…` once a filter param exists), not the aggregation key.

Why this matters: if the user logged a banana at 7:55am without tagging it, and their workout was at 8:00am, the banana is correctly in `pre_window` totals. If they retroactively tag it later, behaviour is unchanged — the tag is for the *list* paths, not the *fueling-summary* path.

Consequence: a meal tagged with workout X but eaten 8h before workout X (e.g. "ride day breakfast") does NOT appear in the pre-window unless the user widens `pre_window_min`. That's correct — `pre_window` means "shortly before" in the fueling sense, not "intent-tagged with this workout."

If the agent wants both views, two calls: `/workouts/{id}/fueling` for the time-window aggregation, `GET /meals?workout_id=X` for the intent-tag list. Future change can add the filter; this proposal doesn't.

### 7. Default window lengths: 240 min pre, 60 min post

- **Pre = 240 min (4h)** — covers the realistic "pre-workout meal" window for most athletes. Short enough to exclude the previous day; long enough to catch a normal breakfast before a 9am ride.
- **Post = 60 min (1h)** — covers the "post-workout recovery meal" window. Glycogen-replenishment science suggests 30-120 min; 60 is a defensible middle.
- **Both bounded [0, 720]** — 12 hours is the practical upper bound for "pre/post workout" semantics. Past that, you're talking about something else (a recovery day, a fasting period).
- A value of `0` is allowed — useful when the agent wants ONLY the intra-window totals.

Out-of-range → `400 window_invalid` with the documented bounds.

### 8. Tool surface: extend five, add one

The existing tools that touch meal_entries / hydration_entries gain an optional `workout_id` field — five tools, each picking up one new pointer-or-empty-string field.

The new `workout_fueling_summary` tool takes:
- `workout_id` (required string)
- `pre_window_min` (optional `*int`)
- `post_window_min` (optional `*int`)

Read-only, no idempotency-key. The tool description explicitly notes:
- Time-window primary (not tag-based) — explained so the agent doesn't expect tag-matching semantics.
- Default windows (240 pre, 60 post) and bounds.
- Nutrition vs hydration are separate sub-objects (so the agent doesn't try to read kcal from `hydration` or ml from `nutrition`).
- Future-compat note: when `workout_fuel_entries` ships, this tool's response will gain contributions from those entries automatically; no contract change.

## Risks / Trade-offs

- **Empty-string sentinel for "clear" is unusual.** Some clients may not handle it gracefully (e.g. a Flutter client might serialize an absent field as `""` by mistake and accidentally unlink). Mitigation: tool descriptions and spec scenarios call it out explicitly; tests cover both states.
- **Time-window aggregation misses tag-only-tagged entries outside the window.** A user who logs "breakfast at 4am" tagged with a 9am ride will not see breakfast in `pre_window` unless they pass `pre_window_min=300+`. Mitigation: defaults are documented; user can widen the window; `GET /meals?workout_id=` (future filter) covers the tag-only view.
- **Validation cost on every meal POST/PATCH that supplies workout_id.** One extra DB lookup per write. Single-user single-request workloads are unaffected; if write rate ever climbs, batch validation or a join-based insert pattern is a follow-up.
- **`/workouts/{id}/fueling` cost scales with N entries in the window.** A 12h window with hundreds of meals (unlikely but possible) is O(N) in aggregation. Mitigation: indexed `logged_at` on both tables already; window cap of 720 min limits worst case.
- **Cross-package coupling for the aggregation.** The workouts handler needs to read from meals + hydration. Either it imports both repos (one-way coupling) or the aggregation logic lives in a neutral package (e.g. a new `internal/fueling/` package). Implementation-time decision; either is acceptable.
- **Snapshot-of-workout omitted means historical-fueling questions degrade when workouts are deleted.** "Show me how I fueled my Feb 14 race" returns nothing if the workout row was deleted, even though the meals still exist. Mitigation: don't delete race-day workouts; if this becomes a real problem, add a `snapshot_workout_label` column later.

## Migration Plan

- Forward: add the column + index on each of the two tables. No backfill needed; NULL is the correct value for existing rows.
- Rollback: drop the column + index on each table.
- The migration is numbered `014_add_workout_link_to_intake` (next after `013_add_body_weight`).

## Open Questions

- Whether to expose a `match_reason` per entry in a future per-entry-detail extension of the fueling endpoint (`?expand=entries`). Tentative answer: defer until the agent shows it needs the per-entry view; today the summary totals are the load-bearing read.
- Whether `pre_window_min` and `post_window_min` should be in the workout row itself (so different sports / different workouts get different defaults). Tentative answer: no — the agent picks the window per query based on context; making it a stored property of the workout adds a config surface for marginal benefit.
- Whether the `intra_window` should be exposed as a separate concept from "during the workout" — e.g. distinguishing "the warmup 20min" from "the main set." Tentative answer: no — that's lap-level granularity which we explicitly excluded from workouts.
