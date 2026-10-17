package store

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	isppr "github.com/jradams472/isppr"
)

// Seed applies the demo seed SQL (seed/*.sql in name order). Idempotent —
// every statement uses ON CONFLICT DO NOTHING.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.ReadDir(isppr.Assets, "seed")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sql, err := isppr.Assets.ReadFile("seed/" + e.Name())
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("seed %s: %w", e.Name(), err)
		}
	}
	return nil
}
