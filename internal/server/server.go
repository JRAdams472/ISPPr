package server

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jradams472/isppr/internal/admin"
	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/jobs"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/storage"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/tenancy"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"golang.org/x/time/rate"
)

// New builds the Echo engine with global middleware and the scaffold routes.
// q/pool may be nil for tests that don't touch the admin surface; rpool may
// be nil when the variant-preview route isn't exercised; st may be nil when
// output download isn't exercised.
func New(cfg config.Config, q *store.Queries, pool *pgxpool.Pool, orch *jobs.Orchestrator, rpool *render.Pool, st storage.Backend) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ProblemHandler

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	// Ingress spans only exist when an exporter is configured — off mode
	// keeps the middleware chain identical to before.
	if cfg.OtelTracesEndpoint != "" {
		e.Use(otelecho.Middleware(cfg.OtelServiceName))
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{cfg.CORSOrigin},
	}))

	e.GET("/healthz", health)
	e.GET("/api/health", health)

	// Versioned API surface: admin management routes land here; the tenant
	// X-API-Key generation surface hangs off /packages (generate lands in
	// MPE-32).
	v1 := e.Group("/api/v1")
	v1.GET("/status", v1Status)
	if q != nil {
		admin.New(q, pool, rpool, st).Mount(v1)
		// Per-client token bucket, keyed on the resolved tenant — the
		// extractor runs after tenancy middleware has authenticated.
		// A non-positive RATE_LIMIT_RPS disables the limiter (tests run
		// config.Config{} and must not be throttled).
		var tenantMW []echo.MiddlewareFunc
		tenantMW = append(tenantMW, tenancy.Middleware(q, "generate"))
		if cfg.RateLimitRPS > 0 {
			tenantMW = append(tenantMW, middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
				IdentifierExtractor: func(c echo.Context) (string, error) {
					if cl := tenancy.Client(c); cl != nil {
						return "cli:" + cl.ID, nil
					}
					return "", echo.ErrForbidden
				},
				Store: middleware.NewRateLimiterMemoryStoreWithConfig(
					middleware.RateLimiterMemoryStoreConfig{
						Rate:      rate.Limit(cfg.RateLimitRPS),
						Burst:     int(cfg.RateLimitRPS * 2),
						ExpiresIn: 3 * time.Minute,
					}),
			}))
		}
		tenant := v1.Group("/packages", tenantMW...)
		tenant.GET("/ping", tenantPing)
		if orch != nil {
			orch.Mount(tenant)
		}
	}

	return e
}

// tenantPing is the auth-probe endpoint: proves the middleware resolved
// tenant context into the request lifecycle.
func tenantPing(c echo.Context) error {
	client := tenancy.Client(c)
	key := tenancy.APIKey(c)
	return c.JSON(http.StatusOK, map[string]string{
		"client_id":  client.ID,
		"client":     client.Name,
		"api_key_id": key.ID,
	})
}

func v1Status(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"api": "v1", "status": "ok"})
}

func health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
