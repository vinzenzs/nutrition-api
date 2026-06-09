## ADDED Requirements

### Requirement: Protein-distribution endpoint

The system SHALL expose `GET /summary/protein-distribution?date=<YYYY-MM-DD>&tz=<iana>&body_weight_kg=<float>` returning a per-meal-entry breakdown of protein intake for a single calendar date, annotated with the muscle-protein-synthesis (MPS) threshold derived from body weight. The endpoint is read-only and SHALL NOT consume an `Idempotency-Key` header.

#### Scenario: Happy path with all meals at or above threshold

- **WHEN** the client calls `GET /summary/protein-distribution?date=2026-06-09&tz=Europe/Berlin&body_weight_kg=72.5`
- **AND** four meals were logged on that local date with `protein_g` values of `28`, `25`, `30`, `40`
- **THEN** the response has shape:
  ```
  {
    "date": "2026-06-09",
    "tz": "Europe/Berlin",
    "body_weight_kg":     72.5,
    "body_weight_source": "explicit",
    "mps_threshold_g":    21.75,
    "total_protein_g":    123.0,
    "meal_count":         4,
    "mps_effective_meal_count": 4,
    "meals": [ ... ]
  }
  ```
- **AND** every entry in `meals` has `mps_effective: true`
- **AND** `meals` is ordered by `logged_at` ascending

#### Scenario: One row per meal_entries row, no implicit grouping

- **WHEN** a user logged three breakfast components (skyr, oats, honey) at `07:30:00`, `07:30:15`, `07:30:30` local time, each typed as `meal_type: breakfast`
- **THEN** the response's `meals` array contains three rows for that date
- **AND** the second row's `gap_minutes_since_previous` is `0` (same-minute logs)
- **AND** the third row's `gap_minutes_since_previous` is `0`

#### Scenario: Default tz falls back to DEFAULT_USER_TZ

- **WHEN** the client omits the `tz` query parameter
- **THEN** the calendar-day buckets and `logged_at_hour` field are computed in the server's configured `DEFAULT_USER_TZ`
- **AND** the response echoes that `tz` value

#### Scenario: Missing date is rejected

- **WHEN** the client omits `date`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_required"}`

#### Scenario: Malformed date is rejected

- **WHEN** `date` cannot be parsed as `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Invalid tz is rejected

- **WHEN** `tz` is supplied but cannot be parsed as an IANA timezone
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

#### Scenario: Invalid body_weight_kg is rejected

- **WHEN** `body_weight_kg` is supplied but is `<= 0` or NaN/Inf
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid"}`

### Requirement: MPS threshold computation

The system SHALL compute `mps_threshold_g = 0.3 × body_weight_kg`, rounded to one decimal place at the response boundary. Each per-meal row's `mps_effective: bool` SHALL be `true` when `protein_g >= mps_threshold_g` and `false` otherwise. The response-level `mps_effective_meal_count` SHALL be the count of `meals[]` entries with `mps_effective: true`.

#### Scenario: Threshold from a 70 kg body weight

- **WHEN** `body_weight_kg` resolves to `70`
- **THEN** `mps_threshold_g` is `21.0` (`0.3 × 70`, no rounding needed)

#### Scenario: Threshold rounding at half boundary

- **WHEN** `body_weight_kg` resolves to `72.5`
- **THEN** `mps_threshold_g` is `21.8` (`0.3 × 72.5 = 21.75` rounded half-away-from-zero to one decimal place)

#### Scenario: Meal at exactly the threshold counts as effective

- **WHEN** the unrounded threshold is `21.75` AND a meal's unrounded `protein_g` is also `21.75`
- **THEN** the row's `mps_effective` is `true` (inclusive lower bound; the comparison runs against the unrounded values so half-boundary thresholds are honest)

#### Scenario: Meal below the threshold is not effective

- **WHEN** the unrounded threshold is `21.75` AND a meal's unrounded `protein_g` is `21.7`
- **THEN** the row's `mps_effective` is `false`

### Requirement: Body weight resolution

