package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jradams472/isppr/internal/storage"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/tenancy"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// response is the spec's egress shape (201 body + the async/status variant).
type response struct {
	RequestID string   `json:"request_id"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	Metrics   *Metrics `json:"metrics,omitempty"`
	Results   *Results `json:"results,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// Mount registers the generation routes on the tenant-gated group
// (/api/v1/packages — X-API-Key middleware already applied upstream).
func (o *Orchestrator) Mount(g *echo.Group) {
	g.POST("/generate", o.generate, middleware.BodyLimit(strconv.FormatInt(o.maxBody, 10)+"B"))
	g.GET("/:requestID", o.status)
	g.GET("/:requestID/download", o.download)
}

func (o *Orchestrator) generate(c echo.Context) error {
	client := tenancy.Client(c)
	if client == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "tenant context missing")
	}
	var req GenerateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload: "+err.Error())
	}
	if err := o.validate(req, client.ID); err != nil {
		return err
	}
	row, replayed, err := o.Submit(c.Request().Context(), client.ID, req,
		c.Request().Header.Get("Idempotency-Key"))
	if err != nil {
		return mapSubmitError(err)
	}
	if !replayed {
		// Sync fast-path: when the package lands inside the budget the
		// client gets the spec's full 201 metadata; otherwise 202.
		if !o.Await(c.Request().Context(), row.ID) {
			return c.JSON(http.StatusAccepted, o.accepted(row))
		}
		if fresh, err := o.q.GetRequest(c.Request().Context(), row.ID); err == nil {
			row = fresh
		}
	}
	return o.respond(c, row)
}

func (o *Orchestrator) status(c echo.Context) error {
	row, err := o.requestForTenant(c)
	if err != nil {
		return err
	}
	// Status polls always answer 200 with the lifecycle state in the body.
	return c.JSON(http.StatusOK, o.responseFromRequest(c.Request().Context(), row))
}

func (o *Orchestrator) download(c echo.Context) error {
	row, err := o.requestForTenant(c)
	if err != nil {
		return err
	}
	if row.Status != StatusCompleted || !row.OutputKey.Valid {
		return echo.NewHTTPError(http.StatusConflict, "package not ready")
	}
	key := row.OutputKey.String
	// S3-backed stores redirect to a fresh presigned URL; local streams.
	if p, ok := o.store.(storage.Presigner); ok {
		url, err := p.Presign(c.Request().Context(), key)
		if err != nil {
			return err
		}
		return c.Redirect(http.StatusFound, url)
	}
	rc, err := o.store.Open(c.Request().Context(), key)
	if err != nil {
		return err
	}
	defer rc.Close()
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="`+artifactBase(key)+`"`)
	ct := "application/zip"
	if outputTypeFor(key) == OutputPDF {
		ct = "application/pdf"
	}
	return c.Stream(http.StatusOK, ct, rc)
}

// requestForTenant scopes the lookup to the resolved client — a foreign
// request_id is a global deny (403), per docs/authz-matrix.md.
func (o *Orchestrator) requestForTenant(c echo.Context) (store.GenerationRequest, error) {
	client := tenancy.Client(c)
	if client == nil {
		return store.GenerationRequest{},
			echo.NewHTTPError(http.StatusUnauthorized, "tenant context missing")
	}
	row, err := o.q.GetRequestForClient(c.Request().Context(), store.GetRequestForClientParams{
		ID: c.Param("requestID"), ClientID: client.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.GenerationRequest{},
			echo.NewHTTPError(http.StatusForbidden, "request_id not owned by tenant")
	}
	return row, err
}

func (o *Orchestrator) validate(req GenerateRequest, clientID string) error {
	bad := func(msg string) error {
		return echo.NewHTTPError(http.StatusBadRequest, msg)
	}
	if req.ClientID != "" && req.ClientID != clientID {
		return echo.NewHTTPError(http.StatusForbidden,
			"payload client_id does not match authenticated tenant")
	}
	if req.CustomerID == "" {
		return bad("customer_id required")
	}
	if req.PackageName == "" {
		return bad("package_name required")
	}
	if len(req.Documents) == 0 {
		return bad("documents required")
	}
	if len(req.Documents) > o.maxDocs {
		return bad("documents exceeds max_docs_per_package")
	}
	for _, d := range req.Documents {
		if d.Template == "" {
			return bad("documents[].template_id required")
		}
		if d.Payload == nil {
			return bad("documents[].payload required")
		}
	}
	return nil
}

func mapSubmitError(err error) error {
	if IsForbidden(err) {
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	}
	return err
}

// respond maps the stored row to a spec-shaped reply: 201 for completed
// packages, 202 for in-flight, 200-with-FAILED for terminal failures.
func (o *Orchestrator) respond(c echo.Context, row store.GenerationRequest) error {
	resp := o.responseFromRequest(c.Request().Context(), row)
	switch row.Status {
	case StatusCompleted:
		return c.JSON(http.StatusCreated, resp)
	case StatusFailed:
		return c.JSON(http.StatusOK, resp)
	default:
		return c.JSON(http.StatusAccepted, resp)
	}
}

func (o *Orchestrator) accepted(row store.GenerationRequest) response {
	return o.responseFromRequest(context.Background(), row)
}

// responseFromRequest maps the stored row; completed rows mint a fresh
// presigned download_url under S3 (never persisted — every read signs
// anew within the TTL), or the tenant download route under local.
func (o *Orchestrator) responseFromRequest(ctx context.Context, row store.GenerationRequest) response {
	resp := response{
		RequestID: row.ID,
		Status:    row.Status,
		CreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if row.Status == StatusCompleted && row.OutputKey.Valid {
		var m Metrics
		if err := json.Unmarshal(row.Metrics, &m); err == nil {
			resp.Metrics = &m
		}
		key := row.OutputKey.String
		dl := "/api/v1/packages/" + row.ID + "/download"
		if p, ok := o.store.(storage.Presigner); ok {
			if url, err := p.Presign(ctx, key); err == nil {
				dl = url
			}
		}
		resp.Results = &Results{
			OutputType:  outputTypeFor(key),
			Filename:    artifactBase(key),
			DownloadURL: dl,
		}
	}
	if row.Status == StatusFailed && row.Error.Valid {
		resp.Error = row.Error.String
	}
	return resp
}
