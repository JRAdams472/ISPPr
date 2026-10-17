// Package store holds the sqlc-generated repositories plus the pool,
// migration, and seed wiring around them. (db.go, models.go, *.sql.go are
// sqlc output — hand-edits belong in pool.go / migrate.go / seed.go.)
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool connects to Postgres and verifies liveness. The returned pool
// satisfies DBTX and is the Queries' backing store.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
