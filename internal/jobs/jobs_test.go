package jobs_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/jobs"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/server"
	"github.com/jradams472/isppr/internal/storage"
	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
)

// jid mints unique fixture IDs — random, since `go test` runs package
// binaries in parallel against the shared database and a per-process
// sequence counter can't prevent cross-process collisions.
func jid(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s_j%x", prefix, b)
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// fixture: tenant client + customer + one master named "tmpl_invoice",
// plus API key plaintext for the X-API-Key header.
type fx struct {
	client, customer, template, key string
}

func seed(t *testing.T, q *store.Queries) fx {
	t.Helper()
	ctx := context.Background()
	f := fx{
		client: jid("cli"), customer: jid("cust"),
		template: "tmpl_invoice_" + jid("n"), key: "k_" + jid("p"),
	}
	if _, err := q.CreateClient(ctx, store.CreateClientParams{ID: f.client, Name: f.client, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateCustomer(ctx, store.CreateCustomerParams{
		ID: f.customer, ClientID: f.client, Name: f.customer,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateTemplateMaster(ctx, store.CreateTemplateMasterParams{
		ID: jid("tm"), ClientID: f.client, Name: f.template,
		Html: "<h1>{{title}}</h1>", Css: "h1{}", Version: 1, Defaults: []byte("{}"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateAPIKey(ctx, store.CreateAPIKeyParams{
		ID: jid("key"), ClientID: f.client, KeyHash: sha256hex(f.key),
		Label: "t", Scopes: []string{"generate"},
	}); err != nil {
		t.Fatal(err)
	}
	return f
}

// env builds the full stack: echo + orchestrator + fake gotenberg + temp
// storage, driven by a single worker.
type env struct {
	e   *echo.Echo
	q   *store.Queries
	dir string
}

func newEnv(t *testing.T, delay time.Duration) *env {
	t.Helper()
	return newEnvBudget(t, delay, 3*time.Second)
}

func newEnvBudget(t *testing.T, delay, budget time.Duration) *env {
	t.Helper()
	return newEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Write([]byte("%PDF-fake"))
	}, budget, nil)
}

// newEnvFlaky mounts a custom gotenberg handler (failures, delays).
func newEnvFlaky(t *testing.T, h func(w http.ResponseWriter)) *env {
	t.Helper()
	return newEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		h(w)
	}, 3*time.Second, nil)
}

func newEnvHandler(t *testing.T, h http.HandlerFunc, budget time.Duration,
	backend storage.Backend, opts ...jobs.Option) *env {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://isppr:isppr@localhost:5433/isppr?sslmode=disable"
	}
	if err := store.RunMigrations(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := store.NewPool(context.Background(), dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)

	gb := httptest.NewServer(h)
	t.Cleanup(gb.Close)

	rpool, _ := render.NewPool([]string{gb.URL}, render.WithTimeout(5*time.Second))
	dir := t.TempDir()
	if backend == nil {
		backend = &storage.LocalStore{Dir: dir}
	}
	base := []jobs.Option{jobs.WithWorkers(1), jobs.WithSyncBudget(budget)}
	orch := jobs.New(store.New(pool), rpool, backend, append(base, opts...)...)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go orch.Start(ctx)

	e := server.New(config.Config{}, store.New(pool), pool, orch)
	return &env{e: e, q: store.New(pool), dir: dir}
}

func doReq(t *testing.T, e *echo.Echo, method, path, body, key, idem string) (int, map[string]any, http.Header) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m, rec.Header()
}

func genBody(f fx) string {
	return fmt.Sprintf(`{"customer_id":%q,"package_name":"Test_Pack","documents":[{"template_id":%q,"payload":{"title":"Hello"}}]}`,
		f.customer, f.template)
}

func TestGenerateSync201(t *testing.T) {
	env := newEnv(t, 0)
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%v)", code, body)
	}
	if body["status"] != "COMPLETED" || body["request_id"] == "" {
		t.Fatalf("bad response: %v", body)
	}
	m := body["metrics"].(map[string]any)
	for _, k := range []string{"total_documents", "parsing_duration_ms", "rendering_duration_ms", "compression_duration_ms", "total_duration_ms"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("metrics missing %s: %v", k, m)
		}
	}
	if m["total_documents"].(float64) != 1 {
		t.Fatalf("total_documents: %v", m)
	}
	r := body["results"].(map[string]any)
	if r["output_type"] != "ZIP" || !strings.HasSuffix(r["filename"].(string), ".zip") ||
		r["download_url"] == "" {
		t.Fatalf("results: %v", r)
	}
}

