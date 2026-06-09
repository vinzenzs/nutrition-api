# meals Specification

## Purpose

Define meal-logging endpoints, validation, and daily/range summary behavior for the nutrition API.

## Requirements

### Requirement: Log a meal entry from a known product

The system SHALL expose `POST /meals` that records a meal entry referencing an existing product with a quantity in grams.

#### Scenario: Successful meal log

- **WHEN** the client posts `{"product_id":"<id>","quantity_g":150,"logged_at":"2026-06-06T12:30:00Z"}` to `POST /meals`
- **THEN** the system creates a meal entry with the supplied fields
- **AND** returns `201 Created` with the new meal entry row including its generated id and the resolved effective nutriments for the entry

#### Scenario: Optional fields are accepted

- **WHEN** the client supplies `meal_type` (one of `breakfast`, `lunch`, `dinner`, `snack`) and `note` (free text)
- **THEN** the system stores both on the meal entry

#### Scenario: Missing product_id is rejected

- **WHEN** the client posts a body without `product_id`
- **THEN** the system returns `400 Bad Request` with `{"error":"product_id_required"}`

#### Scenario: Unknown product_id is rejected

- **WHEN** the client posts `product_id` that does not exist in the products table
- **THEN** the system returns `404 Not Found` with `{"error":"product_not_found"}`

#### Scenario: Non-positive quantity is rejected

- **WHEN** the client posts `quantity_g` that is zero, negative, or absent
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_g_invalid"}`

#### Scenario: logged_at far in the future is rejected

- **WHEN** the client posts a `logged_at` more than 24 hours in the future relative to server time
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`

#### Scenario: Invalid meal_type is rejected

- **WHEN** the client posts `meal_type` that is not one of `breakfast`, `lunch`, `dinner`, `snack`
- **THEN** the system returns `400 Bad Request` with `{"error":"meal_type_invalid"}`

### Requirement: Log a meal entry from freeform LLM-supplied nutriments

The system SHALL expose `POST /meals/freeform` that records a meal entry from a client-supplied name and nutriment estimate without requiring a pre-existing product. The accepted nutriment fields include macros (`kcal`, `protein_g`, `carbs_g`, `fat_g`, `fiber_g`, `sugar_g`, `salt_g`) and micros (`iron_mg`, `calcium_mg`, `vitamin_d_mcg`, `vitamin_b12_mcg`, `vitamin_c_mg`, `magnesium_mg`, `potassium_mg`, `zinc_mg`), all per-100g.

#### Scenario: Freeform log without saving as product

- **WHEN** the client posts `{"name":"banana","nutriments_per_100g":{"kcal":89,"protein_g":1.1,"carbs_g":22.8,"fat_g":0.3,"potassium_mg":358,"vitamin_c_mg":8.7},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}` to `POST /meals/freeform`
- **THEN** the system creates a meal entry with `product_id = null`
- **AND** stores the supplied name in `snapshot_name`
- **AND** stores each supplied nutriment (macro or micro) in the corresponding `snapshot_*_per_100g` column
- **AND** stores unsupplied micros as null
- **AND** returns `201 Created` with the meal entry

#### Scenario: Freeform log with save_as_product creates a reusable product

- **WHEN** the client posts the same body with `"save_as_product": true`
- **THEN** the system creates a manual product with the supplied name and nutriments (macros and micros)
- **AND** links the meal entry's `product_id` to the new product
- **AND** still stores the snapshot columns on the meal entry

#### Scenario: Freeform requires name

- **WHEN** the client omits `name` or supplies an empty string
- **THEN** the system returns `400 Bad Request` with `{"error":"name_required"}`

#### Scenario: Freeform validates nutriments are numeric and non-negative

- **WHEN** any supplied nutriment value (macro or micro) is negative or non-numeric
- **THEN** the system returns `400 Bad Request` with `{"error":"nutriments_invalid","field":"<which>"}`

### Requirement: Retrieve a meal entry

The system SHALL expose `GET /meals/{id}` that returns a single meal entry by id, including its effective nutriments resolved from snapshot or linked product.

#### Scenario: Existing entry is returned with effective nutriments

- **WHEN** the client calls `GET /meals/{id}` with a valid id
- **THEN** the system returns `200 OK` with the meal entry fields
- **AND** includes an `effective_nutriments_per_100g` object where each field is `coalesce(snapshot_<field>, product.<field>)`

#### Scenario: Missing entry returns 404

- **WHEN** the client calls `GET /meals/{id}` with an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"meal_not_found"}`

