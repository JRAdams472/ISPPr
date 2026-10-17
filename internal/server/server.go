package server

import (
	"net/http"

	"github.com/jradams472/isppr/internal/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// New builds the Echo engine with global middleware and the scaffold routes.
// Feature routes (POST /api/v1/packages/generate, admin CRUD) land on top of
// this in later phases.
func New(cfg config.Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{cfg.CORSOrigin},
	}))

	e.GET("/healthz", health)
	e.GET("/api/health", health)

	return e
}

func health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
