## Context

`nutrition_goals` today is a singleton: one row, applied to every day's adherence computation. That made sense when goals were treated as identity-level preferences (the user's daily target was one thing). It breaks the moment training and rest days demand different targets, which is the user's stated reality.

The design fork is whether to model the day-to-day variance as:

1. **Overrides** — a date-keyed table of full goal sets. Simplest model. Per training day, write down what that day's goals are.
2. **Templates + assignment** — named reusable goal sets ("training") + a date-to-template mapping. Cleaner once you have ≥3 day types; one extra concept.
3. **Rules** — auto-classify day type from training data (Garmin TSS, etc.). Free with integration; without it, undefined.

The user has two day types today (training / rest). Two day types don't justify templates' extra concept, and rule-based classification requires Garmin work that's its own change. So: **overrides only**. If the user later finds themselves typing the same training-day goal set every week, templates earn their complexity.

The override resolution sits inside `internal/goals/`. Summary's `DailyFor` / `RangeFor` change one call site each. Everything else stays put.

## Goals / Non-Goals

**Goals:**

- Persist a per-date goal set independent of the default singleton.
- Adherence in daily and range summaries reflects today's effective goals — override-first, default-fallback, none-when-neither.
- The agent and the user can tell which path produced today's adherence (a `goal_source` field on the response).
- Range queries don't pay N-DB-calls; one batched lookup for the whole window.
- Hard-break on legacy `kcal_target` field (consistent with `unify-adherence-shape`).
- Idempotency-Key rejected on the PUT (consistent with `harden-write-paths`).

**Non-Goals:**

- Templates / named goal variants.
- Auto-classification from training data.
- Partial / merge overrides. PUT is full-replace.
- Recurring overrides (bulk-apply across dates).
- Override priority vs default ordering rules — the only rule is "override beats default."
- Validation that an override is "reasonable" relative to the default.

## Decisions

### 1. New `daily_goal_overrides` table, date as primary key

```sql
CREATE TABLE daily_goal_overrides (
    date DATE PRIMARY KEY,

    kcal_min NUMERIC(10, 3),
    kcal_max NUMERIC(10, 3),
    protein_g_min NUMERIC(10, 3),
    protein_g_max NUMERIC(10, 3),
    -- ... every nutrient bound, mirroring nutrition_goals after unify-adherence-shape

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Date as PK means at most one override per calendar date — natural for the use case ("Thursday's training-day override"). No singleton constraint, no row-level lock pattern, no need for a separate `id` UUID.

The column set is identical to `nutrition_goals` post-`unify-adherence-shape` (all 15 nutrients × min/max = 30 columns). This is deliberate: the same `*goals.Goals` Go type serialises both into the same JSON shape, so existing client code and the existing `roundGoals` rounding helper work unchanged.

**Alternatives considered:**

- *Add a `date NULL` column to `nutrition_goals`, treat singleton as `date IS NULL`.* Rejected — overloads one table with two concepts; the singleton constraint becomes "exactly one row WHERE date IS NULL," which is awkward.
- *Store overrides as JSONB on a per-date row.* Rejected — loses the typed columns; the adherence layer would need to decode JSON per day. Pure cost.

### 2. `goals.EffectiveFor(ctx, date)` + `EffectiveForRange(ctx, from, to)`

The resolver lives in `internal/goals/effective.go`:

```go
// EffectiveFor returns the override for `date` if one exists, else the
// default singleton, else nil. The same Goals type is used either way.
func (r *Repo) EffectiveFor(ctx context.Context, date time.Time) (*Goals, GoalSource, error)

// EffectiveForRange returns a map keyed on local date string (YYYY-MM-DD)
// covering every date in [from, to]. Days without an override map to the
// default singleton; the default is fetched once and shared.
func (r *Repo) EffectiveForRange(ctx context.Context, from, to time.Time) (map[string]*Goals, map[string]GoalSource, error)
```

`GoalSource` is one of `default | override | none`. `none` fires when the override is absent AND the singleton is unset (the agent gets a clean "no goals configured" signal rather than empty adherence).

For range queries, one `SELECT … WHERE date BETWEEN from AND to` returns every override in one round-trip. The default is fetched once. Total DB cost: 2 queries regardless of range size. (The existing `RangeFor` already pre-fetches goals once for the un-grouped case; we keep that shape, just extend it to the override map.)

**Alternatives considered:**

- *Per-day calls inside the range loop.* Rejected — O(N) queries for a 92-day range. The batched lookup is trivial in SQL and the response set is tiny.
- *Cache the default at service construction time and only re-fetch overrides per request.* Rejected — the default can change while the service runs (the user updates it via `PUT /goals`); a per-request fetch keeps semantics correct.

### 3. `Daily` and `RangeDay` gain `GoalSource` field

```go
type Daily struct {
    Date      string    `json:"date"`
    TZ        string    `json:"tz"`
    MealType  *string   `json:"meal_type,omitempty"`
    Totals    Totals    `json:"totals"`
    Entries   []*meals.MealEntry `json:"entries"`
    Adherence Adherence `json:"adherence,omitempty"`

    // NEW: which goal set produced the adherence rows.
    // One of "default" | "override" | "none". Always present on responses
    // where adherence is computed; absent when meal_type filter is active
    // (adherence omitted entirely in that case).
    GoalSource string    `json:"goal_source,omitempty"`
}

