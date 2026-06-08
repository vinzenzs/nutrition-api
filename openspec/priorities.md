# Project priorities — endurance-training nutrition

This is the working triage of what's worth building next. **Not a roadmap**
in the sense of "we commit to X by Y" — single-user project, no commitments.
It's a staging area for ideas worth keeping while we wait for real-use data
to confirm which ones matter most.

Last updated: **2026-06-08** (post-archive of `add-carb-load-auto-apply` — T1 #3 closed, so T1 #1, #2, #3, #4 + T2 #6 are all delivered. The only unresolved T1 items are #5 templates, #1A training-phase, #1B rolling-window — and #1B now has a live proposal in `add-rolling-window-summaries`).

---

## What this file is

- A snapshot of what we think is most valuable to ship next, organized by tier.
- A counterweight to "feature backlog" sprawl — items are explicitly bucketed,
  and items get *promoted/demoted* as real use surfaces what matters.
- A capture of which items cluster around shared dependencies (the leverage
  moves), so we don't ship a leaf when the trunk would unblock more.

## What this file isn't

- Not a commitment to build any item.
- Not a sequencing plan (see "Sequencing notes" at the bottom — and
  acknowledge it's a Meta gap we haven't resolved yet).
- Not a substitute for `openspec/changes/` — every actual build still goes
  through a proposal/design/specs/tasks change.

## How to read

- **Tier 1** — high-impact, blocking real use for endurance-training-as-a-system.
  Ship soon, in some order.
- **Tier 2** — meaningful, secondary. Ship after Tier 1 is mostly done.
- **Tier 3** — nice-to-have. Revisit annually.
- **Meta** — framing / process gaps. Not features.
- **Out of scope (correctly)** — explicitly not building. Some carry
  "revisit by <date>" if circumstances may change.

Items annotated with **[added 2026-06-07]** are additions or reclassifications
from the explore-mode session on that date. Everything unannotated came from
the offline analysis the user did before that session.

---

## Snapshot — in-flight as of 2026-06-08 (end-of-day)

```
APPLYING / APPLIED (in flight):
  (none — clean queue post-archive cycle)

PROPOSED (full artifacts, not yet applied):
  add-meal-from-photo            (backend Claude Vision integration;
                                   currently #1 in continuity.md "Up next")
  add-rolling-window-summaries   (delivers T1 #1B — multi-day averages for
                                   metrics that are actually multi-day phenomena
                                   — protein/MPS, EA trend, sodium baseline,
                                   carb-load window) [added 2026-06-08]
  add-flutter-companion-app      (predates the endurance pivot; may need
                                   revisit, see Meta #3)

RECENTLY ARCHIVED (all 2026-06-08):
  add-carb-load-auto-apply       (delivers T1 #3 — `plan_carb_load(apply: true)`
                                   atomically writes per-date overrides;
                                   pure-compute default unchanged)
  add-energy-availability        (delivers T1 #4 — Loucks EA over a window;
                                   pure composition over meals + workouts + weight)
  add-workout-fuel               (delivers T1 #2 — sibling capability to hydration
                                   carrying carbs/sodium/potassium/caffeine/optional ml)
  add-meal-workout-link          (delivers T1 #1 — workout_id on intake +
                                   /workouts/{id}/fueling pre/intra/post summary)
  add-weight-log                 (delivers T2 #6 — body-weight log + 7d rolling trend)
  add-date-varying-goals         (per-date goal overrides for training vs rest)
  add-hydration-tracking         (ml-only intake log)
  add-workouts-capability        (the leverage trunk — earlier on 2026-06-08)
  add-last-logged-quantity       (earlier on 2026-06-08)

EARLIER ARCHIVED:
  add-race-prep-primitives       (2026-06-07)
  unify-adherence-shape, harden-write-paths, product management, … (earlier)
```

---

## The leverage insight (cashed in — 2026-06-08)

**Six tier-1 / tier-2 / tier-3 gaps all shared one dependency: the API had
no concept of a workout.** Shipping `add-workouts-capability` unblocked the
whole cluster cheaply.

```
T1 #1  workout_ref on logs           ┐  ✅ delivered (add-meal-workout-link)
T1 #2  in-session electrolytes/fuel  │  ✅ delivered (add-workout-fuel)
T1 #4  EA computation                │  ✅ delivered (add-energy-availability)
T2 #10 recommend_workout_fuel        │  ⏳ still on the list
T3 #5  GI/RPE on workout fueling     │  ⏳ still on the list
T3 #6  sweat rate test workflow      ┘  ⏳ still on the list (much cheaper now)
```

Three of the six leaves shipped on 2026-06-08. The remaining three are
T2 / T3 — re-evaluate priority when there's a real "I want this" moment;
keep on the list otherwise.

---

## Tier 1 — high-impact, blocking real use for endurance training

### 1. Workouts are invisible to the nutrition log

There's no `workout_id` or session reference on a meal or hydration entry.
You can't ask "what did I eat in the 4h before that brick" or "what was my
intra-ride carb rate." For a triathlete this is the primary nutrition
question. Fix shape: add optional `workout_ref` to `log_meal`,
`log_meal_freeform`, `log_hydration`; add a `workout_fueling_summary`
tool returning carbs/fluid/sodium across pre-, intra-, post-windows.

**Status:** **Delivered** by `add-meal-workout-link` (archived 2026-06-08). The
`/workouts/{id}/fueling` endpoint with `nutrition` + `hydration` + (later)
`workout_fuel` sub-objects now answers the pre/intra/post fueling question
natively.

### 2. Hydration is volume-only — no electrolytes / carbs / caffeine

`log_hydration` records ml and a free-text note. It cannot tell you sodium
intake during a 90-min Z2 ride. Sodium targets during endurance work are
300–800 mg/hr — currently invisible. Fix shape: a separate
`workout_fuel_entries` capability (sister to hydration; deliberately NOT
extending the hydration table because mixing g and mg in one Totals struct
is exactly the footgun we avoided when shipping hydration). Carries
`(workout_ref?, ml?, carbs_g?, sodium_mg?, potassium_mg?, caffeine_mg?, note)`.

**Status:** **Delivered** by `add-workout-fuel` (archived 2026-06-08). Sibling
table to `hydration_entries`; carries `quantity_ml? / carbs_g? / sodium_mg? /
potassium_mg? / caffeine_mg?` with the explicit-zero vs unmeasured distinction
preserved across the wire. Composes into the third sub-object of
`/workouts/{id}/fueling`.

### 3. `plan_carb_load` is computational only — no auto-apply

Returns the schedule but doesn't write the overrides. Two-step manual workflow.
Fix shape: a `plan_and_apply_carb_load` tool (separate from `plan_carb_load`
to keep the side effect explicit) that writes the overrides and returns what
it set.

**Status:** **Delivered** by `add-carb-load-auto-apply` (archived 2026-06-08).
`POST /race-prep/carb-load/apply` (same params as the GET) computes the
schedule and writes the per-day carb min-bound into the per-date goal
overrides in one atomic transaction. Merges into existing overrides
(preserving non-carb fields like training-day kcal/protein templates) via
the new `OverridesRepo.UpsertPatch` repo primitive. MCP `plan_carb_load`
gained an optional `apply: true` flag that routes to the POST endpoint;
default-false stays pure-compute, so existing read-only callers are
unaffected.

### 4. No Energy Availability (EA) computation or flagging

The two most critical numbers for an athlete in a weight-loss block — energy
availability and deficit aggressiveness — are inferable from logs + Garmin
burn but not surfaced. Fix shape: `weekly_energy_summary(date_range,
body_weight_kg, lean_mass_kg?, kcal_burned_per_day)` returning avg EA with
Loucks bands (`<30 kcal/kg FFM/day = low`, `30–45 = sub-optimal`, `>45 = ok`).

**Status:** **Delivered** by `add-energy-availability` (archived 2026-06-08).
`GET /energy/availability?from=&to=&tz=&lean_mass_kg?&body_fat_pct?` returns
per-day EA + window aggregate with the Loucks band classification. FFM
resolution is loud about its source (explicit > stored > 85% fallback); days
with workouts missing `kcal_burned` are listed in `missing_burn_workout_ids`
and excluded from `window.avg_ea`. Pure composition — no schema.

### 5. No training-day vs rest-day goal templates (only per-date overrides)

You can override every day individually. There's no concept of "this is a
training day, apply my training-day template." For a 16-week plan that's
112 individual `set_daily_goal_override` calls vs setting 3-4 templates once.
Fix shape: `goal_templates (training_hard / training_easy / rest / race_week)`
+ `apply_template_to_dates` or `apply_template_to_plan_phase`.

**Status:** Revisit after first real use of date-varying-goals. The non-goal
in `add-date-varying-goals` was deliberate. **See also Tier 1 addition
"Training-phase context"** — that's the broader reframe that templates
naturally fall out of.

### 1A. Training-phase / block context **[added 2026-06-07]**

"I'm in build block 2 of 4, weeks 3–6 of a 16-week plan." Affects daily
goals, adherence interpretation, recovery-window targets. Today every day
is identical to the system; the agent carries the context implicitly.

This is the broader frame that Tier 1 #5 ("templates") sits inside.
Templates are *one* way to express phase ("on training-hard days, use this
goal set"). A `training_phases` table (`{name, start_date, end_date, type,
notes}`) is the data shape; templates become "default goals during a phase."

**Status:** Discuss before building either #5 or #1A — they're two answers
to the same question. Probably one change covers both with the phase
concept as the trunk.

### 1B. Rolling-window summaries **[added 2026-06-07]**

`daily_summary` returns one day. Most nutrition-science recommendations are
3-day or 7-day averages: protein for MPS, EA over a week, carb-load over
72h. Today every such question requires `range_summary` + client-side math.

Fix shape: either extend `daily_summary` with a `rolling_avg: 7d` param, or
add `rolling_summary(window_days, anchor_date)` returning averages over the
trailing N days. T2 #6 sketches this for weight; it should generalize.

**Status:** Proposed as `add-rolling-window-summaries` (2026-06-08, in
Backlog). Shape decided as `GET /summary/rolling?anchor_date=…&window_days=N`
— a dedicated endpoint over an extension to `daily_summary`, so the
half-open vs inclusive-window semantics don't have to compete with the
existing per-day shape. Now that T1 #1, #2, #3, #4 + T2 #6 have all shipped,
this is the cheapest remaining T1 add — pure composition over primitives
that already exist.

---

## Tier 2 — meaningful, secondary

### 6. No body weight / composition log

Coach reads from Garmin. If Garmin isn't synced or you weigh elsewhere, no
record. Also no goal-weight + projected-date calc. Fix shape:
`log_weight(kg, body_fat_pct?, logged_at)` + `weight_trend(from, to)` with
rolling 7-day average to suppress daily noise.

**Status:** **Delivered** by `add-weight-log` (archived 2026-06-08). `POST
/weight` + `GET /weight/trend` (rolling-average with `sample_count` per
point). Body-fat % is supported on entries — used by `add-energy-availability`
for the FFM resolver. Goal-weight projections deferred.

### 7. Protein distribution per meal

Daily total of 180g protein doesn't matter if it's 20g/20g/140g — research
shows ~0.3 g/kg every 3–5h (20–40g per meal × 4–5 meals) maximizes MPS.
`daily_summary` shows daily total but not per-meal distribution. Fix shape:
`daily_summary` could include `protein_distribution: [{meal_type, protein_g,
hour}, …]` or a dedicated `protein_distribution(date)` tool.

### 8. No caffeine tracking

Caffeine pre-race (3–6 mg/kg, 60 min before) is a known performance tool.
Coffee logs currently capture none of this. Fix shape: add `caffeine_mg` to
product nutriments + hydration entries (or to the `workout_fuel_entries`
capability if T1 #2 lands first — caffeine pre-race is *workout fuel*).

### 9. No supplement log distinct from food

B12, D3, iron, omega-3 — daily for a vegetarian endurance athlete.
Currently you'd log each as a freeform meal, which clutters the food log
and inflates entry count. Fix shape: `log_supplement(name, dose_mg,
logged_at)` separate from `log_meal`.

### 10. No workout-fueling template / calculator

There's `plan_carb_load` for races but nothing for everyday sessions.
"Tomorrow's 90-min Z2 ride → recommended pre-ride carbs / intra-ride
carbs/hr / post-ride window protein" should be one tool call. Fix shape:
`recommend_workout_fuel(workout_type, duration_min, intensity_zone,
body_weight_kg)` returning a structured pre/intra/post plan.

**Status:** Becomes much cheaper after `add-workouts-capability` — can take
a `workout_id` and read sport/duration/TSS from the row.

### 6A. Sleep / HRV log **[added 2026-06-07]**

Garmin records sleep duration, sleep stages, HRV. Affects glycogen-window
timing, protein needs on hard training days, tolerance for deficits. Sister
to T2 #6 weight log in cost and shape: one nullable per day, capture tool,
read tool, optionally a rolling-trend tool. Could be one combined "morning
metrics" capability (sleep + HRV + weight all logged ~7am) rather than three
separate ones — worth thinking about when ready to build.

### 6B. `daily_context(date)` aggregator **[added 2026-06-07]**

Single MCP call returning today's bundle: adherence, hydration total,
workouts, weight, sleep, training phase. Otherwise the agent makes 5–7 tool
calls to start every conversation. Pure read-side composition over existing
primitives — no new schema. Tests the "one tool, many sources" pattern.

### 6C. Sweat rate test workflow **[promoted from T3, 2026-06-07]**

For an endurance athlete in heat/cold, default sweat rates miss by 30–50%.
Personalized intake is the difference between bonking and finishing.
Shape after workouts lands: a sweat-rate-test session is a workout +
pre/post weight entries + fluid-in record → derived `ml/hr` sweat loss.
Almost no new code if T2 #6 (weight) and `add-workouts-capability` are
both done.

### 6D. GI distress / RPE on workout fueling entries **[promoted from T3, 2026-06-07]**

THE primary data captured during training — race-fueling rehearsal data
("I tried Maurten 320 at race pace and bonked at 90 min"). Tier 3
understates it; this is what training fueling is *for*. Fix shape: add
nullable `gi_distress_score` (1–5) and `rpe` (1–10) to workout fuel entries
(or to workouts themselves, depending on granularity needed).

### 6E. Retroactive meal correction (freeform → product) **[promoted from T3, 2026-06-07]**

When you log freeform and later realize "oh that was a Snickers, I should
use the product," there's currently no swap. Affects long-term data quality:
six months of "Skyr bowl" logged as freeform vs as a recipe = different
analytics over the same data. Fix shape: `PATCH /meals/{id}` accepts
`product_id` to convert a freeform entry to a product-backed one
(preserving timestamp + quantity, swapping the nutriment source).

### 6F. `coach_recommendation` persistence **[added 2026-06-07, lower confidence]**

The agent computes "today's target is 220g carbs because of tomorrow's long
ride" — that reasoning is ephemeral. Next session it reconstructs. Either
the agent persists rationale as notes on the override, or a tiny
`coach_logs(date, recommendation, reason, scope)` primitive captures it.

**Status:** Tests the project's "agent does synthesis, API records
primitives" principle. Recommendations *are* synthesis. Worth deliberate
discussion before building — the right answer might be "no, keep it
agent-side, accept the loss."

---

## Tier 3 — nice-to-have

| Gap                                                                                            | What it unlocks                                                       |
|------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------|
| Food quality bands (NOVA % kcal, ultra-processed share) — already in products but not adherence | Surfaces processed-food creep that masks deficit gains                |
| Vegetarian/vegan compliance flag in summaries                                                  | Quick visibility, especially when traveling/eating out                |
| Recipe quantity at log time (`log_meal(recipe_id, servings=1.5)`)                              | Today you must call `log_meal` with grams — recipe semantics lost     |
| Streak / trend (consecutive days under protein, rolling 7-day avg)                             | Behavioral nudge surface                                              |
| Data-quality / sparse-window signaling **[added 2026-06-07]**                                  | Tools that need 7+ days return junk silently on sparse windows; flag it |

---

## Meta — framing / process gaps **[all added 2026-06-07]**

These aren't features. They're project-level gaps that have accumulated as
the use case clarified. Each is cheap to fix; cumulatively they keep the
project intelligible.

1. **README / project tagline still says "personal nutrition logging."**
   Doesn't reflect that ~80% of recent gap analysis is endurance-training-
   specific. Cheap update; sets expectations for future contributors
   (including future-you in 2027).

2. **Active changes have documented sequencing now** (see "Sequencing notes"
   below) but no formal dependency graph. With the Tier-1 mechanical cluster
   delivered, dependency order matters less today than it did during the
   workouts/hydration/fuel ramp-up. Revisit if active-change count climbs
   back above ~5.

3. **The Flutter app proposal predates the endurance-training pivot.**
   Its three killer interactions (barcode, photo, hydration widget) don't
   address any tier-1 or tier-2 endurance gap. The phone might want a
   different shape now — e.g., a single "log workout fuel mid-ride"
   button rather than barcode scanning. Worth a discussion before
   `/opsx:apply add-flutter-companion-app`.

4. **No "live status" map.** Five active changes + an archive + this
   priorities file. As of today it fits in your head; at 10+ it won't.
   Either keep this file aggressively current, or add `openspec list`
   output snapshots to a sibling.

---

## Out of scope (currently)

| Item                          | Reason                                                          | Revisit |
|-------------------------------|-----------------------------------------------------------------|---------|
| CGM ingestion                 | Significant integration; agent can't do much with it today      | 2027    |
| Multi-user / sharing          | Single-user project; OAuth + tenancy is a different system      | —       |
| Web / mobile UI               | Flutter app proposal covers mobile; web is unnecessary today    | —       |
| Meal planner                  | Composer over existing primitives; agent-side today             | When workouts + recovery + EA primitives all exist |
| Menstrual cycle / female-athlete physiology | Single-user male; relevant only if generalizing | —       |
| Webhooks / public agent surface | Premature for single-user                                     | —       |
| Audit log / change history    | Nice but doesn't unblock anything                               | —       |
| Soft delete / undo            | Minor footgun; easy workaround                                  | —       |

---

## Sequencing notes (working draft)

Not committed; just current best-guess order.

```
Now:
  └ Queue is clean. add-meal-from-photo is #1 in continuity.md.
    Tier-1 mechanical items (#1, #2, #3, #4) all shipped — what's left in
    T1 is the templates/phase decision (#5 vs #1A) and rolling-window
    (#1B, now proposed as add-rolling-window-summaries).

Next 2-3 changes (any order):
  ├ add-meal-from-photo (independent backend feature; Flutter killer #2)
  ├ add-rolling-window-summaries (T1 #1B; pure composition over existing
  │   primitives — the cheapest remaining T1 add)
  └ (open slot — propose the next leverage move when a real use surfaces it)

Decide before building:
  ├ T1 #5 vs T1 #1A (templates vs phases — same question, two answers)
  ├ add-flutter-companion-app (does it still earn its priority? See Meta #3)
  └ T2 #6F coach_recommendation persistence (tests synthesis principle)

Wait for usage data:
  └ T1 #5 templates (revisit after first multi-week training block planned)
```

---

## How to update this file

- When real use surfaces that a Tier 3 item matters more than thought →
  promote it, mark **[promoted to TX, YYYY-MM-DD]**.
- When a tier-1 item ships → leave in the file with a "delivered by
  `<change-name>`, YYYY-MM-DD" note, so future-you sees the history.
- When a new gap surfaces → add it; mark **[added YYYY-MM-DD]**.
- When an out-of-scope item's revisit date arrives → make the call: stays
  out, or moves in with a new tier.
- Update "Last updated:" at the top whenever you touch the file.
- If this file grows beyond ~400 lines, that's a sign it should split
  (sequencing.md, gaps.md, scope.md) — but don't pre-split.
