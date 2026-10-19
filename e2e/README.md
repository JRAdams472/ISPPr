# e2e/

Pipeline integration suite (MPE-38): real HTTP against the running stack —
seeded tenant → `POST /api/v1/packages/generate` → status polling →
presigned download → ZIP/PDF byte assertions, plus render-failure,
idempotent-replay, and authz-denial flows.

## Run

```bash
node tools/e2e.mjs          # compose up --build, wait for healthz, run suite
node tools/e2e.mjs --down   # same, then tear the stack down
go test ./e2e/...           # against an already-running stack
```

The suite skip-gates when the API is unreachable. In CI the Go job runs it
against a `go run`-hosted API with real postgres/MinIO/Gotenberg.

Portal-browser (Playwright) coverage lands with the admin UI work in MPE-46.
