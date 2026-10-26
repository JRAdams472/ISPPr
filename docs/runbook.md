# Operations runbook

## Daily ops

- **Health:** `GET /healthz` (compose healthcheck; used by CI too).
- **Logs:** `docker compose logs -f api` (JSON `slog`); render failures in
  `docker compose logs gotenberg`.
- **Traces:** Jaeger `http://localhost:16686` — service `isppr`, spans per
  request + per-document render.
- **Structured logs:** Seq `http://localhost:5341` when `SEQ_URL` is set.

## Common tasks

### Stack lifecycle

```bash
docker compose up -d --build     # db + gotenberg + minio + api + web
docker compose logs -f api
docker compose down              # -v to wipe the volume (re-seeds on next up)
```

Seed data loads when `RUN_SEED=1` (compose default): `cli_demo` /
`cust_demo` / `tmpl_invoice` / key `isppr_dev_demo`, admins
`admin@isppr.local` + `viewer@isppr.local` (`isppr-admin`).

### Failed request triage

1. `GET /api/v1/admin/requests/{id}` → `status`, `error`, `webhook_log`,
   render-phase `metrics`.
2. FAILED + `error` mentioning raymond → template won't compile; check
   master HTML (unsupported helpers fail closed).
3. FAILED with gotenberg errors → `docker compose logs gotenberg`.
4. Webhook `EXHAUSTED` → inspect `webhook_log` attempts; fix the endpoint
   and re-generate (webhooks don't retry beyond the attempt budget).

### Rotating / revoking credentials

- API keys: admin portal → client → Keys card, or
  `POST /admin/api-keys/{id}/rotate|revoke`. Rotation is immediate —
  the old hash stops authenticating at once.
- Admin passwords are bcrypt-only; reset by inserting a new
  `admin_users` row or updating `password_hash` via psql.

### Storage

- S3/MinIO: objects under `{request_id}/{filename}`; presigned GET 15 min
  TTL. Lifecycle expiry should match or exceed the TTL.
- Local dev backend (no `S3_ENDPOINT`): artifacts under the api's
  storage dir; `/admin/requests/{id}/download` streams them.

### Migrations

Append-only `migrations/NNNN_name.up.sql`, applied at boot via
`store.RunMigrations`. Never edit shipped migrations — add a new one and
run `sqlc generate` after touching queries.

## Limits & tuning

| Knob | Default | Effect |
|---|---|---|
| `WORKER_COUNT` | 4 | async render workers |
| `SYNC_BUDGET_MS` | 3000 | sync inline-render budget before 202+poll |
| `MAX_BODY_BYTES` | 1 MiB | 413 guard on POST bodies |
| `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` | 20/40 | per-key 429s |
| `RENDER_TIMEOUT_MS` | 30000 | per-document Gotenberg timeout |
| `GOTENBERG_URLS` | compose pool | comma-separated pool endpoints |
| `PRINT_GS_PATH` | `gs` | Ghostscript binary for print-ready post-processing (empty disables) |
| `PRINT_ICC_PROFILE` | — | output ICC profile for CMYK/PDF-X — name resolved under gs's iccprofiles dirs or an absolute path; empty = CMYK only, no OutputIntent |
| `PRINT_PDFX` | true | stamp a PDF/X `/OutputIntent` when a profile resolves |
| `PRINT_TIMEOUT_MS` | 20000 | per-document gs timeout |

Print-ready post-processing runs only on documents whose resolved
`defaults.print.enabled` (or `print_ready` override) is true — RGB
documents pay nothing. gs ships in the api image with
`default_cmyk.icc`; drop a SWOP/GRACoL profile under `icc/` for
production. Failures surface as generation failures with the gs error.

## Shutdown

SIGTERM → server stops accepting, worker drains in-flight renders,
requests stay PENDING for the next boot's reaper — no lost work.

## CI

Four jobs (`ci.yml`): Go fmt/vet/test (+coverage floor), Web
lint/typecheck/test/build (+coverage floor), Playwright e2e (seeded
stack), SonarQube (community, ephemeral). Coverage: `tools/covercheck.mjs`,
80%/area floor.
