## Context

The nutrition API has shipped nine changes establishing meal logging, recipes, hydration, daily goals, per-date overrides, and race-prep math. Each of those treated the body as a system of *intake* — what went in. None of them know about *output* — what the body did. For a recreational user, that's fine. For an endurance athlete using this system as a training-fueling backend, it's the load-bearing missing piece: meal timing relative to a session, sodium per hour of effort, energy availability, gut training, race-day rehearsals — all impossible without a workout primitive.

Garmin is the athlete's source of truth for sessions. An existing external script (`garmin.py`) already reads from Garmin Connect for the coach. The natural shape is: the backend exposes a workout write endpoint; `garmin.py` (and any future writer) translates from its source into that shape. The API stays Garmin-agnostic; the writer stays opinionated.

This is the opposite shape of `off-integration`:

```
off-integration:  external API ──→ backend pulls ──→ products cache
                  (backend holds OFF client + auth + cache)

workouts:         garmin.py ──→ backend exposes write endpoint ──→ workouts table
                  (backend has zero Garmin code; the writer is external)
```

Three reasons this is the right architecture:

1. **OAuth lives where it already lives.** `garmin.py` already authenticates with Garmin. Duplicating OAuth, token refresh, and rate-limit handling in the backend is pure cost.
2. **It generalises.** The same `POST /workouts` endpoint accepts Garmin, Apple Health, Strava, manual REST calls, or a sweat-rate-test workflow. Zero source coupling.
3. **It matches the stated architecture principle.** Backend stores primitives; external systems do heavy domain work. `garmin.py` is to workouts what the user's keyboard is to meals.

## Goals / Non-Goals

**Goals:**

- Capture: log a workout (any source) with the minimum metadata fueling tools need — sport, time window, intensity, optional burn.
- Idempotent persistence: a Garmin writer should be able to "POST every activity it sees" without tracking what's already synced. `external_id` UPSERT handles this.
- Retrieve: list workouts in a window, fetch one, edit / delete with the same hygiene as meals/hydration.
- Agent-accessible: each REST endpoint has a matching MCP tool.
- Generic enough to outlive the current Garmin writer: source-tagged but not source-coupled.

**Non-Goals:**

- Garmin OAuth, scheduled sync, or any backend code that knows what Garmin is.
- Performance analysis (laps, splits, GPS, HR/power streams, training load beyond TSS).
- Workout-aware nutrition tools (`fueling_summary`, `recommend_workout_fuel`, EA computation). All are follow-ups that this change unblocks but does not deliver.
- FK relationship between `meals`/`hydration` and `workouts`. Follow-up change.
- In-session fueling capability. Sibling capability for during-workout intake — follow-up.
- Multisport parent/child modelling. Per-leg flattening is the v1 shape; see Decision 4.
- Snapshot semantics for Garmin re-syncs. UPSERT wins; preserve-original-on-edit is a v2 concern.

## Decisions

### 1. Backend exposes a write endpoint; the writer is external

`POST /workouts` is the integration surface. `garmin.py` reads from Garmin Connect, translates each activity into the API's shape, and POSTs. The backend has zero Garmin client code, no OAuth tokens to store, no scheduled sync.

This is intentionally the inverse of `off-integration`. The asymmetry comes from data direction: OFF data is *requested* per barcode (caller-initiated; backend mediates so clients stay simple), workout data is *pushed* on a schedule the writer controls (writer-initiated; backend is the sink).

**Alternatives considered:**

- *Backend pulls from Garmin directly.* Rejected — duplicates the OAuth and rate-limit handling that already exists in `garmin.py`, locks the backend to Garmin's API quirks, makes the backend stateful in a way it doesn't need to be.
- *No table at all; the agent passes workout windows as parameters every time it asks a fueling question.* Rejected — windows are derived from workout metadata (start, end, sport, intensity). Without persistence, the agent has to remember every workout it has ever discussed, and any tool that aggregates over a date range (EA, training-load summaries) is impossible.

### 2. `external_id` UPSERT is the idempotency mechanism

The Garmin writer's job is: "list activities since last week, POST each one." It does not (and should not have to) track what's already synced. `external_id` is `TEXT NULL UNIQUE`, set to e.g. `"garmin:1234567"` for Garmin activities. The handler issues `INSERT ... ON CONFLICT (external_id) DO UPDATE SET ...`:

```
First POST  { external_id: "garmin:123", kcal_burned: 800 }  → INSERT (201)
Re-sync     { external_id: "garmin:123", kcal_burned: 850 }  → UPDATE (200)
                                                                 (kcal_burned corrected)
```