### Requirement: List meal entries in a window

The system SHALL expose `GET /meals?from=<iso>&to=<iso>&meal_type=<type>` that returns meal entries whose `logged_at` falls within the half-open window `[from, to)`, optionally filtered by meal_type.

#### Scenario: Window filtering returns only entries in range

- **WHEN** the client calls `GET /meals?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **THEN** the system returns entries with `from <= logged_at < to`, ordered by `logged_at` ascending

#### Scenario: meal_type filter narrows results

- **WHEN** the client also supplies `meal_type=breakfast`
- **THEN** only entries with `meal_type = "breakfast"` are returned

#### Scenario: Missing window parameters are rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Inverted window is rejected

- **WHEN** the client supplies `from >= to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

### Requirement: Edit a meal entry

The system SHALL expose `PATCH /meals/{id}` that allows updating `quantity_g`, `logged_at`, `meal_type`, and `note` on an existing meal entry.

#### Scenario: Partial update succeeds

- **WHEN** the client patches `{"quantity_g": 200}` on an existing meal entry
- **THEN** the system updates only that field and leaves others unchanged
- **AND** returns `200 OK` with the updated entry

#### Scenario: Unknown fields are ignored, not rejected

- **WHEN** the client patches a body containing fields beyond the editable set (e.g. `product_id`, `snapshot_*`)
- **THEN** the system ignores those fields and processes the editable ones

#### Scenario: Patching to an invalid quantity is rejected

- **WHEN** the client patches `quantity_g` to zero or a negative number
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_g_invalid"}`

#### Scenario: Patching last_logged_at on the linked product respects monotonicity

- **WHEN** the client patches `logged_at` to a value greater than `products[X].last_logged_at`
- **THEN** the product's `last_logged_at` advances to the new value
- **AND** if the patched value is earlier, the product's `last_logged_at` is unchanged

### Requirement: Delete a meal entry

The system SHALL expose `DELETE /meals/{id}` that removes a meal entry.

#### Scenario: Delete returns 204 on success

