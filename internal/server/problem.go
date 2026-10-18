package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

// Problem is an RFC 7807 problem-details body — the single error shape for
// every API surface (generation + management).
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// ProblemHandler is the central Echo error handler: every handler error —
// echo.HTTPError, pgx.ErrNoRows, or a plain error — serializes as
// application/problem+json.
func ProblemHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	status := http.StatusInternalServerError
	title := http.StatusText(status)
	detail := err.Error()

	var he *echo.HTTPError
	if errors.As(err, &he) {
		status = he.Code
		title = http.StatusText(status)
		if m, ok := he.Message.(string); ok && m != "" {
			detail = m
		} else {
			detail = title
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		status = http.StatusNotFound
		title = http.StatusText(status)
		detail = "not found"
	}

	p := Problem{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: c.Request().URL.Path,
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/problem+json")
	c.Response().WriteHeader(status)
	_ = json.NewEncoder(c.Response()).Encode(p)
}
