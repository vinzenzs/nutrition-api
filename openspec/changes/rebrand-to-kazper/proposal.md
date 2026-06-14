## Why

The project name `nutrition-api` describes a backend, not the product. The product is **Kazper** — an endurance-fueling and training coach that the app embodies. The in-app coach persona (shipped generically in `expand-chat-to-coach` phase 3) should *be* Kazper: one identity shared by the product and the assistant, matching the direction that arc already set (one coach, embodied by the app). This change rebrands the user-facing identity to Kazper without disturbing the codebase's internal plumbing.

**The name.** Kazper (Casper → Kazper) reads as a *name*, not a category: trademark-clean, no mattress-brand collision, ownable via the K/Z spelling. It carries two intended readings — **Casper**, the friendly, ever-present companion (the tone: warm, ambient, non-judgmental), and **Kaspar the Magus**, the wise guide who follows a star bearing gifts (the arc: guiding the athlete over a months-long journey to a destination race).

## What Changes

- **Scope: rebrand only — the Go module path stays `github.com/vinzenzs/nutrition-api`.** No 186-file import sweep. The rename is user-facing identity + build/deploy names, not internal package structure.
- **The AI coach is named Kazper.** `internal/chat`'s system prompt names the assistant Kazper (a follow-on edit to the persona shipped in `expand-chat-to-coach` phase 3).
- **Binary + entrypoint:** `bin/nutrition-api` → `bin/kazper`; `cmd/nutrition-api/` → `cmd/kazper/`; Taskfile `build`/`install` targets and the installed `~/.local/bin/` name.
- **API + MCP identity:** the REST API swag `@title "Nutrition API"` → "Kazper"; the MCP server announced name `"nutrition"` → `"kazper"` (visible in the agent's server list).
- **Deploy:** Helm chart `deploy/helm/nutrition-api/` → `deploy/helm/kazper/` + `Chart.yaml` name (a redeploy under the new release name, not an in-place upgrade — see design).
- **Companion + satellites:** the Flutter app display name + `pubspec.yaml` name, the `garmin-bridge` and `cookidoo` extension naming/strings.
- **Docs:** README, RUN_LOCAL, CLAUDE.md, and other prose referring to "nutrition-api"/"Nutrition API".
- **Explicitly kept (internal, low-value-to-churn):** the Go module path, the Postgres database name, env var names (unprefixed today — `CHAT_*`, `ANTHROPIC_API_KEY`, …), and the OpenSpec capability spec directory names (`nutrition-chat`, etc.).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `nutrition-chat`: the assistant identifies itself as **Kazper** (a named persona). Additive — layers on top of the coaching persona from `expand-chat-to-coach`.
- `mcp-server`: the server's announced identity is **kazper** (and the REST API title is Kazper) rather than "nutrition".

## Impact

- **Code:** `cmd/` dir rename (main package — no importers); `internal/chat/prompt.go` (coach name); swag `@title` + `task swag` regen; `internal/mcpserver/server.go` server name. No module-path or import churn.
- **Build/deploy:** `Taskfile.yml` targets, `Dockerfile`, Helm chart dir + `Chart.yaml`/templates, GitHub workflows (`pr.yml`, `main.yml`, `release.yml`) referencing the binary/image/chart names.
- **Satellites:** `apps/companion/` (pubspec name + Android app label), `apps/garmin-bridge/`, `extensions/cookidoo/manifest.json`.
- **Outward-facing (needs the user / `gh`):** GitHub repo rename and the local working-dir rename are **out of scope for this change** — flagged for the user to do separately, since they break clone URLs and tooling paths.
- **Sequencing:** lands **after** `expand-chat-to-coach` and `unify-mcp-tool-registry` archive, so the coach persona and the agenttools/MCP surface are stable before the Kazper naming layers on (avoids rewriting in-flight code/proposals).
