## Context

The repository today is laptop-shaped. `task dev` runs the binary against
a local Postgres container; there is no container image, no chart, no CI.
The author's stated trajectory is to host the API on a personal Kubernetes
cluster so the Flutter companion app (separately in flight under
`add-flutter-companion-app`) and a few experimental MCP integrations can
reach it without the laptop being awake.

Constraints worth naming up front:

- **Single-user, single-cluster.** No multi-tenant concerns, no
  high-availability multi-replica story. The Deployment runs `replicas:
  1` with `strategy: Recreate`. A future change can re-examine this when
  there's a real reason to.
- **Existing CLI shape is fine.** The binary already has the right
  subcommands. We are NOT introducing a separate `migrate` Job — the
  serve container runs `MIGRATE_ON_START=true` (the existing default).
  This is a deliberate trade against the more common "run migrations as
  a pre-install Helm hook" pattern; see Decision 4.
- **Postgres is external.** The author runs Postgres outside the cluster
  (a separate machine or a hosted instance). The chart never tries to
  provision one.
- **Tokens are hand-applied secrets.** No external-secrets, no Vault,
  no SOPS. The Secret object is applied once per cluster via
  `kubectl apply -f` and referenced by the Deployment env.

## Goals / Non-Goals

**Goals:**

- A single `helm install` (or `helm upgrade --install`) brings the API up
  from a fresh cluster, given a working `DATABASE_URL` and three secret
  values.
- Every commit to `main` produces a discoverable image at
  `ghcr.io/vinzenzs/nutrition-api:main` and at `:sha-<short>`.
- Every `v*` tag produces a semver-named image AND a semver-named OCI
  Helm chart, so deploys can be pinned to a release and rolled back by
  changing one value.
- PR CI catches build regressions before merge. Test suite continues to
  use testcontainers (in CI, a service-container Postgres feeds them).
- The Helm chart README is enough documentation for a fresh-cluster
  install. No external runbook.

**Non-Goals:**

- Multi-environment overlays (staging / prod). One values file per
  install. Deferred.
- ArgoCD / Flux integration. The deploy command is `helm upgrade
  --install` run by hand or from a future workflow. Deferred.
- Multi-arch images. `linux/amd64` only in v1.
- Image signing (cosign), SBOM emission. Deferred — single-user
  threat model doesn't demand it yet.
- Chart-test (`ct`) lint/install harness. `helm template` in PR CI
  catches the obvious mistakes; full lint comes later if regressions
  show up.
- Horizontal autoscaling. `replicas: 1` only.
- Postgres provisioning. Out of scope as called out above.

## Decisions

### 1. Distroless multi-stage Dockerfile, embed everything

The build stage uses `golang:1.24-alpine` (matches `go.mod`); the runtime
stage is `gcr.io/distroless/static-debian12:nonroot`. The binary is
statically linked with `CGO_ENABLED=0` (no sqlite, no native deps) and
embeds:

- The migrations (already done via `embed.FS` in `internal/store/`).
- The generated Swagger docs (already done via `docs/`).

So the runtime stage has one file: `/app/nutrition-api`. No shell, no
package manager, no migration files copied alongside. This is the
smallest reasonable image (~25 MB compressed) and the smallest
reasonable attack surface.

**Alternatives considered:**

- *Alpine runtime.* Adds `/bin/sh`, a libc that's been a CVE source,
  ~5 MB extra. No upside for an API that never shells out.
- *Scratch runtime.* Even smaller, but distroless gives us a non-root
  user out of the box and `nonroot:nonroot` ownership without
  Dockerfile boilerplate.
- *Single-stage build using `golang:alpine` as the runtime.* Trivially
  smaller diff in the Dockerfile but ~10x the final image size.

### 2. Helm chart at `deploy/helm/nutrition-api/`, Postgres external

The chart layout follows Bitnami's de-facto convention:

