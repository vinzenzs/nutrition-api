## MODIFIED Requirements

### Requirement: Write tools auto-derive idempotency keys when none is supplied

The system SHALL set the `Idempotency-Key` HTTP header on every POST-style write tool call (`log_meal`, `log_meal_freeform`, `patch_meal`, `delete_meal`, `create_recipe`, `recompute_recipe`). When the tool input contains an explicit `idempotency_key`, that value SHALL be used; otherwise the wrapper SHALL compute a stable key as `sha256_hex(<tool_name> + "|" + canonical_json(<tool_args_without_idempotency_key>))`. PUT-style write tools (`set_goals` today, plus any future PUT-backed tool) SHALL NOT expose an `idempotency_key` field in their input schema and SHALL NOT set the `Idempotency-Key` header on the backend request; the backend rejects the header on PUT with `400 idempotency_unsupported_for_put` regardless.

#### Scenario: Explicit idempotency_key is forwarded verbatim

- **WHEN** the agent calls `log_meal_freeform` with `{"name":"X","quantity_g":100,"logged_at":"…","idempotency_key":"abc-123"}`
- **THEN** the REST request carries `Idempotency-Key: abc-123`

#### Scenario: Missing idempotency_key is derived from arguments

- **WHEN** the agent calls `log_meal_freeform` twice in a row with byte-identical other arguments and no `idempotency_key`
- **THEN** both REST requests carry the same auto-derived `Idempotency-Key` header
- **AND** the second call returns the same meal id as the first

#### Scenario: Different arguments produce different auto-derived keys

- **WHEN** the agent calls `log_meal_freeform` twice with the same name but different `quantity_g`
- **THEN** the two REST requests carry different `Idempotency-Key` headers
- **AND** both meals are persisted independently

#### Scenario: Derivation excludes the idempotency_key field from the canonical form

- **WHEN** the agent calls `log_meal_freeform` once with no `idempotency_key` and once with the same args plus an explicit `idempotency_key`
- **THEN** the two REST requests carry different `Idempotency-Key` values (the explicit one wins; the auto-derived one is not used)

#### Scenario: set_goals does not expose idempotency_key

- **WHEN** the agent inspects the `set_goals` tool input schema
- **THEN** the schema does NOT include an `idempotency_key` property

#### Scenario: set_goals does not send Idempotency-Key

- **WHEN** the agent calls `set_goals` with any input
- **THEN** the wrapper issues the `PUT /goals` request without an `Idempotency-Key` header
- **AND** the wrapper does NOT auto-derive a key for this tool

#### Scenario: A PUT-style tool's description points users at retry-safety

- **WHEN** the agent reads the `set_goals` tool description
- **THEN** the description notes that retries of `set_goals` may land twice on transient network failure
- **AND** points future work at ETag/If-Match optimistic concurrency