The Idempotency-Key middleware is *not* in conflict with this — the existing middleware processes the header normally if supplied. The Garmin writer doesn't send Idempotency-Key (relies on `external_id`); manual writers may send it (separate concern).

Manual workouts use `external_id: null` and `source: "manual"`. They always INSERT — duplicate prevention for manual entries is the caller's job (and rarely matters for a single-user system).

**Alternatives considered:**

- *Always require Idempotency-Key, use it as the dedup key instead of `external_id`.* Rejected — fights Garmin re-sync semantics. When Garmin recalculates an activity's kcal, the writer would have to invent a new Idempotency-Key for the corrected version, but the same body-hash check would still 409 on a different key. Two layers of conflicting protection.
- *Use PUT /workouts/external/{external_id} for Garmin and POST /workouts for manual.* Rejected as REST-pure but fiddly — two endpoints with subtly different semantics; one verb (POST) with one rule (UPSERT-if-external_id) is simpler.

### 3. TSS is the intensity signal; no `intensity` enum column

Store `tss` (Training Stress Score) as the single intensity signal. No separate `intensity` enum column. Tools that want a classification ("was this easy / moderate / hard?") derive it from TSS at call time via a small helper — initially something like `TSS < 50 = easy, 50–100 = moderate, > 100 = hard` (refined as real tools surface needs).

```
storage:    tss = nullable, avg_hr = nullable, kcal_burned = nullable
API:        returns raw signals; no classification field
downstream: tools that need a band derive it from tss at the call site
            (a tssBand(tss) helper lives in internal/workouts or wherever
             the first consumer is)
```

Rationale: TSS encodes intensity-weighted volume in a way humans don't naturally classify consistently. Asking the importer to also emit an `intensity` enum doubles the surface and invites disagreement between two sources of truth ("Garmin says TSS=85 but the importer tagged it `easy`"). Single source of truth, derive when needed.

Workouts without TSS (strength sessions, some manual entries) have no derivable intensity — that's honest; tools that need it can fall back to duration + sport heuristics or surface the gap to the agent.

The `race` concept is deliberately not present in this change. Race-day classification is intent, not measurement; whatever shape it takes (a `races` table, an `is_race` boolean, agent-side tagging via `notes`) is a follow-up. Out of scope here.

**Alternatives considered:**

- *Importer classifies + store an `intensity` enum column alongside TSS.* Rejected — two sources of truth invites drift, doubles the validation surface, and forces the importer to make a subjective call the agent can make better at read time.
- *Store both raw TSS AND derived `intensity`, with derived hidden behind a flag.* Same issue: storing a derived value is asking for staleness. Compute on read.
- *Drop TSS too; store only raw HR signals and let tools derive everything.* Rejected — TSS is what Garmin gives you; rebuilding it from HR streams is the integrator's job and the data is lossy without per-second samples we explicitly don't store.

### 4. Multisport: one row per leg

A triathlon brick is recorded by Garmin as one parent activity with N children (swim leg, T1, bike leg, T2, run leg, finish). For nutrition purposes each leg is a distinct fueling context (different sport, different intensity, different fuel preferences). The importer flattens the parent into per-leg rows:

```
Garmin: "Olympic Tri" (parent)              →   API: three workout rows
            ├ swim 1.5km (child)                ├ { sport: "swim", started_at: 8:00, ended_at: 8:25 }
            ├ T1 (transition)                   ├ { sport: "bike", started_at: 8:27, ended_at: 9:35 }
            ├ bike 40km (child)                 └ { sport: "run",  started_at: 9:37, ended_at: 10:25 }
            ├ T2
            └ run 10km (child)
```

Transitions are dropped — they're not nutrition-relevant windows.

If "tell me about the whole brick as one unit" ever becomes a real need, a `parent_external_id` column is a cheap follow-up. v1 doesn't need it: the agent can group N workouts by adjacent timestamps if asked.

**Alternatives considered:**

- *Store the parent only; lose per-leg granularity.* Rejected — every fueling tool wants leg-level data ("what did I take during the run leg of the tri?"). Parent-only collapses the most useful dimension.
- *Parent + children with FK relationship.* Rejected as overkill for v1 — doubles the model for a feature with no concrete consumer yet.

### 5. Minimum field set; aggressive about what we don't store

The table contains:

