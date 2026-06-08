# off-integration Specification

## Purpose

Define how the nutrition API integrates with Open Food Facts (OFF) for barcode lookups, including parsing, caching, error mapping, and testing.

## Requirements

### Requirement: Open Food Facts client uses the v2 product endpoint with a polite User-Agent

The Open Food Facts client SHALL fetch products from `https://world.openfoodfacts.org/api/v2/product/{barcode}.json` with a `User-Agent` header that identifies the application name, version, and contact.

#### Scenario: User-Agent is set on every OFF request

- **WHEN** the client issues any HTTP request to Open Food Facts
- **THEN** the request includes a `User-Agent` header of the form `nutrition-api/<version> (+<contact-url-or-email>)`
- **AND** the contact value is sourced from the `OFF_USER_AGENT_CONTACT` env var, falling back to a hardcoded default if unset

#### Scenario: Request times out after the configured timeout

- **WHEN** the OFF endpoint does not respond within `OFF_TIMEOUT_SECONDS` (default 5)
- **THEN** the client cancels the request and returns a timeout error to the caller

### Requirement: Successful OFF response is parsed into typed columns and the raw payload is retained

The client SHALL parse the fields it cares about into typed values AND store the complete OFF response JSON unchanged for future re-extraction. The "fields it cares about" include both macros and the micronutrients introduced by the daily-use-essentials change.

#### Scenario: Well-formed product parses all known nutriments

- **WHEN** the OFF response has `status: 1` and a `product.nutriments` object containing macro fields (`energy-kcal_100g`, `proteins_100g`, `carbohydrates_100g`, `fat_100g`, `fiber_100g`, `sugars_100g`, `salt_100g`) and any subset of micro fields (`iron_100g`, `calcium_100g`, `vitamin-d_100g`, `vitamin-b12_100g`, `vitamin-c_100g`, `magnesium_100g`, `potassium_100g`, `zinc_100g`)
- **THEN** the client extracts each present value into the corresponding typed column on the product
- **AND** stores the entire response body verbatim in `off_payload` as JSONB

#### Scenario: kcal is derived from kJ when only energy_100g is present

- **WHEN** the OFF response contains `energy_100g` (kJ) but no `energy-kcal_100g`
- **THEN** the client computes kcal as `energy_100g / 4.184` rounded to one decimal place
- **AND** stores the result in `kcal_per_100g`

#### Scenario: Missing nutriment fields are stored as null, not zero

- **WHEN** any individual macro or micro nutriment field is absent from the OFF response
- **THEN** the corresponding typed column on the product is `null`
- **AND** no error is raised

#### Scenario: Micro units are interpreted per OFF's documented units

- **WHEN** the OFF response contains `iron_100g`, `calcium_100g`, `vitamin-c_100g`, `magnesium_100g`, `potassium_100g`, or `zinc_100g`
- **THEN** the client stores the value as-is (OFF reports these in milligrams per 100g)
- **AND** when the response contains `vitamin-d_100g` or `vitamin-b12_100g`, the client stores the value as-is (OFF reports these in micrograms per 100g)

#### Scenario: Parseable serving_size is converted to grams

- **WHEN** `product.serving_size` is a string like `"30g"`, `"125 g"`, or `"  30 grams"`
- **THEN** the client extracts the numeric value into `serving_size_g`

#### Scenario: Unparseable serving_size is tolerated

- **WHEN** `product.serving_size` is a string the client cannot parse (e.g. `"≈ 2 slices"`, `"1 cup (240ml)"`, empty)
- **THEN** the client leaves `serving_size_g` null
- **AND** records a structured log line at WARN level with the offending value
- **AND** does not fail the lookup

### Requirement: Unknown barcode response is mapped to a 404 with an actionable next step

When Open Food Facts returns `status: 0` for a barcode, the system SHALL surface this to the calling client as a `404` with a body that tells an LLM agent which endpoint to try next.

#### Scenario: status:0 produces actionable 404

- **WHEN** the OFF response body is `{"status": 0, "status_verbose": "product not found", ...}`
- **THEN** the API returns `404 Not Found` with body `{"error":"product_not_found","barcode":"<barcode>","next":"POST /meals/freeform"}`
- **AND** no product row is created

#### Scenario: Not-found responses are not cached

- **WHEN** OFF returns `status: 0` for a barcode
- **THEN** the next lookup for the same barcode re-queries OFF (does not return a stale not-found from cache)

### Requirement: Upstream timeouts and 5xx errors are surfaced as 504

When the Open Food Facts request times out or returns a 5xx, the system SHALL return `504 Gateway Timeout` and not persist any product row.

#### Scenario: OFF timeout returns 504 with retry hint

- **WHEN** the OFF request exceeds the configured timeout
- **THEN** the API returns `504 Gateway Timeout` with body `{"error":"upstream_timeout","retry_after_seconds":30}`

#### Scenario: OFF 5xx returns 504 with retry hint

- **WHEN** OFF returns a 500-range status
- **THEN** the API returns `504 Gateway Timeout` with body `{"error":"upstream_error","retry_after_seconds":30}`

#### Scenario: Failures are not cached as products

- **WHEN** any failure mode occurs on a fresh lookup
- **THEN** no `products` row is inserted
- **AND** the next lookup re-attempts the OFF call

### Requirement: OFF 4xx other than 404 are surfaced as 502

When Open Food Facts returns a 4xx that is not the documented `status: 0` not-found, the system SHALL return `502 Bad Gateway`.

#### Scenario: Unexpected 4xx from OFF

- **WHEN** OFF returns `400`, `403`, or any unexpected 4xx
- **THEN** the API returns `502 Bad Gateway` with body `{"error":"upstream_unexpected_response","status":<n>}`

### Requirement: Raw OFF payload is preserved for re-extraction

The system SHALL persist the entire Open Food Facts response body in `products.off_payload` as JSONB on every successful fetch.

#### Scenario: Raw payload is stored on initial fetch

- **WHEN** a barcode is fetched for the first time
- **THEN** `off_payload` contains the unmodified OFF response body

#### Scenario: Raw payload is updated on refresh

- **WHEN** a barcode is re-fetched via `?refresh=true`
- **THEN** `off_payload` is overwritten with the new response body
- **AND** `fetched_at` is updated

#### Scenario: Raw payload is not stored on failure

- **WHEN** a lookup fails (timeout, 5xx, status:0)
- **THEN** no product row is created and no payload is stored

### Requirement: OFF client tests run against recorded JSON fixtures

The OFF client SHALL be testable without making live HTTP calls to Open Food Facts.

#### Scenario: Tests use recorded fixtures from testdata/off/

- **WHEN** the OFF client test suite runs
- **THEN** the client is wired to read fixtures from `testdata/off/<barcode>.json`
- **AND** the fixtures cover at minimum: a fully-populated product (macros + every supported micro), a product with missing nutriments, a product with macros only and no micros, a product with only kJ energy, a product with unparseable serving_size, and a `status: 0` not-found response
