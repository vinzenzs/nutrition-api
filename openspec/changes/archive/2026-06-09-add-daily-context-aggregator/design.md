## Context

The codebase has accumulated a lot of primitives over the last two days: meals, hydration, workouts, workout-fuel, body-weight, energy-availability, training-phases. Each primitive ships with its own MCP tool, which is the right shape for *editing* — the agent reaches for the specific tool when it has a specific intent. But for *reading the morning state*, the agent makes the same 5–7 calls every session:

```
list_workouts(today)
daily_summary(today)
daily_hydration_summary(today)
list_workout_fuel(today)
list_weights(today)       # or weight_trend if no recent
get_daily_goal_override(today)
list_phases(today, today) # since training-phases shipped
```

Latency-wise the bundle is dominated by Postgres round-trips and the MCP transport (stdio). With 7 sequential calls that's ~3-5s before the user's actual question gets framed. Conceptually it's also a poor frame: the agent is meant to be the synthesis layer, but right now its first 7 turns are mechanical I/O.

This proposal collapses the 7 reads into 1 endpoint + 1 tool. No new schema, no new write path. Tests the "one tool, many sources" pattern explicitly noted in priorities.md T2 #6B as "worth doing because it informs whether `weekly_context` or similar broader aggregators belong as v2."

## Goals / Non-Goals

**Goals:**

- One tool call returns everything the agent needs to start a session: adherence, totals, hydration ml, today's workouts, today's fueling, latest weight, training-phase context, presence of a goal override.
- Latency floor = slowest single read, not sum. Parallel fetch via `errgroup.WithContext`.
- Re-use existing response shapes verbatim where they exist (the `adherence` map, the `totals` block, the `goal_source`+`phase_name` pair). The agent already knows how to read those.
- Empty-day case is well-shaped: empty arrays, null nullable blocks, no error.
- v1 ships small enough that v2 (sleep block, etc.) is purely additive.

**Non-Goals:**

- **Full meal entries in the bundle.** A heavy log day can easily hit 30 entries × multiple components — would blow up the bundle.
- **Weekly / range aggregator.** Different shape; revisit after the daily one sees real use.
- **Recommendations / advisory text.** This is data. Synthesis stays agent-side.
- **Template bounds inline.** Phase carries `default_template_name`; agent calls `get_goal_template` if it needs the actual goal bounds.
- **Etag / 304 / cache layer.** Reads are cheap and the call is per-session, not per-second.
- **A POST variant.** Reads, always.

## Decisions

### 1. New `/context/daily` endpoint at top level, not under `/summary/`

```
GET /context/daily?date=YYYY-MM-DD&tz=…
```

Considered nesting as `/summary/context` for shape-affinity. Rejected — the existing `/summary/*` endpoints all share a shape (per-day totals + adherence with optional groupings); `/context/daily` deliberately covers more ground (workouts, weights, phases, etc.). Putting it under `/summary/` would mis-set the agent's expectations.

The path is `/context/daily` so an obvious future `/context/weekly` doesn't have to fight for a name.

### 2. Parallel fetch via `errgroup.WithContext`

```go
g, gctx := errgroup.WithContext(ctx)
g.Go(func() error { … fetch adherence })
g.Go(func() error { … fetch totals })
g.Go(func() error { … fetch hydration ml })
g.Go(func() error { … fetch workouts })
g.Go(func() error { … fetch workout-fuel })
g.Go(func() error { … fetch weight })
g.Go(func() error { … fetch phase covering today })
g.Go(func() error { … fetch goal override for today })
if err := g.Wait(); err != nil { return nil, err }
// assemble bundle
```

At single-user scale all 8 reads against the same `*pgxpool.Pool` go to a pool with `pool.MaxConns >= 8` (the default is `4 * runtime.GOMAXPROCS(0)`), so they really do run in parallel. The wall-clock cost is the slowest read, which is currently the goals resolver doing `EffectiveFor(date)` (override → phase → template → default chain — 3-4 round trips in the worst case).

If any individual read errors, the errgroup cancels the rest and returns the error. There's no partial-bundle response — that would force the agent to handle a half-shape, and the operations are cheap enough that retry is fine.

**Alternative considered:** sequential reads with the dependency-ordering implicit in goal_source needing the phase. Rejected — the resolver hides that dependency, the service layer just needs the resolver's result.

### 3. Re-use the summary service's `DailyFor` for adherence + totals

The adherence object + totals + `goal_source` + `phase_name` are already produced by `summary.Service.DailyFor(date)`. The aggregator service depends on `*summary.Service` and calls `DailyFor` for that slice, then keeps only the fields the bundle exposes (drops the full entries array). Reuse > re-deriving.

The bundle's `nutrition.entries_count` comes from `len(daily.Entries)`. The bundle drops `daily.Entries` from the wire — the size cost would compound on heavy log days.

### 4. Hydration bundle uses a tiny totals call, not the daily-hydration handler's shape

`/summary/hydration/daily` returns `{total_ml, date, tz, entries_count}`. The aggregator could re-use it; cleaner to call `hydration.Service` directly and pull just `(total_ml, entries_count)` into the bundle's `HydrationBlock`. Avoids embedding a sub-response shape that has its own date/tz fields the bundle already carries at the top level.

