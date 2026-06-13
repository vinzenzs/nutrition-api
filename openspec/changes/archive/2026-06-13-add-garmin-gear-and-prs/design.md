## Context

The recovery/fitness/hydration-balance snapshots the bridge already syncs are **date-keyed**: one row per day, upserted by `(date)` (or `(date, identity)`). Gear and personal records are different in kind — they are **slowly-changing inventory**: a shoe accrues mileage across months, a PR is set once and then occasionally beaten. They are identified by a stable Garmin id, not a date, and a daily sync re-observes the *same* objects with updated totals rather than appending a new day.

This is the most tangential-to-nutrition slice of the arc; it adds two read-only mirror capabilities whose only consumer is the chat coaching agent (gear-retirement nudges, PR-freshness framing). No fueling math reads them.

Constraints that shape the design:
- **One package per capability** — `gear` and `personal-records` are two new packages, not child tables of an existing capability.
- **Repo against `store.Querier`**, sentinel errors → API codes, `numfmt.Round1` at the response boundary (gear distance, PR value).
- **Unit isolation** — gear distance (m) and PR value never leak into `summary`'s Totals struct.
- **Append-only sequential migrations**; arc order B=036, A=037, C=038 ⇒ this is `039` (one migration, two tables). Verify head on disk before scaffolding.
- **garmin-bridge tolerates per-capability failure** — a failing `get_gear`/`get_personal_records` must degrade to "no inventory refresh this sync", never an aborted day.

## Goals / Non-Goals

**Goals:**
- Mirror Garmin gear inventory (shoe/bike mileage, retirement, activity count) and personal records, each upserted by its stable Garmin id.
- Surface both as read + upsert REST and as MCP list tools for the coaching agent.
- Keep re-sync idempotent: re-observing the same gear/PR updates the row in place (no duplicates).

**Non-Goals:**
- No PATCH/DELETE on either capability — Garmin owns the truth; the backend is a mirror. (Manual edits would be clobbered by the next sync and add no coaching value.)
- No reconciliation of gear/PRs *removed* on Garmin's side — see D3 (accepted limitation).
- No fueling-math coupling: gear distance and PR value are coaching context only, never inputs to carb/hydration/energy computation.
- No per-day history of gear mileage or PR progression — only the current state is mirrored (a follow-up could keystone history if the agent ever wants trend lines).

## Decisions

### D1 — Inventory refreshes on each daily sync via idempotent upsert (chosen) vs. a separate `POST /sync/inventory` step

**Chosen: fold gear + PR refresh into the existing daily `POST /sync`.** Each sync, after the date-keyed work, the bridge fetches `get_gear` + `get_gear_stats` + `get_personal_records` (guarded), maps them, and POSTs each item to `POST /gear` / `POST /personal-records`, which upsert by external id. This is cheap (gear/PR endpoints return a handful of rows for a single user), idempotent (re-observing the same object updates in place), and matches the existing per-capability failure tolerance — a stale-but-present inventory is fine, and the next sync corrects it.

*Alternative considered — a separate `POST /sync/inventory` bridge step (and matching schedule).* Rejected for now: it adds a second cron entry and a second code path for no benefit at single-user scale. The daily refresh is so cheap that gating it behind a separate cadence is premature. If the gear/PR fetches ever become rate-limited or slow, splitting them into a weekly `POST /sync/inventory` is a clean future move — the REST upsert endpoints are cadence-agnostic, so only the bridge's scheduling changes, not the backend contract.

### D2 — Upsert by external id, one capability per package, no date keying

`gear` keys on the Garmin gear `uuid`; `personal_records` keys on the Garmin PR `id`. Each table carries a UNIQUE `external_id` (or equivalently-named) column; `POST` does `INSERT … ON CONFLICT (external_id) DO UPDATE`. This mirrors the workouts `external_id` UPSERT pattern (`garmin:<id>`) rather than the date-keyed snapshot pattern. The two capabilities are separate packages (CLAUDE.md "one package per capability") even though they share a sync trigger, because their shapes, validation, and error codes are unrelated.

### D3 — Stale / retired handling: best-effort flag, no delete-reconcile (accepted limitation)

