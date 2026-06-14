# Proposal: add-coach-methodology

## Why

The hand-curated methodology that lives alongside the schedule in `Plan.md` — Key
Principles, the per-phase "Why" blocks with their training-science citations
(Seiler, San Millán, Friel, Coggan/Allen), the Rowing Strategy table, the phase
narratives — has no home on the server. When the schedule is retired into the
server `training-plan` (the `add-slot-duration-override` track and the vault-side
migration), that prose stays in the vault as the human-facing reference. But the
in-app coach Kazper should be able to *read* it when it grounds advice: the "why"
behind a Base or Build block is exactly the context that makes a recommendation
specific instead of generic. Today it can't — the coach's grounding bundle
(`/context/training`) hands it the covering phase but nothing about the reasoning
that shaped it.

The "flat server text is cramped" objection that kept this prose in the vault was
a *human-rendering* concern. Kazper is an LLM; it reads Markdown — citations,
tables, headings — natively, so storing the methodology as Markdown purely for the
coach to read is not a compromise but the ideal format.

## What Changes

- **Per-phase methodology on `training_phases`**: a nullable `methodology` Markdown
  text column, distinct from the existing operational `notes`. It holds the cited
  "Why this phase" narrative. Because `/context/training` already resolves the
  training-phase covering the anchor date, this surfaces to the coach **for free**
  in the bundle it already reads.
- **Plan-level methodology on `training_plans`**: a nullable `methodology` Markdown
  text column for the cross-cutting content that isn't phase-specific (Key
  Principles, Rowing Strategy). Returned by the existing `GET /training-plans/{id}`
  the coach can already fetch.
- **`/context/training` surfaces phase methodology**: the bundle's `PhaseLite`
  projection gains a `methodology` field carrying the covering phase's prose, so
  both AI surfaces (MCP coach + in-app chat coach, which share this bundle) get the
  current phase's reasoning whenever they ground training advice.
- **Write surface widens, no new endpoints**: the phase create/update path and
  `PATCH /training-plans/{id}` accept `methodology`; the existing `patch_training_plan`
  and phase-write MCP tools carry it. No new tools — expected-tools list unchanged.
  (The verbatim push from the vault Markdown into these fields is the vault-side
  compile, out of this repo — like the schedule data-load.)
- **Migration**: `ALTER training_phases ADD methodology TEXT NULL` and
  `ALTER training_plans ADD methodology TEXT NULL`.

## Capabilities

### New Capabilities

<!-- None. Extends existing capabilities. -->

### Modified Capabilities

- `training-phases`: a phase carries an optional `methodology` Markdown text,
  distinct from `notes`, accepted on create/update and returned on read.
- `training-plan`: a plan carries an optional `methodology` Markdown text, distinct
  from `notes`, accepted on `PATCH` and returned on `GET`.
- `coach-context`: the `/context/training` bundle's phase slice surfaces the
  covering phase's `methodology`.

<!-- MCP payload widening (patch_training_plan / phase-write carry methodology) is
     captured as scenarios inside the training-plan and training-phases deltas,
     mirroring add-plan-slot-targets — not a separate mcp-server delta. -->

## Impact

- **Depends on** `add-coach-context-endpoints` (the `/context/training` bundle +
  `PhaseLite`), `training-phases`, and `training-plan` — all implemented.
- **Independent of** `add-slot-duration-override`: that change touches slots and
  materialize; this touches phase/plan text columns and the context bundle. No file
  overlap; either can land first.
- **New code**: two nullable text columns (migration), widened phase/plan
  write+read paths, the `PhaseLite.methodology` field + its population in the
  context builder, widened MCP payloads. `task swag` after handler/struct changes.
- **No breaking changes**: additive; a phase/plan with null `methodology` behaves
  exactly as today and the bundle field serializes as null.
- **Deliberately out of scope**:
  - A standalone "coach knowledge base" / general reference store with its own
    tables, endpoints, and tags. Methodology is genuinely *about this plan and
    these phases* today; promote to a general store only when a second kind of
    reference (race-day fueling, etc.) actually appears.
  - Injecting **plan-level** methodology into `/context/training` (would need a
    covering-plan-by-date-span resolution; plans have no explicit end column). The
    coach reaches plan-level prose via `get_training_plan`. Revisit if per-turn
    access to Key Principles proves necessary. (See design Open Questions.)
  - The vault-side compile that pushes Markdown into these fields, and the
    `Plan.md` → `Methodology.md` split — both vault-repo work.
  - Retrieval machinery (embeddings / RAG): single-user, a handful of phases;
    surfacing the covering phase deterministically is sufficient.
