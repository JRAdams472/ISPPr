package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/store"
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

func toRequestOut(row store.GenerationRequest) requestOut {
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
	return out
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
	return c.JSON(http.StatusOK, toRequestOut(row))
}

// listRequests: GET /admin/requests?client_id&status&limit — newest-first
// filtered list for the portal's request inspectors. Read route: viewer
// sessions reach it like every other GET under /admin.
func (a *API) listRequests(c echo.Context) error {
	var p store.ListRequestsFilteredParams
	if v := c.QueryParam("client_id"); v != "" {
		p.ClientID = pgtype.Text{String: v, Valid: true}
	}
	if v := c.QueryParam("status"); v != "" {
		p.Status = pgtype.Text{String: v, Valid: true}
	}
	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			return echo.NewHTTPError(http.StatusBadRequest, "limit must be 1-500")
		}
		p.Limit = pgtype.Int4{Int32: int32(n), Valid: true}
	}
	rows, err := a.q.ListRequestsFiltered(c.Request().Context(), p)
	if err != nil {
		return err
	}
	out := make([]requestOut, len(rows))
	for i, r := range rows {
		out[i] = toRequestOut(r)
	}
	return c.JSON(http.StatusOK, out)
}
