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
	"github.com/jradams472/isppr/internal/server"
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

	e := server.New(cfg, store.New(pool), pool)

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
