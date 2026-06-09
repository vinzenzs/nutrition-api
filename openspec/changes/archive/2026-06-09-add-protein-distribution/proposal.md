## Why

For an athlete in a deficit, the *daily protein total* is necessary-but-not-sufficient. Muscle protein synthesis (MPS) is triggered per-meal, not per-day: the well-validated number is **~0.3 g of protein per kg body weight per meal**, every 3–5 hours, across 4–5 meals. A user hitting their 180 g/day target as 20 g + 20 g + 140 g (the classic "I forgot to eat breakfast, big dinner") spent the day below the MPS threshold for two out of three meals, then crossed the protein-leucine ceiling on the third. The same daily total split as 45 g × 4 meals lands every meal in the MPS-effective band and triggers MPS four times.

Today the API answers "how much protein did I eat today?" (`daily_summary.totals.protein_g`) but it does NOT answer "did each meal cross the MPS threshold?" The agent has to call `daily_summary?meal_type=breakfast`, `?meal_type=lunch`, `?meal_type=dinner`, `?meal_type=snack` separately and do the per-meal-weight math client-side. That's 4 round-trips per day to answer a question that's structurally one bucketing pass over the existing meals.

Closes T2 #7 in `openspec/priorities.md`. Tier-1 is exhausted as of 2026-06-09; T2 #7 is the smallest of the remaining tier-2 items by both surface area and behavioural impact (every meal in a multi-day window benefits — not just race weeks).

The shape is a single new MCP tool + REST endpoint that returns one row per meal-type (or per individual meal — design decides), each annotated with `protein_g`, an MPS-threshold flag relative to the user's body weight, and the timing context (`logged_at_hour`, gap-since-previous-meal). No new schema. No migration. No write paths.

## What Changes

- **New `GET /summary/protein-distribution` endpoint**: query params `date` (YYYY-MM-DD), `tz` (IANA, defaults to `DEFAULT_USER_TZ`), `body_weight_kg` (optional, overrides resolution from the body-weight log). Returns one row per logged meal on the date.
- **Row shape**:
  ```json
  {
    "date": "2026-06-09",
    "tz": "Europe/Berlin",
    "body_weight_kg":     72.5,
    "body_weight_source": "rolling_7d_avg" | "explicit" | "last_before_date",
    "mps_threshold_g":    21.75,       // 0.3 × body_weight_kg, rounded to 0.1
    "total_protein_g":    142.0,
    "meal_count":         4,
    "mps_effective_meal_count": 3,     // meals at or above mps_threshold_g
    "meals": [
      {
        "logged_at":      "2026-06-09T07:30:00Z",
        "logged_at_hour": 9,            // local hour 0..23 in tz
        "meal_type":      "breakfast",  // or null when meal_type wasn't set
        "protein_g":      28.0,
        "mps_effective":  true,
        "gap_minutes_since_previous": null   // first meal → null
      },
      {
        "logged_at":      "2026-06-09T11:00:00Z",
        "logged_at_hour": 13,
        "meal_type":      "lunch",
        "protein_g":      18.0,
        "mps_effective":  false,
        "gap_minutes_since_previous": 210
      },
      ...
    ]
  }
  ```
