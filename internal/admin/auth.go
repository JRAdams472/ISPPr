package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminOut struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func adminDTO(u store.AdminUser) adminOut {
	return adminOut{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role}
}

// login verifies email+password (bcrypt) and returns the session token once;
// also sets the session cookie for browser flows.
func (a *API) login(c echo.Context) error {
	var req loginReq
	if err := c.Bind(&req); err != nil || req.Email == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email and password required")
	}
	u, err := a.q.GetAdminUserByEmail(c.Request().Context(), req.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		// Same 401 for unknown email and bad password — no user enumeration.
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	token := newToken()
	exp := time.Now().Add(a.ttl)
	if _, err := a.q.CreateAdminSession(c.Request().Context(), store.CreateAdminSessionParams{
		ID: newID("ses"), AdminUserID: u.ID,
		TokenHash: hashToken(token), ExpiresAt: timestamptz(exp),
	}); err != nil {
		return err
	}
	c.SetCookie(&http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: exp,
	})
	return c.JSON(http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
		"user":       adminDTO(u),
	})
}

func (a *API) logout(c echo.Context) error {
	token := bearer(c.Request().Header.Get(echo.HeaderAuthorization))
	if token == "" {
		if ck, err := c.Cookie(sessionCookie); err == nil {
			token = ck.Value
		}
	}
	_ = a.q.DeleteSessionByTokenHash(c.Request().Context(), hashToken(token))
	c.SetCookie(&http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	return c.NoContent(http.StatusNoContent)
}

func (a *API) me(c echo.Context) error {
	return c.JSON(http.StatusOK, adminDTO(c.Get("adminUser").(store.AdminUser)))
}

func newToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return "ses_tok_" + hex.EncodeToString(b[:])
}
