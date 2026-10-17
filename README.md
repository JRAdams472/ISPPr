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
go run ./cmd/api
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

## Layout

- `cmd/api/` — Go API entrypoint
- `internal/` — server, config, sqlc store (`queries/` are sqlc inputs)
- `migrations/` — append-only SQL migrations
- `clients/web/` — Next.js admin portal
- `projectplan.md` — DGE specification

## Status

MPE-23 scaffold: repo skeleton, schema v1 (`clients`, `customers`,
`api_keys`, `template_masters`, `template_variants`,
`generation_requests`), compose stack, CI, agent tooling. See Linear team
**MPE** for the phased plan.
