# Decision: single-user by design — and how to keep the multi-tenant door open

**Status:** active · **Date:** 2026-06-10 · **Scope:** backend (and, by extension, every client)

## Decision

The system is **single-user by design**, and we will **not build multi-tenancy
now**. The eventual *option* of becoming a multi-tenant SaaS is preserved by a
small set of cheap habits (below), not by pre-built plumbing.

This is the backend-level companion to the app-level non-goal already recorded
in `openspec/changes/.../add-flutter-companion-app/design.md` ("Real OAuth,
multi-user, or per-user data isolation" — out of scope).

## Why not plumb it now

The ask that triggered this note was *"keep the door open for a future SaaS,"*
not *"build multi-user."* Treating it as an **option-value** question rather
than a feature request changes the answer. Break the retrofit into its parts
and ask, for each, whether *waiting makes it more expensive*:

| Retrofit cost            | Cheapest when paid       | Grows with time?               |
|--------------------------|--------------------------|--------------------------------|
| Schema: `owner_id` on every table | **LATER** — exactly one user to backfill (`UPDATE … SET owner = user_0`) | **No** — trivial until user #2 |
| Identity / sessions / signup      | build when needed — net-new code, not a retrofit | **No** — additive |
| Query scoping (`WHERE owner_id = …`) | ~constant per query, but you must touch every one; a miss = data leak | **Yes** — more endpoints = more queries |
| External contract breakage        | breaks other parties' clients | **Yes, if** a public API is ever published |

Two of the four (schema, identity) are cheap-later or purely additive, so
pre-building them for a single user is cost-for-nothing — and it would tax every
feature in the meantime. The other two (query scoping, contract surface) are the
only parts that *grow* with the app, so those are where deliberate habits pay off.

The counter-intuitive part: the schema retrofit people fear is *cheap precisely
because we're single-user at migration time* — there is exactly one existing
user to assign every row to.

## Guardrails (cheap now, preserve the option)

1. **Keep data access funneled through the per-package `repo.go` over
   `store.Querier`.** A future "scope every query by owner" pass is then
   mechanical, not archaeology. (Already the convention — just don't break it
   with ad-hoc queries scattered across handlers.)

2. **Stay Row-Level-Security-compatible.** If we go multi-tenant, Postgres RLS
   turns "audit 200 queries" into "set one session var per request" — the lever
   that makes query-scoping tractable. Staying compatible mostly means: don't
   write queries that *need* to span users, and don't fight a per-request
   user-context model. (`pgxpool` reuses connections, so RLS would use
   `set_config(..., is_local => true)` inside the request's transaction — a known
   pattern, noted here so it isn't a surprise later.)

3. **Never hang per-user state on a shareable entity.** The canonical example is
   already in the schema: `products.last_logged_quantity_g`. Products are the one
   entity that is genuinely *global* in a SaaS world (Open Food Facts data is
   identical for everyone), but `last_logged_quantity_g` is genuinely *per-user*.
   Today that fusion is correct (one user); at scale it forces the column out of
   `products` into a side table:

   ```
   single-user (today)          multi-tenant (someday)
   products                     products            ← stays GLOBAL (shared cache)
     …nutriments…                 …nutriments…
     last_logged_quantity_g  ──▶ user_product_stats ← the per-user bit
                                   (user_id, product_id, last_logged_quantity_g)
   ```

   **Convention:** when adding per-entity user state, check whether the entity is
   shareable; if so, put the state in a side table, not on the entity row.

4. **Keep clients first-party and replaceable.** Today the app, the MCP agent,
   and the Cookidoo extension are all ours, so single-user-shaped responses lock
   in nobody. Contract-surface risk stays ~0 until a public/third-party API is
   published — at which point single-user response shapes become hard to change.

## The seam map (where the eventual migration starts)

If/when an `add-multi-tenancy` change is opened, these are the load-bearing seams:

```
1. IDENTITY    auth middleware sets ClientID {mobile|agent} today — a *surface*,
               not a person. Multi-user: token/session ──▶ user_id.
2. ISOLATION   add owner_id to every owned table; scope every query (or RLS).
3. SHARED DATA products = global OFF cache; manual products / recipes = per-user;
               per-user usage stats split into a side table (guardrail #3).
4. AGENT ACT-AS one AGENT_TOKEN = "the user" today. Multi-user: per-athlete
               tokens, or an X-On-Behalf-Of header threaded through the MCP tools
               (which are 1-tool-=-1-HTTP-call, so every write tool gains athlete
               context).
5. SINGLETONS  goals (singleton + per-date overrides), training phases, templates
               become "one row per user" instead of "one row."
6. PAIRING     the companion QR hands out a shared token today; multi-user hands
               out a user-scoped token. The app's pairing model otherwise survives.
```

## Honest caveat

The SaaS option may be worth less than it feels. A personal triathlon-fueling
tool → competing with Cronometer / MyFitnessPal is a *product* leap (support,
GDPR, billing, churn), not just a technical one. That argues for the *cheapest
possible* guardrails (the four above) and nothing more — which is exactly this
decision. Revisit only when there is a concrete second user, not a hypothetical
market.