```
deploy/helm/nutrition-api/
  Chart.yaml          ← name, version, appVersion
  values.yaml         ← documented defaults
  README.md           ← install/upgrade walkthrough
  templates/
    _helpers.tpl
    deployment.yaml
    service.yaml
    ingress.yaml      ← guarded by .Values.ingress.enabled
    secret.yaml       ← guarded by .Values.existingSecret == ""
    configmap.yaml
    serviceaccount.yaml
    NOTES.txt         ← post-install hints
```

The `Secret` template only renders if the user did NOT pass
`existingSecret: my-tokens` — that's the path for users who want to
manage the secret out of band (e.g., kubectl-applied by hand). When the
chart owns the Secret, values can be passed via `--set` or a private
values file.

**Why no Postgres subchart**: the author already runs Postgres elsewhere,
and bundling Bitnami's chart entangles app + DB lifecycle (a `helm
uninstall` could nuke the DB). Cleaner to assume external. The chart's
`values.yaml` exposes `config.databaseUrl` and `secrets.databaseUrl` so
the URL can be either non-sensitive (rare) or sensitive (typical).

### 3. ingress-nginx + cert-manager assumptions, ingress off by default

`templates/ingress.yaml` is only rendered when
`.Values.ingress.enabled = true`. When enabled, it adds:

- `ingressClassName: {{ .Values.ingress.className | default "nginx" }}`
- `cert-manager.io/cluster-issuer: {{ .Values.ingress.tls.issuer }}` (only
  if `.Values.ingress.tls.enabled = true`)
- `nginx.ingress.kubernetes.io/proxy-body-size: 10m` — matches the
  default `MEAL_FROM_PHOTO_MAX_BYTES` so photo uploads aren't truncated
  by the ingress before reaching the app.

Tailscale operator / Cloudflare Tunnel / bare ClusterIP all remain
available — the user disables ingress and wires their preferred edge
themselves. The chart never assumes a Tailscale operator is present.

**Alternatives considered:**

- *Bare Service only.* Simpler chart, but every install needs a
  hand-written Ingress alongside.
- *Tailscale operator service annotations.* Right shape for the author,
  but locks the chart to one operator. The Tailscale path can be reached
  by disabling Ingress and adding annotations via a separate manifest.

### 4. Migrations run in the serve container, not a pre-install Helm hook

The existing binary already has `MIGRATE_ON_START=true` as the default —
on boot, `store.Migrate(cfg.DatabaseURL)` is called before the HTTP
server starts. The chart leaves this default in place.

A Helm `pre-install` / `pre-upgrade` hook running `nutrition-api migrate`
would be more idiomatic for k8s shops where multiple replicas race on
startup. Here, with `replicas: 1` and `strategy: Recreate`, there is
exactly one process touching the database, and `golang-migrate` already
takes a `migrations` advisory lock. The hook would add YAML and a second
moving part for no real benefit.

If a future change introduces multi-replica or a blue-green strategy,
revisit this — at that point a Job-based pre-upgrade hook becomes the
right shape.

### 5. Three workflows, registry = GHCR

Three workflows under `.github/workflows/`:

- `pr.yml` — triggered on `pull_request`. Steps:
  1. checkout
  2. setup-go (with `go.mod` cache)
  3. `go vet ./...`
  4. `go test ./...` against a `postgres:17-alpine` GitHub service
     container so testcontainers attaches to the local docker daemon
     instead of pulling its own image (faster + smaller surface).
  5. `docker buildx build` with no push (smoke test the Dockerfile).

- `main.yml` — triggered on `push` to `main`. Steps:
  1. checkout, setup-go, vet, test (same as PR but no Docker smoke
     build — the next step does it).
  2. `docker/login-action` to GHCR using `secrets.GITHUB_TOKEN`.
  3. `docker/build-push-action` push tags
     `ghcr.io/${{ github.repository_owner }}/nutrition-api:main` and
     `:sha-${{ github.sha[:7] }}`.

- `release.yml` — triggered on `push` of tags matching `v*`. Steps:
  1. Same vet + test (we don't want a broken main to ship as a release).
  2. `docker/build-push-action` with `--build-arg
     VERSION=${{ github.ref_name }} --build-arg COMMIT=${{ github.sha
     }}` and tags `:${{ github.ref_name }}` and `:latest`.
  3. `helm package deploy/helm/nutrition-api/ --version
     ${{ github.ref_name }} --app-version ${{ github.ref_name }}`.
  4. `helm push nutrition-api-${{ github.ref_name }}.tgz
     oci://ghcr.io/${{ github.repository_owner }}/charts`.

