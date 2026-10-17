package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewPoolBadDSN(t *testing.T) {
	if _, err := NewPool(context.Background(), "postgres://bad host"); err == nil {
		t.Fatal("expected error for malformed DSN")
	}
	if _, err := NewPool(context.Background(), "postgres://isppr:isppr@localhost:1/nope?sslmode=disable&connect_timeout=1"); err == nil {
		t.Fatal("expected error for unreachable database")
	}
}

func TestRunMigrationsBadDSN(t *testing.T) {
	if err := RunMigrations("postgres://"); err == nil {
		t.Fatal("expected error for unopenable database URL")
	}
}

func TestSeedFailsWithoutDB(t *testing.T) {
	// pgxpool.New is lazy — Exec against an unreachable server must error.
	pool, err := pgxpool.New(context.Background(), "postgres://isppr:isppr@localhost:1/nope?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("pool construct: %v", err)
	}
	defer pool.Close()
	if err := Seed(context.Background(), pool); err == nil {
		t.Fatal("expected seed error on unreachable database")
	}
}
