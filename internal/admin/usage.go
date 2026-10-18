package admin

import (
	"net/http"
	"strconv"
	"time"

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
	rows, err := a.q.UsageByDay(ctx, days)
	if err != nil {
		return err
	}
	lat, err := a.q.UsageLatency(ctx, days)
	if err != nil {
		return err
	}
	byDay := make([]usageDayOut, len(rows))
	for i, r := range rows {
		byDay[i] = usageDayOut{
			Day: r.Day.Time.Format(time.DateOnly), ClientID: r.ClientID,
			CustomerID: r.CustomerID, Requests: r.Requests,
			Completed: r.Completed, Failed: r.Failed,
		}
	}
	return c.JSON(http.StatusOK, usageOut{
		Days:  days,
		ByDay: byDay,
		Latency: latencyOut{
			P50Ms: lat.P50Ms, P95Ms: lat.P95Ms, Samples: lat.Samples,
		},
	})
}