The chart's published OCI URL becomes
`oci://ghcr.io/vinzenzs/charts/nutrition-api` — install via `helm
upgrade --install nutrition-api oci://ghcr.io/vinzenzs/charts/nutrition-api
--version vX.Y.Z`.

**Why GHCR over Docker Hub**: free for public images, scoped to the
repo's permissions, no extra credential to plumb. The single repository
owner namespace also nests image + chart cleanly.

**Alternatives considered:**

- *Single workflow with conditional steps.* More compact YAML; harder to
  read failure traces ("which trigger fired which step?"). Three files
  per trigger is simpler.
- *Separate workflow for chart packaging on every main push.* Would
  produce a stream of chart versions; the value-add is unclear. Chart
  ships with each tagged release only.

### 6. Version stamping via -ldflags, no goreleaser

`Dockerfile` accepts two build args: `VERSION` (default `dev`) and
`COMMIT` (default `unknown`). The Go build line:

```
go build -trimpath -ldflags="-X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/nutrition-api ./cmd/nutrition-api
```

The existing `cmd/nutrition-api/version.go` already has `var version,
commit string` that the `version` subcommand prints — those vars come
through cleanly.

**Why not goreleaser**: nice tool but pulls in a release pipeline shape
(multiple binaries, OS matrices, archive publishing) we don't need.
Single-binary single-arch keeps the bash one-liner readable.

### 7. PR CI: Postgres as a GitHub service container, not Docker-in-Docker

GitHub Actions can attach a `services:` block to a job — the test step
gets `DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres`
for free. testcontainers in the Go test suite ALREADY supports an
externally-provided Postgres via `TESTCONTAINERS_HOST_OVERRIDE`. But our
testcontainers harness specifically boots its own container per
package — that's where the Ryuk-reaper-disabled hack lives.

So the simpler move is: don't change the test harness. Let testcontainers
boot its own Postgres images on the runner. CI installs Docker (it's
already there on `ubuntu-latest`) and the suite runs unchanged. A
`services: postgres:` block would conflict with testcontainers naming the
same image. Discard the idea — it's there for completeness but the
implementation skips it.

### 8. Chart values: what's in `values.yaml` vs the Secret

`values.yaml` carries:

- `image.repository`, `image.tag`, `image.pullPolicy`
- `replicaCount` (only meaningful at `1` for now)
- `resources` (requests/limits — defaults small, ~50m CPU / 64Mi mem)
- `service.port`
- `ingress.{enabled, host, className, tls}`
- `config.{defaultUserTz, migrateOnStart, swaggerEnabled, offTimeoutSeconds,
  offUserAgentContact, idempotencyTtlHours, claudeVisionModel,
  visionTimeoutSeconds, mealFromPhotoMaxBytes}`
- `existingSecret` (string, default `""`)
- `secrets.{databaseUrl, mobileApiToken, agentApiToken, anthropicApiKey}` —
  only used when the chart creates the Secret itself.

The Secret holds: `DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`,
`ANTHROPIC_API_KEY` (optional — falls back to "" → 503 vision_unavailable
as the existing meal-from-photo path already handles).

Storing the database URL in the Secret rather than the ConfigMap is
intentional: postgres URLs often contain passwords.

### 9. Probes hit existing endpoints

- `livenessProbe`: HTTP GET `/healthz` — already returns `{status:"ok"}`
  unconditionally.
- `readinessProbe`: HTTP GET `/readyz` — already returns 503 when
  `pool.Ping()` fails within 2s.

