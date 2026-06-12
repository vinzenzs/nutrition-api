## Context

Change B imports per-activity normalized power and `secs_in_zone_1..5`, but those numbers are only half-interpretable without the athlete's physiology: B's own open question noted that `intensity_factor` stays NULL with no FTP to divide normalized power by, and the zone seconds carry no labeled boundaries (we store "240s in zone 4" without recording what zone 4's HR range is). Garmin holds this configuration in the user profile and heart-rate-zone settings, which the bridge already authenticates against on every sync.

This config is **slowly-changing physiology**, not a per-day snapshot. FTP shifts a handful of times a season; threshold HR and zone boundaries change rarely. It is identified by nothing — there is one athlete, one configuration — so the natural shape is a **singleton row**, exactly like `nutrition_goals`, not a date-keyed snapshot like recovery/fitness/daily-summary.

Constraints that shape the design:
- **One package per capability** — `athlete-config` is a new package `internal/athleteconfig/`, not a child of `workouts` or `goals`.
- **Singleton pattern** — model on `nutrition_goals`: a fixed sentinel primary key, `INSERT … ON CONFLICT (id) DO UPDATE`, lazy-created on first write, `GET` returns `{"athlete_config": null}` before any write.
- **PUT rejects `Idempotency-Key`** — per the harden-write-paths rule that PUT means full-replace and a replayed PUT would lie about intermediate state; `400 idempotency_unsupported_for_put`.
- **`numfmt.Round1` at the response boundary** for every float.
- **Unit isolation** — config stays on the `athlete-config` shape; it must never leak into `summary`'s Totals struct or any other capability's response.
- **Append-only sequential migrations** embedded in the binary; arc order B=036, A=037, C=038, D=039 ⇒ this is `040`. Verify the head on disk before scaffolding.
- **garmin-bridge tolerates per-capability failure** — a failing `get_user_profile` / `get_heart_rate_zones` must degrade to "no config refresh this sync", never an aborted day.

## Goals / Non-Goals

**Goals:**
- Capture the athlete's physiology configuration so workout-detail data becomes interpretable: FTP, threshold HR, threshold run pace, threshold swim pace, max HR, lactate-threshold HR, and the HR-zone (and optionally power-zone) boundaries.
- Store it as a singleton with the same `GET`/`PUT` REST surface as `nutrition-goals`, refreshed from Garmin on each daily sync via an in-place upsert.
- Surface it through one MCP read tool (`athlete_config_get`) for the coaching agent.

**Non-Goals (explicit — this change is CAPTURE ONLY):**
- **No derivation of `intensity_factor` from FTP.** B's `intensity_factor` stays exactly as B left it (mapped if Garmin's activity summary carries it, NULL otherwise). Computing `normalized_power_w / ftp_watts` to back-fill it is a follow-up that would touch B's mapper. Out of scope (D2).
- **No relating stored `secs_in_zone_*` to the imported zone boundaries.** This change stores both halves (the seconds on `workouts` from B, the boundaries on `athlete_config` here); *joining* them to emit labeled zone ranges (e.g. "zone 4 = 155–168 bpm, 240s") is a follow-up. Out of scope (D2).
- **No raceprep / fueling-math coupling.** FTP and thresholds are stored as context only; no carb/hydration/intensity computation reads them in this change. Wiring FTP into the raceprep intensity math is a follow-up. Out of scope (D2).
- **No per-field history.** Only the current configuration is mirrored; a config that changed mid-season overwrites the prior value (a follow-up could keystone history if a trend line is ever wanted).
- **No new capability for power zones if Garmin omits them** — power-zone boundaries are optional columns on the same singleton, not a separate shape.

## Decisions

### D1 — Singleton table modeled on `nutrition_goals`, not a date-keyed snapshot

`athlete_config` is a one-row table with a fixed sentinel primary key (e.g. `00000000-0000-0000-0000-000000000001`), `INSERT … ON CONFLICT (id) DO UPDATE` upsert, lazy-created on first write. `GET /athlete-config` returns `{"athlete_config": null}` until the first write, then `{"athlete_config": <config>}`. `PUT /athlete-config` is full-replace: absent fields are stored as NULL (cleared), matching `PUT /goals`. This is deliberately the `goals` shape and **not** the recovery/fitness/daily-summary date-keyed-snapshot shape, because the config has no per-day dimension — there is one athlete with one current physiology.
*Alternative considered:* a date-keyed `athlete_config(date)` table so each day records the FTP in force that day. Rejected for this change — it adds a date dimension nothing consumes yet, and the singleton is the simpler honest model of "current config". History is a documented follow-up (non-goal).

