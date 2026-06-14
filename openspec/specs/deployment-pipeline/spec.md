# deployment-pipeline Specification

## Purpose

Define the contract for shipping the kazper from a git commit to a running pod: the container image's shape (multi-stage distroless, embedded migrations, ldflags-stamped version), the Helm chart's required objects and value surface (single-replica Deployment, optional Ingress with cert-manager, externally provisioned Postgres), and the GitHub Actions workflows that turn commits/tags into artifacts (PR validation, `:main`/`:sha-<short>` on push, semver image + OCI Helm chart on `v*` tag).

## Requirements

### Requirement: The container image is a multi-stage, distroless, statically-linked build

The system SHALL ship as a single container image built from a Dockerfile at the repo root using a multi-stage build. The build stage compiles the binary with `CGO_ENABLED=0` and `-trimpath`; the runtime stage uses a distroless base image and contains the binary as its only file. Migrations and Swagger docs are embedded in the binary at compile time (via `embed.FS`), so the runtime image SHALL NOT need to copy any extra files alongside.

#### Scenario: Image runs as a non-root user

- **WHEN** the image is inspected via `docker inspect <image>`
- **THEN** the configured user is `nonroot` (UID 65532)
- **AND** the entrypoint is the embedded `kazper` binary
- **AND** the working directory is `/app`

#### Scenario: Image is statically linked and shell-free

- **WHEN** the image is run with `docker run --rm <image> /bin/sh`
- **THEN** the run fails with "no such file" — the runtime stage has no shell

#### Scenario: Migrations are embedded, not bind-mounted

- **WHEN** the container starts with `MIGRATE_ON_START=true` and a fresh database
- **THEN** the binary applies all embedded migrations from `internal/store/migrations/` without needing any volume mount
- **AND** the `kazper migrate` subcommand applies migrations against an arbitrary `DATABASE_URL` using the same embedded files

#### Scenario: Image size stays under 30 MB compressed

- **WHEN** the release image is pulled from GHCR
- **THEN** the compressed image size SHALL be under 30 MB

### Requirement: The binary reports its release identity via build-time ldflags

The system SHALL accept `VERSION` and `COMMIT` as Docker build args and inject them into the binary as `main.version` and `main.commit` via `-ldflags`. The existing `kazper version` subcommand SHALL print these values.

#### Scenario: Version subcommand reflects build args

- **WHEN** the image is built with `--build-arg VERSION=v1.2.3 --build-arg COMMIT=abcdef0`
- **AND** the container is run with `kazper version`
- **THEN** the output contains `version=v1.2.3` and `commit=abcdef0`

#### Scenario: Default values for unstamped builds

- **WHEN** the image is built without build args (e.g., a local `docker build` for testing)
- **THEN** the binary reports `version=dev` and `commit=unknown`

### Requirement: Helm chart packages the API as a single-replica Deployment with no Postgres

The system SHALL ship a Helm chart at `deploy/helm/kazper/` that, on `helm install`, creates a Kubernetes `Deployment` with `replicas: 1` and `strategy.type: Recreate`, a `ClusterIP` `Service`, an optional `Ingress`, an optional `Secret`, and a `ConfigMap` carrying non-secret configuration. The chart SHALL NOT include a Postgres subchart, a Postgres operator dependency, or any in-chart database resource. The chart SHALL expose `DATABASE_URL` exclusively via the Secret (whether chart-managed or externally provided).

#### Scenario: Fresh install yields the expected object set

- **WHEN** the user runs `helm install kazper deploy/helm/kazper/ --set-string secrets.databaseUrl=... --set-string secrets.mobileApiToken=... --set-string secrets.agentApiToken=...`
- **THEN** Kubernetes objects created are exactly: `Deployment/kazper`, `Service/kazper`, `ConfigMap/kazper`, `Secret/kazper`, `ServiceAccount/kazper`
- **AND** no Postgres-related objects are created

#### Scenario: Externally managed Secret bypasses the chart's Secret template

