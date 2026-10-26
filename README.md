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

Web UI: http://localhost:3100 · API: http://localhost:8090 (`/healthz`,
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

# sonarqube scan (local instance on :9000 — start with
# `docker compose -f docker-compose.sonarqube.yml up -d`, project key isppr).
# Mount source subdirs only — a whole-repo mount crawls node_modules/.next.
docker run --rm \
  -v "$PWD/internal:/usr/src/internal" -v "$PWD/cmd:/usr/src/cmd" \
  -v "$PWD/e2e:/usr/src/e2e" -v "$PWD/migrations:/usr/src/migrations" \
  -v "$PWD/tools:/usr/src/tools" \
  -v "$PWD/clients/web/app:/usr/src/clients/web/app" \
  -v "$PWD/clients/web/lib:/usr/src/clients/web/lib" \
  -v "$PWD/clients/web/__tests__:/usr/src/clients/web/__tests__" \
  -v "$PWD/clients/web/e2e:/usr/src/clients/web/e2e" \
  -v "$PWD/coverage.out:/usr/src/coverage.out" \
  -v "$PWD/clients/web/coverage/lcov.info:/usr/src/clients/web/coverage/lcov.info" \
  -v "$PWD/sonar-project.properties:/usr/src/sonar-project.properties" \
  -e SONAR_TOKEN=<token> sonarsource/sonar-scanner-cli \
  -Dsonar.host.url=http://host.docker.internal:9000
# (Git Bash: use "$(pwd -W)/internal" for the mount paths.)
```

## Layout

- `cmd/dge-api/` — Go API entrypoint
- `internal/` — `config`, `server` (Echo wiring + RFC 7807 errors),
  `store` (sqlc), `tenancy` (API-key auth + rate limit), `templates`
  (resolution engine), `render` (raymond binding + Gotenberg pool),
  `jobs` (DB queue + pipeline orchestrator), `storage` (S3/local +
  presigned downloads), `admin` (management API), `telemetry` (OTel)
- `migrations/` — append-only `NNNN_name.up.sql` (golang-migrate, embedded)
- `clients/web/` — Next.js admin portal
- `templates/base/` — embedded fallback template · `seed/` — demo tenant
  (`RUN_SEED=1`) · `e2e/` — Go pipeline suite · `clients/web/e2e/` —
  Playwright portal + API suite · `docs/`
- `tools/covercheck.mjs` — 80% per-area coverage floor
- `projectplan.md` — DGE specification

## Status

The MPE-22 megaplan (P0–P4) is shipped end to end: tenant API
(`POST /api/v1/packages/generate`, status, presigned download, idempotent
retries), template masters + customer variants, management/admin API,
Next.js portal (login, client/customer/variant management, request
monitoring), a WYSIWYG template editor (paged TipTap canvas, merge-field
chips, page setup with headers/footers, conditional regions, data-bound
tables, print breaks, live PDF preview, source mode), webhooks, OTel
traces+logs, hardening (413/429 limits,
graceful drain), and CI running Go/Web/Playwright-e2e/SonarQube jobs on
every PR. See Linear team **MPE** for the phased plan and docs/ for the
design notes.