### D2 — CAPTURE ONLY: store the config, consume nothing

This change persists FTP, thresholds, and zone boundaries and exposes them for reading. It deliberately does **not**:
- derive `intensity_factor` from FTP (would touch B's `map_workouts` / the workouts mapper),
- label or join the stored `secs_in_zone_*` against the imported boundaries (would touch B's response shape and the summary/coaching surfaces),
- feed FTP/thresholds into the raceprep intensity or carb-load math.

The reason to split capture from consumption: each consumer is a distinct, independently-reviewable change with its own spec deltas (B's mapper, the raceprep math), and bundling them would couple this slow-changing-config import to fast-moving fueling logic. Storing the config first makes those follow-ups trivial (the data is already there) without blocking this slice on them. **This is the headline design decision of the change.**

### D3 — Garmin is source-of-truth; daily sync overwrites manual PUT edits (chosen)

There is a tension: the same singleton is writable by both the user (via `PUT /athlete-config`) and the daily Garmin sync (also via `PUT /athlete-config`, in-place upsert). If the user hand-sets a field, the next sync overwrites it.

**Chosen stance: Garmin is source-of-truth for these fields.** The daily sync's `PUT /athlete-config` wins; a manual edit is transient and the next sync corrects it back to Garmin's value. This matches D's gear/PR stance ("Garmin owns the truth; the backend is a mirror") and is the right default because FTP/threshold/zone boundaries are *measured and computed by Garmin* (auto-detected threshold, Firstbeat zone math) — the athlete's own source of truth already lives in Garmin Connect, and the `PUT` endpoint exists mainly for manual override in dev/test or when Garmin lacks a field. The behavior is documented so it is not a surprise: hand-edits to Garmin-sourced fields do not persist across a sync.
*Alternative considered — merge: sync only fills NULL fields, never overwrites a user-set value.* Rejected for this change: it needs a "manually set vs Garmin-set" provenance flag per field (or a last-writer-wins timestamp), which is complexity nothing yet needs at single-user scale. If a real need to pin a manual override surfaces, a follow-up can add a per-field provenance flag without changing this requirement.

### D4 — Non-date-keyed refresh folded into the existing daily `POST /sync`

Like D's gear/PR inventory, the config fetch is cheap (two profile/zone endpoints return a handful of fields for one user) and idempotent (re-observing the same config re-writes the same singleton). So it rides the existing daily `POST /sync` rather than a separate cadence: after the date-keyed work, the bridge fetches the profile + HR zones (guarded), maps them, and `PUT`s the singleton. A stale-but-present config is fine; the next sync corrects it.
*Alternative considered — a separate `POST /sync/config` step on its own (e.g. weekly) cadence.* Rejected for now: a second cron entry and code path for no benefit at single-user scale, given how cheap the fetch is. The `PUT /athlete-config` endpoint is cadence-agnostic, so moving to a weekly refresh later changes only the bridge's scheduling, not the backend contract.

### D5 — Field set, units, and zone cardinality

All fields nullable (NULL is meaningful — "Garmin didn't provide it"):
- `ftp_watts` (INTEGER, cycling functional threshold power)
- `threshold_hr` (INTEGER, functional threshold heart rate, bpm)
- `lactate_threshold_hr` (INTEGER, Garmin's lactate-threshold HR, bpm — distinct from FTHR; kept separate because Garmin exposes both)
- `max_hr` (INTEGER, bpm)
- `threshold_pace_sec_per_km` (run threshold pace, stored as seconds per kilometre — a sport-agnostic SI-ish rate; pace strings are derivable client-side)
- `threshold_swim_pace_sec_per_100m` (swim threshold pace, seconds per 100 m — the conventional swim unit)
- `hr_zone_1_max` … `hr_zone_5_max` (five INTEGERs, bpm; the *upper* bound of each HR zone — fixed cardinality of 5, so columns not a child table, mirroring B's `secs_in_zone_1..5` decision)
- `power_zone_1_max` … `power_zone_5_max` (five INTEGERs, watts; optional — present only if `get_heart_rate_zones` / profile carries power zones)

Zone boundaries are stored as the per-zone *max* (upper bound); zone 1's lower bound is 0/resting and each subsequent zone's lower bound is the previous zone's max, so five maxima fully describe the boundaries without redundant storage. Fixed cardinality (always 5 zones) → columns, not a `athlete_config_zones` child table — join-free and symmetric with B's zone-seconds columns.

### D6 — Bridge fetch guarded per endpoint, mapped defensively

`fetch_day` gains `get_user_profile` (and/or `get_userprofile_settings`, whichever carries FTP/thresholds) and `get_heart_rate_zones`, each wrapped in the existing `safe()` pattern — a failing or account-unavailable endpoint yields a missing key, not an aborted day. `map_athlete_config` reads whatever is present and omits what is absent (the singleton treats absent as distinct from a real zero, like every other mapper). The mapper is pure and exhaustively tested against the recorded fixture, consistent with the rest of `mapping.py`.

### D7 — MCP mirrors REST 1:1; one new read tool

`athlete_config_get` → `GET /athlete-config`, building one HTTP request via `apiClient` and forwarding the body verbatim (`toToolResult`), per the MCP-mirrors-REST rule. Read tool — never sends `Idempotency-Key`. No write tool: the agent does not `PUT` config (config writes are Garmin/bridge-owned per D3, plus manual dev override). The `mcp_integration_test` expected-tools list grows by exactly one.

## Risks / Trade-offs

- **Daily sync overwrites manual edits** → accepted and documented (D3); Garmin is source-of-truth, and the `PUT` endpoint is mainly for dev override / fields Garmin lacks. A follow-up can add per-field provenance if a real pin-the-override need surfaces.
- **Captured-but-unconsumed data** → the FTP/zone boundaries sit unused until the follow-ups (D2) wire them into IF derivation / zone labeling / raceprep. Accepted: storing first makes those follow-ups trivial and keeps this slice small and reviewable.
- **Garmin profile/zone shape drift** → isolated in `mapping.py` behind `safe()` and defensive extraction; a drift degrades to "config field omitted / no refresh this sync", not a failed sync — same posture as the rest of the bridge.
- **Lactate-threshold HR vs functional-threshold HR confusion** → kept as two distinct nullable columns (`threshold_hr`, `lactate_threshold_hr`) because Garmin exposes both and conflating them would silently lose information; consumers pick the one they want.

## Migration Plan

1. **Confirm the migration head on disk** before `task migrate:new NAME=add_athlete_config` — out-of-band work has occasionally taken a slot, and the on-disk head may lag the arc order (B=036, A=037, C=038, D=039) if siblings have not yet landed. Arc order fixes this change at `040`.
2. `040_add_athlete_config.up.sql`: `CREATE TABLE athlete_config` with a fixed sentinel `id` primary key (the singleton), all physiology columns nullable, CHECKs mirroring existing conventions (non-negative / positive where sensible, e.g. `ftp_watts IS NULL OR ftp_watts > 0`, `max_hr IS NULL OR max_hr > 0`), and `created_at` / `updated_at`.
3. `.down.sql`: drop `athlete_config`.
4. Rollback is clean — one additive table, no data transform; no other table references it.

## Open Questions

- **Which Garmin endpoint actually carries FTP / threshold pace:** `get_user_profile`, `get_userprofile_settings`, or a per-sport settings call — the mapper reads from whichever is present (defensive extraction), and a field Garmin omits simply stays NULL. (Implementation-time confirmation, not a contract decision.)
- **Whether Garmin exposes power-zone boundaries at all for this account:** the `power_zone_*` columns are present but optional; if Garmin returns no power zones, they stay NULL and the singleton is HR-zones-only. No contract impact.
- **Threshold-pace unit:** locked to `sec_per_km` (run) and `sec_per_100m` (swim) above — sport-conventional and pace-string-derivable client-side; not storing pace strings. **Resolved in spec.**
