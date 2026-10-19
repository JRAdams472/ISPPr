package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool migrates the schema and returns a live pool, skipping when no
// Postgres is reachable (CI provides one via service container).
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://isppr:isppr@localhost:5433/isppr?sslmode=disable"
	}
	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := NewPool(context.Background(), dsn)
	if err != nil {
		t.Skipf("postgres unavailable (%v) — start via docker compose up db", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testID(prefix string) string {
	// Random, not sequential — `go test` runs package binaries in
	// parallel against the shared database, so a per-process sequence
	// can't prevent cross-process ID collisions.
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s_t%x", prefix, b)
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestClientCustomerKeyRoundTrip(t *testing.T) {
	q := New(testPool(t))
	ctx := context.Background()

	client, err := q.CreateClient(ctx, CreateClientParams{
		ID: testID("cli"), Name: "Roundtrip Corp", Status: "active",
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	customer, err := q.CreateCustomer(ctx, CreateCustomerParams{
		ID: testID("cust"), ClientID: client.ID, Name: "Roundtrip Customer",
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if customer.ClientID != client.ID {
		t.Fatalf("customer client mismatch: %q", customer.ClientID)
	}

	keyHash := sha256hex("rt_" + client.ID)
	key, err := q.CreateAPIKey(ctx, CreateAPIKeyParams{
		ID: testID("key"), ClientID: client.ID, KeyHash: keyHash,
		Label: "rt", Scopes: []string{"generate"},
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	// Chain readback: key hash -> owning client -> its customer.
	gotClient, err := q.GetClientByAPIKeyHash(ctx, keyHash)
	if err != nil || gotClient.ID != client.ID {
		t.Fatalf("client by key hash: %v", err)
	}
	gotCustomer, err := q.GetCustomerForClient(ctx, GetCustomerForClientParams{
		ID: customer.ID, ClientID: client.ID,
	})
	if err != nil || gotCustomer.ID != customer.ID {
		t.Fatalf("customer for client: %v", err)
	}

	// Tenant isolation: another client must not see this customer.
	foreign := GetCustomerForClientParams{ID: customer.ID, ClientID: "cli_other"}
	if _, err := q.GetCustomerForClient(ctx, foreign); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for foreign client, got %v", err)
	}

	// Revocation removes the key from the auth path.
	if _, err := q.RevokeAPIKey(ctx, key.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := q.GetClientByAPIKeyHash(ctx, keyHash); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after revoke, got %v", err)
	}
}

func TestSeedDemoTenant(t *testing.T) {
	pool := testPool(t)
	q := New(pool)
	ctx := context.Background()

	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Idempotent: running twice must not error.
	if err := Seed(ctx, pool); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	adm, err := q.GetAdminUserByEmail(ctx, "admin@isppr.local")
	if err != nil || adm.Role != "admin" {
		t.Fatalf("demo admin user: %v", err)
	}
	cli, err := q.GetClientByAPIKeyHash(ctx, sha256hex("isppr_dev_demo"))
	if err != nil || cli.ID != "cli_demo" {
		t.Fatalf("demo client by key hash: %v", err)
	}
}
