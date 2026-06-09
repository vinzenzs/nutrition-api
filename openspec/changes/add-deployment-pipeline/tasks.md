## 1. Dockerfile

- [x] 1.1 Create `Dockerfile` at the repo root with a multi-stage build: `golang:1.26-alpine` build stage (matches `go.mod` 1.26), `gcr.io/distroless/static-debian12:nonroot` runtime stage.
- [x] 1.2 Accept `VERSION` and `COMMIT` as `ARG`s with defaults `dev` / `unknown`; pass them through to the Go build via `-ldflags="-X main.version=${VERSION} -X main.commit=${COMMIT}"` (plus `-s -w` for symbol stripping).
- [x] 1.3 Build with `CGO_ENABLED=0` and `-trimpath`; output the binary at `/out/nutrition-api` in the build stage and copy to `/app/nutrition-api` in the runtime stage.
- [x] 1.4 Set `WORKDIR /app`, `USER nonroot:nonroot`, `EXPOSE 8080`, `ENTRYPOINT ["/app/nutrition-api"]`, default `CMD ["serve"]`.
- [x] 1.5 Create `.dockerignore` excluding `bin/`, `.env*`, `apps/`, `docs/api/`, `openspec/`, `.github/`, `deploy/`, `extensions/`, `*.md`, `internal/store/storetest/`, and test data so the build context stays small.
- [x] 1.6 Local smoke: `docker build -t nutrition-api:local --build-arg VERSION=test --build-arg COMMIT=abc1234 .` succeeds.
- [x] 1.7 Local smoke: `docker run --rm nutrition-api:local version` prints `version=test commit=abc1234` (the existing binary uses `=` not `:` formatting).
- [x] 1.8 Local smoke: `docker run --rm --entrypoint /bin/sh nutrition-api:local` fails confirming the distroless shape has no shell.
- [x] 1.9 Verify the compressed image size is under 30 MB via `docker save | gzip | wc -c` (uncompressed 42 MB, compressed 12.8 MB — well under the 30 MB target).

## 2. Helm chart scaffold

- [x] 2.1 Create `deploy/helm/nutrition-api/Chart.yaml` with `apiVersion: v2`, `name: nutrition-api`, `version: 0.1.0`, `appVersion: 0.1.0`, `description`, `type: application`.
- [x] 2.2 Create `deploy/helm/nutrition-api/.helmignore` excluding `*.md` except `README.md`, plus `*.swp`, `.DS_Store`, `__pycache__/`, etc.
- [x] 2.3 Create `deploy/helm/nutrition-api/values.yaml` with the structure documented in design.md §8 (image, replicaCount, resources, service, ingress, config, existingSecret, secrets) and inline comments explaining each value.
- [x] 2.4 Create `deploy/helm/nutrition-api/templates/_helpers.tpl` with `nutrition-api.fullname`, `nutrition-api.name`, `nutrition-api.labels`, `nutrition-api.selectorLabels`, `nutrition-api.serviceAccountName`, `nutrition-api.secretName` helper functions.

## 3. Helm chart templates

