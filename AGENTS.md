# Project Rules for Devin — ISPPr

ISPPr is a **Document Generation Engine (DGE)**: a stateless, multi-tenant
service that merges JSON payloads into Handlebars HTML/CSS templates and
renders them to PDF through a Gotenberg pool, packaging results as ZIPs
served via short-lived S3 presigned URLs. Authoritative spec:
`projectplan.md` (DGE specification + stack evaluation).

Stack: Go + Echo API, PostgreSQL via sqlc, Gotenberg render pool,
S3/MinIO object store. Admin portal: `clients/web` — Next.js (App Router) +
TypeScript + Tailwind. Dark UI theme: GitHub-dark palette (near-black
surfaces, laser-blue accent).

Non-negotiable invariants:

- **Stateless engine** — no per-request state in process memory. All state
  lives in Postgres (tenants, templates, request audit) or S3 (outputs).
- **Tenant isolation enforced in the engine** — `client_id` is derived from
  the authenticated API key, never trusted from the payload; every
  tenant-scoped query filters on it. Gateway key validation is additive,
  not a substitute.
- **No binaries on the response stream** — `POST /api/v1/packages/generate`
  returns metadata JSON + a presigned URL only (spec §6).
- **Presigned URL TTL = 15 minutes** — enforced by S3/MinIO presign/lifecycle,
  not by response text.
- **Idempotent generation** — `Idempotency-Key` maps to
  `UNIQUE (client_id, idempotency_key)` on `generation_requests`; a retried
  POST returns the original result — never a second render or stored object.
- **Concurrent render fan-out** — documents in a package render in parallel
  against the Gotenberg pool (errgroup). Never serial per document.

## Git Workflow

- Each phase of a plan is developed on its own ticket-prefixed branch:
  `MPE-<n>-phase-<m>` (e.g. `MPE-24-phase-1`). Descriptive slugs allowed
  after the ticket: `MPE-23-scaffold`.
- **One PR per phase subticket (required)** — never batch phase tickets into
  a single PR.
- **Do not push commits directly to `main`.** Branches and PR titles start
  with the ticket number: `MPE-24: Phase 1 — tenant model + API keys`.
- **Do not open a PR without a Linear ticket.**
- Before opening a PR: `gofmt`, `go vet`, `go test` over `cmd/` +
  `internal/` (never `./...` — `node_modules` contains stray Go packages),
  and web `lint`/`typecheck`/`test`/`build` all green. Commit `go.sum` and
  `clients/web/package-lock.json` changes — CI fails when they drift.
- **No PR or phase merges without the user's explicit approval.** Present
  the PR link + proof, then wait.

## Linear Workflow (required)

- Team **MPE**. Every plan gets a **parent ticket**; once planning is done
  and work begins, the parent moves to **In Progress**.
- **Attach the plan document to the parent ticket before creating phase
  sub-issues** — the megaplan goes on the parent first, then the subtickets.
- Every phase gets a **sub-issue** under the parent named for its branch,
  with scope + acceptance criteria; mark it **In Progress** when its phase
  starts.
- On phase completion, post **proof of completion** on the sub-issue before
  asking for merge approval:
  - Run logs (build/test/lint output).
  - **Playwright screenshots** for any user-facing change (`playwright` MCP —
    capture real screens, don't describe them).
  - Link/summary of the green **GitHub CI run**.
- Linear attachments: `prepare_attachment_upload` → PUT the file to the
  signed URL with the exact signed headers → `create_attachment_from_upload`.
  `SignatureDoesNotMatch` means the headers drifted — request a fresh URL
  and PUT immediately (URLs expire in ~60s).

## Code Conventions

- Go: handlers live in `internal/` (server, store, render, package as it
  grows) and return `error`; map failures to RFC 7807 `problem+json` in a
  central error handler. Migrations are append-only
  `migrations/NNNN_name.sql`.
- DB access via sqlc-generated code (`sqlc.yaml` → `internal/store`). Run
  `sqlc generate` after touching `migrations/` or `internal/store/queries/`
  and commit the generated output.
- Template resolution order: **customer variant overlay → client master →
  base template** — encode it in one resolver, don't scatter fallbacks.
- Web: App Router + Tailwind; typed fetch helpers in `lib/`; template editor
  work loads CodeMirror/Monaco client-only.
- Boy-scout rule: fix trivial adjacent issues in a touched file when
  low-risk; never let it balloon the diff.

## Evidence Gathering

- Raw logs: `docker compose logs api --since 30m`. Render failures:
  `docker compose logs gotenberg`.
- **MCP helpers configured** (`.devin/mcp_config.local.json`, gitignored —
  copy from the MPEx project folder if missing): `playwright` (real
  screenshots for UI proof), `linear` (ticket workflow above), `sonarqube`
  (local SonarQube at `host.docker.internal:9000`).
- Observability stack can be added later — file a ticket when needed rather
  than silently shipping without it.

## Plan Close-Out

After the final phase of a plan merges:

1. Delete merged phase branches locally and on GitHub (verify PR state with
   `gh` first).
2. No new feature left at zero coverage — every new service, handler, or
   page gets at least one unit or integration test.
3. File Linear tickets (team **MPE**) for improvements or new features
   inspired by the completed work.
4. Update `README.md` + `docs/` for what shipped — sweep for stale claims
   (endpoint lists, env vars, workflow names).
5. **Size & coverage report** on the parent ticket at close-out:
   `tokei -e node_modules cmd internal migrations clients/web/app
   clients/web/lib`, source/test split via `git ls-files`, `go test`
   results, `npm test` numbers.

## Verification Commands

```bash
gofmt -l cmd internal && go vet ./cmd/... ./internal/... && go test ./cmd/... ./internal/...
cd clients/web && npm run lint && npm run typecheck && npm test && npm run build
docker compose up --build        # db + gotenberg + minio + api + web
```