- **WHEN** the client calls `DELETE /meals/{id}` with an existing id
- **THEN** the system removes the meal entry
- **AND** returns `204 No Content`

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client calls `DELETE /meals/{id}` with an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"meal_not_found"}`

#### Scenario: Deleting the most recent entry for a product does not revert last_logged_at

- **WHEN** a meal entry is deleted
- **THEN** the linked product's `last_logged_at` is unchanged

### Requirement: Daily summary

The system SHALL expose `GET /summary/daily?date=<YYYY-MM-DD>&tz=<iana>&meal_type=<type>` that returns total nutriments (macros and micros), the list of meal entries for the given calendar date in the supplied timezone, and an `adherence` block when goals are set. The optional `meal_type` query parameter scopes the response to entries of that meal type only.

#### Scenario: Daily totals are computed from effective nutriments

- **WHEN** the client calls `GET /summary/daily?date=2026-06-06&tz=Europe/Berlin`
- **THEN** the system selects meal entries whose `logged_at` falls within `[2026-06-06 00:00:00 Europe/Berlin, 2026-06-07 00:00:00 Europe/Berlin)` converted to UTC
- **AND** computes totals as `sum(coalesce(snapshot_<field>, product.<field>) * quantity_g / 100)` for each macro and each micro
- **AND** omits any micro from `totals` for which no contributing entry has a non-null effective value (no fake-zero)
- **AND** returns `{"date":"2026-06-06","tz":"Europe/Berlin","totals":{...},"entries":[...],"adherence":{...}}`

#### Scenario: Adherence is computed from goals when present

- **WHEN** the goals row has values for kcal_target and `protein_g` `{min, max}`
- **THEN** the response `adherence` object contains entries for kcal and protein_g per the nutrition-goals adherence rules
- **AND** when no goals are set, `adherence` is omitted from the response (not an empty object)

#### Scenario: meal_type filter scopes totals and entries

- **WHEN** the client calls `GET /summary/daily?date=2026-06-06&tz=Europe/Berlin&meal_type=breakfast`
- **THEN** the response `totals` reflect only entries with `meal_type = "breakfast"` on that day
- **AND** the response `entries` array contains only those entries
- **AND** `adherence` is omitted (per-day goals do not apply to a single meal type)
- **AND** the response includes `meal_type: "breakfast"` to echo the filter

#### Scenario: Invalid meal_type filter is rejected

- **WHEN** the client supplies `meal_type` that is not one of `breakfast`, `lunch`, `dinner`, `snack`
- **THEN** the system returns `400 Bad Request` with `{"error":"meal_type_invalid"}`

#### Scenario: Missing tz falls back to DEFAULT_USER_TZ

- **WHEN** the client omits `tz` and `DEFAULT_USER_TZ=Europe/Berlin` is set
- **THEN** the system computes the day window in `Europe/Berlin`
- **AND** the response `tz` field reflects `Europe/Berlin`

#### Scenario: Invalid tz is rejected

- **WHEN** the client supplies a `tz` that is not a valid IANA name
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

#### Scenario: Invalid date format is rejected

- **WHEN** the client supplies `date` not matching `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Day with no entries returns zero totals for macros

- **WHEN** no meal entries fall within the day window
- **THEN** the system returns `200 OK` with macro totals all zero and an empty `entries` array
- **AND** micro totals are omitted entirely (not zeroed)
- **AND** `adherence` is omitted

### Requirement: Range summary

The system SHALL expose `GET /summary/range?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>&tz=<iana>&group_by=<mode>` that returns per-day totals across the inclusive date range. The optional `group_by` parameter accepts `meal_type`; when set, each day's totals are broken out per meal type instead of being aggregated.

#### Scenario: Range summary returns one entry per day

- **WHEN** the client calls `GET /summary/range?from=2026-06-01&to=2026-06-07&tz=Europe/Berlin`
- **THEN** the system returns `{"from":"2026-06-01","to":"2026-06-07","tz":"Europe/Berlin","days":[...]}`
- **AND** `days` contains seven entries, one per calendar date in the range, each with its own `totals` object and an `adherence` object when goals are set

#### Scenario: group_by=meal_type returns per-meal-type totals per day

- **WHEN** the client calls `GET /summary/range?from=2026-06-01&to=2026-06-07&tz=Europe/Berlin&group_by=meal_type`
- **THEN** each day in `days` has `by_meal_type: {breakfast: {totals: {...}}, lunch: {...}, dinner: {...}, snack: {...}}` instead of a top-level `totals`
- **AND** meal types with no entries on a day are omitted from `by_meal_type` for that day
- **AND** `adherence` is omitted from each day (per-meal-type adherence is out of scope)

#### Scenario: Invalid group_by is rejected

- **WHEN** the client supplies `group_by` that is not `meal_type`
- **THEN** the system returns `400 Bad Request` with `{"error":"group_by_invalid"}`

#### Scenario: Days with no entries appear with zero totals

- **WHEN** a day in the range has no logged meals
- **THEN** that day still appears in `days` with macro totals all zero
- **AND** micro totals are omitted for that day
- **AND** `by_meal_type` is omitted for that day when `group_by=meal_type` is set

