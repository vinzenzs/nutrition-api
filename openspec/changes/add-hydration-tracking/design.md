## Context

The existing meal-logging surface has a clear shape: a `meal_entries` table for the event, a `products` table for reusable definitions, a `meals` capability that owns CRUD + summary endpoints, an MCP tool per agent-relevant endpoint. Hydration could either be (A) bolted onto that shape — extending `meal_entries` with `quantity_ml`, special-casing summary math — or (B) introduced as a parallel structure: its own table, its own endpoints, its own MCP tools.

Path A saves a table at the cost of confusing semantics: every meal-related query now has to remember to ignore the ml-shaped rows, and every nutrient computation has to skip them. Path B costs ~150 lines of mechanical code but keeps both domains comprehensible.

The user's framing is firm: hydration is its own thing, the agent reasons about context-sensitive targets, the API just records volume + time. So path B with the smallest possible surface.

## Goals / Non-Goals

**Goals:**

- Capture: log an arbitrary volume of fluid at a moment in time, with an optional free-text note.
- Retrieve: list within a window, fetch a daily total, edit and delete entries with the same hygiene as meals.
- Agent-accessible: each REST endpoint has a matching MCP tool.
- Unit-isolated: `quantity_ml` lives in its own table and its own response shape; never mixed with grams.

**Non-Goals:**

- Beverage taxonomy (water / coffee / electrolytes). Use the note field.
- Workout-window tagging. The agent classifies from training data; we just timestamp.
- Stored hydration goals. The right target depends on day, temperature, and training volume — context the agent already has, encoding a single static number hides the variance.
- Integration with the nutrition daily summary. Mixed-unit Totals would be a footgun.
- Caffeine / alcohol / sugar tracking via beverages. If a drink has nutriments, log it as a meal (a Coke is a product) — hydration captures volume separately.

## Decisions

### 1. Separate `hydration_entries` table, parallel to `meal_entries`

```
hydration_entries
─────────────────
id              UUID PRIMARY KEY
logged_at       TIMESTAMPTZ NOT NULL
quantity_ml     NUMERIC(10, 1) NOT NULL CHECK (quantity_ml > 0)
note            TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

INDEX hydration_entries_logged_at_idx ON (logged_at)
```

No FKs. No product link (hydration entries are not nutriment-bearing in this model). `quantity_ml` uses one decimal place — enough precision for 0.1 ml, more than any human needs.

**Alternatives considered:**

- *Extend `meal_entries` with `quantity_ml` and a `kind` discriminator.* Rejected — every existing query and the existing snapshot-materialisation logic would have to learn about the new shape. Cost compounds across capabilities; benefit is one fewer table.
- *Store hydration as a product (e.g. "water 0 kcal/100g") and log via the existing `/meals` path with quantity_g treated as ml.* Rejected — silently using grams to mean ml is the worst possible semantic compromise.

### 2. Five endpoints mirror the meals shape

```
POST   /hydration                          create (idempotent via header)
GET    /hydration?from=…&to=…              list in [from, to)
PATCH  /hydration/{id}                     partial update
DELETE /hydration/{id}                     remove
GET    /summary/hydration/daily?date=…&tz=…  total + entries for one day
```

Validation rules borrow from meals:

- `quantity_ml > 0` → `400 quantity_ml_invalid`
- `logged_at` not more than 24 h in the future → `400 logged_at_too_far_future`
- `note` length capped at 500 chars → `400 note_too_long`
- Window queries require both `from` and `to`, `from < to`, RFC 3339, max 92-day span — same as `/meals`.

The summary endpoint uses the same TZ resolution as the existing summaries: explicit `tz` query param wins, else the configured `DEFAULT_USER_TZ`, with a warning log when fallback fires.

**Alternatives considered:**

- *Skip PATCH/DELETE for v1.* Considered for tool-surface minimalism, but rejected for consistency: `meals` has both, the agent expects symmetric CRUD, the cost is two endpoints + two tools.
- *Pagination on `GET /hydration`.* Skipped — at single-user volumes, a day or week of entries comfortably fits one response. Add limit/offset if it ever becomes a real concern.

### 3. Daily summary returns total + entries, no rounding to integer

