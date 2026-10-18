package server_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/server"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/tenancy"
	"github.com/labstack/echo/v4"
)

// Request-level coverage for the X-API-Key tenant surface. Runs against
// real Postgres (compose db locally, service container in CI).

var tenantSeq int64

func tid(prefix string) string {
	tenantSeq++
	return fmt.Sprintf("%s_t%d_%d", prefix, time.Now().UnixNano()%1e6, tenantSeq)
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func testEnv(t *testing.T) (*echo.Echo, *store.Queries) {
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
		store.New(pool), pool), store.New(pool)
}

// tenantFixture builds two tenants each owning a customer, a template master,
// and a key — plus revoked and scope-less keys on tenant A.
type tenantFixture struct {
	clientA, clientB string
	custA, custB     string
	masterA, masterB string
	keyA, keyRevoked string // plaintext values
	keyNoScope       string
}

func seedTenants(t *testing.T, q *store.Queries) tenantFixture {
	t.Helper()
	ctx := context.Background()
	f := tenantFixture{
		clientA: tid("cli_a"), clientB: tid("cli_b"),
		custA: tid("cu_a"), custB: tid("cu_b"),
		masterA: tid("tm_a"), masterB: tid("tm_b"),
		keyA: "k_a_" + tid("p"), keyRevoked: "k_rv_" + tid("p"), keyNoScope: "k_ns_" + tid("p"),
	}
	for _, c := range []struct{ id string }{{f.clientA}, {f.clientB}} {
		if _, err := q.CreateClient(ctx, store.CreateClientParams{ID: c.id, Name: c.id, Status: "active"}); err != nil {
			t.Fatalf("client %s: %v", c.id, err)
		}
	}
	for _, cu := range []struct{ id, cli string }{{f.custA, f.clientA}, {f.custB, f.clientB}} {
		if _, err := q.CreateCustomer(ctx, store.CreateCustomerParams{
			ID: cu.id, ClientID: cu.cli, Name: cu.id,
			ExternalRef: pgtype.Text{String: cu.id, Valid: true},
		}); err != nil {
			t.Fatalf("customer %s: %v", cu.id, err)
		}
	}
	for _, m := range []struct{ id, cli string }{{f.masterA, f.clientA}, {f.masterB, f.clientB}} {
		if _, err := q.CreateTemplateMaster(ctx, store.CreateTemplateMasterParams{
			ID: m.id, ClientID: m.cli, Name: m.id,
			Html: "<p>x</p>", Css: "p{}", Version: 1, Defaults: []byte("{}"),
		}); err != nil {
			t.Fatalf("master %s: %v", m.id, err)
		}
	}
	keys := []struct {
		id, plaintext string
		scopes        []string
		revoke        bool
	}{
		{tid("key_a"), f.keyA, []string{"generate"}, false},
		{tid("key_rv"), f.keyRevoked, []string{"generate"}, true},
		{tid("key_ns"), f.keyNoScope, []string{"read"}, false},
	}
	for _, k := range keys {
		row, err := q.CreateAPIKey(ctx, store.CreateAPIKeyParams{
			ID: k.id, ClientID: f.clientA, KeyHash: sha256hex(k.plaintext),
			Label: "test", Scopes: k.scopes,
		})
		if err != nil {
			t.Fatalf("api key %s: %v", k.id, err)
		}
		if k.revoke {
			if _, err := q.RevokeAPIKey(ctx, row.ID); err != nil {
				t.Fatalf("revoke %s: %v", k.id, err)
			}
		}
	}
	return f
}

func pingReq(t *testing.T, e *echo.Echo, key string) (int, string, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages/ping", nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, rec.Header().Get(echo.HeaderContentType), body
}

func TestTenantAuth_MissingKey(t *testing.T) {
	e, _ := testEnv(t)
	code, ct, body := pingReq(t, e, "")
	if code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d (%v)", code, body)
	}
	if ct != "application/problem+json" {
		t.Fatalf("want problem+json, got %q", ct)
	}
	if body["status"].(float64) != 401 {
		t.Fatalf("problem status: %v", body)
	}
}

func TestTenantAuth_InvalidKey(t *testing.T) {
	e, _ := testEnv(t)
	code, _, _ := pingReq(t, e, "not-a-real-key")
	if code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", code)
	}
}

func TestTenantAuth_RevokedKey(t *testing.T) {
	e, q := testEnv(t)
	f := seedTenants(t, q)
	code, _, _ := pingReq(t, e, f.keyRevoked)
	if code != http.StatusUnauthorized {
		t.Fatalf("revoked key: want 401, got %d", code)
	}
}

// Rotated keys are the model's "expired" case: rotation replaces the stored
// hash, so previously-issued material stops resolving (no expires_at column
// by design — revocation/rotation is the expiry mechanism).
func TestTenantAuth_RotatedKey(t *testing.T) {
	e, q := testEnv(t)
	f := seedTenants(t, q)
	ctx := context.Background()
	old, err := q.GetAPIKeyByHash(ctx, sha256hex(f.keyA))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if _, err := q.RotateAPIKey(ctx, store.RotateAPIKeyParams{
		ID: old.ID, KeyHash: sha256hex("new_" + tid("p")),
	}); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	code, _, _ := pingReq(t, e, f.keyA) // old plaintext
	if code != http.StatusUnauthorized {
		t.Fatalf("rotated key: want 401, got %d", code)
	}
}