### 5. Weight block falls back to "carryover from before today" when no fresh entry

Single-user daily weight log isn't religious — entries land every 1–3 days. The agent always wants *some* signal. Logic:

1. Find a `body_weight_entries` row whose `logged_at` is on the day's UTC window → return it, `is_carryover: false`.
2. Else call `bodyweight.Repo.LatestBefore(ctx, dayStart)` → return that row, `is_carryover: true`.
3. Else `WeightBlock` is `nil` (no weight ever logged).

The `is_carryover` flag is the discriminator the agent uses to decide whether to ask "did you weigh in this morning?"

**Alternative:** always return only fresh, leave gaps to the agent. Rejected — pushes a decision (look-back logic) onto the agent that the API can do once and correctly.

### 6. Phase block tracks "the resolver-picked phase for this date"

When multiple phases overlap, the resolver picks most-recently-updated. The bundle reports the same phase the *adherence* computation sees, so the agent doesn't read "your phase is recovery" but adhere as if in race_week (which is what the resolver picked). Implementation: call `phases.PhaseFor(date)` (same method the resolver uses).

If no phase covers the date → `phase: null`. The fact that `goal_source` and `phase_name` already convey the same information at the top level is intentional redundancy — `phase: {…}` carries the *full* phase row (id, type, dates, notes), useful for the agent to say "you're 3 days into a 5-day race week."

### 7. Goal override block is `{present: bool, goals: …|null}`

The override either exists or it doesn't. Two-field shape (`present` + `goals`) is more agent-readable than an alternating `null` vs object. Reading the bundle, the agent's branch is `if context.goal_override.present { … }`, not `if context.goal_override != null { … }`.

Trivially cheap (one `goals.OverridesRepo.GetOverride` call).

### 8. Adherence echoes the existing daily summary's shape

`AdherenceBlock` carries:
```
{
  goal_source: "override" | "phase_template" | "default" | "none",
  phase_name: "<string>"?,         // matches the summary's omitempty rule
  adherence: { kcal: {actual, target, delta_pct?, status}, ... }  // verbatim from summary
}
```

The whole point of "one tool, many sources" is the agent's mental model is already trained on these shapes. Don't reinvent them.

### 9. `tz` defaults to `DEFAULT_USER_TZ` (matches every other endpoint)

Same convention as `/summary/daily`. No surprise.

### 10. MCP tool wraps a single GET; no idempotency key

Read-only. The wrapper does NOT expose an `idempotency_key` field. Tool description names the trade explicitly: "Use this as the first call of a session to load the bundle. For deep dives into one slice, use the dedicated tools (`daily_summary`, `list_workouts`, etc.) — they include the per-entry detail this aggregator deliberately omits."

## Risks / Trade-offs

- **Wide constructor.** The aggregator's `Service` has 9-10 dependencies. That's a lot, but each is a real consumer; collapsing to fewer types would mean a service-of-services indirection that obscures the shape. Tolerate the long argument list; it's load-bearing.
- **Schema drift risk.** When any of the consumed primitives' response shapes change, the bundle must update. Mitigation: the aggregator test should construct a fully-populated bundle and assert every nested field; a breaking change to any consumed type breaks the test loud.
- **Two paths to the same data.** Calling `daily_context` and `daily_summary` independently returns overlapping data; if the agent does both per session there's redundant work. Mitigation: the tool description guides agents to choose. Some redundancy is acceptable while the pattern bakes in.
- **Failure aggregation.** Any one slice failing fails the whole bundle. Could be argued as wrong — "I want to know my hydration even if workouts errored." Rejected because: at single-user scale, sub-second pool-of-many query failures are nearly always transient (cold pool, DB blip) and the agent's right move is retry, not partial-state recovery. If real use surfaces a sticky-broken slice (e.g. a corrupt workouts row that bombs the query), revisit.
- **Bundle size growth.** Each new primitive that joins (`sleep`, `daily_caffeine`, …) bloats the response. Mitigation: when v2 lands, add optional `include` query params to suppress slices the agent doesn't need (`?include=adherence,workouts`). v1 ships the full bundle every time.
- **Carryover weight semantics.** Returning a 3-day-old weight as "carryover" can mislead the agent into adherence calculations based on stale FFM. Mitigation: `is_carryover: true` is the flag; the energy-availability endpoint still does its own freshness check. The bundle is informational, not load-bearing for math.

## Migration Plan

No schema. Forward = deploy the new endpoint + tool + docs. Rollback = remove them (purely additive).

## Open Questions

- **`include=…` param for v1?** Tentative: no. Agents always want the full bundle today. Adding the knob too early lets agents fragment; if the bundle gets unwieldy (~v3) we can hold-back fields then.
- **Should the response carry the configured timezone offset for the day (DST-aware)?** Tentative: no — `tz` echo is enough; the dates are already calculated against it.
- **Should `phase` include the template's actual bounds (so the agent reads them without a second tool call)?** Tentative: no, per design decision #6's reasoning. Revisit if usage shows the second call is reflexive.
- **Top-level `date` in the response — should it echo the request's date or the resolved local-day date in case `tz` changes the day?** Tentative: echo the request's date for simplicity; the bundle is from the perspective of "your day in `tz`."
