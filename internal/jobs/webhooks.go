package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jradams472/isppr/internal/store"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Webhook delivery statuses recorded on the request row.
const (
	WebhookNone      = "NONE"
	WebhookPending   = "PENDING"
	WebhookDelivered = "DELIVERED"
	WebhookExhausted = "EXHAUSTED"
)

// SignatureHeader carries the HMAC-SHA256 over the raw body:
// "sha256=<hex>". Receivers verify with the shared WEBHOOK_SECRET.
const SignatureHeader = "X-ISPPr-Signature"

// webhookEntry is one attempt in webhook_log.
type webhookEntry struct {
	At      string `json:"at"`
	Attempt int    `json:"attempt"`
	Code    int    `json:"code,omitempty"`
	Error   string `json:"error,omitempty"`
}

// deliver posts the completion callback to the customer's webhook_url,
// retrying with exponential backoff up to maxAttempts. Every attempt is
// appended to webhook_log and the terminal status lands in
// webhook_status — the admin request view surfaces both.
func (o *Orchestrator) deliver(parent context.Context, requestID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// Detached from the request/job contexts (they're already done), but
	// linked back to the job trace so Jaeger shows the association.
	ctx, span := otel.Tracer("isppr/jobs").Start(ctx, "webhook.deliver",
		trace.WithAttributes(attribute.String("request.id", requestID)),
		trace.WithLinks(trace.LinkFromContext(parent)))
	defer span.End()

	req, err := o.q.GetRequest(ctx, requestID)
	if err != nil {
		slog.WarnContext(ctx, "webhook: load request", "request", requestID, "err", err)
		return
	}
	target, err := o.q.GetCustomerWebhook(ctx, store.GetCustomerWebhookParams{
		ID: req.CustomerID, ClientID: req.ClientID,
	})
	if err != nil || !target.Valid || target.String == "" {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.WarnContext(ctx, "webhook: lookup customer", "request", requestID, "err", err)
		}
		return // stays NONE — customer has no webhook_url
	}

	body, err := json.Marshal(o.responseFromRequest(ctx, req))
	if err != nil {
		slog.WarnContext(ctx, "webhook: marshal payload", "request", requestID, "err", err)
		return
	}
	_ = o.q.SetWebhookStatus(ctx, store.SetWebhookStatusParams{
		ID: requestID, WebhookStatus: WebhookPending,
	})

	for attempt := 1; attempt <= o.webhookAttempts; attempt++ {
		code, derr := o.postWebhook(ctx, target.String, body)
		o.logWebhook(ctx, requestID, webhookEntry{
			At:      time.Now().UTC().Format(time.RFC3339),
			Attempt: attempt,
			Code:    code,
			Error:   errString(derr),
		})
		if derr == nil && code >= 200 && code < 300 {
			_ = o.q.SetWebhookStatus(ctx, store.SetWebhookStatusParams{
				ID: requestID, WebhookStatus: WebhookDelivered,
			})
			slog.InfoContext(ctx, "webhook delivered", "request", requestID, "attempt", attempt)
			return
		}
		if attempt < o.webhookAttempts {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(1<<attempt) * 500 * time.Millisecond):
				// backoff: 1s, 2s, 4s, 8s, …
			}
		}
	}
	_ = o.q.SetWebhookStatus(ctx, store.SetWebhookStatusParams{
		ID: requestID, WebhookStatus: WebhookExhausted,
	})
	slog.WarnContext(ctx, "webhook exhausted", "request", requestID)
}

// postWebhook sends one signed POST. Response code is returned even on
// non-2xx so the log can distinguish 4xx/5xx from transport errors.
func (o *Orchestrator) postWebhook(ctx context.Context, url string, body []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, "sha256="+sign(o.webhookSecret, body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func (o *Orchestrator) logWebhook(ctx context.Context, requestID string, e webhookEntry) {
	b, _ := json.Marshal([]webhookEntry{e})
	if err := o.q.AppendWebhookLog(ctx, store.AppendWebhookLogParams{
		ID: requestID, Entry: b,
	}); err != nil {
		slog.WarnContext(ctx, "webhook: append log", "request", requestID, "err", err)
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
