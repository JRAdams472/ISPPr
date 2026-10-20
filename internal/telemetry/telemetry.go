// Package telemetry wires optional OpenTelemetry exporters (traces →
// Jaeger, logs → Seq) behind environment gating. With no endpoints
// configured the process runs exactly as before: noop tracer provider,
// default slog handler, zero exporter goroutines.
package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	otlploghttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	otlptracegrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Config selects the exporters. Empty endpoints mean "off" — the whole
// observability surface is environment-gated per the megaplan.
type Config struct {
	// ServiceName lands in resource service.name (default "isppr").
	ServiceName string
	// TracesEndpoint is the OTLP/gRPC collector for traces
	// (Jaeger all-in-one: "jaeger:4317").
	TracesEndpoint string
	// LogsEndpoint is the OTLP/HTTP base for logs (Seq:
	// "http://seq:5341/ingest/otlp"); the /v1/logs signal path is
	// appended when absent.
	LogsEndpoint string
}

// Enabled reports whether any exporter is configured.
func (c Config) Enabled() bool { return c.TracesEndpoint != "" || c.LogsEndpoint != "" }

// Setup installs the global tracer provider and (when logs are enabled)
// a slog handler that mirrors records to both stderr JSON and the OTLP
// pipeline so `docker logs` and Seq stay correlated by trace ID.
// Disabled config returns an inert shutdown and changes nothing.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled() {
		return func(context.Context) error { return nil }, nil
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "isppr"
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
	))
	if err != nil {
		return nil, err
	}

	var shutdowns []func(context.Context) error
	if cfg.TracesEndpoint != "" {
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.TracesEndpoint),
			otlptracegrpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res))
		otel.SetTracerProvider(tp)
		shutdowns = append(shutdowns, tp.Shutdown)
	}

	if cfg.LogsEndpoint != "" {
		exp, err := otlploghttp.New(ctx,
			otlploghttp.WithEndpointURL(logsURL(cfg.LogsEndpoint)),
			otlploghttp.WithInsecure())
		if err != nil {
			return nil, err
		}
		lp := log.NewLoggerProvider(
			log.WithProcessor(log.NewBatchProcessor(exp)),
			log.WithResource(res))
		shutdowns = append(shutdowns, lp.Shutdown)
		slog.SetDefault(slog.New(fanout{
			slog.NewJSONHandler(os.Stderr, nil),
			otelslog.NewHandler(cfg.ServiceName, otelslog.WithLoggerProvider(lp)),
		}))
	}

	return func(ctx context.Context) error {
		var errs []error
		for _, s := range shutdowns {
			errs = append(errs, s(ctx))
		}
		return errors.Join(errs...)
	}, nil
}

// logsURL appends the OTLP logs signal path when the configured base
// stops at the ingest root (Seq's convention).
func logsURL(base string) string {
	if strings.HasSuffix(base, "/v1/logs") {
		return base
	}
	if u, err := url.Parse(base); err == nil && u.Path != "" {
		return strings.TrimSuffix(base, "/") + "/v1/logs"
	}
	return base
}

// fanout mirrors each record to every handler so stderr stays greppable
// while Seq gets the same records with trace correlation.
type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f {
		if err := h.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (f fanout) WithAttrs(a []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(a)
	}
	return out
}

func (f fanout) WithGroup(name string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(name)
	}
	return out
}