func TestGenerateAsync202AndStatus(t *testing.T) {
	// 500ms render vs 50ms budget → guaranteed async path.
	env := newEnvBudget(t, 500*time.Millisecond, 50*time.Millisecond)
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != http.StatusAccepted {
		t.Fatalf("want 202, got %d (%v)", code, body)
	}
	rid, _ := body["request_id"].(string)
	if rid == "" {
		t.Fatalf("no request_id: %v", body)
	}
	// Poll status until terminal.
	var final map[string]any
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		code, final, _ = doReq(t, env.e, http.MethodGet, "/api/v1/packages/"+rid, "", f.key, "")
		if code != http.StatusOK {
			t.Fatalf("status: %d %v", code, final)
		}
		if final["status"] == "COMPLETED" {
			return
		}
		if final["status"] == "FAILED" {
			t.Fatalf("job failed: %v", final)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("never completed: %v", final)
}

func TestGenerateIdempotentRetry(t *testing.T) {
	env := newEnv(t, 0)
	f := seed(t, env.q)
	idem := "idem-" + jid("x")
	code1, b1, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, idem)
	code2, b2, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, idem)
	if code1 != 201 || code2 != 201 {
		t.Fatalf("codes %d/%d (%v / %v)", code1, code2, b1, b2)
	}
	if b1["request_id"] != b2["request_id"] {
		t.Fatalf("idempotent retry created new request: %v vs %v", b1["request_id"], b2["request_id"])
	}
	// Exactly one request row for this idempotency key.
	row, err := env.q.GetRequestByIdempotency(context.Background(), store.GetRequestByIdempotencyParams{
		ClientID: f.client, IdempotencyKey: pgtype.Text{String: idem, Valid: true},
	})
	if err != nil || row.ID != b1["request_id"] {
		t.Fatalf("stored request: %v %v", row.ID, err)
	}
}

func TestGenerateValidation(t *testing.T) {
	env := newEnv(t, 0)
	f := seed(t, env.q)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty", `{}`, 400},
		{"no customer", `{"package_name":"p","documents":[{"template_id":"t","payload":{}}]}`, 400},
		{"no docs", fmt.Sprintf(`{"customer_id":%q,"package_name":"p","documents":[]}`, f.customer), 400},
		{"no template", fmt.Sprintf(`{"customer_id":%q,"package_name":"p","documents":[{"payload":{}}]}`, f.customer), 400},
		{"bad client", `{"client_id":"cli_other","customer_id":"x","package_name":"p","documents":[{"template_id":"t","payload":{}}]}`, 403},
		{"foreign customer", fmt.Sprintf(`{"customer_id":"cust_nope","package_name":"p","documents":[{"template_id":%q,"payload":{}}]}`, f.template), 403},
		{"foreign template", fmt.Sprintf(`{"customer_id":%q,"package_name":"p","documents":[{"template_id":"tmpl_nope","payload":{}}]}`, f.customer), 403},
	}
	for _, tc := range cases {
		code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", tc.body, f.key, "")
		if code != tc.want {
			t.Errorf("%s: want %d, got %d (%v)", tc.name, tc.want, code, body)
		}
	}
}

func TestCrossTenantStatusDenied(t *testing.T) {
	env := newEnv(t, 0)
	f := seed(t, env.q)
	other := seed(t, env.q) // second tenant
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != 201 {
		t.Fatalf("%d %v", code, body)
	}
	rid := body["request_id"].(string)
	code, _, _ = doReq(t, env.e, http.MethodGet, "/api/v1/packages/"+rid, "", other.key, "")
	if code != http.StatusForbidden {
		t.Fatalf("cross-tenant status: want 403, got %d", code)
	}
}

// A doc the pool rejects (400) fails the request permanently — FAILED
// status + error surface on the status endpoint.
func TestGenerateRenderFailed(t *testing.T) {
	env := newEnvFlaky(t, func(w http.ResponseWriter) { http.Error(w, "bad", 400) })
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != http.StatusOK { // FAILED responses return 200 + status FAILED
		t.Fatalf("want 200(FAILED), got %d %v", code, body)
	}
	if body["status"] != "FAILED" || body["error"] == "" {
		t.Fatalf("expected FAILED+error: %v", body)
	}
}

// Transient pool failure (503) is retried by the worker — the request
// still completes after the job is requeued.
func TestGenerateTransientRetry(t *testing.T) {
	var calls atomic.Int32
	env := newEnvFlaky(t, func(w http.ResponseWriter) {
		if calls.Add(1) == 1 {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		w.Write([]byte("%PDF-ok"))
	})
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != 201 && code != 202 {
		t.Fatalf("unexpected code %d %v", code, body)
	}
	rid := body["request_id"].(string)
	var final map[string]any
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, final, _ = doReq(t, env.e, http.MethodGet, "/api/v1/packages/"+rid, "", f.key, "")
		if final["status"] == "COMPLETED" {
			return
		}
		if final["status"] == "FAILED" {
			t.Fatalf("transient failure was not retried: %v", final)
		}
		time.Sleep(80 * time.Millisecond)
	}
	t.Fatalf("never completed: %v", final)
}

