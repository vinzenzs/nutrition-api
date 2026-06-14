## Context

`nutrition-api` is a Go + Gin + Postgres backend with satellites (`apps/companion` Flutter app, `apps/garmin-bridge`, `extensions/cookidoo`) and a Helm chart. The product is being rebranded to **Kazper**, which is also the name of the in-app AI coach (the persona shipped generically in `expand-chat-to-coach` phase 3). The user chose a **rebrand-only** scope: the Go module path (`github.com/vinzenzs/nutrition-api`, imported by 186 files) stays; only user-facing identity and build/deploy names change. Captured as an OpenSpec change, sequenced after the in-flight chat-coach + registry-unification work archives.

## Goals / Non-Goals

**Goals:**
- The AI coach calls itself Kazper; the REST API and MCP server identify as Kazper.
- Binary, entrypoint dir, Taskfile targets, Helm chart, docs, and satellite app names say Kazper.
- Zero churn to internal package structure, imports, DB, or env contracts.

**Non-Goals:**
- No Go module-path rename (and therefore no import sweep).
- No Postgres database rename, no env-var renames (unprefixed today), no OpenSpec capability-dir renames.
- No GitHub repo rename or local working-dir rename (outward-facing; the user does these separately).
- No behavior change beyond identity strings (the coach's *behavior* was set by `expand-chat-to-coach`; this only names it).

## Decisions

### DD1 — Rebrand-only: module path is untouched
Keep `github.com/vinzenzs/nutrition-api`. The module path is an internal coordinate; renaming it would touch 186 files and the GitHub URL for no user-visible gain. The product name (Kazper) and the import path can legitimately differ — many projects ship a binary whose name ≠ its module path. This keeps the change small and low-risk.

### DD2 — `cmd/nutrition-api/` → `cmd/kazper/`; binary → `kazper`
The Cobra entrypoint dir is a `main` package (no importers), so renaming it is safe. Build output becomes `bin/kazper`; `task install` copies to `~/.local/bin/kazper` (and re-signs for macOS as today). Update Taskfile `build`/`install`/`dev` targets and any `./cmd/nutrition-api` references in workflows/Dockerfile.

### DD3 — Identity strings: coach, API title, MCP server name
Three user-facing identity points change:
- **Coach** — `internal/chat/prompt.go`: the system prompt names the assistant "Kazper" (e.g. *"You are Kazper, <user>'s endurance-fueling and training coach…"*). This edits the persona requirement that `expand-chat-to-coach` introduces, so it must land after that archives (DD7).
- **REST API** — swag `@title "Nutrition API"` → `"Kazper"`; run `task swag` to regenerate `docs/`.
- **MCP server** — `internal/mcpserver/server.go` announced name `"nutrition"` → `"kazper"` (shows in the agent's server list). Bump `mcp_integration_test.go` if it asserts the server name.

### DD4 — Helm chart rename is a redeploy, not an upgrade
Renaming `deploy/helm/nutrition-api/` → `deploy/helm/kazper/` and `Chart.yaml` `name: nutrition-api` → `kazper` changes the Helm **release identity**: a `helm upgrade` won't match the old release; it's effectively a new install (new resource names, possibly a new Ingress host). Single-user, pre-release, so acceptable — but call it out: the deploy step is "install kazper, delete the old nutrition-api release," not an in-place rename. Keep the **Postgres database name unchanged** so no data migration is implied (the chart points at the same DB).

### DD5 — Keep internal contracts: DB name, env vars, capability dirs
- **DB name** stays (renaming a live database is a stateful migration with zero user benefit; it's invisible).
- **Env vars** stay (no prefix today; `CHAT_*`, `SYNC_*`, `ANTHROPIC_API_KEY` are descriptive, not branded).
- **OpenSpec capability spec dirs** (`nutrition-chat`, `mobile-companion`, …) stay — they're internal organizational names, not user-facing; renaming them is high-churn, low-value, and would rewrite cross-references across the archive.

### DD6 — Satellites rebrand to their own surfaces
`apps/companion`: `pubspec.yaml` `name:` (Dart package id — internal, but conventionally matches) and the Android display label / app title → Kazper. `apps/garmin-bridge` + `extensions/cookidoo/manifest.json`: user-facing names/strings → Kazper. These are independent of the Go rename and can be done in the same sweep.

### DD7 — Sequence after the in-flight arc
Land after `expand-chat-to-coach` and `unify-mcp-tool-registry` archive. Rationale: the coach persona (DD3) and the agenttools/MCP surface are being actively rewritten by those changes; naming Kazper on top of a moving target would create conflicts in both the code and the proposals' text. Once they archive, the persona requirement exists in `openspec/specs/nutrition-chat/` and the rebrand layers cleanly.

### DD8 — GitHub repo + working-dir rename are the user's, out of scope
Renaming the GitHub repo (`gh repo rename kazper`) and the local dir breaks clone URLs, remotes, and any absolute tooling paths (including this session's working dir and the memory directory). These are outward-facing and the user's call; this change documents them as a manual follow-up, not an automated task.

## Risks / Trade-offs

- **[Helm release discontinuity]** DD4 means a redeploy, not an upgrade. Mitigation: explicit deploy runbook step; DB untouched so no data risk.
- **[Module-path/product-name mismatch confuses newcomers]** The repo imports `nutrition-api` but the product is Kazper. Mitigation: a one-line note in CLAUDE.md/README explaining the deliberate split.
- **[Sequencing drift]** If the rebrand is done before the arc archives, the coach-persona edit conflicts. Mitigation: DD7 — gate on archive; until then this change sits proposed.
- **[Stray references]** 353 Go files + docs + build configs; some "nutrition" strings are domain-correct (it *is* a nutrition app) and must NOT be blindly replaced. Mitigation: tasks target specific files/symbols, not a global `s/nutrition/kazper/`.

## Migration Plan

1. Gate on `expand-chat-to-coach` + `unify-mcp-tool-registry` archived (DD7).
2. Code identity: `cmd/` dir + binary (DD2), coach prompt + API title + MCP name (DD3), `task swag`.
3. Build/deploy: Taskfile, Dockerfile, workflows, Helm chart rename (DD4).
4. Satellites: companion, garmin-bridge, cookidoo (DD6).
5. Docs: README, RUN_LOCAL, CLAUDE.md (incl. the module-path/product-name note).
6. `task test` + `task vet` + `task build`/`install` smoke under the new binary name.
7. Hand off the GitHub repo + working-dir rename to the user (DD8).