type RangeDay struct {
    Date        string             `json:"date"`
    Totals      *Totals            `json:"totals,omitempty"`
    ByMealType  map[string]Totals  `json:"by_meal_type,omitempty"`
    Adherence   Adherence          `json:"adherence,omitempty"`

    // NEW: same semantics as Daily.GoalSource.
    GoalSource  string             `json:"goal_source,omitempty"`
}
```

Why surface this rather than hide it? Per the user's "agent does synthesis" principle: the agent reading "today, you used your training-day goals" produces meaningfully different commentary than reading "today's adherence was perfect" with no context. The field is one string; the cost is trivial.

`omitempty` keeps the field absent when the meal_type-filtered daily summary suppresses adherence (no point reporting a goal source when there are no goals being applied).

**Alternatives considered:**

- *Embed the GoalSource inside the Adherence map.* Rejected — adherence is a `map[nutrient]entry`; goal source is per-day, not per-nutrient.
- *Skip the field entirely; agent infers from "the response goals don't match my default."* Rejected — agents shouldn't need to remember the default to interpret today's adherence.

### 4. Endpoint surface mirrors the singleton pattern

```
PUT    /goals/overrides/{date}     upsert (full-replace, no Idempotency-Key)
GET    /goals/overrides/{date}     404 when none exists
DELETE /goals/overrides/{date}
GET    /goals/overrides?from=&to=  map of {date → goals} in range
```

`{date}` parses as `YYYY-MM-DD`. Invalid format → `400 date_invalid`. Range query requires both `from` and `to`, `from <= to`, max 366-day span (we permit a full year here since the use case is "show me what's planned for the next month" rather than the meal-summary 92-day cap).

PUT shares the validation surface with `PUT /goals`: same `Range` shape, same legacy-kcal_target rejection via `DisallowUnknownFields`, same min/max sanity, same empty-`{}` rejection. The handler delegates to the existing `validateGoals` function — no duplication.

The DELETE returns 204 on success, 404 on unknown date (overrides are date-keyed and deletion is naturally idempotent).

**Alternatives considered:**

- *Have `PUT /goals?date=…` overload the default endpoint.* Rejected — couples two lifecycles; default goals are set once and rarely change, overrides are set frequently per training cycle. Separate URLs keep the mental model clean.

### 5. Backward compatibility

- `PUT /goals`, `GET /goals` unchanged. The default singleton is still THE singleton.
- `set_goals`, `get_goals` MCP tools unchanged.
- `GET /summary/daily`, `GET /summary/range` unchanged in shape (the new `goal_source` field is additive with `omitempty`).
- A pre-change consumer that never sets an override sees zero behaviour change — `EffectiveFor` falls back to the singleton, adherence computes exactly as today.

The only behavioural change for existing clients: when an override exists for the queried date, adherence reflects the override. This is the entire point; documented in the proposal's What Changes.

### 6. MCP tool surface

```
set_daily_goal_override    PUT    /goals/overrides/{date}    no Idempotency-Key
                                                              (matches set_goals)
get_daily_goal_override    GET    /goals/overrides/{date}
delete_daily_goal_override DELETE /goals/overrides/{date}    auto-derive key
list_daily_goal_overrides  GET    /goals/overrides?from=&to=
```

`set_daily_goal_override` follows the same pattern as `set_goals` post-`harden-write-paths`: the schema does NOT expose `idempotency_key`, the wrapper never sends one, and the backend would reject the header anyway.

Tool descriptions:

- `set_daily_goal_override`: "Override the default daily goals for a specific date — full-replace semantics, the override completely replaces the default for that date. Use for training days, rest days, race weeks. Date format: YYYY-MM-DD."
- `list_daily_goal_overrides`: "Enumerate dates that have an explicit override in the [from, to] range. Useful for checking 'what's set for this week' before deciding whether to add or change overrides."

## Risks / Trade-offs

- **Overrides require typing the full goal set per day.** For training/rest cycling this means writing the same training-day numbers repeatedly. *Mitigation:* the agent can store the values once in its own context and replay the PUT — and if this becomes painful, the proposal explicitly forward-points templates as a follow-up.
- **Range queries with 92 days of mixed overrides return up to 92 adherence variants.** Slightly larger responses. *Mitigation:* the batched lookup is one query; the response growth is one `goal_source` string per day. Trivial.
- **The user can confuse "no override" with "no goals" if the default is also unset.** *Mitigation:* `goal_source: "none"` makes the distinction explicit. Adherence is omitted in this case as before.
- **Override date is timezone-naive (`DATE` not `TIMESTAMPTZ`).** A user travelling across timezones might be surprised "what counts as today" diverges from their phone. *Mitigation:* the summary endpoint accepts a `tz` parameter that picks the date; the override is keyed on whatever date the user pinned it to. Single-user pragmatism.
- **The default-singleton fallback means deleting the override silently restores default behaviour.** A user who didn't realise an override was set might think their goals changed. *Mitigation:* `goal_source` in summary responses makes the override visible; `list_daily_goal_overrides` lets them audit.

## Migration Plan

- Forward migration creates `daily_goal_overrides`. No backfill.
- Rollback drops the table. The default `nutrition_goals` row is untouched. Any data the user stored in overrides is lost; this is acceptable for a pre-1.0 personal app.

## Open Questions

- Whether `list_daily_goal_overrides` should also return the default goals alongside the override list (so the agent has one round-trip to see "here's the default, here are the deviations"). For v1, it returns just the overrides; the agent calls `get_goals` separately if it needs the default. Cheap to bundle later if asked.
- Whether the `set_daily_goal_override` MCP tool should accept an `Etag` placeholder for future optimistic-concurrency support, even though we don't enforce it today. Tentative answer: no — empty hooks rot. Wait until the broader ETag work surfaces.
- Whether a "clear all overrides in a window" bulk endpoint earns its weight (the agent currently does N deletes). Tentative answer: no — N is small in practice; YAGNI.
