# Tasks: add-meal-plan

## 1. Storage

- [x] 1.1 Check migration head, then `task migrate:new NAME=add_planned_meals` — `planned_meals` table per design (slot/status CHECKs, product FK, nullable meal_entry_id without FK)

## 2. Capability package

- [x] 2.1 `internal/mealplan/types.go` — PlannedMeal struct with JSON tags (`omitempty` nullables)
- [x] 2.2 `repo.go` — CRUD + range list against `store.Querier` (ordered date asc, slot order), product-name join for display
- [x] 2.3 `service.go` — validation (slot/status enums, required product_id via injected products repo, date parsing), sentinel errors incl. `ErrProductNotFound`, `ErrAlreadyEaten`, transition guards (planned→eaten/skipped, skipped→planned, eaten terminal)
- [x] 2.4 Eaten transition: single-tx flow opening tx in service, meals create through injected meals service, status flip + `meal_entry_id` store; effective-quantity resolution (body → plan → serving default); future `logged_at` rejection
- [x] 2.5 `handlers.go` — POST/GET/{id}/GET range/PATCH/DELETE `/plan` + `POST /plan/{id}/eaten`, swag annotations, `Register(rg)`

## 3. Wiring & tests

- [x] 3.1 Wire in `internal/httpserver`: instantiate repo/service, `SetProductsRepo(...)`, `SetMealsService(...)`, register routes behind auth + idempotency
- [x] 3.2 Handler integration tests: CRUD round-trip, range ordering + inclusivity, unknown product 404, double-eaten 409 (fresh idempotency key), atomicity (meals create failure leaves status planned), future logged_at 400, two dinners legal, eaten terminal, skipped→planned
- [x] 3.3 Verify idempotency replay on `POST /plan/{id}/eaten` returns the original response without a second meal entry

## 4. MCP & docs

- [x] 4.1 Register the five plan tools in `internal/mcpserver` with descriptions per spec; bump expected-tools list in `mcp_integration_test.go`
- [x] 4.2 `task swag`; `task vet` + `task test` green
