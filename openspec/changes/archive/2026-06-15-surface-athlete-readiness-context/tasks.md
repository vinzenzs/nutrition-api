## 1. Service plumbing

- [x] 1.1 Confirm `athleteconfig.Repo.Get` behavior on an unset singleton (zero-value vs not-found) so the goroutine can map "unset" to a nil `AthleteConfig`
- [x] 1.2 Add `athleteConfigRepo *athleteconfig.Repo` and `bodyWeightRepo *bodyweight.Repo` fields + constructor params to `coachcontext.Service` (`NewService`)
- [x] 1.3 Update the `coachcontext.NewService(...)` call site in `internal/httpserver/server.go` to pass `athleteConfigRepo` and `bodyWeightRepo`

## 2. Bundle shape + derivation

- [x] 2.1 Add `AthleteConfig *athleteconfig.AthleteConfig` (`json:"athlete_config"`) and `WattsPerKg *float64` (`json:"watts_per_kg"`) to `TrainingContext` in `internal/coachcontext/types.go`, with doc comments mirroring the `acwr` field
- [x] 2.2 Add a `wattsPerKg(cfg, bwKg)` helper returning `*float64` — non-nil only when FTP present and bodyweight kg > 0, rounded via `numfmt.Round1` (mirror the existing `acwr` helper)
- [x] 2.3 Add an `errgroup` goroutine in `BuildTraining` that fetches the athlete config and `bodyWeightRepo.LatestBefore(ctx, dayEnd)`, sets `out.AthleteConfig` (nil when unset) and `out.WattsPerKg`; no error on absent data

## 3. Tests

- [x] 3.1 Service/handler test: athlete_config + bodyweight present → `athlete_config` block surfaced and `watts_per_kg` = FTP ÷ kg (rounded)
- [x] 3.2 Service/handler test: FTP missing OR no bodyweight → `watts_per_kg` is null, response still 200
- [x] 3.3 Service/handler test: no athlete_config row → `athlete_config` serializes null, bundle otherwise unchanged
- [x] 3.4 Confirm the existing "quiet history is not an error" path still returns 200 with the new fields null

## 4. Docs + verification

- [x] 4.1 Run `task swag` to regenerate `docs/` for the new `GET /context/training` response fields
- [x] 4.2 Run `go test -count=1 ./internal/coachcontext/...` and `task vet`; confirm green