- **New MCP tool `protein_distribution`** wrapping the endpoint. Tool description leads with the MPS rule (~0.3 g/kg/meal) and the gap-since-previous heuristic (>5h or <3h are both signals: meals too far apart miss MPS triggers, meals too close together aren't independent triggers).
- **Per-meal grouping vs per-meal-entry**: rows are one-per-`meal_entries` row, not one-per-meal-type. A user who logs two snacks 4 hours apart gets two separate rows. Multi-product meals (e.g. an entry chain typed as `breakfast` × 3 rows logged within the same hour) stay as separate rows — collapsing them is the agent's call if needed.

## Capabilities

### Modified Capabilities

- `meals`: Adds the protein-distribution requirement alongside the existing daily / range / rolling requirements.
- `mcp-server`: Adds one new tool requirement for `protein_distribution`.

## Impact

- **Prerequisites (all already shipped)**: `add-weight-log` (body weight + rolling-7d trend for the auto-resolution path); the existing meals + summary pipeline.
- **No schema migration**. Pure aggregation over `meal_entries` + `body_weight_entries`.
- **New code**:
  - `internal/summary/protein.go`: new types (`ProteinDistribution`, `ProteinMeal`) + service method `ProteinDistributionFor(ctx, ProteinDistributionParams) (*ProteinDistribution, error)`. Sits next to `DailyFor` / `RangeFor` / `RollingFor`.
  - `internal/summary/handlers.go`: fourth route `GET /summary/protein-distribution` next to the existing three.
  - `internal/mcpserver/tools_summary.go`: add `ProteinDistributionArgs` + `handleProteinDistribution` + register the tool.
- **Body weight resolution**: mirrors the four-tier rule already implemented in `internal/energy/composition.go`. Re-use the resolver if it can be lifted; otherwise duplicate the small helper (the energy package owns the canonical resolver; if a shared helper falls out cleanly, hoist it). Body-weight resolution path is reported in `body_weight_source`.
- **Tests**:
  - Per-meal happy path: 4 meals, body weight 72.5 kg → mps_threshold_g = 21.75 → assert per-meal flags.
  - Body-weight resolution paths (explicit / rolling 7d / last-before-date / `weight_data_missing` 400).
  - Calendar-day boundaries in non-UTC TZ (consistent with daily/rolling/EA).
  - `gap_minutes_since_previous` for the first meal is null; second meal gets correct gap; same-second meals get 0.
  - Adherence-style empty-day shape: empty day returns `meal_count: 0`, `meals: []`, body-weight composition still resolved if data exists.
  - MCP recorder tests for `protein_distribution`.
- **Documentation**: `task swag`; README "Summaries" subsection gains the protein-distribution example; MCP tools table gains `protein_distribution`. RUN_LOCAL.md gets a one-liner.
- **No idempotency middleware impact** — read-only.

### Out of scope (explicit non-goals)

- **Range-over-many-days protein distribution.** Same surface logic, but range_summary already lives next door; the agent composes if it wants a weekly view.
- **Carb / fat / fibre per-meal breakdowns.** Protein is uniquely tied to MPS — the per-meal carb signal is intra-workout-window-specific and is owned by the workouts fueling summary. Adding a generic "per-meal macros" surface invites the agent to over-tune.
- **MPS threshold customization (e.g. an athlete on a hard cutting block who wants a higher 0.4 g/kg/meal target).** Hard-code 0.3 g/kg/meal in v1 — it's the validated literature value. If real use shows the friction, an optional `mps_threshold_g_per_kg` parameter is a one-liner follow-up that doesn't change the response shape.
- **Leucine-specific tracking** (`leucine_mg` per meal). Most products in the cache don't carry leucine, and the MPS-trigger story is well-modeled by protein-g until products carry the data. Re-evaluate once Open Food Facts surfaces leucine for >50% of products.
- **Streak / consecutive-day "every meal hit the threshold" metrics.** Behavioural-nudge surface; separate proposal.
- **Per-meal-type aggregation** ("average breakfast protein this week"). Out of scope; `range_summary?group_by=meal_type` already serves that question.
- **`recommend_protein_for_next_meal` predictive surface.** Composition over this endpoint + the goals layer; agent owns the synthesis.
- **Auto-detecting "meals" from clusters of `meal_entries`.** The endpoint trusts the meal-entry rows as logged. If a user logs three entries within 10 minutes typed as `breakfast`, they appear as three rows; the agent can collapse on output if needed.
- **MPS gap warnings on the API surface** ("you went 8 hours between protein doses"). The response carries `gap_minutes_since_previous` honestly; "warn" semantics are agent-side framing.