func TestTenantAuth_DisabledClient(t *testing.T) {
	e, q := testEnv(t)
	f := seedTenants(t, q)
	if _, err := q.SetClientStatus(context.Background(), store.SetClientStatusParams{
		ID: f.clientA, Status: "suspended",
	}); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	code, ct, _ := pingReq(t, e, f.keyA)
	if code != http.StatusForbidden {
		t.Fatalf("disabled tenant: want 403, got %d", code)
	}
	if ct != "application/problem+json" {
		t.Fatalf("want problem+json, got %q", ct)
	}
}

func TestTenantAuth_MissingScope(t *testing.T) {
	e, q := testEnv(t)
	f := seedTenants(t, q)
	code, ct, _ := pingReq(t, e, f.keyNoScope)
	if code != http.StatusForbidden {
		t.Fatalf("scope-less key: want 403, got %d", code)
	}
	if ct != "application/problem+json" {
		t.Fatalf("want problem+json, got %q", ct)
	}
}

func TestTenantAuth_HappyPath(t *testing.T) {
	e, q := testEnv(t)
	f := seedTenants(t, q)
	code, _, body := pingReq(t, e, f.keyA)
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%v)", code, body)
	}
	if body["client_id"] != f.clientA {
		t.Fatalf("tenant context resolved wrong client: %v", body)
	}
	if body["api_key_id"] == "" {
		t.Fatalf("api key context missing: %v", body)
	}
	// Auth recorded last_used_at.
	key, err := q.GetAPIKeyByHash(context.Background(), sha256hex(f.keyA))
	if err != nil || !key.LastUsedAt.Valid {
		t.Fatalf("last_used_at not stamped: %v %v", key.LastUsedAt, err)
	}
}

// authzEcho mounts a probe route that exercises RequireOwnership the way the
// generation handler will: customer_id + template_id[] from the payload.
func authzEcho(t *testing.T, q *store.Queries) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = server.ProblemHandler
	e.GET("/authz", func(c echo.Context) error {
		if err := tenancy.RequireOwnership(c, q,
			c.QueryParam("customer_id"), c.QueryParams()["template_id"]); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}, tenancy.Middleware(q, "generate"))
	return e
}

func authzReq(t *testing.T, e *echo.Echo, key, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Header().Get(echo.HeaderContentType)
}

func TestOwnership_OwnResources(t *testing.T) {
	_, q := testEnv(t)
	f := seedTenants(t, q)
	e := authzEcho(t, q)
	code, _ := authzReq(t, e, f.keyA,
		fmt.Sprintf("/authz?customer_id=%s&template_id=%s", f.custA, f.masterA))
	if code != http.StatusNoContent {
		t.Fatalf("own resources: want 204, got %d", code)
	}
}

func TestOwnership_CrossTenantCustomer(t *testing.T) {
	_, q := testEnv(t)
	f := seedTenants(t, q)
	e := authzEcho(t, q)
	code, ct := authzReq(t, e, f.keyA,
		fmt.Sprintf("/authz?customer_id=%s&template_id=%s", f.custB, f.masterA))
	if code != http.StatusForbidden {
		t.Fatalf("cross-tenant customer: want 403, got %d", code)
	}
	if ct != "application/problem+json" {
		t.Fatalf("want problem+json, got %q", ct)
	}
}

func TestOwnership_CrossTenantTemplate(t *testing.T) {
	_, q := testEnv(t)
	f := seedTenants(t, q)
	e := authzEcho(t, q)
	code, _ := authzReq(t, e, f.keyA,
		fmt.Sprintf("/authz?customer_id=%s&template_id=%s", f.custA, f.masterB))
	if code != http.StatusForbidden {
		t.Fatalf("cross-tenant template: want 403, got %d", code)
	}
}

func TestOwnership_GlobalDenyOnAnyMismatch(t *testing.T) {
	_, q := testEnv(t)
	f := seedTenants(t, q)
	e := authzEcho(t, q)
	// One owned template + one foreign template -> whole request denied.
	code, _ := authzReq(t, e, f.keyA,
		fmt.Sprintf("/authz?customer_id=%s&template_id=%s&template_id=%s",
			f.custA, f.masterA, f.masterB))
	if code != http.StatusForbidden {
		t.Fatalf("mixed ownership: want global 403, got %d", code)
	}
}

func TestOwnership_MissingTemplate(t *testing.T) {
	_, q := testEnv(t)
	f := seedTenants(t, q)
	e := authzEcho(t, q)
	code, _ := authzReq(t, e, f.keyA,
		fmt.Sprintf("/authz?customer_id=%s&template_id=%s", f.custA, "tm_nonexistent"))
	if code != http.StatusForbidden {
		t.Fatalf("unknown template: want 403, got %d", code)
	}
}
