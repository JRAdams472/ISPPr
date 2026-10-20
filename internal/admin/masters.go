package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/templates"
	"github.com/labstack/echo/v4"
)

// --- template masters (versioned, append-only)

func (a *API) listMasters(c echo.Context) error {
	rows, err := a.q.ListMasters(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	out := make([]masterOut, len(rows))
	for i, r := range rows {
		out[i] = masterDTO(r)
	}
	return c.JSON(http.StatusOK, out)
}

type masterReq struct {
	Name     string          `json:"name"`
	HTML     string          `json:"html"`
	CSS      string          `json:"css"`
	Defaults json.RawMessage `json:"defaults"`
}

// createMaster publishes v1 of a named master; 409 if the name exists.
func (a *API) createMaster(c echo.Context) error {
	var req masterReq
	if err := c.Bind(&req); err != nil || req.Name == "" || req.HTML == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and html required")
	}
	clientID := c.Param("id")
	if _, err := a.q.GetLatestMaster(c.Request().Context(), store.GetLatestMasterParams{
		ClientID: clientID, Name: req.Name,
	}); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "master name exists — publish a version instead")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if len(req.Defaults) == 0 {
		req.Defaults = json.RawMessage(`{}`)
	}
	row, err := a.q.CreateTemplateMaster(c.Request().Context(), store.CreateTemplateMasterParams{
		ID: newID("tmpl"), ClientID: clientID, Name: req.Name,
		Html: req.HTML, Css: req.CSS, Version: 1, Defaults: req.Defaults,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, masterDTO(row))
}

// publishMasterVersion appends the next version under the same name.
func (a *API) publishMasterVersion(c echo.Context) error {
	var req masterReq
	if err := c.Bind(&req); err != nil || req.HTML == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "html required")
	}
	clientID, name := c.Param("id"), c.Param("name")
	// 404 early if the master name doesn't exist under this client.
	if _, err := a.q.GetLatestMaster(c.Request().Context(), store.GetLatestMasterParams{
		ClientID: clientID, Name: name,
	}); err != nil {
		return err
	}
	next, err := a.q.NextMasterVersion(c.Request().Context(), store.NextMasterVersionParams{
		ClientID: clientID, Name: name,
	})
	if err != nil {
		return err
	}
	if len(req.Defaults) == 0 {
		req.Defaults = json.RawMessage(`{}`)
	}
	row, err := a.q.CreateTemplateMaster(c.Request().Context(), store.CreateTemplateMasterParams{
		ID: newID("tmpl"), ClientID: clientID, Name: name,
		Html: req.HTML, Css: req.CSS, Version: next, Defaults: req.Defaults,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, masterDTO(row))
}

func (a *API) getMaster(c echo.Context) error {
	row, err := a.q.GetMaster(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, masterDTO(row))
}

// --- template variants

type variantReq struct {
	MasterID   string          `json:"master_id"`
	CustomerID string          `json:"customer_id"`
	Overlay    json.RawMessage `json:"overlay"`
}

func (a *API) upsertVariant(c echo.Context) error {
	var req variantReq
	if err := c.Bind(&req); err != nil || req.MasterID == "" || req.CustomerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "master_id and customer_id required")
	}
	if len(req.Overlay) == 0 {
		req.Overlay = json.RawMessage(`{}`)
	}
	row, err := a.q.UpsertVariant(c.Request().Context(), store.UpsertVariantParams{
		ID: newID("var"), MasterID: req.MasterID,
		CustomerID: req.CustomerID, Overlay: req.Overlay,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, variantDTO(row))
}

// getVariant: GET /admin/template-variants?master_id=&customer_id=
func (a *API) getVariant(c echo.Context) error {
	row, err := a.q.GetVariant(c.Request().Context(), store.GetVariantParams{
		MasterID:   c.QueryParam("master_id"),
		CustomerID: c.QueryParam("customer_id"),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, variantDTO(row))
}

func (a *API) listCustomerVariants(c echo.Context) error {
	rows, err := a.q.ListVariantsForCustomer(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	out := make([]variantOut, len(rows))
	for i, r := range rows {
		out[i] = variantOut{
			ID: r.ID, MasterID: r.MasterID, MasterName: r.MasterName,
			CustomerID: r.CustomerID, Overlay: r.Overlay,
			UpdatedAt: r.UpdatedAt.Time,
		}
	}
	return c.JSON(http.StatusOK, out)
}

// previewReq is the draft-variant preview payload: resolve master + the
// supplied overlay against sample data without persisting anything.
type previewReq struct {
	MasterID string          `json:"master_id"`
	Overlay  json.RawMessage `json:"overlay"`
	Data     json.RawMessage `json:"data"`
}

// previewVariant renders a draft overlay to PDF through the same resolve →
// Handlebars → Gotenberg path the pipeline uses, so the preview is faithful
// to what generation will produce once the variant is saved. Read-only: the
// overlay is applied in memory, never upserted.
func (a *API) previewVariant(c echo.Context) error {
	if a.rpool == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "render pool unavailable")
	}
	var req previewReq
	if err := c.Bind(&req); err != nil || req.MasterID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "master_id required")
	}
	ctx := c.Request().Context()
	master, err := a.q.GetMaster(ctx, req.MasterID)
	if err != nil {
		return err
	}
	base, err := templates.Base()
	if err != nil {
		return err
	}
	ov, err := templates.ParseOverlay(req.Overlay)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid overlay: "+err.Error())
	}
	resolved, err := templates.Resolve(base, &master, &ov)
	if err != nil {
		return err
	}
	var data map[string]any
	if len(req.Data) > 0 {
		if err := json.Unmarshal(req.Data, &data); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "data must be a JSON object")
		}
	}
	html, err := render.Render(resolved, data)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "render: "+err.Error())
	}
	pdf, err := a.rpool.PDF(ctx, html, resolved.CSS)
	if err != nil {
		var rerr *render.RenderError
		if errors.As(err, &rerr) && rerr.Kind == render.KindRenderFailed {
			return echo.NewHTTPError(http.StatusBadRequest, rerr.Error())
		}
		return err
	}
	return c.Blob(http.StatusOK, "application/pdf", pdf)
}