The system SHALL resolve `body_weight_kg` using the following ordered rules, stopping at the first that applies. The chosen rule SHALL be reported in `body_weight_source`.

1. If `body_weight_kg` query param supplied: use it directly. `body_weight_source = "explicit"`.
2. Else if at least one body-weight entry's `logged_at` falls in the seven local days ending at `date` (inclusive, computed as `[localMidnight(date − 6d), localMidnight(date + 1d))`): `body_weight_kg = rolling 7-day mean` of those entries. `body_weight_source = "rolling_7d_avg"`.
3. Else if at least one body-weight entry exists with `logged_at < localMidnight(date)`: `body_weight_kg = weight` of the most-recent such entry. `body_weight_source = "last_before_date"`.
4. Else: `400 weight_data_missing`.

The same rule applies regardless of whether any meals were logged on the date — the threshold is informational even on empty days.

#### Scenario: Explicit body_weight_kg wins over stored data

- **WHEN** the request supplies `body_weight_kg=70` AND the user has body-weight entries in the rolling window
- **THEN** `body_weight_kg` in the response is `70.0`
- **AND** `body_weight_source` is `"explicit"`

#### Scenario: Rolling 7d average from in-window entries

- **WHEN** the user has three body-weight entries in the seven local days ending at `date`, with values `73`, `72`, `71`
- **AND** no explicit override is supplied
- **THEN** `body_weight_kg` is `72.0` (mean of 73 + 72 + 71)
- **AND** `body_weight_source` is `"rolling_7d_avg"`

#### Scenario: Last-before-date fallback

- **WHEN** the user has no body-weight entries in the seven local days ending at `date`, but has an entry from 14 days earlier with weight `75`
- **THEN** `body_weight_kg` is `75.0`
- **AND** `body_weight_source` is `"last_before_date"`

#### Scenario: No weight data and no override returns 400

- **WHEN** the user has zero body-weight entries AND no `body_weight_kg` override
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_data_missing"}`

#### Scenario: Empty day with weight data still returns the threshold

- **WHEN** the date has zero logged meals AND body-weight data exists
- **THEN** the system returns `200 OK` with `meal_count: 0`, `meals: []`, `mps_effective_meal_count: 0`, `total_protein_g: 0`
- **AND** `mps_threshold_g` is computed from the resolved body weight

### Requirement: Per-row gap-since-previous and circadian context

Each entry in the response's `meals` array SHALL carry `gap_minutes_since_previous: int | null` and `logged_at_hour: int (0..23)`. `gap_minutes_since_previous` is `null` for the first meal in the response; for subsequent meals it is `int((logged_at − previous_logged_at).minutes)`. `logged_at_hour` is the local hour-of-day (`0..23`) in the requested `tz`.

#### Scenario: First meal gap is null

- **WHEN** the response contains meals at `07:30`, `12:00`, `19:00` local
- **THEN** the first meal's `gap_minutes_since_previous` is `null`
- **AND** the second meal's `gap_minutes_since_previous` is `270`
- **AND** the third meal's `gap_minutes_since_previous` is `420`

#### Scenario: Same-minute logs get gap 0

- **WHEN** two meals are logged within the same calendar minute
- **THEN** the second meal's `gap_minutes_since_previous` is `0`

#### Scenario: logged_at_hour reflects the requested tz

- **WHEN** a meal's `logged_at` is `2026-06-09T22:30:00Z` and `tz=Europe/Berlin` (UTC+2 in summer)
- **THEN** the meal's `logged_at_hour` is `0` (00:30 local on June 10)
- **AND** the meal contributes to the `2026-06-10` calendar-day query, not the `2026-06-09` query

### Requirement: Numeric outputs rounded at the response boundary

The system SHALL round `mps_threshold_g`, every `protein_g` field, `total_protein_g`, and `body_weight_kg` to one decimal place at the response boundary. `gap_minutes_since_previous` and `logged_at_hour` are integer fields and SHALL NOT be rounded (they are computed as integers).

#### Scenario: Rounding example

- **WHEN** the underlying computation yields `mps_threshold_g = 21.7666...`
- **THEN** the response shows `21.8`
