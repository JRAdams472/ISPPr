package templates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jradams472/isppr/internal/store"
)

func testQueries(t *testing.T) (*store.Queries, *pgxpool.Pool) {
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
	return store.New(pool), pool
}

var tidSeq atomic.Int64

func tid(prefix string) string {
	// Windows wall-clock granularity is ~15ms — sequence guards rapid calls.
	return fmt.Sprintf("%s_t%d_%d", prefix,
		time.Now().UnixNano()%1_000_000_000, tidSeq.Add(1))
}

// Full documented chain: master persists, versions resolve latest-first,
// variant overlay applies, foreign tenants are invisible.
func TestResolveForChain(t *testing.T) {
	q, _ := testQueries(t)
	ctx := context.Background()

	cli, err := q.CreateClient(ctx, store.CreateClientParams{
		ID: tid("cli"), Name: "T", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	cust, err := q.CreateCustomer(ctx, store.CreateCustomerParams{
		ID: tid("cust"), ClientID: cli.ID, Name: "C"})
	if err != nil {
		t.Fatal(err)
	}

	// Two versions of the same master; resolution must pick v2.
	m1, err := q.CreateTemplateMaster(ctx, store.CreateTemplateMasterParams{
		ID: tid("tmpl"), ClientID: cli.ID, Name: "invoice",
		Html: "<h1>v1</h1>", Css: "v1{}", Version: 1,
		Defaults: json.RawMessage(`{"toggles":{"header":true}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m2, err := q.CreateTemplateMaster(ctx, store.CreateTemplateMasterParams{
		ID: tid("tmpl"), ClientID: cli.ID, Name: "invoice",
		Html: "<h1>v2</h1>", Css: "v2{}", Version: 2,
		Defaults: json.RawMessage(`{"branding":{"logo":"m.png"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Variant pinned to the v1 row must still apply to v2 (any-version rule).
	if _, err = q.UpsertVariant(ctx, store.UpsertVariantParams{
		ID: tid("var"), MasterID: m1.ID, CustomerID: cust.ID,
		Overlay: json.RawMessage(`{"css_patch":"p{}","toggles":{"header":false},"branding":{"logo":"c.png"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveFor(ctx, q, cli.ID, cust.ID, "invoice")
	if err != nil {
		t.Fatal(err)
	}
	if got.HTML != m2.Html {
		t.Fatalf("expected latest master html, got %q", got.HTML)
	}
	if !strings.Contains(got.CSS, "font-family: sans-serif") ||
		!strings.HasSuffix(got.CSS, "v2{}\np{}") {
		t.Fatalf("css chain wrong:\n%q", got.CSS)
	}
	blocks, ok := got.Context["blocks"].(map[string]bool)
	if !ok || blocks["header"] != false {
		t.Fatalf("variant toggle not applied: %+v", got.Context["blocks"])
	}
	if got.Context["branding"].(map[string]string)["logo"] != "c.png" {
		t.Fatalf("variant branding not applied: %+v", got.Context["branding"])
	}

	// Unknown template name → embedded base.
	base, err := ResolveFor(ctx, q, cli.ID, cust.ID, "no-such-template")
	if err != nil {
		t.Fatal(err)
	}
	if base.HTML == got.HTML || base.CSS == got.CSS {
		t.Fatal("expected base fallback")
	}

	// Isolation: another customer's resolve sees the master but not the variant.
	other, err := ResolveFor(ctx, q, cli.ID, "cust_other", "invoice")
	if err != nil {
		t.Fatal(err)
	}
	if other.Context["branding"].(map[string]string)["logo"] != "m.png" {
		t.Fatalf("foreign customer leaked variant branding: %+v", other.Context["branding"])
	}

	// Isolation: foreign client sees neither master nor variant → base.
	foreign, err := ResolveFor(ctx, q, "cli_other", cust.ID, "invoice")
	if err != nil {
		t.Fatal(err)
	}
	if foreign.HTML != base.HTML {
		t.Fatal("foreign client resolved another tenant's master")
	}
}

func TestNextMasterVersion(t *testing.T) {
	q, _ := testQueries(t)
	ctx := context.Background()
	cli, err := q.CreateClient(ctx, store.CreateClientParams{
		ID: tid("cli"), Name: "T", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	n, err := q.NextMasterVersion(ctx, store.NextMasterVersionParams{
		ClientID: cli.ID, Name: "new"})
	if err != nil || n != 1 {
		t.Fatalf("first version should be 1, got %d (%v)", n, err)
	}
	_, _ = q.CreateTemplateMaster(ctx, store.CreateTemplateMasterParams{
		ID: tid("tmpl"), ClientID: cli.ID, Name: "new",
		Html: "x", Css: "", Version: 1, Defaults: json.RawMessage(`{}`)})
	n, _ = q.NextMasterVersion(ctx, store.NextMasterVersionParams{
		ClientID: cli.ID, Name: "new"})
	if n != 2 {
		t.Fatalf("next version should be 2, got %d", n)
	}
}

func TestGetVariantMiss(t *testing.T) {
	q, _ := testQueries(t)
	_, err := q.GetVariant(context.Background(), store.GetVariantParams{
		MasterID: "nope", CustomerID: "nope"})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}
