# nutrition-api Helm chart

Single-replica deployment of the [nutrition-api](https://github.com/vinzenzs/nutrition-api)
REST backend. Ships a `Deployment`, `Service`, `ConfigMap`, optional
`Ingress`, optional `Secret`, and a `ServiceAccount`. Expects an
externally provisioned Postgres reachable via `DATABASE_URL` — the chart
does **not** include a Postgres subchart.

## Prerequisites

- Kubernetes 1.27+
- A reachable Postgres instance (managed or self-hosted)
- For Ingress + TLS (optional): `ingress-nginx` + `cert-manager` already
  installed in the cluster

## Required values

When the chart owns the Secret (default), three values MUST be set or
`helm install` fails with a clear error:

| Value | What it is |
|---|---|
| `secrets.databaseUrl` | Postgres connection string |
| `secrets.mobileApiToken` | Bearer token for the mobile companion app |
| `secrets.agentApiToken` | Bearer token for the LLM agent (must differ from `mobileApiToken`) |

`secrets.anthropicApiKey` is optional. When empty, the `POST
/meals/from_photo` endpoint returns `503 vision_unavailable` (existing
behaviour); the rest of the API works normally.

To manage the Secret outside the chart instead (e.g., applied via
`kubectl apply` from a SOPS-encrypted file), pass `--set
existingSecret=my-secret-name`. The Secret must contain these keys:
`DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN` (and optionally
`ANTHROPIC_API_KEY`).

## Install a tagged release (OCI from GHCR)

```bash
helm upgrade --install nutrition-api \
    oci://ghcr.io/vinzenzs/charts/nutrition-api \
    --version v0.1.0 \
    --namespace nutrition-api --create-namespace \
    --set secrets.databaseUrl='postgres://nutrition:...@db.internal:5432/nutrition?sslmode=disable' \
    --set secrets.mobileApiToken='<openssl rand -hex 32>' \
    --set secrets.agentApiToken='<openssl rand -hex 32>'
```

For sensitive values, prefer a private values file over `--set` (which
ends up in shell history):

```bash
# private-values.yaml — keep out of git, e.g. SOPS-encrypted or in a secrets manager
secrets:
  databaseUrl: postgres://...
  mobileApiToken: ...
  agentApiToken: ...
  anthropicApiKey: sk-ant-...   # optional
ingress:
  enabled: true
  host: nutrition.example.com
  tls:
    enabled: true
    issuer: letsencrypt-prod
```

```bash
helm upgrade --install nutrition-api \
    oci://ghcr.io/vinzenzs/charts/nutrition-api \
    --version v0.1.0 \
    --namespace nutrition-api --create-namespace \
    -f private-values.yaml
```

## Install from the repo (untagged / development)

```bash
helm upgrade --install nutrition-api \
    ./deploy/helm/nutrition-api/ \
    --set image.tag=main \
    --set secrets.databaseUrl=... \
    --set secrets.mobileApiToken=... \
    --set secrets.agentApiToken=...
```

## Upgrades

Same command — Helm idempotently reconciles. The Deployment uses
`strategy.type: Recreate` (one replica, single-process migrations), so
expect brief downtime during each upgrade.

The chart sets a `checksum/config` (and `checksum/secret` when
chart-managed) pod annotation, so config-only changes trigger a pod
restart automatically.

## Rollback

```bash
helm history nutrition-api --namespace nutrition-api
helm rollback nutrition-api <REVISION> --namespace nutrition-api
```

Rolling back the chart restores the prior image + values. If the
rollback target has a different schema_migrations head, the binary will
re-apply forward to whatever the embedded migrations claim (so rolling
the binary back across a migration boundary requires a separate
`nutrition-api migrate -version <N> down` step against the database
beforehand).

## Smoke-test the install

```bash
kubectl -n nutrition-api rollout status deploy/nutrition-api
kubectl -n nutrition-api port-forward svc/nutrition-api 8080:80 &
curl -s http://localhost:8080/healthz   # {"status":"ok"}
curl -s http://localhost:8080/readyz    # {"status":"ok"}
```

`nutrition-api version` inside the pod reports the embedded build
identity:

```bash
kubectl -n nutrition-api exec deploy/nutrition-api -- /app/nutrition-api version
# nutrition-api version=v0.1.0 commit=<sha> date=unknown
```

## Probes

- `livenessProbe` → `/healthz` — unconditional 200; only fails if the
  process is wedged.
- `readinessProbe` → `/readyz` — pings Postgres; flips the pod out of
  the Service's endpoints within ~30 s of a Postgres outage. Desired —
  but means a Postgres flap is visible to clients as a 503 from the
  edge.

## Resources

The defaults (`50m CPU / 64Mi memory` requests, `500m / 256Mi` limits)
fit a personal cluster; bump them if you see throttling or OOM in
`kubectl top`.

## Tearing down

```bash
helm uninstall nutrition-api --namespace nutrition-api
kubectl delete namespace nutrition-api   # if you created it for this chart
```

The chart does NOT touch your Postgres — data survives uninstall.

## Out of scope

- Postgres provisioning (run separately, point `databaseUrl` at it).
- Multi-replica, autoscaling, PodDisruptionBudget — single user, single
  replica.
- Image signing, SBOM, multi-arch builds — `linux/amd64` only.
- Multi-environment overlays (`values-staging.yaml` etc.) — one
  install, one values file.
