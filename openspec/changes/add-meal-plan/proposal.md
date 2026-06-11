# Proposal: add-meal-plan

## Why

The meal-recommendation flow ("what should I eat today / the next 3 days?") needs somewhere to put a *selection* that is not yet an *event*. A meal entry in this system is logged, past-tense — picking tonight's dinner at 9am is a plan, and logging it future-dated would lie in the meal history and corrupt adherence math. There is currently no primitive between "recommended" and "eaten".

## What Changes

- New `meal-plan` capability: a planned meal is `{plan_date, slot, product_id, quantity_g?, status, notes?}` with `slot ∈ {breakfast, lunch, dinner, snack}` and `status ∈ {planned, eaten, skipped}`.
- CRUD over planned meals plus a range read (`GET /plan?from&to`) for the ~3-day horizon.
- A one-tap transition `POST /plan/{id}/eaten` that atomically creates a real meal entry (logged now, via the existing meals capability) and marks the plan entry `eaten` — this is the only way plan data enters the meal history.
- MCP tools mirroring the new endpoints so the desktop agent shares the same plan.

## Capabilities

### New Capabilities

- `meal-plan`: planned meals with a `planned → eaten|skipped` lifecycle; eaten emits a real meal entry.

### Modified Capabilities

- `mcp-server`: new plan tools (`create_planned_meal`, `list_planned_meals`, `update_planned_meal`, `delete_planned_meal`, `mark_planned_meal_eaten`), each issuing exactly one HTTP call.

## Impact

- **DB**: one migration creating `planned_meals` (FK → `products`, no FK into `meal_entries` — the link is recorded on the plan row after transition).
- **Code**: new `internal/mealplan` package (standard capability shape); `internal/httpserver` wiring with cross-injection of the products repo (FK validation) and the meals service (eaten transition), following the existing `mealsSvc.SetWorkoutsRepo(...)` precedent; `internal/mcpserver` tools + expected-tools bump.
- **Docs**: `task swag`.
- **Unaffected**: `meals` requirements are unchanged — the transition calls the existing create path; `summary`/adherence only ever see real meal entries.
- **Sequencing**: independent of `add-recipe-ingredients`; consumed by `add-chat-backend` and `add-companion-chat`.