- **WHEN** the user installs the chart with `--set existingSecret=my-tokens`
- **THEN** the chart does NOT render a `Secret` of its own
- **AND** the Deployment's env references `my-tokens` for `DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`, `ANTHROPIC_API_KEY`

#### Scenario: Required token values block install

- **WHEN** the chart's Secret is being rendered (no `existingSecret`) and `secrets.databaseUrl`, `secrets.mobileApiToken`, or `secrets.agentApiToken` is empty
- **THEN** `helm install` fails with a clear error naming the missing value

#### Scenario: ANTHROPIC_API_KEY is optional

- **WHEN** the user installs the chart without providing `secrets.anthropicApiKey`
- **THEN** the install succeeds
- **AND** the pod runs with `ANTHROPIC_API_KEY` unset, which the existing meal-from-photo code path handles by returning `503 vision_unavailable`

### Requirement: Helm chart probes use the existing /healthz and /readyz endpoints

The Deployment SHALL configure `livenessProbe` against `GET /healthz` and `readinessProbe` against `GET /readyz` on the container port. Probe defaults SHALL be tuned to the existing 5-second `ReadHeaderTimeout` so an outage of Postgres flips the pod out of the Service's endpoints within ~30 seconds but does not kill the pod.

#### Scenario: Liveness uses /healthz

- **WHEN** the Deployment manifest is rendered
- **THEN** `livenessProbe.httpGet.path` equals `/healthz`
- **AND** `initialDelaySeconds: 10, periodSeconds: 10, failureThreshold: 3`

#### Scenario: Readiness uses /readyz and is fast to fail

- **WHEN** the Deployment manifest is rendered
- **THEN** `readinessProbe.httpGet.path` equals `/readyz`
- **AND** `initialDelaySeconds: 3, periodSeconds: 10, failureThreshold: 3`

### Requirement: Ingress is opt-in and assumes ingress-nginx + cert-manager when enabled

The chart SHALL render an `Ingress` only when `ingress.enabled: true`. When rendered, the Ingress SHALL set `ingressClassName` from values (default `nginx`) and SHALL add `cert-manager.io/cluster-issuer` and TLS section when `ingress.tls.enabled: true`. The Ingress SHALL set `nginx.ingress.kubernetes.io/proxy-body-size` to at least `10m` so photo uploads to `/meals/from_photo` are not truncated by the edge.

#### Scenario: Ingress disabled by default

- **WHEN** the chart is installed with default values
- **THEN** no `Ingress` resource is created

#### Scenario: Enabled Ingress carries cert-manager + proxy-body-size annotations

- **WHEN** the chart is installed with `ingress.enabled=true`, `ingress.host=nutrition.example`, `ingress.tls.enabled=true`, `ingress.tls.issuer=letsencrypt-prod`
- **THEN** the rendered Ingress has `cert-manager.io/cluster-issuer: letsencrypt-prod`
- **AND** has `nginx.ingress.kubernetes.io/proxy-body-size: 10m`
- **AND** has a `tls` section listing `nutrition.example` with `secretName: kazper-tls`

### Requirement: Migrations run in-process at startup, not as a Helm hook

