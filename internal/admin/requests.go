package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

// requestOut is the admin view of a generation request — the operational
// surface for lifecycle state and webhook delivery (MPE-37).
type requestOut struct {
	ID            string          `json:"id"`
	ClientID      string          `json:"client_id"`
	CustomerID    string          `json:"customer_id"`
	PackageName   string          `json:"package_name"`
	Status        string          `json:"status"`
	Error         string          `json:"error,omitempty"`
	Metrics       json.RawMessage `json:"metrics,omitempty"`
	WebhookStatus string          `json:"webhook_status"`
	WebhookLog    json.RawMessage `json:"webhook_log"`
	CreatedAt     string          `json:"created_at"`
}

// getRequest: GET /admin/requests/:id — request row + webhook delivery
// status/log. Admin surface spans tenants (ops view, not tenant-scoped).
func (a *API) getRequest(c echo.Context) error {
	row, err := a.q.GetRequest(c.Request().Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "request not found")
	}
	if err != nil {
		return err
	}
	out := requestOut{
		ID:            row.ID,
		ClientID:      row.ClientID,
		CustomerID:    row.CustomerID,
		PackageName:   row.PackageName,
		Status:        row.Status,
		WebhookStatus: row.WebhookStatus,
		WebhookLog:    json.RawMessage(row.WebhookLog),
		CreatedAt:     row.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if row.Error.Valid {
		out.Error = row.Error.String
	}
	if len(row.Metrics) > 0 {
		out.Metrics = json.RawMessage(row.Metrics)
	}
	return c.JSON(http.StatusOK, out)
}