```
workouts
────────────
id              UUID PK
external_id     TEXT NULL                -- "garmin:1234567" / "strava:9876" / null
source          TEXT NOT NULL            -- enum: garmin | manual | other
sport           TEXT NOT NULL            -- enum: run | bike | swim | strength | other
name            TEXT NULL                -- e.g. "Morning Z2 ride"
started_at      TIMESTAMPTZ NOT NULL
ended_at        TIMESTAMPTZ NOT NULL     -- duration is derived; storing both is redundant
kcal_burned     NUMERIC(10, 1) NULL      -- not every source provides; manual logs often skip
avg_hr          INTEGER NULL
tss             NUMERIC(10, 2) NULL      -- Training Stress Score; THE intensity signal
notes           TEXT NULL
created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

CHECK: ended_at > started_at
CHECK: source IN ('garmin', 'manual', 'other')
CHECK: sport IN ('run', 'bike', 'swim', 'strength', 'other')
CHECK: kcal_burned IS NULL OR kcal_burned > 0
CHECK: avg_hr IS NULL OR avg_hr > 0
CHECK: tss IS NULL OR tss >= 0

INDEX  workouts_started_at_idx ON (started_at)
UNIQUE INDEX workouts_external_id_uidx ON (external_id) WHERE external_id IS NOT NULL
```

