# Design: add-coach-methodology

## Context

The methodology prose currently cohabits `Plan.md` with the schedule, but only the
schedule wants to move to the server. The prose stays in the vault as the
human-facing, Obsidian-rendered, git-tracked, cited reference. This change gives
the coach a *read copy* on the server: the server holds Markdown text the LLM reads
directly, the vault stays the source of truth, and a vault-side compile pushes the
text in (same source→disposable-artifact pattern as the schedule, but a verbatim
copy rather than a lossy parse). The only seam needed already exists:
`/context/training` resolves the training-phase covering the anchor date, so
attaching the per-phase "Why" to that phase makes it ride the coach's grounding
call for free.

## Goals / Non-Goals

**Goals:**
- Give Kazper the cited per-phase "Why" in the grounding bundle it already reads.
- Give it the plan-level Key Principles / Rowing Strategy via a read it can fetch.
- Keep the vault the source of truth; the server fields are a pushed, disposable
  copy.
- Store Markdown verbatim — the LLM's ideal format — with zero retrieval machinery.

**Non-Goals:**
- No standalone coach-knowledge store (tables/endpoints/tags). Methodology is about
  *this plan and these phases*; generalize only when a second reference type exists.
- No embeddings/RAG. Single-user, few phases; deterministic covering-phase surfacing
  is enough.
- No covering-plan-by-date resolution for the bundle (see D3 + Open Questions).
- No vault-side compile here (it pushes the text; out of repo).

## Decisions

### D1: `methodology` is a column distinct from `notes`

Both `training_phases` and `training_plans` already have a free-text `notes`.
Methodology gets its **own** nullable `TEXT` column rather than reusing `notes`:

```
ALTER TABLE training_phases ADD COLUMN methodology TEXT NULL;
ALTER TABLE training_plans  ADD COLUMN methodology TEXT NULL;
```

`notes` is operational scratch the athlete hand-edits on the server ("moved the
bike to Thursday this week"); `methodology` is curated, cited reference **owned by
the vault** and overwritten wholesale by the compile push. Keeping them separate
means the push can replace methodology without clobbering operational notes, and
the coach can be fed reference prose without operational noise. No length cap
beyond Postgres TEXT; content is small and single-user.

### D2: Phase methodology surfaces through the existing bundle

`/context/training` already returns a `PhaseLite` projection of the covering phase
(name, type, dates). Add `methodology` to `PhaseLite` and populate it from the
resolved phase. This is the whole high-frequency win: every time the coach grounds
training advice, the current phase's cited reasoning is already in hand — no extra
fetch, no tool the coach has to remember to call. `PhaseLite` is a projection, so
the field is added explicitly (not inherited from the `Phase` row). A phase with
null methodology serializes the field as null.

### D3: Plan methodology lives on the plan row, read on demand

Plan-level content (Key Principles, Rowing Strategy) is not phase-specific and not
date-keyed — it's stable philosophy. It attaches to `training_plans.methodology`
and is returned by the existing `GET /training-plans/{id}` (and the nested tree),
which the coach reaches via `get_training_plan`. It is **not** injected into
`/context/training`, because the bundle is date-keyed and plans have no explicit
end column — resolving "the plan covering today" means computing each plan's span
from `start_date + max(week ordinal)`, an extra query for content that doesn't
change day to day. The coach fetches it once when it needs the philosophy, not
every turn. (Open Questions revisits this if per-turn access proves necessary.)

### D4: Markdown stored verbatim; the coach reads it raw

The fields hold Markdown exactly as authored in the vault — tables, citation
links, headings. The LLM tokenizes Markdown natively, so no server-side rendering,
sanitization, or transformation. The "cramped flat text" concern that kept this in
the vault was about a *human* viewer on the server; the coach is not that viewer.
The human keeps reading the rendered source in Obsidian.

### D5: Write paths widen, no new endpoints or tools

- `PATCH /training-plans/{id}` accepts `methodology` (nullable; supplying it sets,
  `null`/omit follows the path's existing patch semantics for nullable text).
- The phase create/update path accepts `methodology` the same way.
- MCP `patch_training_plan` and the phase-write tool carry `methodology` in their
  payloads. No new tools; the expected-tools list is unchanged.

The vault compile uses these existing paths to push the text — the server just
exposes the fields. (Push direction and cadence are vault-side, out of repo.)

## Risks / Trade-offs

- **Plan-level methodology is a fetch, not ambient.** If the coach reliably needs
  Key Principles every turn and doesn't fetch the plan, those principles won't
  ground its advice. Mitigated by phase methodology (the higher-value, per-phase
  reasoning) being ambient in the bundle; promote plan-level into the bundle later
  if usage shows the gap.
- **Two sources for the same prose (vault + server copy) can drift.** The server
  copy is disposable and overwritten by the push; the vault is source. Drift is
  resolved by re-pushing, exactly like the schedule artifact. Acceptable.
- **No general store yet** means a future second reference type needs its own
  decision. Deliberate — avoid building a knowledge base for one document.

## Migration Plan

Two additive `ALTER … ADD COLUMN methodology TEXT NULL` statements (one per table),
no backfill (NULL = today's behavior; bundle field serializes null). Down migration
drops both columns. Verify the migration head on disk before scaffolding and take
the next free number (CLAUDE.md warns numbering has been claimed out-of-band).

## Open Questions

- **Should plan-level methodology also enter `/context/training`?** Leaning no for
  now (D3). If it should, the cheapest resolution is "the most recent plan with
  `start_date <= anchor_date`" rather than a full span computation — single-user
  means one active plan in practice. Revisit after the coach has used phase
  methodology for a while.
- **One methodology field per plan, or per plan-week too?** The per-week `notes`
  exist, but per-week methodology seems redundant with per-phase. Holding at
  plan + phase; add per-week only if a week needs reasoning its phase doesn't carry.
