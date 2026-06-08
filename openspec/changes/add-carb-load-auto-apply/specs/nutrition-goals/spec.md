## ADDED Requirements

### Requirement: Goal overrides support partial-update merge writes

The system SHALL provide a partial-update write path on the daily goal overrides store, used by the race-prep carb-load apply step (and reserved for future similar planner-then-apply primitives). The partial-update path SHALL take a date and a `Goals` patch object, and SHALL overlay ONLY the non-null fields of the patch onto the existing override row for that date — every field that is `null` on the patch leaves the existing row's value unchanged. If no override exists for the date, the patch creates a new override row containing only the patch's non-null fields. The patch never sets a field to `null` — to clear a field, the caller uses the existing full-replace `PUT /goals/overrides/{date}` path.

The partial-update path is NOT exposed as a public REST verb in this change — it is a repository-level capability used by the race-prep apply endpoint. Adding a public `PATCH /goals/overrides/{date}` verb is out of scope; if a second consumer surfaces later, that verb can be added without changing this requirement.

The partial-update path SHALL share the same validation rules as the full-replace path (negative/NaN values rejected, inverted min/max rejected, empty `{}` range rejected, legacy `kcal_target` rejected).

#### Scenario: Partial update preserves existing non-patched fields

- **WHEN** an override exists for `2026-07-22` with `{kcal: {min: 2090, max: 2310}, protein_g: {min: 150, max: 190}}`
- **AND** the partial-update path is invoked with date=`2026-07-22` and patch=`{carbs_g: {min: 700}}`
- **THEN** the row's stored bounds are `{kcal: {min: 2090, max: 2310}, protein_g: {min: 150, max: 190}, carbs_g: {min: 700}}`
- **AND** all other nutrient fields remain null
- **AND** subsequent `GET /goals/overrides/2026-07-22` returns those three populated targets

#### Scenario: Partial update replaces an existing value for the patched field

- **WHEN** an override has `{carbs_g: {min: 500, max: 600}, kcal: {min: 2200}}`
- **AND** the partial-update path patches `{carbs_g: {min: 700}}`
- **THEN** the row's `carbs_g` becomes `{min: 700}` (the new value, min-only — no max)
- **AND** `kcal` is unchanged at `{min: 2200}`

#### Scenario: Partial update on a date with no existing override creates a new row

- **WHEN** no override exists for `2026-07-22`
- **AND** the partial-update path patches `{carbs_g: {min: 700}}`
- **THEN** a new override row is created on `2026-07-22` with `carbs_g_min = 700` and every other nutrient bound null
- **AND** subsequent `GET /goals/overrides/2026-07-22` returns `{carbs_g: {min: 700}}`

#### Scenario: Partial update never clears a field

- **WHEN** an override has `{kcal: {min: 2200}, carbs_g: {min: 500}}`
- **AND** the partial-update path patches `{carbs_g: {min: 700}}` (no `kcal` field on the patch)
- **THEN** `kcal` is still `{min: 2200}`
- **AND** clearing `kcal` requires a full-replace `PUT /goals/overrides/{date}` without the `kcal` field

#### Scenario: Partial update validates patched fields

- **WHEN** the partial-update path receives a patch containing `{carbs_g: {min: -50}}`
- **THEN** the underlying error is the same `goal_value_invalid` surfaced by the full-replace path
- **AND** no row is modified

- **WHEN** the patch contains `{carbs_g: {min: 800, max: 700}}` (inverted)
- **THEN** the underlying error is `goal_range_invalid` on `carbs_g`

- **WHEN** the patch contains `{carbs_g: {}}` (empty range object)
- **THEN** the underlying error is `goal_value_invalid` on `carbs_g`

#### Scenario: Partial update is the only writer used by race-prep apply

- **WHEN** `POST /race-prep/carb-load/apply` is invoked for a 4-day schedule
- **THEN** for each scheduled date the apply step invokes the partial-update path (not the full-replace path)
- **AND** the race-prep apply step never writes any field other than `carbs_g` via this mechanism

#### Scenario: Partial update is transaction-aware

- **WHEN** the race-prep apply step opens a `pgx.Tx` and constructs the override store over that transaction's querier
- **THEN** all N partial-update calls inside the loop participate in the same transaction
- **AND** a rollback of the transaction leaves zero rows persisted or modified
- **AND** a commit persists all N writes atomically