#### Scenario: Range exceeding 92 days is rejected

- **WHEN** the supplied range covers more than 92 calendar days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

#### Scenario: Inverted range is rejected

- **WHEN** `from > to`
- **THEN** the system returns `400 Bad Request` with `{"error":"range_invalid"}`

### Requirement: Rolling-window summary

The system SHALL expose `GET /summary/rolling?anchor_date=<YYYY-MM-DD>&window_days=<int>&tz=<iana>` returning the trailing-window average of nutrition totals as of `anchor_date`. The window is `[anchor_date − (window_days − 1) days, anchor_date]`, both inclusive, with calendar-day buckets resolved in the requested `tz` (defaulting to `DEFAULT_USER_TZ`). The endpoint is read-only and SHALL NOT consume an `Idempotency-Key` header.

#### Scenario: Complete-data window

- **WHEN** the client calls `GET /summary/rolling?anchor_date=2026-06-08&window_days=7&tz=Europe/Berlin`
- **AND** every calendar day in the inclusive window `2026-06-02..2026-06-08` has at least one logged meal
- **THEN** the response has shape:
  ```
  {
    "anchor_date": "2026-06-08",
    "window_days": 7,
    "tz": "Europe/Berlin",
    "averages": { "kcal": 2280.5, "protein_g": 128.0, ... },
    "days_with_data": 7,
    "total_days": 7,
    "days": [
      { "date": "2026-06-02", "totals": { ... }, "has_data": true },
      ...
      { "date": "2026-06-08", "totals": { ... }, "has_data": true }
    ],
    "adherence": { ... },
    "goal_source": "default" | "override"
  }
  ```
- **AND** `days` is ordered ascending by `date` with exactly seven entries (one per calendar day in the window)
- **AND** `averages.kcal` equals the mean of the seven per-day `totals.kcal` values

#### Scenario: Default tz falls back to DEFAULT_USER_TZ

- **WHEN** the client omits the `tz` query parameter
- **THEN** the calendar-day buckets are computed in the server's configured `DEFAULT_USER_TZ`
- **AND** the response echoes that `tz` value

### Requirement: Rolling averages use days-with-data divisor

The system SHALL compute each entry in `averages` as the mean of the per-day totals across days where at least one meal was logged (`has_data: true`). Days with no meals SHALL NOT contribute to the divisor. The response SHALL expose `days_with_data` and `total_days` so the caller can detect sparse windows.

#### Scenario: Sparse window divides by logged days only

- **WHEN** the window covers seven days and the client has logged meals on only six of them (one day is empty)
- **AND** total kcal across those six days is `12 600`
- **THEN** `averages.kcal` is `2100.0` (`12600 / 6`), not `1800.0` (`12600 / 7`)
- **AND** `days_with_data` is `6`
- **AND** `total_days` is `7`

#### Scenario: Empty window returns null averages

- **WHEN** the window has zero days with data (no meals logged anywhere in the seven-day range)
- **THEN** every entry in `averages` is `0.0` (or `null` for the nullable-micro fields)
- **AND** `days_with_data` is `0`
- **AND** every per-day row has `has_data: false`
- **AND** the response status remains `200 OK`

### Requirement: Per-day rows distinguish "no data" from "logged zero"

Each entry in the response's `days` array SHALL carry a `has_data: bool` field. `has_data` is `true` when at least one meal was logged on that calendar day, regardless of whether the resulting totals are zero. `has_data` is `false` when no meal was logged on that day.

#### Scenario: No-meal day is flagged

- **WHEN** the user logged no meals on `2026-06-05`
- **THEN** the response's row for `2026-06-05` has `has_data: false`
- **AND** every numeric field in that row's `totals` is `0.0` (or `null` for nullable micros)

#### Scenario: Zero-kcal meal day is NOT flagged as missing