The chart SHALL leave `MIGRATE_ON_START` set to `true` (the binary's default) so the serve container runs `store.Migrate(databaseUrl)` before listening. The chart SHALL NOT include a `pre-install` or `pre-upgrade` Helm hook that runs `kazper migrate` as a separate Job.

#### Scenario: No migration hook resources

- **WHEN** the chart is rendered via `helm template`
- **THEN** no resources carry the annotation `helm.sh/hook: pre-install` or `helm.sh/hook: pre-upgrade`
- **AND** no resource named `*-migrate` or `*-migration` is produced

#### Scenario: Single-replica strategy makes in-process migration safe

- **WHEN** the Deployment manifest is rendered
- **THEN** `spec.strategy.type` equals `Recreate`
- **AND** `spec.replicas` equals 1
- **AND** the env passed to the container includes `MIGRATE_ON_START=true`

### Requirement: PR workflow validates vet, test, and Docker build without publishing

The system SHALL include `.github/workflows/pr.yml` that runs on `pull_request` events. The workflow SHALL run `go vet ./...`, `go test ./...` (with testcontainers booting its own Postgres), and `docker buildx build` without `--push`. The workflow SHALL fail the PR if any step fails.

#### Scenario: A PR with a failing test blocks merge

- **WHEN** a PR is opened with a change that breaks `go test`
- **THEN** the `pr.yml` workflow run reports failure
- **AND** the failure is associated with the PR's required-checks status

#### Scenario: A PR with a broken Dockerfile blocks merge

- **WHEN** a PR is opened with a change that makes `docker buildx build` fail
- **THEN** the workflow run reports failure
- **AND** no image is pushed to GHCR

#### Scenario: PR also smoke-tests the Helm chart

- **WHEN** the workflow runs
- **THEN** `helm template deploy/helm/kazper/ --debug` is executed
- **AND** the workflow fails if templating produces an error

### Requirement: Main-branch workflow publishes :main and :sha-<short> images to GHCR

The system SHALL include `.github/workflows/main.yml` that runs on `push` to the `main` branch. The workflow SHALL build the Docker image and push it to `ghcr.io/<repo-owner>/kazper` with tags `:main` and `:sha-<short>` where `<short>` is the first 7 characters of the commit SHA. Authentication SHALL use the workflow's default `GITHUB_TOKEN` with `packages: write` permission.

#### Scenario: Successful push to main publishes both tags

- **WHEN** a commit lands on `main` and the workflow runs to completion
- **THEN** `ghcr.io/<owner>/kazper:main` is updated to the new image
- **AND** `ghcr.io/<owner>/kazper:sha-<short>` exists with the same digest

#### Scenario: Failed test blocks image push

- **WHEN** `go test` fails during the main workflow
- **THEN** the Docker push step does NOT run
- **AND** the previous `:main` tag remains unchanged

### Requirement: Tag workflow publishes a semver image and a packaged Helm chart

The system SHALL include `.github/workflows/release.yml` that runs on `push` of tags matching `v*` (e.g., `v0.1.0`, `v1.2.3-rc1`). The workflow SHALL build the Docker image with `--build-arg VERSION=<tag> --build-arg COMMIT=<sha>` and push it tagged `:<tag>` and `:latest`. The workflow SHALL package the Helm chart with `helm package --version <tag> --app-version <tag>` and push the resulting `.tgz` to `oci://ghcr.io/<repo-owner>/charts`.

#### Scenario: A v1.2.3 tag produces an image and a chart

- **WHEN** the user pushes tag `v1.2.3`
- **THEN** `ghcr.io/<owner>/kazper:v1.2.3` and `:latest` exist
- **AND** `oci://ghcr.io/<owner>/charts/kazper:v1.2.3` is installable via `helm install ... oci://ghcr.io/<owner>/charts/kazper --version v1.2.3`
- **AND** `kazper version` inside the image reports `version=v1.2.3`

#### Scenario: Failed test blocks release

- **WHEN** `go test` fails during the release workflow
- **THEN** neither the image nor the chart is pushed
- **AND** the tag remains in the repository but no artifacts are emitted

### Requirement: Chart README documents the install / upgrade path

The chart SHALL ship a `README.md` at `deploy/helm/kazper/README.md` that documents (at minimum) the required values, the OCI install command for a tagged release, the `helm upgrade --install` idiom for updates, and the rollback command. The repo-root `README.md` SHALL link to the chart README from a "Deploying" subsection.

#### Scenario: Chart README lists the three required tokens

- **WHEN** the chart README is read
- **THEN** it names `secrets.databaseUrl`, `secrets.mobileApiToken`, and `secrets.agentApiToken` as required values
- **AND** notes `secrets.anthropicApiKey` as optional with the 503 fallback behavior

#### Scenario: Repo README points at the chart

- **WHEN** the repo-root `README.md` is read
- **THEN** a "Deploying" section links to `deploy/helm/kazper/README.md`
- **AND** mentions the GHCR image URL `ghcr.io/<owner>/kazper`