- [x] 3.1 Write `templates/serviceaccount.yaml` (a minimal SA so RBAC additions later don't require a chart structure change).
- [x] 3.2 Write `templates/configmap.yaml` carrying every non-secret env var (DEFAULT_USER_TZ, MIGRATE_ON_START, SWAGGER_ENABLED, OFF_TIMEOUT_SECONDS, OFF_USER_AGENT_CONTACT, IDEMPOTENCY_TTL_HOURS, CLAUDE_VISION_MODEL, VISION_TIMEOUT_SECONDS, MEAL_FROM_PHOTO_MAX_BYTES) from `.Values.config.*`.
- [x] 3.3 Write `templates/secret.yaml` guarded by `if not .Values.existingSecret`. Populate from `.Values.secrets.{databaseUrl, mobileApiToken, agentApiToken, anthropicApiKey}`. Use `required` for the three mandatory tokens; `anthropicApiKey` defaults to `""`.
- [x] 3.4 Write `templates/service.yaml`: `ClusterIP` on port `.Values.service.port` (default 80) → targetPort 8080.
- [x] 3.5 Write `templates/deployment.yaml`:
    - `replicas: {{ .Values.replicaCount }}` (default 1)
    - `strategy.type: Recreate`
    - container env: every key in the ConfigMap via `envFrom.configMapRef`, every key in the Secret via `envFrom.secretRef` (Secret name = `.Values.existingSecret` if set, otherwise the chart-managed Secret).
    - `livenessProbe` → `/healthz` with the design.md §9 defaults
    - `readinessProbe` → `/readyz` with the design.md §9 defaults
    - resources from `.Values.resources`
    - `securityContext.runAsNonRoot: true`, `readOnlyRootFilesystem: true`
- [x] 3.6 Write `templates/ingress.yaml` guarded by `if .Values.ingress.enabled`. Set `ingressClassName`, optional `cert-manager.io/cluster-issuer` annotation, `nginx.ingress.kubernetes.io/proxy-body-size: 10m` annotation. Render `tls:` only when `.Values.ingress.tls.enabled`.
- [x] 3.7 Write `templates/NOTES.txt` with post-install guidance: name of the rendered Secret, reminder about the three required tokens, and the URL to reach the API (Ingress host if enabled, otherwise the `kubectl port-forward` command).
- [x] 3.8 `helm lint deploy/helm/nutrition-api/` clean.
- [x] 3.9 `helm template deploy/helm/nutrition-api/ --set secrets.databaseUrl=foo --set secrets.mobileApiToken=bar --set secrets.agentApiToken=baz` renders without errors and includes Deployment + Service + ConfigMap + Secret + ServiceAccount, no Ingress.
- [x] 3.10 `helm template ... --set ingress.enabled=true --set ingress.host=test.example --set ingress.tls.enabled=true --set ingress.tls.issuer=letsencrypt-prod` renders an Ingress with the cert-manager + proxy-body-size annotations.
- [x] 3.11 `helm template ... --set existingSecret=my-tokens` renders no Secret and the Deployment references `my-tokens` for env.

## 4. Chart documentation

- [x] 4.1 Write `deploy/helm/nutrition-api/README.md`: required values table, OCI install one-liner for tagged releases, `helm upgrade --install` idiom, `helm rollback` example, post-install Secret check.
- [x] 4.2 Update repo-root `README.md` to add a "Deploying" subsection after "MCP server" pointing at `deploy/helm/nutrition-api/README.md` and naming the GHCR URL `ghcr.io/vinzenzs/nutrition-api`.

## 5. GitHub Actions: PR workflow

- [x] 5.1 Create `.github/workflows/pr.yml` triggered on `pull_request: { branches: [main] }` with `paths-ignore: ['apps/**', 'extensions/**', '**/*.md', 'openspec/**']` so Flutter / Cookidoo / spec-only changes don't burn runner minutes.
- [x] 5.2 Single job `validate` on `ubuntu-latest`: checkout, `actions/setup-go@v5` with cache.
- [x] 5.3 Step `go vet ./...`.
- [x] 5.4 Step `go test ./...` (testcontainers boots its own Postgres; runner already has Docker installed).
- [x] 5.5 Step `docker buildx build --build-arg VERSION=pr-${{ github.event.number }} --build-arg COMMIT=${{ github.sha }} .` (no `--push`).
- [x] 5.6 Step `helm lint deploy/helm/nutrition-api/` + `helm template deploy/helm/nutrition-api/ --set secrets.databaseUrl=x --set secrets.mobileApiToken=x --set secrets.agentApiToken=x --debug`.
- [x] 5.7 Local syntax check: `actionlint .github/workflows/pr.yml` clean (install via `brew install actionlint` for the smoke check; not a CI gate).

## 6. GitHub Actions: main workflow

- [x] 6.1 Create `.github/workflows/main.yml` triggered on `push: { branches: [main] }` with the same `paths-ignore` as `pr.yml`.
- [x] 6.2 Same vet + test steps as PR (keep the gate identical so a green PR never fails on main for non-flaky reasons).
- [x] 6.3 `docker/login-action@v3` against `ghcr.io` using `${{ secrets.GITHUB_TOKEN }}`.
- [x] 6.4 `docker/build-push-action@v6` with platforms `linux/amd64`, build args `VERSION=main-${{ github.sha[:7] }}` and `COMMIT=${{ github.sha }}`, tags `ghcr.io/${{ github.repository_owner }}/nutrition-api:main` and `:sha-${{ github.sha[:7] }}`. _Implementation note: GitHub expressions don't support `${{ github.sha[:7] }}` slicing; uses a `steps.sha` step computing the substring via shell._
- [x] 6.5 Job-level `permissions: { contents: read, packages: write }`.
- [x] 6.6 `actionlint .github/workflows/main.yml` clean.

## 7. GitHub Actions: release workflow

- [x] 7.1 Create `.github/workflows/release.yml` triggered on `push: { tags: ['v*'] }` (note: tag pushes ignore `paths-ignore`, which is the correct behavior — tags don't have branch context).
- [x] 7.2 Vet + test steps as in `main.yml`.
- [x] 7.3 `docker/login-action@v3` to `ghcr.io`.
- [x] 7.4 `docker/build-push-action@v6` with build args `VERSION=${{ github.ref_name }} COMMIT=${{ github.sha }}`, tags `ghcr.io/${{ github.repository_owner }}/nutrition-api:${{ github.ref_name }}` and `:latest`.
- [x] 7.5 Step: `helm package` with `--version` stripped of leading `v` (Helm v4 requires SemVer; verified locally — `helm package ... --version 0.1.0` succeeds, output `nutrition-api-0.1.0.tgz`).
- [x] 7.6 Step: `helm registry login ghcr.io -u ${{ github.actor }} --password-stdin` and `helm push nutrition-api-*.tgz oci://ghcr.io/${{ github.repository_owner }}/charts`.
- [x] 7.7 Job-level `permissions: { contents: read, packages: write }`.
- [x] 7.8 `actionlint .github/workflows/release.yml` clean.

## 8. Smoke verification (manual / local)

- [ ] 8.1 Run the PR workflow's job locally via `act -j validate` (if `act` is installed) to catch obvious typos before push.
- [ ] 8.2 Build the image locally with `--build-arg VERSION=v0.0.0-test`, push to a kind cluster, and confirm `kubectl logs` shows the API starting and `nutrition-api migrate` applying migrations.
- [ ] 8.3 `helm install nutrition-api ./deploy/helm/nutrition-api/ --set secrets.databaseUrl=... --set image.repository=local/nutrition-api --set image.tag=v0.0.0-test --set image.pullPolicy=Never` against a kind cluster with a side-car Postgres deployment; `kubectl wait` for ready, `kubectl port-forward svc/nutrition-api 8080:80`, `curl localhost:8080/healthz` → `{"status":"ok"}`.
- [ ] 8.4 `helm upgrade nutrition-api ./deploy/helm/nutrition-api/ ...` with a changed image tag and confirm the Recreate strategy + readiness probe sequencing works (pod removed from endpoints before being deleted).

## 9. Pre-merge

- [x] 9.1 `task vet` clean (Go side untouched but verify nothing broke).
- [x] 9.2 `task test` green.
- [x] 9.3 `openspec validate add-deployment-pipeline --strict` passes.
- [ ] 9.4 Manual: confirm `gh repo view --json` shows the repo has `packages: write` for the default GITHUB_TOKEN (it does on personal repos by default; just sanity-check).
- [ ] 9.5 Manual: cut a test tag `v0.0.1-rc1` on a throwaway branch, watch the release workflow, confirm both the image and the chart land in GHCR, then delete the test tag and the GHCR artifacts.
