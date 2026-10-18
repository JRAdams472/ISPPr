package tenancy

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
)

// Context keys for the resolved tenant identity.
const (
	ctxClient = "tenantClient"
	ctxAPIKey = "tenantAPIKey"
)

// Middleware validates the X-API-Key header on the tenant surface and
// resolves the owning client into the request lifecycle. scope is the
// api_keys.scopes entry the key must carry (e.g. "generate"); revoked keys
// never resolve (the query filters them, then the hash is re-verified with
// a constant-time compare).
func Middleware(q *store.Queries, scope string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get("X-API-Key")
			if key == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "X-API-Key header required")
			}
			hash := hashKey(key)
			row, err := q.GetActiveAPIKeyByHash(c.Request().Context(), hash)
			if errors.Is(err, pgx.ErrNoRows) {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or revoked API key")
			}
			if err != nil {
				return err
			}
			// Defense-in-depth: verify the looked-up hash with a constant-time
			// compare rather than trusting the DB match alone.
			if subtle.ConstantTimeCompare([]byte(row.KeyHash), []byte(hash)) != 1 {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or revoked API key")
			}
			if scope != "" && !slices.Contains(row.Scopes, scope) {
				return echo.NewHTTPError(http.StatusForbidden, "key lacks required scope: "+scope)
			}
			client, err := q.GetClient(c.Request().Context(), row.ClientID)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "key owner missing")
			}
			if client.Status != "active" {
				return echo.NewHTTPError(http.StatusForbidden, "tenant disabled")
			}
			_ = q.TouchAPIKeyLastUsed(c.Request().Context(), hash)
			c.Set(ctxClient, client)
			c.Set(ctxAPIKey, row)
			return next(c)
		}
	}
}

// Client returns the resolved tenant, or nil on unauthenticated routes.
func Client(c echo.Context) *store.Client {
	if v, ok := c.Get(ctxClient).(store.Client); ok {
		return &v
	}
	return nil
}

// APIKey returns the resolved credential row, or nil.
func APIKey(c echo.Context) *store.ApiKey {
	if v, ok := c.Get(ctxAPIKey).(store.ApiKey); ok {
		return &v
	}
	return nil
}

// RequireOwnership enforces in-engine tenant authorization: the resolved
// client must own customerID and every templateID in the payload. Any
// mismatch is a global deny (403) — never a partial resolve.
func RequireOwnership(c echo.Context, q *store.Queries, customerID string, templateIDs []string) error {
	client := Client(c)
	if client == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tenant context missing")
	}
	ctx := c.Request().Context()
	if customerID != "" {
		if _, err := q.GetCustomerForClient(ctx, store.GetCustomerForClientParams{
			ID: customerID, ClientID: client.ID,
		}); errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusForbidden, "customer_id not owned by tenant")
		} else if err != nil {
			return err
		}
	}
	for _, tid := range templateIDs {
		if _, err := q.GetMasterForClient(ctx, store.GetMasterForClientParams{
			ID: tid, ClientID: client.ID,
		}); errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusForbidden, "template_id not owned by tenant")
		} else if err != nil {
			return err
		}
	}
	return nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
