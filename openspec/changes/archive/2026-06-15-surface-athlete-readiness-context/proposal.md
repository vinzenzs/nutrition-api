## Why

When the coach grounds on `GET /context/training` (via `get_training_context`), it can reason about phase, fitness, load, and the schedule — but it has no view of the athlete's physiology. `athlete_config` (FTP, thresholds, HR/power zones) is captured but surfaced nowhere in the bundle, and power-to-weight (W/kg) — the single number that frames cycling/threshold prescriptions — is computed nowhere despite FTP and bodyweight both being stored. The coach therefore can't anchor intensity advice to the athlete's actual zones or W/kg in the same grounding call it already makes.

## What Changes

- Extend the `GET /context/training` bundle with an `athlete_config` block (the singleton FTP/threshold/zone row, or null when unset) so the coach grounds on zones and thresholds in the same call.
- Add a derived `watts_per_kg` to the bundle: `ftp_watts ÷ latest bodyweight (kg)`, computed only when both are present and bodyweight is non-zero, null otherwise — mirroring the existing null-when-incomplete `acwr` derivation. Never stored.
- Wire `athleteconfig.Repo` and `bodyweight.Repo` into `coachcontext.Service`, fetched in parallel with the existing fan-out (no partial bundle on error).
- Regenerate `docs/` (swag) for the new response fields.
- **Out of scope (documented non-goal):** ACWR null. The server already derives ACWR (acute ÷ chronic) and accepts those loads via `log_fitness_metrics`; populating `acute_load`/`chronic_load` is a Garmin-bridge ingestion concern, not a change to this Go service. No server-side TSS-derived fallback is added.
- **Already shipped (no work):** phase `methodology` surfacing — delivered by the archived `add-coach-methodology` change.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `coach-context`: the training context aggregate read SHALL additionally surface the athlete-config block and a derived `watts_per_kg`, with null serialization when the inputs are absent.

## Impact

- **Code:** `internal/coachcontext/{types.go,service.go}` (new fields + parallel reads + W/kg derivation), `internal/httpserver/server.go` (pass the two new repos into `coachcontext.NewService`). Read-only deps `internal/athleteconfig` and `internal/bodyweight` are consumed, not modified.
- **API:** additive fields on `GET /context/training` (and therefore the `get_training_context` MCP tool, which forwards the body verbatim). No request-shape change; backward compatible.
- **Docs:** `docs/` regenerated via `task swag`.
- **No migration** — composition-only read, consistent with the rest of `coachcontext`.