Notably absent: laps, splits, GPS coordinates, HR samples, power samples, cadence, elevation gain/loss, max HR, max power, segments, training-load metrics beyond TSS, perceived exertion (RPE — that's a tier-3 follow-up), `intensity` enum (TSS replaces it per Decision 3), `is_race` flag (race-day classification deferred). The line in the sand: **nutrition tools only need the minimum to compute fueling windows and adherence**. Performance analysis is somebody else's job.

**Alternatives considered:**

- *Include `duration_seconds` as a stored column.* Rejected — derivable from `ended_at - started_at`; storing both invites drift.
- *Include `distance_m`, `elevation_gain_m`, `avg_power`.* Considered. Useful for fueling formulae that key off distance or vertical, but `tss` already captures intensity-weighted volume; adding distance creates pressure to add cadence, route, etc. Kept out. Easy to add later if a specific tool needs them.
- *Use a JSONB `source_metadata` column for arbitrary writer-side data.* Rejected for v1 — encourages writers to dump opaque blobs that the API can never query. If real cross-source variance forces this, add it deliberately.

### 6. PATCH is restricted to mutable fields only

`source`, `external_id`, `sport`, `started_at`, `ended_at` are immutable via PATCH:

- Changing `external_id` would break the idempotency invariant (the next sync would create a duplicate).
- Changing `source` would lie about provenance.
- Changing `sport` / `started_at` / `ended_at` is structurally a different workout — delete and re-POST instead.

PATCH accepts: `name`, `notes`, `kcal_burned`, `avg_hr`, `tss`. The Garmin writer never PATCHes (POST UPSERTs everything). PATCH exists for human corrections via the agent — "actually, that session's TSS should be 75 not 60, my FTP changed last month" or "add a note about how the fueling went."

**Alternatives considered:**

- *Allow PATCH on all fields.* Rejected — `external_id` mutation breaks the idempotency invariant, and there's no real human use case for renaming a sport.
- *No PATCH at all; require delete + re-POST for any change.* Rejected — common corrections (TSS recalculation, notes addition) shouldn't require nuking and re-syncing the row.

### 7. MCP tool descriptions emphasise the agent's role

The agent rarely creates workouts (those come from the importer) but often patches them — adding notes, correcting TSS, supplying a missing kcal estimate. Tool descriptions lean into that:

- `log_workout`: "Record a workout. Most workouts come from the Garmin importer with `source: garmin` and an `external_id`; use this tool for manual entries (gym sessions without a watch, sweat-rate test windows). For manual writes, leave `external_id` null — duplicate prevention is your responsibility."
- `patch_workout`: "Adjust mutable fields after the fact — typically `notes` (capture how the fueling went), `kcal_burned` (when the source didn't provide it), or `tss` (corrections after an FTP change). `sport`, `started_at`, `ended_at`, `source`, and `external_id` are immutable — delete and re-create if those are wrong."
- `get_workout` / `list_workouts` / `delete_workout`: standard CRUD.

The descriptions explicitly call out that `external_id` is the dedup mechanism so the agent doesn't try to invent Idempotency-Keys for Garmin-sourced data.

### 8. Bulk endpoint accepts independent upserts, per-item results, partial failure allowed

`POST /workouts/bulk` exists for batch import scenarios (initial Garmin backfill of years of activities; future writers that fetch in pages). Shape:

```
POST /workouts/bulk
body: { "workouts": [Workout, Workout, ...] }   -- max 100 per call

response (200 when the request itself is well-formed):
{
  "results": [
    { "index": 0, "id": "<uuid>", "created": true },                -- new INSERT
    { "index": 1, "id": "<uuid>", "created": false },               -- UPSERT updated existing
    { "index": 2, "error": "sport_invalid" },                       -- per-item validation failure
    ...
  ]
}
```

Per-item processing rules:
- Each item is validated and upserted independently — partial failure is allowed.
- An item with the same `external_id` as a prior item in the same batch is rewritten (last-write-wins); the writer is responsible for not having duplicates within a batch.
- The overall HTTP status is `200` if the request body parsed and the array was within the cap; per-item failures don't promote to a request-level error code.
- A request with > 100 items returns `400 bulk_too_large, max: 100` before any persistence.
- An empty `workouts` array returns `400 bulk_empty`.

No MCP wrapper for bulk. Bulk is a writer-side concern (garmin.py), not an agent concern. The agent's tools stay single-item, which keeps the tool count manageable and the agent's mental model simple.

**Alternatives considered:**

- *Abort the batch on first error; return the failure index.* Rejected — for first-time backfill of years of data, a single bad item shouldn't force the writer to slice the batch. Per-item results let the writer log + skip + continue.
- *Make the batch itself transactional (all-or-nothing).* Rejected — same reasoning; one corrupted Garmin activity should not block 99 good ones.
- *Use `207 Multi-Status` when some items fail.* Considered — REST-pure but every Go HTTP client treats non-2xx as an error path. Sticking with `200` + per-item status keeps the writer simple. The body's `results` array carries the truth.
- *Add an MCP `bulk_log_workouts` tool too.* Rejected for v1 — the agent doesn't have a batching use case; adding the tool now is empty surface area.

## Risks / Trade-offs

- **TSS may be the wrong intensity signal for some sports.** Strength sessions and short-interval workouts can have a TSS that misleads downstream classifiers (a 30-min hard interval can have a TSS lower than a 4-hour easy ride). Mitigation: per-sport derivation rules in the helper that maps TSS → band, and surface the gap (e.g. "no TSS recorded for this strength session") rather than guess. If the heuristic proves consistently bad, add an explicit `intensity` field then.
- **TSS may be absent for many workouts.** Strength, swimming without a HR strap, manual entries — none of these typically have TSS. Tools that want a classification have to handle "TSS is null." Mitigation: downstream tools spec their fallback behaviour explicitly; the API surfaces honest nulls rather than guessing.
- **`avg_hr` may never be read by any v1 tool.** Storing a nullable field nothing uses is a small cost, but it preserves data the future will want. Pulling it out of Garmin and dropping it on the floor would be lossy. Alternative was a JSONB metadata blob — rejected as ungrep-able.
- **Garmin re-syncs silently update rows.** If kcal_burned changes from 800 → 850 after a Garmin recalculation, prior nutrition summaries (computed in-flight from the workouts row) would change retroactively. Acceptable for v1 — the system has no historical snapshots elsewhere either. Mitigation if this bites: add `snapshot_*` columns later, mirroring the `meal_entries` pattern. Not now.
- **Multisport per-leg flattening means losing the "this was one session" relationship.** A future agent question like "did I fuel adequately for the whole tri?" requires the agent to compose across N adjacent rows. Mitigation: the agent's calendar / training-plan context tells it which legs belong together. If this becomes a real friction, `parent_external_id` column.
- **Bulk endpoint per-item partial failure has a sharp edge.** The `200 + per-item results` shape means a careless writer that ignores the `results` array will think everything succeeded when in fact 5 of 100 items failed silently. Mitigation: design.md and the spec scenarios call this out explicitly; the writer (garmin.py) is the only consumer in v1 and can be made to log/alert on per-item errors.
- **No backend-side rate limiting on the write endpoint.** garmin.py could DDoS itself if it loops on errors. The standard auth middleware doesn't protect against this. For single-user-with-shared-secret, the threat is operator error, not malice; logging and a writer-side delay are the right mitigation. If real abuse vectors emerge (e.g. multi-tenant ever), add rate-limit middleware.

## Migration Plan

- Forward migration creates `workouts` table + the two indexes. No backfill.
- Rollback drops the table. No data outside the new table changes.

## Open Questions

- Whether `sport: "other"` is too escape-hatchy — could `yoga | hike | row | ski` be added explicitly? Tentative answer: ship with `other`; extend the enum when a specific sport's fueling tools need it.
- Whether `notes` should be markdown or plain text. Tentative answer: plain text, no rendering; if a future client wants markdown it can interpret on read.
- Whether the API should validate that `started_at` is not in the far future (>24h, same rule as `meals.logged_at`). Tentative answer: yes — same validation; cheap; catches typos. Documented in spec.
- What exactly the TSS-to-band mapping should be in the first downstream consumer. Tentative shape: `< 50 = easy, 50–100 = moderate, 100–200 = hard, > 200 = very_hard`, but with per-sport refinements. Out of scope here; lands with the first tool that needs the classification.
