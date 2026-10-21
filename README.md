# ISPPr

**ISPPr** is a Document Generation Engine (DGE): a stateless, multi-tenant
service that accepts a package of `{template_id, payload}` documents, binds
each payload into a Handlebars HTML/CSS template, renders PDFs through a
Gotenberg pool, zips the result, and returns a metadata JSON response with a
short-lived presigned download URL. Full spec: [`projectplan.md`](projectplan.md).

## Stack

| Layer | Technology |
|---|---|
| API | Go, Echo |
| DB | PostgreSQL (sqlc-generated store, append-only migrations) |
| Templating | Handlebars-compatible binding (raymond/goja — decided in the templating phase) |
| Rendering | Gotenberg pool (HTML → PDF) |
| Object store | S3 / MinIO (presigned URLs, 15-min TTL) |
| Web | Next.js (App Router), TypeScript, Tailwind |
| Dev/deploy | Docker Compose |

## Getting started

```bash
docker compose up --build          # db + gotenberg + minio + api + web
# or, API only (hot reload):
docker compose up db gotenberg minio -d
go run ./cmd/dge-api
```

Web UI: http://localhost:3000 · API: http://localhost:8090 (`/healthz`,
`/api/health`) · Gotenberg: http://localhost:3001 · MinIO console:
http://localhost:9001.

(API defaults to port **8090** so it can run alongside MPEx on :8080.)

## Observability

The compose stack includes **Jaeger** (traces) and **Seq** (structured
logs). The api exports only when the `OTEL_*` env vars are set — with
them unset the process is exporter-free (noop tracer, default slog):

- `OTEL_EXPORTER_OTLP_ENDPOINT` — OTLP/gRPC trace collector
  (`jaeger:4317` in compose → UI http://localhost:16686).
- `OTEL_LOGS_ENDPOINT` — OTLP/HTTP logs base (`http://seq:5341/ingest/otlp`
  in compose → UI http://localhost:5341).
- `OTEL_SERVICE_NAME` — resource `service.name` (default `isppr`).

A generate request produces an ingress span (`POST /packages/generate` →
`queue.enqueue`) and a detached worker tree `job.execute` → `doc.bind` →
`render.batch` → `gotenberg.convert` / `zip.compress` / `storage.put`,
with the spec's metrics block stamped as `metrics.*` span attributes.
Webhook delivery is a separate `webhook.deliver` trace linked back to
the job span. slog records flow to Seq via the OTLP logs pipeline
(and stay on stderr as JSON), correlated by trace/span IDs.

## Verification

```bash
gofmt -l cmd internal && go vet ./cmd/... ./internal/... && go test ./cmd/... ./internal/...
cd clients/web && npm run lint && npm run typecheck && npm test && npm run build
```

CI (`.github/workflows/ci.yml`) runs both jobs on every PR + push to `main`
— these are the repo's required checks. Branch protection isn't available
on the private free tier, so requiredness is enforced by workflow rule
(see `AGENTS.md`).

## Tasks

```bash
# build
go build ./cmd/... ./internal/...
docker compose build

# test
go test ./cmd/... ./internal/...           # add -coverprofile=coverage.out for SonarQube
cd clients/web && npm test                 # npm run test:coverage emits lcov.info
cd clients/web && npm run test:e2e         # Playwright — needs the live stack (compose or CI env)

# migrations apply at api boot (embedded); seed demo data with RUN_SEED=1
RUN_SEED=1 docker compose up api --build

# coverage floor — 80% per code area, 95% overall goal (see AGENTS.md)
go test -coverprofile=coverage.out ./cmd/... ./internal/...
node tools/covercheck.mjs coverage.out
(cd clients/web && npx vitest run --coverage)
node tools/covercheck.mjs clients/web/coverage/lcov.info

# sonarqube scan (local instance on :9000 — shared lena2-sonarqube)
docker run --rm --network host -e SONAR_HOST_URL=http://localhost:9000 \
  -e SONAR_TOKEN=<token> -v "$PWD:/usr/src" sonarsource/sonar-scanner-cli
docker run --rm --network host -e SONAR_HOST_URL=http://localhost:9000 \
  -e SONAR_TOKEN=<token> -v "$PWD:/usr/src" sonarsource/sonar-scanner-cli
```

## Layout

- `cmd/dge-api/` — Go API entrypoint
- `internal/` — `config`, `server`, `store` (sqlc), plus domain stubs
  `tenancy`, `templates`, `render`, `jobs`, `storage`, `admin`
- `migrations/` — append-only `NNNN_name.up.sql` (golang-migrate, embedded)
- `clients/web/` — Next.js admin portal
- `templates/base/` — embedded fallback template · `seed/` — demo tenant
  (`RUN_SEED=1`) · `e2e/` — Go pipeline suite · `clients/web/e2e/` —
  Playwright portal + API suite · `docs/`
- `tools/covercheck.mjs` — 80% per-area coverage floor
- `projectplan.md` — DGE specification

## Status

P0 scaffold + P1 data layer shipped: sqlc repositories over `clients`,
`customers`, `api_keys`, `admin_users`, versioned `template_masters` +
customer `template_variants` (resolution spec:
`docs/template-resolution.md`), embedded migrations at boot, demo seed,
SonarQube + coverage-gated CI. See Linear team **MPE** for the phased plan.
