## Context

`map_athlete_config` derives threshold paces from `user_profile.userData.lactateThresholdSpeed` / `lactateThresholdSwimSpeed` via `_speed_to_pace_per_km` / `_speed_to_pace_per_100m`, which compute `1000 / value` and `100 / value` — i.e. they assume the field is metres-per-second. The prior change (fix-garmin-bridge-athlete-config-mapping) observed the result was garbage and added a plausibility guard that *drops* it, leaving the field unpopulated and flagging the unit as a follow-up. This change is that follow-up.

Empirical confirmation (live, account device): `lactateThresholdSpeed = 0.26944369`.
- As m/s → `1000 / 0.269 = 3711 s/km` (61:53/km) — impossible.
- `1 / 0.269 = 3.711 m/s`, and `0.269 × 1000 = 269.4 s/km = 4:29/km`.
- Real easy runs: `averageSpeed` 2.16–2.27 m/s (7:20–7:42/km). A threshold of 3.71 m/s (faster than easy) is correct; 0.27 m/s (slower than walking) is not.

Conclusion: the field is **seconds-per-metre** (a pace). `pace_sec_per_km = value × 1000`; `pace_sec_per_100m = value × 100`.

## Goals / Non-Goals

**Goals:**
- Convert threshold pace correctly (s/m → pace) so `threshold_pace_sec_per_km` populates.
- Keep a plausibility safety net for genuinely bad values.

**Non-Goals:**
- Changing any other athlete-config field.
- Direct confirmation of the swim field's unit (absent for this account; applied by analogy).

## Decisions

- **Reinterpret the field as s/m.** Replace `1000 / value` with `value * 1000` and `100 / value` with `value * 100`. Rename the helpers to `_pace_per_km` / `_pace_per_100m` and the parameter to `sec_per_m` so the unit is self-documenting (the old `_speed_to_pace_*` / `speed_mps` names asserted the wrong unit).
- **Retain the plausibility guard, re-justified.** Keep the run band (≈90–1200 s/km) and swim band (≈30–600 s/100m) as a defence against a future garbage value, but the legitimate `269.4`/`125` now pass. The guard is no longer the reason the field is empty.
- **Fixture realism.** Set `lactateThresholdSpeed` to a realistic s/m value so the test asserts a real pace under the corrected math (e.g. `0.25 → 250 s/km`), and a swim value (`1.5 → 150 s/100m`). Repurpose the implausible-pace test to a value that is genuinely out-of-band under the *correct* unit (the old `0.269` is now valid).

## Risks / Trade-offs

- [Swim unit inferred, not confirmed] → Same field family and API as the confirmed run field; the s/m interpretation is the consistent choice. If a real swim value later contradicts it, it's a one-line follow-up, and the plausibility guard limits blast radius.
- [Guard could mask a real edge value] → Bands are generous (1:30–20:00/km run, 0:30–10:00/100m swim); real threshold paces sit comfortably inside.