Defaults: liveness `initialDelaySeconds: 10`, readiness `initialDelaySeconds:
3`, both `periodSeconds: 10`, `failureThreshold: 3`. These match the
existing 5-second `ReadHeaderTimeout` in `httpserver/server.go`.

## Risks / Trade-offs

- **`replicas: 1` plus `MIGRATE_ON_START` means brief downtime on every
  deploy.** Tradeoff is acceptable for a single-user API. A future change
  can switch to a pre-upgrade migration hook and `strategy: RollingUpdate`
  if the user wants zero-downtime deploys.
- **Hand-applied Secret is a step the install README has to be explicit
  about.** Easy to forget on a fresh cluster; chart `NOTES.txt` after
  install will name the Secret that needs to exist.
- **No CT-style chart linting in CI.** A broken `values.yaml` reference
  ships as a release. Mitigation: `pr.yml` runs `helm template
  deploy/helm/nutrition-api/ --debug` as a smoke step.
- **No multi-arch image.** Single-arch is right for the current target
  but locks out arm64 home clusters (e.g., Raspberry Pi). Adding
  `linux/arm64` to the buildx platform list is a one-line change in a
  follow-up.
- **`MIGRATE_ON_START` in-process means an unattended migration with no
  rollback.** `golang-migrate`'s advisory lock prevents racing, but a
  bad migration deploys to the pod and stays there. Mitigation: keep
  migrations append-only and reversible (the existing repo discipline);
  the rollback path is `helm rollback nutrition-api` + a follow-up
  `nutrition-api migrate down` if needed.
- **CI runs testcontainers per package, sequentially in jobs that pull
  images.** PR feedback time is in the 3-5 minute range. Mitigation:
  `actions/cache@v4` keyed on `go.sum` for the Go build cache; future
  follow-up could parallelize package tests.
- **Image push uses `GITHUB_TOKEN`** (no PAT). That token expires with
  the workflow run, which is fine for our use case. If the user later
  wants a long-lived bot account, swap in a PAT in `secrets.GHCR_PAT`.

## Migration Plan

This change creates new files only. There is no existing pipeline to
migrate from. Adoption order:

1. **Land the change** (PR with Dockerfile + chart + workflows).
2. **First image build runs on merge to `main`** — pushes
   `ghcr.io/vinzenzs/nutrition-api:main`.
3. **Author cuts the first release tag** (`v0.1.0`) — pushes
   `:v0.1.0` + `:latest` + the chart.
4. **Author installs on the personal cluster** via the chart's README
   walkthrough.

Rollback (if v1 of this change turns out badly):

- `git revert` removes the files; nothing in the repo's runtime path
  depends on them.
- GHCR images stay available — even after revert the chart is still
  installable from the pushed OCI URL.

## Open Questions

- **Should the workflows also run on changes inside `apps/companion/`?**
  Probably not — the Flutter app has no CI gate by design. The PR /
  main workflows can filter on `paths-ignore: ['apps/**', '**/*.md']`
  to avoid burning runner minutes on docs-only PRs. Will pick a sane
  filter list during implementation.
- **`/readyz` returns 503 if Postgres is unreachable.** That means a
  brief Postgres outage causes the pod to be removed from the Service
  endpoints — desired behavior, but worth noting that if Postgres
  flaps, the API flaps too. Could revisit with a "degraded but
  available" mode later.
- **Chart `appVersion` vs `version`.** They're kept in sync (both
  `vX.Y.Z`) by the release workflow. Helm's convention is `version`
  for the chart and `appVersion` for the app; aligning them is the
  least-surprising shape for a single-app chart.
- **GHCR retention.** GitHub keeps untagged images by default; the
  per-sha tags will accumulate. A future cleanup workflow can prune
  `:sha-*` images older than N days. Out of scope here.
- **Default `replicas: 1` plus PDB**: should the chart ship a
  PodDisruptionBudget? At one replica it would block node drains,
  which is wrong for a personal cluster. Skip the PDB; revisit if the
  user moves to multi-replica.