An upsert-only mirror **cannot** observe deletions: a shoe the user removes from Garmin, or a PR Garmin recomputes away, leaves a stale row in the backend that no sync will clear. Two mitigations:
- **Retirement is mirrored, not inferred:** `gear.retired` comes straight from Garmin's gear record, so a retired-but-still-present shoe is correctly flagged (the common case — users retire gear far more often than they delete it).
- **Deletion is an accepted limitation, documented as a future reconcile.** If it ever matters, a future change can add a reconcile pass: the bridge sends the full current set of external ids and the backend marks-or-deletes any gear/PR not in that set (the same shape as the existing workout-reconciliation change). This change does **not** build that — the chosen daily-upsert refresh is the documented simple path, and a phantom retired shoe is harmless coaching context.

### D4 — Read + upsert only; `numfmt.Round1` on distance/value floats

REST surface is `GET /gear`, `GET /gear/{id}`, `POST /gear` (upsert) and `GET /personal-records`, `POST /personal-records` (upsert). No PATCH/DELETE (D1/non-goals). `gear.total_distance_m` and `personal_records.value` are stored at full precision and rounded with `numfmt.Round1` only at serialization, consistent with every other nutrient/measurement float. Personal-records `pr_type` carries an implied unit; `value` is a bare numeric and the meaning (seconds for a 5k time, metres for longest-ride) is conveyed by an accompanying `unit` note string so the agent can render it without a hard-coded lookup.

### D5 — MCP mirrors REST 1:1; two new read tools

`gear_list` → `GET /gear`; `personal_records_list` → `GET /personal-records`. Each builds one HTTP request via `apiClient` and forwards the body verbatim (`toToolResult`), per the MCP-mirrors-REST rule. No write tools (`POST /gear` etc. are bridge-only — the agent never upserts gear). The `mcp_integration_test` expected-tools list grows by exactly two.

### D6 — Bridge fetch guarded per endpoint, mapped defensively

`fetch_day` (or a sibling inventory-fetch block invoked from the same sync) gains `get_gear`, `get_gear_stats`, and `get_personal_records`, each wrapped in the existing `safe()` pattern — a failing or account-unavailable endpoint yields a missing key, not an aborted day. `get_gear` lists the gear objects; `get_gear_stats` carries per-gear mileage/activity totals; the mapper joins them by gear uuid and emits one `/gear` item per shoe/bike. Absent fields are omitted (the backend treats absent as distinct from zero). PRs map straight from `get_personal_records`.

## Risks / Trade-offs

- **Phantom rows for deleted Garmin gear/PRs** → accepted (D3); harmless coaching context, future reconcile available if it ever matters.
- **`get_gear_stats` join coupling** → if Garmin's gear-stats shape drifts, the mapper degrades to gear-without-mileage (distance omitted), not a failed sync — same defensive-extraction posture as the rest of `mapping.py`.
- **Two new packages for a tangential feature** → justified by the one-package-per-capability rule and unrelated shapes; the surface is small (read + upsert), so the cost is mostly boilerplate, not complexity.
- **Daily refresh re-posts unchanged inventory** → negligible at single-user scale; the upsert is a no-op write when nothing changed.

## Migration Plan

1. **Confirm migration head on disk** (expect `038` after siblings B/A/C land; if siblings are not yet on disk, verify the actual highest number) before `task migrate:new NAME=add_gear_and_personal_records` — out-of-band work has occasionally taken a slot. Arc order fixes this change at `039`.
2. `039_add_gear_and_personal_records.up.sql`: `CREATE TABLE gear` and `CREATE TABLE personal_records`, each with a UNIQUE external-id column and CHECKs mirroring existing conventions (non-negative distance/value/counts).
3. `.down.sql`: drop `personal_records`, then `gear`.
4. Rollback is clean — two additive tables, no data transform.

## Open Questions

- **PR `value` unit representation:** store a free-text `unit` note string alongside the numeric `value` (chosen above), or normalise every PR to SI and let the client format? Leaning `unit` note (sport-agnostic, no hard-coded PR-type table). **Resolved in spec: `value` numeric + `unit` note string.**
- **Gear `gear_type` enum breadth:** lock to `{shoes, bike, other}` (Garmin's `gearTypeName` collapses cleanly into these); anything unmapped → `other`, mirroring the bridge's sport-enum fallback. **Resolved in spec: three-value enum with `other` fallback.**
- **Whether `get_gear` alone carries mileage** (making `get_gear_stats` redundant): kept as two guarded fetches because Garmin splits inventory from stats; if `get_gear` turns out to carry totals, the stats fetch simply stays absent and the mapper reads from gear. (Implementation-time confirmation, not a contract decision.)
