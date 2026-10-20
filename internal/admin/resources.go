package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
)

// --- DTOs: response shapes never leak secret material (key/password hashes).

type clientOut struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	Customers  int64     `json:"customers,omitempty"`
	ActiveKeys int64     `json:"active_keys,omitempty"`
}

func clientDTO(c store.Client) clientOut {
	return clientOut{ID: c.ID, Name: c.Name, Status: c.Status, CreatedAt: c.CreatedAt.Time}
}

type customerOut struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	ExternalRef *string   `json:"external_ref"`
	Name        string    `json:"name"`
	WebhookURL  *string   `json:"webhook_url"`
	CreatedAt   time.Time `json:"created_at"`
}

func customerDTO(c store.Customer) customerOut {
	return customerOut{
		ID: c.ID, ClientID: c.ClientID, Name: c.Name,
		ExternalRef: textPtr(c.ExternalRef), WebhookURL: textPtr(c.WebhookUrl),
		CreatedAt: c.CreatedAt.Time,
	}
}

type masterOut struct {
	ID        string          `json:"id"`
	ClientID  string          `json:"client_id"`
	Name      string          `json:"name"`
	Version   int32           `json:"version"`
	HTML      string          `json:"html"`
	CSS       string          `json:"css"`
	Defaults  json.RawMessage `json:"defaults"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func masterDTO(m store.TemplateMaster) masterOut {
	return masterOut{
		ID: m.ID, ClientID: m.ClientID, Name: m.Name, Version: m.Version,
		HTML: m.Html, CSS: m.Css, Defaults: m.Defaults,
		CreatedAt: m.CreatedAt.Time, UpdatedAt: m.UpdatedAt.Time,
	}
}

type variantOut struct {
	ID         string          `json:"id"`
	MasterID   string          `json:"master_id"`
	CustomerID string          `json:"customer_id"`
	Overlay    json.RawMessage `json:"overlay"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func variantDTO(v store.TemplateVariant) variantOut {
	return variantOut{
		ID: v.ID, MasterID: v.MasterID, CustomerID: v.CustomerID,
		Overlay: v.Overlay, UpdatedAt: v.UpdatedAt.Time,
	}
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func optText(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

// --- clients

func (a *API) listClients(c echo.Context) error {
	rows, err := a.q.ListClients(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]clientOut, len(rows))
	for i, r := range rows {
		out[i] = clientOut{
			ID: r.ID, Name: r.Name, Status: r.Status, CreatedAt: r.CreatedAt.Time,
			Customers: r.Customers, ActiveKeys: r.ActiveKeys,
		}
	}
	return c.JSON(http.StatusOK, out)
}

type clientReq struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (a *API) createClient(c echo.Context) error {
	var req clientReq
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}
	if req.Status == "" {
		req.Status = "active"
	}
	row, err := a.q.CreateClient(c.Request().Context(), store.CreateClientParams{
		ID: newID("cli"), Name: req.Name, Status: req.Status,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, clientDTO(row))
}

func (a *API) getClient(c echo.Context) error {
	row, err := a.q.GetClient(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err // ErrNoRows → 404 via problem handler
	}
	return c.JSON(http.StatusOK, clientDTO(row))
}

func (a *API) updateClient(c echo.Context) error {
	var req struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	row, err := a.q.UpdateClient(c.Request().Context(), store.UpdateClientParams{
		ID: c.Param("id"), Name: optText(req.Name), Status: optText(req.Status),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, clientDTO(row))
}

// --- customers

func (a *API) listCustomers(c echo.Context) error {
	rows, err := a.q.ListCustomersForClient(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	out := make([]customerOut, len(rows))
	for i, r := range rows {
		out[i] = customerDTO(r)
	}
	return c.JSON(http.StatusOK, out)
}

type customerReq struct {
	Name        string  `json:"name"`
	ExternalRef *string `json:"external_ref"`
	WebhookURL  *string `json:"webhook_url"`
}

func (a *API) createCustomer(c echo.Context) error {
	var req customerReq
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}
	row, err := a.q.CreateCustomer(c.Request().Context(), store.CreateCustomerParams{
		ID: newID("cust"), ClientID: c.Param("id"), Name: req.Name,
		ExternalRef: optText(req.ExternalRef), WebhookUrl: optText(req.WebhookURL),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, customerDTO(row))
}

func (a *API) getCustomer(c echo.Context) error {
	row, err := a.q.GetCustomer(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, customerDTO(row))
}

func (a *API) updateCustomer(c echo.Context) error {
	var req struct {
		Name        *string `json:"name"`
		ExternalRef *string `json:"external_ref"`
		WebhookURL  *string `json:"webhook_url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	row, err := a.q.UpdateCustomer(c.Request().Context(), store.UpdateCustomerParams{
		ID: c.Param("id"), Name: optText(req.Name), ExternalRef: optText(req.ExternalRef),
		WebhookUrl: optText(req.WebhookURL),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, customerDTO(row))
}
