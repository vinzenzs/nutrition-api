## Why

The API has shipped 17+ change-driven features but still lives only on the
author's laptop — there is no Dockerfile, no container image, no Helm
chart, no CI. Moving it to a personal Kubernetes cluster requires hand
building binaries, hand crafting manifests, and hand updating them on
every change. That friction is the reason in-progress features keep
stacking on top of each other instead of going live and being used by the
mobile companion app (which itself blocks on the API being reachable from
a phone over the LAN or internet).

This change establishes the boring-but-load-bearing path from `git push`
to a running pod: a Dockerfile, a Helm chart, and three GitHub Actions
workflows. One-shot setup, then every future change ships by tagging a
release.

## What Changes

- **New `Dockerfile`** — multi-stage Go build, distroless runtime, scratch
  binary embedding migrations and Swagger docs. Single `nutrition-api`
  binary (the existing Cobra entry point) with `serve`, `mcp`, `migrate`,
  `version` subcommands.
- **New Helm chart at `deploy/helm/nutrition-api/`** — chart packages
  `Deployment`, `Service` (ClusterIP), `Ingress` (ingress-nginx +
  cert-manager annotations), `Secret` (env vars), `ConfigMap` (non-secret
  config), readiness/liveness probes pointing at `/readyz` / `/healthz`.
  The chart assumes an externally provisioned Postgres reachable via
  `DATABASE_URL` — no Postgres subchart.
- **New `.github/workflows/` directory** with three workflows:
  - `pr.yml` — on pull request: `go vet`, `go test ./...` (boots a
    Postgres service container so testcontainers can attach), and a
    Docker `build` (no push) to catch Dockerfile regressions.
  - `main.yml` — on push to `main`: build + push image to
    `ghcr.io/vinzenzs/nutrition-api:main` and
    `ghcr.io/vinzenzs/nutrition-api:sha-<shortsha>`.
  - `release.yml` — on tag `v*`: build + push the same image tagged
    `:vX.Y.Z` and `:latest`, then `helm package deploy/helm/nutrition-api/
    --version vX.Y.Z --app-version vX.Y.Z` and push the OCI artifact to
    `ghcr.io/vinzenzs/charts/nutrition-api:vX.Y.Z`.
- **New `deploy/helm/nutrition-api/values.yaml` with sensible defaults**
  — image repo + tag, resource requests, probe knobs, ingress host (off
  by default), TLS issuer name, and the env vars Viper consumes
  (`DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`,
  `ANTHROPIC_API_KEY`, `DEFAULT_USER_TZ`, etc.). Token + key values come
  from a referenced Secret, never the values file.
- **New version stamping path** — `release.yml` injects
  `-ldflags "-X main.version=vX.Y.Z -X main.commit=<sha>"` so
  `nutrition-api version` reports the real release identity.
- **New `deploy/helm/nutrition-api/README.md`** — install / upgrade
  commands, the values that MUST be set (the three tokens), and a
  minimal example values override.
- **Repo-root `README.md` gains a "Deploying" subsection** — pointer to
  the chart README, GHCR image URL, and a one-line install command.

## Capabilities

### New Capabilities

- `deployment-pipeline`: The contract for how the API is packaged
  (container image shape, embedded migrations, version stamping), how
  the Helm chart is structured (released objects, required values,
  probe paths), and how CI turns commits into shippable artifacts (PR
  validation, main-image publish, tag-release semver image + chart
  push). Written so a future change that swaps registries, switches
  ingress controllers, or adds a staging cluster can re-examine the
  decisions here.

### Modified Capabilities

None. The Go binary's behaviour is unchanged — this change wraps it for
distribution.

## Impact

- **New files only** — no existing code paths are touched. Rollback is
  `rm -rf .github deploy Dockerfile` and removing the README subsection.
- **New dependencies** — none in `go.mod`. The Helm chart depends on
  ingress-nginx + cert-manager being present in the target cluster
  (operator-installed; not something the chart pulls in).
- **New external account state** — the GitHub repository must have
  `packages: write` permission for the GITHUB_TOKEN (default on personal
  repos) and publish images to `ghcr.io/vinzenzs/...`. No third-party
  secrets are required for v1.
- **Build duration** — PR CI runs `go vet` + `go test` (gated on a
  Postgres service container) + Docker build. Expect 3-5 minutes per PR.
  Release workflow adds image push + chart push; expect 5-8 minutes per
  tag.
- **Image size target** — multi-stage build with distroless base; target
  is < 30 MB compressed. The Go binary itself is around 15 MB.
- **No CI gate on the Flutter companion app** (per the existing
  `add-flutter-companion-app` decision). Workflows trigger only on
  changes outside `apps/companion/**` — or run regardless and skip the
  build step if only mobile files changed.
- **Documentation**: `README.md` gains a deployment pointer; new chart
  README covers install/upgrade; existing `RUN_LOCAL.md` is untouched
  (that flow stays binary-on-laptop).

### Out of scope (explicit non-goals)

- A Postgres subchart, Postgres operator, or any in-chart database. The
  chart assumes a managed or separately-deployed Postgres reachable from
  the pod. The author runs a personal Postgres outside the cluster.
- Multi-environment values (staging, prod). One chart, one set of values
  per install. A future change can add `values-staging.yaml` overlays.
- Helm chart testing harness (`helm-unittest`, `chart-testing`). Manual
  `helm template` + `helm install` in a kind cluster is sufficient for
  v1; add CT in a follow-up if regressions appear.
- GitOps (ArgoCD / Flux) integration. The deploy step in v1 is the user
  running `helm upgrade` against their cluster. A follow-up change can
  introduce an `Application` manifest in this repo or a separate
  manifests repo.
- Multi-arch image builds. v1 ships `linux/amd64` only — the author's
  cluster is amd64. arm64 added later if needed.
- Signing (cosign / sigstore). Image and chart are unsigned in v1; add
  signing in a follow-up if the threat model changes.
- Secrets management beyond a plain Kubernetes Secret (no
  external-secrets, no Vault, no SOPS). The author hand-applies the
  Secret once per cluster.