- **WHEN** the user logged a single meal on `2026-06-05` whose computed kcal is `0`
- **THEN** the response's row for `2026-06-05` has `has_data: true`
- **AND** `totals.kcal` is `0.0`

### Requirement: Adherence is computed against the goal resolved at the anchor

The system SHALL compute the `adherence` object against the goal that applies at `anchor_date`, honoring any per-date override in `daily_goal_overrides` for that exact date. Adherence semantics (`actual` / `target` / `delta_pct` / `status`) match the existing `daily_summary` adherence contract (`unify-adherence-shape`). `actual` is the window-average value for each nutrient; `target` is the goal range; `status` is `under` / `on` / `over` / `no_data`.

#### Scenario: Adherence against default goals

- **WHEN** no override applies at the anchor date
- **THEN** `goal_source` is `"default"`
- **AND** each adherence entry's `target` reflects the default `nutrition_goals` row

#### Scenario: Adherence honors an override at the anchor

- **WHEN** an entry exists in `daily_goal_overrides` for `anchor_date`
- **THEN** `goal_source` is `"override"`
- **AND** each adherence entry's `target` reflects that override

#### Scenario: Adherence status is no_data when the window has no logged data

- **WHEN** `days_with_data` is `0`
- **THEN** every adherence entry has `status: "no_data"` and `actual: null`

### Requirement: Window-day bounds and parameter validation

The system SHALL enforce `2 <= window_days <= 30`. Invalid windows return `400 Bad Request` with the corresponding error code.

#### Scenario: Below minimum is rejected

- **WHEN** `window_days` is `1` (or less)
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":2,"max":30}}`

#### Scenario: Above maximum is rejected

- **WHEN** `window_days` is `31` (or more)
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":2,"max":30}}`

#### Scenario: Missing anchor_date is rejected

- **WHEN** the client omits `anchor_date`
- **THEN** the system returns `400 Bad Request` with `{"error":"anchor_date_required"}`

#### Scenario: Malformed anchor_date is rejected

- **WHEN** `anchor_date` is supplied but cannot be parsed as `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"anchor_date_invalid"}`

#### Scenario: Missing window_days is rejected

- **WHEN** the client omits `window_days`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_required"}`

#### Scenario: Malformed window_days is rejected

- **WHEN** `window_days` is supplied but cannot be parsed as an integer
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":2,"max":30}}`

#### Scenario: Invalid tz is rejected

- **WHEN** `tz` is supplied but cannot be parsed as an IANA timezone
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

### Requirement: Rolling-window numeric outputs rounded at the response boundary

The system SHALL round every numeric field in `averages`, every per-day `totals` field, and every adherence `actual` / `delta_pct` to one decimal place at the response boundary (matching the rounding rule used by `daily_summary` / `range_summary`).

#### Scenario: Average kcal rounds at the response boundary

- **WHEN** the underlying computation produces an average of `2280.4666...`
- **THEN** the response shows `2280.5`

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

### Requirement: Meal entries can be retrieved with recipe component expansion

The system SHALL accept `?expand=components` on `GET /meals/{id}`. When the entry's linked product is a recipe (`source = "recipe"`), the response includes a `components` array showing how the meal breaks down into the recipe's component products, scaled to the meal's `quantity_g`.

#### Scenario: Expanded retrieval shows scaled component breakdown

- **WHEN** the client calls `GET /meals/{id}?expand=components` for a meal entry whose product is the recipe "Morning skyr bowl" (skyr 200g + oats 40g + honey 10g, total 250g), logged with `quantity_g = 500`
- **THEN** the response includes `components: [{"product_id":"<skyr>","name":"Skyr","quantity_g":400}, {"product_id":"<oats>","name":"Oats","quantity_g":80}, {"product_id":"<honey>","name":"Honey","quantity_g":20}]`
- **AND** each entry reflects the recipe's component grams scaled by `meal.quantity_g / recipe.serving_size_g` (or `/100` if no serving_size_g is set)

