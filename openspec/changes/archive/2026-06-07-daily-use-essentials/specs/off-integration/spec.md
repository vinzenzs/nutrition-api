## MODIFIED Requirements

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

### Requirement: OFF client tests run against recorded JSON fixtures

The OFF client SHALL be testable without making live HTTP calls to Open Food Facts.

#### Scenario: Tests use recorded fixtures from testdata/off/

- **WHEN** the OFF client test suite runs
- **THEN** the client is wired to read fixtures from `testdata/off/<barcode>.json`
- **AND** the fixtures cover at minimum: a fully-populated product (macros + every supported micro), a product with missing nutriments, a product with macros only and no micros, a product with only kJ energy, a product with unparseable serving_size, and a `status: 0` not-found response