func TestDownloadNotReady(t *testing.T) {
	env := newEnvBudget(t, 500*time.Millisecond, 50*time.Millisecond)
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != 202 {
		t.Fatalf("%d %v", code, body)
	}
	rid := body["request_id"].(string)
	code, _, _ = doReq(t, env.e, http.MethodGet, "/api/v1/packages/"+rid+"/download", "", f.key, "")
	if code != http.StatusConflict {
		t.Fatalf("download before completion: want 409, got %d", code)
	}
}

func TestDownload(t *testing.T) {
	env := newEnv(t, 0)
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != 201 {
		t.Fatalf("%d %v", code, body)
	}
	rid := body["request_id"].(string)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages/"+rid+"/download", nil)
	req.Header.Set("X-API-Key", f.key)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("download: %d", rec.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	if len(zr.File) != 1 || !strings.HasSuffix(zr.File[0].Name, ".pdf") {
		t.Fatalf("zip entries: %v", zr.File)
	}
	rc, _ := zr.File[0].Open()
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "%PDF-fake" {
		t.Fatalf("pdf content: %q", b)
	}
}

// presignStore wraps LocalStore with the S3 Presigner capability — the
// download route redirects and response bodies embed the minted URL.
type presignStore struct{ *storage.LocalStore }

func (presignStore) Presign(context.Context, string) (string, error) {
	return "https://s3.example/isppr-docs/presigned-ok", nil
}

func TestDownloadPresignRedirect(t *testing.T) {
	env := newEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("%PDF-fake"))
	}, 3*time.Second, presignStore{&storage.LocalStore{Dir: t.TempDir()}})
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != 201 {
		t.Fatalf("%d %v", code, body)
	}
	results := body["results"].(map[string]any)
	if results["download_url"] != "https://s3.example/isppr-docs/presigned-ok" {
		t.Fatalf("download_url should be the presigned URL: %v", results)
	}
	rid := body["request_id"].(string)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages/"+rid+"/download", nil)
	req.Header.Set("X-API-Key", f.key)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 redirect to presigned URL, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://s3.example/isppr-docs/presigned-ok" {
		t.Fatalf("redirect location: %q", loc)
	}
	// Status polls re-mint the presigned URL too.
	_, poll, _ := doReq(t, env.e, http.MethodGet, "/api/v1/packages/"+rid, "", f.key, "")
	if poll["results"].(map[string]any)["download_url"] != "https://s3.example/isppr-docs/presigned-ok" {
		t.Fatalf("status download_url: %v", poll)
	}
}

// SINGLE_DOC_SKIP_ZIP: a one-document package stores the raw PDF and
// reports output_type PDF — no zip wrapper.
func TestSingleDocSkipZip(t *testing.T) {
	env := newEnvHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("%PDF-fake"))
	}, 3*time.Second, nil, jobs.WithSingleDocSkipZip(true))
	f := seed(t, env.q)
	code, body, _ := doReq(t, env.e, http.MethodPost, "/api/v1/packages/generate", genBody(f), f.key, "")
	if code != 201 {
		t.Fatalf("%d %v", code, body)
	}
	results := body["results"].(map[string]any)
	if results["output_type"] != "PDF" {
		t.Fatalf("want output_type PDF, got %v", results)
	}
	if !strings.HasSuffix(results["filename"].(string), ".pdf") {
		t.Fatalf("filename should be .pdf: %v", results)
	}
	rid := body["request_id"].(string)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages/"+rid+"/download", nil)
	req.Header.Set("X-API-Key", f.key)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get(echo.HeaderContentType) != "application/pdf" {
		t.Fatalf("download: %d %q", rec.Code, rec.Header().Get(echo.HeaderContentType))
	}
	if string(rec.Body.Bytes()) != "%PDF-fake" {
		t.Fatalf("raw pdf content: %q", rec.Body.Bytes())
	}
}

func TestArtifactNaming(t *testing.T) {
	if got := jobs.ArtifactName("My Pack!", "req_1", false); got != "My_Pack_req_1.zip" {
		t.Fatalf("zip name: %q", got)
	}
	if got := jobs.ArtifactName("My Pack!", "req_1", true); got != "My_Pack_req_1.pdf" {
		t.Fatalf("pdf name: %q", got)
	}
}