#### Scenario: Expand on a non-recipe entry returns an empty components array

- **WHEN** the client calls `GET /meals/{id}?expand=components` for a meal entry whose product is not a recipe (or is null, i.e. freeform)
- **THEN** the response includes `components: []`

#### Scenario: Omitting expand returns the unmodified entry shape

- **WHEN** the client calls `GET /meals/{id}` without `expand=components`
- **THEN** the response does not include a `components` field

### Requirement: Meal entries may reference a workout

The system SHALL allow a meal entry to carry an optional `workout_id` referencing a row in `workouts`. The link is metadata; meal validation, nutriment computation, snapshot semantics, and CRUD behaviour are otherwise unchanged. When the referenced workout is deleted, the meal's `workout_id` is automatically cleared to NULL (the meal itself stays in the log).

#### Scenario: Meal entries table has a nullable workout_id column

- **WHEN** the migration set is applied
- **THEN** `meal_entries.workout_id` exists as `UUID NULL REFERENCES workouts(id) ON DELETE SET NULL`
- **AND** a partial index `meal_entries_workout_id_idx ON (workout_id) WHERE workout_id IS NOT NULL` exists

#### Scenario: POST /meals accepts workout_id

- **WHEN** the client posts a meal body that includes `"workout_id": "<uuid>"` referencing an existing workout
- **THEN** the system creates the meal with that link
- **AND** the response includes the `workout_id` field
- **AND** the standard `Idempotency-Key` semantics apply unchanged

#### Scenario: POST /meals/freeform accepts workout_id

- **WHEN** the client posts a freeform meal body that includes `workout_id`
- **THEN** the same persist + return behaviour as POST /meals applies

#### Scenario: workout_id omitted persists as null

- **WHEN** the client posts a meal without `workout_id`
- **THEN** the row is stored with `workout_id = NULL`
- **AND** the response omits the `workout_id` field (omitempty)
- **AND** existing-shape consumers see no change

#### Scenario: workout_id referencing an unknown workout is rejected

- **WHEN** the client posts a meal with a `workout_id` that does not exist in `workouts`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** no meal row is created

#### Scenario: PATCH sets workout_id to a new UUID

- **WHEN** the client patches an existing meal with `{"workout_id":"<uuid>"}` referencing an existing workout
- **THEN** the response shows the new link
- **AND** other fields are unchanged

#### Scenario: PATCH clears workout_id via empty string

- **WHEN** the client patches an existing meal with `{"workout_id":""}`
- **THEN** the response shows the meal with `workout_id` cleared (omitted from the JSON)
- **AND** the row's `workout_id` column is NULL

#### Scenario: PATCH omitting workout_id leaves the link unchanged

- **WHEN** the client patches an existing meal without the `workout_id` field in the body
- **THEN** the previously-stored `workout_id` value is preserved

#### Scenario: PATCH workout_id to a non-existent UUID is rejected

- **WHEN** the client patches with `{"workout_id":"<unknown-uuid>"}`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** the meal's existing `workout_id` is unchanged

#### Scenario: Deleting a referenced workout clears workout_id on its meals

- **WHEN** a meal exists with `workout_id = X` and the workout `X` is deleted via `DELETE /workouts/X`
- **THEN** the meal row's `workout_id` becomes NULL automatically (FK ON DELETE SET NULL)
- **AND** subsequent reads of the meal omit the `workout_id` field
- **AND** the meal's other fields are unchanged

#### Scenario: GET /meals/{id} returns workout_id when set

- **WHEN** a meal has `workout_id` populated and the client fetches it
- **THEN** the response includes the `workout_id` field

#### Scenario: GET /meals (list) returns workout_id when set per entry

- **WHEN** the client lists meals in a window
- **THEN** each meal in the response includes `workout_id` when set (omitted when null)
