# Design: add-meal-plan

## Context

priorities.md deliberately deferred a "meal planner" as agent-side composition. This change does NOT build that planner — it adds the minimal persistence the planner composes over: a selected dish for a date+slot with a lifecycle. The selection comes from the chat agent (or desktop agent); the eating moment comes from the companion app's Today screen. The user's stated contract: "I select what I want to eat and log only when I eat it."

## Goals / Non-Goals

**Goals:**
- A plan entry that references a real product (recipes included) so "ate it" needs zero re-entry.
- Honest history: meal entries only ever created at eating time, `logged_at = now`.
- Range reads for a 3-day horizon in one call.
- Same surface for both agents and the app (REST + MCP + outbox-compatible writes).

**Non-Goals:**
- No recurring plans, templates, or auto-planning.
- No freeform planned meals in v1 — `product_id` is required (the chat agent imports a Cookidoo recipe first, which is the flow that motivated this).
- No adherence/summary integration of *planned* (uneaten) entries.

## Decisions

### D1: Plan entries reference products, not embedded nutriments

`product_id` FK (RESTRICT delete — products capability already preserves history via its delete rules; verify interaction at implementation) plus optional `quantity_g` override. Alternative — snapshotting nutriments onto the plan row — rejected: plans are short-lived (days), and the product row is the single source the eaten transition logs against anyway.

### D2: `eaten` transition is a dedicated endpoint doing both writes in one transaction

`POST /plan/{id}/eaten` (optional body `{quantity_g?, logged_at?}` for corrections, `logged_at` defaulting to now and never in the future) creates the meal entry through the meals service and updates the plan row `status=planned → eaten`, storing the created `meal_entry_id` on the plan row. Both happen in one DB transaction — repos already accept `store.Querier` (pool or tx), so the mealplan service opens the tx and passes it to both repos. Alternatives rejected: (a) client makes two calls — the app outbox can't express atomicity, partial failure leaves a lie in one direction or the other; (b) plan PATCH with side effects — hidden writes behind a generic update verb.

### D3: Transitions are one-way and guarded

`planned → eaten` and `planned → skipped` only. Re-marking an `eaten` entry returns `409 plan_entry_already_eaten` (idempotency middleware handles the retry-replay case before this guard is ever hit; a genuine second tap is a conflict). `skipped → planned` is allowed (un-skip, e.g. plans changed back); `eaten` is terminal — undo means deleting the meal entry via the existing meals surface, which does NOT auto-revert the plan row (cross-capability cascade rejected for v1; the plan row keeps `meal_entry_id` so the agent can reconcile).

### D4: Slot uniqueness is NOT enforced

Two planned dinners on the same date are legal (options the user hasn't decided between, or two dishes in one meal). The chat UX may present one pick per slot, but the API doesn't encode that policy. Alternative — unique `(plan_date, slot)` — rejected as a UX policy masquerading as a data constraint.

### D5: Standard capability shape, two cross-injections

`internal/mealplan/{types,repo,service,handlers}_test.go` per the package template. `httpserver` wires `SetProductsRepo(...)` (FK validation on create/update, mirroring `mealsSvc.SetWorkoutsRepo`) and `SetMealsService(...)` (eaten transition). No import cycle: `mealplan` imports `meals`, never the reverse.

## Risks / Trade-offs

- [Plan rows go stale (yesterday's `planned` entries linger)] → range reads naturally scope by date; no auto-expiry in v1 — stale `planned` rows are honest data ("I planned it and didn't eat it"), useful to the coaching agent.
- [Eaten-then-meal-deleted leaves a dangling `meal_entry_id`] → accepted (D3); the column is informational, not FK-enforced, precisely so meals deletion stays untouched.
- [Required `product_id` blocks "plan something freeform"] → accepted for v1; the importer + freeform product creation cover the gap, and relaxing later is additive.

## Migration Plan

One migration: `planned_meals` table (id, plan_date date, slot text CHECK, product_id uuid FK, quantity_g numeric null, status text CHECK default 'planned', meal_entry_id uuid null, notes text null, created_at, updated_at). Down drops the table. Additive deploy, safe rollback.

## Open Questions

- None blocking. Whether the Today widget (Kotlin) should surface today's plan is deferred to `add-companion-chat`.
