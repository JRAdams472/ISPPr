package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/jobs"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/server"
	"github.com/jradams472/isppr/internal/storage"
	"github.com/jradams472/isppr/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Migrations are append-only and applied at boot; the process refuses to
	// serve on a schema it can't reach.
	if err := store.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Single pool for the request path (queries + admin sessions).
	pool, err := store.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if cfg.RunSeed {
		if err := store.Seed(context.Background(), pool); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Print("seed: demo tenant applied")
	}

	rpool, err := render.NewPool(cfg.GotenbergEndpoints,
		render.WithTimeout(cfg.RenderTimeout))
	if err != nil {
		log.Fatalf("render pool: %v", err)
	}
	orch := jobs.New(store.New(pool), rpool,
		&storage.LocalStore{Dir: cfg.StorageDir},
		jobs.WithMaxDocs(cfg.MaxDocsPerPkg),
		jobs.WithSyncBudget(cfg.SyncBudget),
		jobs.WithWorkers(cfg.WorkerCount))
	go orch.Start(context.Background())

	e := server.New(cfg, store.New(pool), pool, orch)

	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			e.Logger.Fatal("listen: ", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		e.Logger.Fatal("shutdown: ", err)
	}
}
