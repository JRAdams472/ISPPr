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
- `migrations/` — append-only SQL migrations
- `clients/web/` — Next.js admin portal
- `seed/` — demo data (MPE-27) · `e2e/` — Playwright suite (MPE-46) · `docs/`
- `projectplan.md` — DGE specification

## Status

MPE-23 scaffold: repo skeleton, schema v1 (`clients`, `customers`,
`api_keys`, `template_masters`, `template_variants`,
`generation_requests`), compose stack, CI, agent tooling. See Linear team
**MPE** for the phased plan.
