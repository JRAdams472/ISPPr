package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/jobs"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/server"
	"github.com/jradams472/isppr/internal/storage"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Exporters are environment-gated: with no endpoints configured this
	// is an inert no-op and the process runs exactly as before.
	otelShutdown, err := telemetry.Setup(context.Background(), telemetry.Config{
		ServiceName:    cfg.OtelServiceName,
		TracesEndpoint: cfg.OtelTracesEndpoint,
		LogsEndpoint:   cfg.OtelLogsEndpoint,
	})
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(shCtx)
	}()

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

	var backend storage.Backend
	switch cfg.StorageBackend {
	case "s3":
		s3, err := storage.NewS3(context.Background(), storage.S3Config{
			Endpoint:       cfg.S3Endpoint,
			PublicEndpoint: cfg.S3PublicEndpoint,
			Bucket:         cfg.S3Bucket,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			Region:         cfg.S3Region,
			PresignTTL:     cfg.S3PresignTTL,
		})
		if err != nil {
			log.Fatalf("s3 store: %v", err)
		}
		backend = s3
	default:
		backend = &storage.LocalStore{Dir: cfg.StorageDir}
	}
	orch := jobs.New(store.New(pool), rpool, backend,
		jobs.WithMaxDocs(cfg.MaxDocsPerPkg),
		jobs.WithSyncBudget(cfg.SyncBudget),
		jobs.WithWorkers(cfg.WorkerCount),
		jobs.WithJobTimeout(cfg.JobTimeout),
		jobs.WithMaxBody(cfg.MaxBodyBytes),
		jobs.WithSingleDocSkipZip(cfg.SingleDocSkipZip),
		jobs.WithWebhook(cfg.WebhookSecret, cfg.WebhookMaxAttempts))
	// Workers run on their own cancellable context so shutdown can drain
	// them after HTTP intake stops.
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	go orch.Start(workerCtx)

	e := server.New(cfg, store.New(pool), pool, orch, rpool, backend)

	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			e.Logger.Fatal("listen: ", err)
		}
	}()

	// docker stop and k8s rollouts deliver SIGTERM — os.Interrupt alone
	// would let the process run until SIGKILL, stranding claimed jobs.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	// Graceful order: stop accepting requests, then stop claiming and let
	// in-flight jobs finish within the grace window; survivors are
	// requeued by Shutdown so no job is left in CLAIMED. The two phases
	// get independent budgets so HTTP drain can't starve job requeue.
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer httpCancel()
	if err := e.Shutdown(httpCtx); err != nil {
		slog.Error("http shutdown", "err", err)
	}
	stopWorkers()
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer drainCancel()
	orch.Shutdown(drainCtx)
}
