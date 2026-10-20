package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jradams472/isppr/internal/store"
	"github.com/labstack/echo/v4"
)

type usageDayOut struct {
	Day        string `json:"day"`
	ClientID   string `json:"client_id"`
	CustomerID string `json:"customer_id"`
	Requests   int64  `json:"requests"`
	Completed  int64  `json:"completed"`
	Failed     int64  `json:"failed"`
}

type usageOut struct {
	Days    int32         `json:"days"`
	ByDay   []usageDayOut `json:"by_day"`
	Latency latencyOut    `json:"latency"`
}

type latencyOut struct {
	P50Ms   float64 `json:"p50_ms"`
	P95Ms   float64 `json:"p95_ms"`
	Samples int64   `json:"samples"`
}

// usage: GET /admin/usage?days=30 — request counts per day/client/customer
// plus render latency percentiles from generation_requests.metrics.
func (a *API) usage(c echo.Context) error {
	days := int32(30)
	if d := c.QueryParam("days"); d != "" {
		n, err := strconv.Atoi(d)
		if err != nil || n < 1 || n > 365 {
			return echo.NewHTTPError(http.StatusBadRequest, "days must be 1-365")
		}
		days = int32(n)
	}
	ctx := c.Request().Context()
	clientID := c.QueryParam("client_id")

	byDay := []usageDayOut{}
	lat := latencyOut{}
	if clientID == "" {
		rows, err := a.q.UsageByDay(ctx, days)
		if err != nil {
			return err
		}
		for _, r := range rows {
			byDay = append(byDay, usageDayOut{
				Day: r.Day.Time.Format(time.DateOnly), ClientID: r.ClientID,
				CustomerID: r.CustomerID, Requests: r.Requests,
				Completed: r.Completed, Failed: r.Failed,
			})
		}
		l, err := a.q.UsageLatency(ctx, days)
		if err != nil {
			return err
		}
		lat = latencyOut{P50Ms: l.P50Ms, P95Ms: l.P95Ms, Samples: l.Samples}
	} else {
		rows, err := a.q.UsageByDayForClient(ctx,
			store.UsageByDayForClientParams{Days: days, ClientID: clientID})
		if err != nil {
			return err
		}
		for _, r := range rows {
			byDay = append(byDay, usageDayOut{
				Day: r.Day.Time.Format(time.DateOnly), ClientID: r.ClientID,
				CustomerID: r.CustomerID, Requests: r.Requests,
				Completed: r.Completed, Failed: r.Failed,
			})
		}
		l, err := a.q.UsageLatencyForClient(ctx,
			store.UsageLatencyForClientParams{Days: days, ClientID: clientID})
		if err != nil {
			return err
		}
		lat = latencyOut{P50Ms: l.P50Ms, P95Ms: l.P95Ms, Samples: l.Samples}
	}
	return c.JSON(http.StatusOK, usageOut{
		Days:    days,
		ByDay:   byDay,
		Latency: lat,
	})
}
