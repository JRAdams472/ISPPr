package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
)

// API is the management surface: portal-session-gated CRUD under
// /api/v1/admin — distinct from the tenant X-API-Key generation surface.
type API struct {
	q     *store.Queries
	pool  *pgxpool.Pool
	rpool *render.Pool
	ttl   time.Duration
}

const (
	sessionCookie = "isppr_admin"
	sessionTTL    = 12 * time.Hour
)

func New(q *store.Queries, pool *pgxpool.Pool, rpool *render.Pool) *API {
	return &API{q: q, pool: pool, rpool: rpool, ttl: sessionTTL}
}

// Mount wires the admin routes onto the versioned group. Login is public;
// everything else requires a portal session.
func (a *API) Mount(v1 *echo.Group) {
	v1.POST("/admin/login", a.login)

	g := v1.Group("/admin", a.requireAdmin)
	g.POST("/logout", a.logout)
	g.GET("/me", a.me)

	g.GET("/clients", a.listClients)
	g.POST("/clients", a.createClient, a.requireWrite)
	g.GET("/clients/:id", a.getClient)
	g.PATCH("/clients/:id", a.updateClient, a.requireWrite)

	g.GET("/clients/:id/customers", a.listCustomers)
	g.POST("/clients/:id/customers", a.createCustomer, a.requireWrite)
	g.GET("/customers/:id", a.getCustomer)
	g.PATCH("/customers/:id", a.updateCustomer, a.requireWrite)

	g.GET("/clients/:id/api-keys", a.listAPIKeys)
	g.POST("/clients/:id/api-keys", a.createAPIKey, a.requireWrite)
	g.POST("/api-keys/:id/rotate", a.rotateAPIKey, a.requireWrite)
	g.POST("/api-keys/:id/revoke", a.revokeAPIKey, a.requireWrite)

	g.GET("/clients/:id/template-masters", a.listMasters)
	g.POST("/clients/:id/template-masters", a.createMaster, a.requireWrite)
	g.POST("/clients/:id/template-masters/:name/versions", a.publishMasterVersion, a.requireWrite)
	g.GET("/template-masters/:id", a.getMaster)

	g.PUT("/template-variants", a.upsertVariant, a.requireWrite)
	g.GET("/template-variants", a.getVariant)
	g.POST("/template-variants/preview", a.previewVariant)
	g.GET("/customers/:id/variants", a.listCustomerVariants)

	g.GET("/requests", a.listRequests)
	g.GET("/requests/:id", a.getRequest)
	g.GET("/usage", a.usage)
}

// requireAdmin resolves a session token (Bearer header or cookie) to the
// admin user; anything else gets a 401 problem.
func (a *API) requireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := bearer(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			if ck, err := c.Cookie(sessionCookie); err == nil {
				token = ck.Value
			}
		}
		if token == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "session required")
		}
		ses, err := a.q.GetSessionByTokenHash(c.Request().Context(), hashToken(token))
		if err == pgx.ErrNoRows {
			return echo.NewHTTPError(http.StatusUnauthorized, "session expired or invalid")
		}
		if err != nil {
			return err
		}
		u, err := a.q.GetAdminUser(c.Request().Context(), ses.AdminUserID)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "session owner missing")
		}
		c.Set("adminUser", u)
		return next(c)
	}
}

// requireWrite gates a mutating route to the admin role; every other role
// (viewer, developer, ...) is read-only. Runs after requireAdmin, so the
// resolved user is already in context.
func (a *API) requireWrite(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		u, ok := c.Get("adminUser").(store.AdminUser)
		if !ok || u.Role != "admin" {
			return echo.NewHTTPError(http.StatusForbidden, "admin role required")
		}
		return next(c)
	}
}

func bearer(h string) string {
	const p = "Bearer "
	if len(h) > len(p) && h[:len(p)] == p {
		return h[len(p):]
	}
	return ""
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newID mints a prefixed id: prefix + "_" + 16 hex chars of randomness.
func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}

// newAPIKey returns the plaintext credential and its sha-256 hash; the
// plaintext is shown once and never stored.
func newAPIKey() (plaintext, hash string) {
	var b [24]byte
	_, _ = rand.Read(b[:])
	plaintext = "isppr_" + hex.EncodeToString(b[:])
	return plaintext, hashToken(plaintext)
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