```json
{
  "date": "2026-06-07",
  "tz": "Europe/Berlin",
  "total_ml": 2475.5,
  "entry_count": 8,
  "entries": [
    {"id": "...", "logged_at": "...", "quantity_ml": 500, "note": "morning water"},
    ...
  ]
}
```

`total_ml` is rounded to one decimal at serialization (using the existing `numfmt.Round1` helper from `unify-adherence-shape`) for consistency with the nutrient-rounding rule. `entry_count` is a tiny convenience — agents asking "did I log anything today" don't need to count the array length client-side.

**Alternatives considered:**

- *Round to integer ml.* Probably what most callers want, but 1 dp matches the existing convention and costs nothing. Easy to change later.
- *Include hourly buckets / time-distributed totals.* Useful for "did I hydrate during the long ride" but pure analytics on top of the entries array — agent territory.

### 4. New capability: `hydration`

The capability is genuinely independent of `meals` — different schema, different units, different aggregation. Bundling it inside `meals` would force the meals spec to grow a "this requirement doesn't apply if it's a hydration entry" caveat on every rule. Cleaner to ship as its own capability.

Spec lives at `openspec/specs/hydration/spec.md` after the change is archived.

**Alternatives considered:**

- *Extend `meals` capability with hydration sub-requirements.* Rejected per above.
- *Group with `nutrition-goals` as "daily intake".* Goals are per-nutrient targets; hydration is logged events. Different shape entirely.

### 5. MCP tools follow the existing patterns

```
log_hydration               POST   /hydration              auto-derive idem key
list_hydration              GET    /hydration              read-only, no key
patch_hydration             PATCH  /hydration/{id}         auto-derive idem key
delete_hydration            DELETE /hydration/{id}         auto-derive idem key
daily_hydration_summary     GET    /summary/hydration/daily read-only, no key
```

Tool descriptions:

- `log_hydration`: "Record a volume of fluid the user drank at a specific time. The note field carries beverage context (e.g. 'water', 'iced coffee', 'electrolytes')."
- `list_hydration`: emphasise the half-open window + 92-day cap.
- `daily_hydration_summary`: emphasise it's separate from `daily_summary` (which is nutrient-only).
- `patch_hydration` / `delete_hydration`: standard CRUD.

The integration test (`mcp_integration_test.go`) gains five entries in its expected-tools list.

## Risks / Trade-offs

- **Parallel table doubles the meal-vs-hydration mental model.** Some users will reasonably expect a single "intake log" view. *Mitigation:* if real demand surfaces, a future capability can compose the two via a union view. Today, the agent does the synthesis.
- **No stored goal means no automatic adherence row.** Daily hydration summary returns total + entries; the agent must compare against context. *Mitigation:* consistent with the explicit design choice and the user's stated principle ("data capture + computation primitives, let me do the synthesis").
- **`note` as free text is parser-unfriendly.** Future analytics ("how many coffees did I have this week?") would need NLP. *Mitigation:* by design — the agent does this from conversation context already. If we hit a wall, add an enum then.
- **The `/summary/hydration/daily` URL nests under `/summary/`.** Slightly unusual; the existing `/summary/daily` and `/summary/range` don't have nested suffixes. *Mitigation:* the nested form keeps domain separation explicit (vs `/hydration/summary/daily` which mixes domain and verb). Documented in design.
- **Five new MCP tools push the surface to 17.** Each one is small, but the tools/list response grows. *Mitigation:* this is the natural surface for the capability; tool-count anxiety isn't a real constraint here.

## Migration Plan

- Forward migration adds one table + one index. No backfill.
- Rollback drops the table. No data outside the new table changes.

## Open Questions

- Whether the summary should expose a "first sip" / "last sip" timestamp pair as a cheap proxy for "how long did the user go without drinking?" Out of scope for v1; trivial to add later.
- Whether `note` should have a tiny suggested vocabulary baked into the MCP tool description (e.g. "common values: water, coffee, tea, electrolytes, alcohol") to nudge the agent toward consistency. Marginal; deferred.
- Whether to model a `hydration_target_ml` on `nutrition_goals` despite the design's "no stored goal" stance. If real-world usage shows agents struggle to pick a target consistently, this becomes one nullable column. Not now.
