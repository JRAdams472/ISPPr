package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/storage"
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
	Results       *requestResults `json:"results,omitempty"`
	WebhookStatus string          `json:"webhook_status"`
	WebhookLog    json.RawMessage `json:"webhook_log"`
	CreatedAt     string          `json:"created_at"`
}

// requestResults mirrors the tenant egress results block — emitted on the
// detail view only, since minting presigned URLs per list row is wasteful.
type requestResults struct {
	OutputType  string `json:"output_type"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
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
// status/log + a fresh download URL for completed outputs. Admin surface
// spans tenants (ops view, not tenant-scoped).
func (a *API) getRequest(c echo.Context) error {
	row, err := a.q.GetRequest(c.Request().Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "request not found")
	}
	if err != nil {
		return err
	}
	out := toRequestOut(row)
	if row.Status == "COMPLETED" && row.OutputKey.Valid && a.store != nil {
		key := row.OutputKey.String
		dl := "/api/v1/admin/requests/" + row.ID + "/download"
		// S3-backed stores mint a fresh presigned URL per read (never
		// persisted); local stores fall back to the admin download route.
		if p, ok := a.store.(storage.Presigner); ok {
			if u, err := p.Presign(c.Request().Context(), key); err == nil {
				dl = u
			}
		}
		out.Results = &requestResults{
			OutputType:  outputType(key),
			Filename:    artifactBase(key),
			DownloadURL: dl,
		}
	}
	return c.JSON(http.StatusOK, out)
}

// downloadRequestOutput: GET /admin/requests/:id/download — the re-fetch
// path behind results.download_url when the store can't presign (LocalStore
// dev), and a stable fetch URL regardless of backend.
func (a *API) downloadRequestOutput(c echo.Context) error {
	row, err := a.q.GetRequest(c.Request().Context(), c.Param("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "request not found")
	}
	if err != nil {
		return err
	}
	if row.Status != "COMPLETED" || !row.OutputKey.Valid {
		return echo.NewHTTPError(http.StatusConflict, "package output not available")
	}
	if a.store == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "storage backend unavailable")
	}
	key := row.OutputKey.String
	if p, ok := a.store.(storage.Presigner); ok {
		u, err := p.Presign(c.Request().Context(), key)
		if err != nil {
			return err
		}
		return c.Redirect(http.StatusFound, u)
	}
	rc, err := a.store.Open(c.Request().Context(), key)
	if err != nil {
		return err
	}
	defer rc.Close()
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="`+artifactBase(key)+`"`)
	ct := "application/zip"
	if outputType(key) == "PDF" {
		ct = "application/pdf"
	}
	return c.Stream(http.StatusOK, ct, rc)
}

func artifactBase(key string) string {
	if i := strings.LastIndex(key, "/"); i != -1 {
		return key[i+1:]
	}
	return key
}

func outputType(key string) string {
	if strings.HasSuffix(key, ".pdf") {
		return "PDF"
	}
	return "ZIP"
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
