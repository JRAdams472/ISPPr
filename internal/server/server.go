package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jradams472/isppr/internal/admin"
	"github.com/jradams472/isppr/internal/config"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/tenancy"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// New builds the Echo engine with global middleware and the scaffold routes.
// q/pool may be nil for tests that don't touch the admin surface.
func New(cfg config.Config, q *store.Queries, pool *pgxpool.Pool) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ProblemHandler

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
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
		admin.New(q, pool).Mount(v1)
		tenant := v1.Group("/packages", tenancy.Middleware(q, "generate"))
		tenant.GET("/ping", tenantPing)
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
