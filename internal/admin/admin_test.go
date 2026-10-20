package admin_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/server"
	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Request-level coverage for the whole /api/v1/admin surface. Runs against
// real Postgres (compose db locally, service container in CI).
func testServer(t *testing.T, rpool *render.Pool) (*echo.Echo, *store.Queries, *pgxpool.Pool) {
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
		t.Skipf("postgres unavailable (%v) — start via docker compose up db", err)
	}
	t.Cleanup(pool.Close)
	return server.New(config.Config{CORSOrigin: "http://localhost:3100"},
		store.New(pool), pool, nil, rpool), store.New(pool), pool
}

func do(t *testing.T, e *echo.Echo, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func doList(t *testing.T, e *echo.Echo, path, token string) (int, []map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var out []map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestAdminAPI(t *testing.T) {
	e, q, pool := testServer(t, nil)
	ctx := context.Background()

	// Seed an admin user to log in with (unique email per run — the dev DB
	// persists between test runs).
	sfx := fmt.Sprintf("%d", rand.Int63())
	email := "t" + sfx + "@isppr.local"
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	_, err := q.CreateAdminUser(ctx, store.CreateAdminUserParams{
		ID: "adm_t" + sfx, Email: email, Name: "T", PasswordHash: string(hash), Role: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	// --- auth gate
	if code, _ := do(t, e, http.MethodGet, "/api/v1/admin/clients", "", ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated access: got %d, want 401", code)
	}
	if code, _ := do(t, e, http.MethodGet, "/api/v1/admin/clients", "", "bogus"); code != http.StatusUnauthorized {
		t.Fatalf("bogus token: got %d, want 401", code)
	}
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/login",
		fmt.Sprintf(`{"email":%q,"password":"wrong"}`, email), ""); code != http.StatusUnauthorized {
		t.Fatalf("bad login: got %d, want 401", code)
	}
	code, login := do(t, e, http.MethodPost, "/api/v1/admin/login",
		fmt.Sprintf(`{"email":%q,"password":"pw"}`, email), "")
	if code != http.StatusOK || login["token"] == nil {
		t.Fatalf("login: %d %+v", code, login)
	}
	tok := login["token"].(string)
	if login["user"].(map[string]any)["password_hash"] != nil {
		t.Fatal("login response leaks password_hash")
	}

	// Problem+json contract on errors.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/me", nil))
	if ct := rec.Header().Get(echo.HeaderContentType); ct != "application/problem+json" {
		t.Fatalf("error content-type: %q", ct)
	}

	// --- me
	code, me := do(t, e, http.MethodGet, "/api/v1/admin/me", "", tok)
	if code != 200 || me["email"] != email {
		t.Fatalf("me: %d %+v", code, me)
	}

	// --- clients CRUD
	code, cli := do(t, e, http.MethodPost, "/api/v1/admin/clients", `{"name":"Acme"}`, tok)
	if code != http.StatusCreated || !strings.HasPrefix(cli["id"].(string), "cli_") {
		t.Fatalf("create client: %d %+v", code, cli)
	}
	cliID := cli["id"].(string)
	code, cli = do(t, e, http.MethodPatch, "/api/v1/admin/clients/"+cliID, `{"status":"disabled"}`, tok)
	if code != 200 || cli["status"] != "disabled" {
		t.Fatalf("patch client: %d %+v", code, cli)
	}
	code, rows := doList(t, e, "/api/v1/admin/clients", tok)
	if code != 200 || len(rows) == 0 {
		t.Fatalf("list clients: %d", code)
	}
	if code, _ := do(t, e, http.MethodGet, "/api/v1/admin/clients/nope", "", tok); code != http.StatusNotFound {
		t.Fatalf("missing client: %d, want 404", code)
	}

	// --- customers CRUD
	code, cust := do(t, e, http.MethodPost, "/api/v1/admin/clients/"+cliID+"/customers",
		`{"name":"Acme Cust","external_ref":"ext-1"}`, tok)
	if code != http.StatusCreated || cust["client_id"] != cliID {
		t.Fatalf("create customer: %d %+v", code, cust)
	}
	custID := cust["id"].(string)
	code, cust = do(t, e, http.MethodPatch, "/api/v1/admin/customers/"+custID,
		`{"webhook_url":"https://h.local/wh"}`, tok)
	if code != 200 || cust["webhook_url"] != "https://h.local/wh" {
		t.Fatalf("patch customer: %d %+v", code, cust)
	}
	code, rows = doList(t, e, "/api/v1/admin/clients/"+cliID+"/customers", tok)
	if code != 200 || len(rows) != 1 {
		t.Fatalf("list customers: %d %d rows", code, len(rows))
	}

	// --- template masters: create, 409 dup, version publish
	code, m1 := do(t, e, http.MethodPost, "/api/v1/admin/clients/"+cliID+"/template-masters",
		`{"name":"invoice","html":"<h1>{{title}}</h1>","css":"h1{}","defaults":{"toggles":{"hdr":true}}}`, tok)
	if code != http.StatusCreated || m1["version"] != 1.0 {
		t.Fatalf("create master: %d %+v", code, m1)
	}
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/clients/"+cliID+"/template-masters",
		`{"name":"invoice","html":"x"}`, tok); code != http.StatusConflict {
		t.Fatalf("dup master: got %d, want 409", code)
	}
	code, m2 := do(t, e, http.MethodPost, "/api/v1/admin/clients/"+cliID+"/template-masters/invoice/versions",
		`{"html":"<h1>v2</h1>"}`, tok)
	if code != http.StatusCreated || m2["version"] != 2.0 {
		t.Fatalf("publish v2: %d %+v", code, m2)
	}
	code, rows = doList(t, e, "/api/v1/admin/clients/"+cliID+"/template-masters", tok)
	if code != 200 || len(rows) != 2 {
		t.Fatalf("master history: %d rows=%d", code, len(rows))
	}
	code, m := do(t, e, http.MethodGet, "/api/v1/admin/template-masters/"+m2["id"].(string), "", tok)
	if code != 200 || m["version"] != 2.0 {
		t.Fatalf("get master: %d %+v", code, m)
	}

	// --- variants
	code, vr := do(t, e, http.MethodPut, "/api/v1/admin/template-variants",
		`{"master_id":"`+m2["id"].(string)+`","customer_id":"`+custID+`","overlay":{"toggles":{"hdr":false}}}`, tok)
	if code != 200 || vr["customer_id"] != custID {
		t.Fatalf("upsert variant: %d %+v", code, vr)
	}
	code, vr2 := do(t, e, http.MethodGet,
		"/api/v1/admin/template-variants?master_id="+m2["id"].(string)+"&customer_id="+custID, "", tok)
	if code != 200 || vr2["overlay"].(map[string]any)["toggles"].(map[string]any)["hdr"] != false {
		t.Fatalf("get variant: %d %+v", code, vr2)
	}
	code, rows = doList(t, e, "/api/v1/admin/customers/"+custID+"/variants", tok)
	if code != 200 || len(rows) != 1 {
		t.Fatalf("customer variants: %d rows=%d", code, len(rows))
	}

	// --- api keys: create (plaintext once), rotate (old dead), revoke
	code, k := do(t, e, http.MethodPost, "/api/v1/admin/clients/"+cliID+"/api-keys",
		`{"label":"k1","scopes":["generate"]}`, tok)
	if code != http.StatusCreated || !strings.HasPrefix(k["key"].(string), "isppr_") {
		t.Fatalf("create key: %d %+v", code, k)
	}
	keyID := k["id"].(string)
	plain := k["key"].(string)

	code, kr := do(t, e, http.MethodPost, "/api/v1/admin/api-keys/"+keyID+"/rotate", "", tok)
	if code != 200 || kr["key"] == nil || kr["rotated_at"] == nil {
		t.Fatalf("rotate: %d %+v", code, kr)
	}
	// Old plaintext must no longer resolve.
	if _, err := q.GetClientByAPIKeyHash(ctx, sha256hex(plain)); err == nil {
		t.Fatal("rotated-out key still resolves")
	}
	// Keys list must not leak hashes.
	code, keys := doList(t, e, "/api/v1/admin/clients/"+cliID+"/api-keys", tok)
	if code != 200 || len(keys) != 1 || keys[0]["key_hash"] != nil {
		t.Fatalf("list keys: %d %+v", code, keys)
	}
	code, kv := do(t, e, http.MethodPost, "/api/v1/admin/api-keys/"+keyID+"/revoke", "", tok)
	if code != 200 || kv["revoked_at"] == nil {
		t.Fatalf("revoke: %d %+v", code, kv)
	}

	// --- usage + requests list: seed two request rows, verify aggregation
	// shape and the filtered list surface.
	for i, row := range []struct{ status, ms string }{
		{"COMPLETED", "10"}, {"FAILED", "40"},
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO generation_requests (id, client_id, customer_id, package_name, status, error, metrics)
			 VALUES ($1, $2, $3, 'pkg', $4,
			         CASE WHEN $4 = 'FAILED' THEN 'render blew up' ELSE NULL END,
			         $5::jsonb)
			 ON CONFLICT (id) DO NOTHING`,
			fmt.Sprintf("gr_t%d_%s", i, sfx), cliID, custID, row.status,
			`{"total_duration_ms": `+row.ms+`}`); err != nil {
			t.Fatal(err)
		}
	}
	code, u := do(t, e, http.MethodGet, "/api/v1/admin/usage?days=1", "", tok)
	if code != 200 {
		t.Fatalf("usage: %d", code)
	}
	lat := u["latency"].(map[string]any)
	// Aggregates span all clients — earlier test runs and the dev stack's
	// real renders contribute samples at arbitrary durations, so only the
	// shape and a nonzero sample count are contract here.
	if lat["samples"].(float64) < 2 || lat["p50_ms"].(float64) < 0 || lat["p95_ms"].(float64) < lat["p50_ms"].(float64) {
		t.Fatalf("usage latency: %+v", lat)
	}
	days := u["by_day"].([]any)
	found := false
	for _, d := range days {
		dd := d.(map[string]any)
		if dd["client_id"] == cliID && dd["requests"] == 2.0 && dd["failed"] == 1.0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("usage by_day missing seeded rows: %+v", days)
	}

	// Per-client usage: samples must be exactly our two rows (other test
	// runs contribute other clients only).
	code, cu := do(t, e, http.MethodGet, "/api/v1/admin/usage?days=1&client_id="+cliID, "", tok)
	if code != 200 {
		t.Fatalf("client usage: %d", code)
	}
	clat := cu["latency"].(map[string]any)
	if clat["samples"].(float64) != 2 || clat["p50_ms"].(float64) < 10 || clat["p95_ms"].(float64) > 40 {
		t.Fatalf("client usage latency: %+v", clat)
	}
	for _, d := range cu["by_day"].([]any) {
		if d.(map[string]any)["client_id"] != cliID {
			t.Fatalf("client-filtered by_day leaked another client: %+v", d)
		}
	}

	// --- requests list: client/status filters isolate the seeded pair.
	code, reqs := doList(t, e, "/api/v1/admin/requests?client_id="+cliID, tok)
	if code != 200 || len(reqs) != 2 {
		t.Fatalf("requests by client: %d rows=%d", code, len(reqs))
	}
	code, reqs = doList(t, e, "/api/v1/admin/requests?client_id="+cliID+"&status=FAILED", tok)
	if code != 200 || len(reqs) != 1 || reqs[0]["status"] != "FAILED" {
		t.Fatalf("requests failed filter: %d %+v", code, reqs)
	}
	if reqs[0]["error"] != "render blew up" || reqs[0]["webhook_status"] == nil {
		t.Fatalf("failed request row shape: %+v", reqs[0])
	}
	code, reqs = doList(t, e, "/api/v1/admin/requests?client_id=nope", tok)
	if code != 200 || len(reqs) != 0 {
		t.Fatalf("unknown client filter: %d rows=%d", code, len(reqs))
	}
	if code, _ := do(t, e, http.MethodGet, "/api/v1/admin/requests?limit=0", "", tok); code != http.StatusBadRequest {
		t.Fatalf("bad limit: got %d, want 400", code)
	}
	code, reqs = doList(t, e, "/api/v1/admin/requests?client_id="+cliID+"&limit=1", tok)
	if code != 200 || len(reqs) != 1 {
		t.Fatalf("limit: %d rows=%d", code, len(reqs))
	}

	// --- logout kills the session
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/logout", "", tok); code != http.StatusNoContent {
		t.Fatalf("logout: %d", code)
	}
	if code, _ := do(t, e, http.MethodGet, "/api/v1/admin/me", "", tok); code != http.StatusUnauthorized {
		t.Fatalf("post-logout: got %d, want 401", code)
	}
}

func TestAdminRoleGate(t *testing.T) {
	e, q, _ := testServer(t, nil)
	ctx := context.Background()

	// A viewer can log in and read, but writes are 403 — roles gate
	// destructive actions server-side, not just in the portal.
	sfx := fmt.Sprintf("%d", rand.Int63())
	email := "v" + sfx + "@isppr.local"
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	_, err := q.CreateAdminUser(ctx, store.CreateAdminUserParams{
		ID: "adm_v" + sfx, Email: email, Name: "V", PasswordHash: string(hash), Role: "viewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	code, login := do(t, e, http.MethodPost, "/api/v1/admin/login",
		fmt.Sprintf(`{"email":%q,"password":"pw"}`, email), "")
	if code != http.StatusOK {
		t.Fatalf("viewer login: %d", code)
	}
	tok := login["token"].(string)

	if code, _ := do(t, e, http.MethodGet, "/api/v1/admin/clients", "", tok); code != http.StatusOK {
		t.Fatalf("viewer read: got %d, want 200", code)
	}
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/clients",
		`{"name":"x"}`, tok); code != http.StatusForbidden {
		t.Fatalf("viewer write: got %d, want 403", code)
	}
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/logout", "", tok); code != http.StatusNoContent {
		t.Fatalf("viewer logout: got %d, want 204", code)
	}
}

// TestVariantPreview exercises the draft-overlay preview path end-to-end:
// the overlay is applied in memory (never upserted) and the fake Gotenberg
// sees the toggled/patched document.
func TestVariantPreview(t *testing.T) {
	var gotBody []byte
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-fake"))
	}))
	t.Cleanup(fake.Close)
	rpool, err := render.NewPool([]string{fake.URL})
	if err != nil {
		t.Fatal(err)
	}
	e, q, _ := testServer(t, rpool)
	ctx := context.Background()

	sfx := fmt.Sprintf("%d", rand.Int63())
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	if _, err := q.CreateAdminUser(ctx, store.CreateAdminUserParams{
		ID: "adm_p" + sfx, Email: "p" + sfx + "@isppr.local",
		Name: "P", PasswordHash: string(hash), Role: "admin",
	}); err != nil {
		t.Fatal(err)
	}
	code, login := do(t, e, http.MethodPost, "/api/v1/admin/login",
		fmt.Sprintf(`{"email":%q,"password":"pw"}`, "p"+sfx+"@isppr.local"), "")
	if code != http.StatusOK {
		t.Fatalf("login: %d", code)
	}
	tok := login["token"].(string)

	code, cli := do(t, e, http.MethodPost, "/api/v1/admin/clients", `{"name":"PV"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("client: %d", code)
	}
	cliID := cli["id"].(string)
	code, m := do(t, e, http.MethodPost, "/api/v1/admin/clients/"+cliID+"/template-masters",
		`{"name":"doc","html":"<h1>{{doc_title}}</h1>{{#if blocks.extra}}<p>EXTRA</p>{{/if}}",
		 "css":"h1{color:black}","defaults":{"toggles":{"extra":false}}}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("master: %d", code)
	}
	masterID := m["id"].(string)

	// Draft overlay flips the toggle + appends a css_patch — the fake
	// Gotenberg must see both, and no variant row is written.
	code, _ = do(t, e, http.MethodPost, "/api/v1/admin/template-variants/preview",
		`{"master_id":"`+masterID+`","overlay":{"toggles":{"extra":true},"css_patch":".patch{color:red}"},
		  "data":{"doc_title":"Hi"}}`, tok)
	if code != http.StatusOK {
		t.Fatalf("preview: %d", code)
	}
	body := string(gotBody)
	if !strings.Contains(body, "EXTRA") || !strings.Contains(body, ".patch{color:red}") || !strings.Contains(body, "Hi") {
		t.Fatalf("resolved doc missing overlay/data effects: %s", body)
	}

	// Nothing persisted: no variant row exists for this master.
	if code, _ := do(t, e, http.MethodGet,
		"/api/v1/admin/template-variants?master_id="+masterID+"&customer_id=cust_none", "", tok); code != http.StatusNotFound {
		t.Fatalf("preview persisted a variant: got %d, want 404", code)
	}

	// Bad payload shapes → 400, unknown master → 404.
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/template-variants/preview",
		`{"master_id":"`+masterID+`","overlay":"not-json"}`, tok); code != http.StatusBadRequest {
		t.Fatalf("bad overlay: got %d, want 400", code)
	}
	if code, _ := do(t, e, http.MethodPost, "/api/v1/admin/template-variants/preview",
		`{"master_id":"tmpl_nope","overlay":{}}`, tok); code != http.StatusNotFound {
		t.Fatalf("unknown master: got %d, want 404", code)
	}
}
