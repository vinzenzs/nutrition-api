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
