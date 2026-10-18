package admin

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
)

type keyOut struct {
	ID         string     `json:"id"`
	ClientID   string     `json:"client_id"`
	Label      string     `json:"label"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	RotatedAt  *time.Time `json:"rotated_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	// Key is set exactly once, on create/rotate — plaintext shown then gone.
	Key string `json:"key,omitempty"`
}

func keyDTO(k store.ApiKey) keyOut {
	return keyOut{
		ID: k.ID, ClientID: k.ClientID, Label: k.Label, Scopes: k.Scopes,
		CreatedAt: k.CreatedAt.Time,
		RotatedAt: tzPtr(k.RotatedAt), RevokedAt: tzPtr(k.RevokedAt),
		LastUsedAt: tzPtr(k.LastUsedAt),
	}
}

func tzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// createAPIKey: POST /admin/clients/:id/api-keys — plaintext returned once.
func (a *API) createAPIKey(c echo.Context) error {
	var req struct {
		Label  string   `json:"label"`
		Scopes []string `json:"scopes"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if len(req.Scopes) == 0 {
		req.Scopes = []string{"generate"}
	}
	plain, hash := newAPIKey()
	row, err := a.q.CreateAPIKey(c.Request().Context(), store.CreateAPIKeyParams{
		ID: newID("key"), ClientID: c.Param("id"), KeyHash: hash,
		Label: req.Label, Scopes: req.Scopes,
	})
	if err != nil {
		return err
	}
	out := keyDTO(row)
	out.Key = plain
	return c.JSON(http.StatusCreated, out)
}

func (a *API) listAPIKeys(c echo.Context) error {
	rows, err := a.q.ListAPIKeysForClient(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	out := make([]keyOut, len(rows))
	for i, r := range rows {
		out[i] = keyDTO(r)
	}
	return c.JSON(http.StatusOK, out)
}

// rotateAPIKey swaps key material in place: old hash is dead immediately,
// new plaintext returned once.
func (a *API) rotateAPIKey(c echo.Context) error {
	plain, hash := newAPIKey()
	row, err := a.q.RotateAPIKey(c.Request().Context(), store.RotateAPIKeyParams{
		ID: c.Param("id"), KeyHash: hash,
	})
	if err != nil {
		return err
	}
	out := keyDTO(row)
	out.Key = plain
	return c.JSON(http.StatusOK, out)
}

func (a *API) revokeAPIKey(c echo.Context) error {
	row, err := a.q.RevokeAPIKey(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, keyDTO(row))
}
