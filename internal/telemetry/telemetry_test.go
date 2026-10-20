package telemetry

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
)

func TestConfigEnabled(t *testing.T) {
	if (Config{}).Enabled() {
		t.Fatal("empty config should be disabled")
	}
	for name, c := range map[string]Config{
		"traces": {TracesEndpoint: "jaeger:4317"},
		"logs":   {LogsEndpoint: "http://seq:5341/ingest/otlp"},
		"both":   {TracesEndpoint: "jaeger:4317", LogsEndpoint: "http://seq:5341"},
	} {
		if !c.Enabled() {
			t.Fatalf("%s: expected enabled", name)
		}
	}
}

func TestSetupDisabledIsNoop(t *testing.T) {
	before := otel.GetTracerProvider()
	shutdown, err := Setup(context.Background(), Config{})
	if err != nil {
		t.Fatalf("disabled setup: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("disabled shutdown: %v", err)
	}
	if got := otel.GetTracerProvider(); got != before {
		t.Fatal("disabled setup must not install a provider")
	}
	if _, ok := slog.Default().Handler().(fanout); ok {
		t.Fatal("disabled setup must not replace the slog handler")
	}
}

func TestSetupEnabledInstallsProviders(t *testing.T) {
	// Exporters dial lazily — configuring against unused ports exercises
	// the full provider construction path without a collector.
	shutdown, err := Setup(context.Background(), Config{
		TracesEndpoint: "localhost:1",
		LogsEndpoint:   "http://localhost:1/ingest/otlp",
	})
	if err != nil {
		t.Fatalf("enabled setup: %v", err)
	}
	if _, ok := slog.Default().Handler().(fanout); !ok {
		t.Fatal("logs enabled: slog should fan out to stderr + OTLP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestLogsURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://seq:5341/ingest/otlp":         "http://seq:5341/ingest/otlp/v1/logs",
		"http://seq:5341/ingest/otlp/":        "http://seq:5341/ingest/otlp/v1/logs",
		"http://seq:5341/ingest/otlp/v1/logs": "http://seq:5341/ingest/otlp/v1/logs",
		"seq:5341":                            "seq:5341",
	} {
		if got := logsURL(in); got != want {
			t.Errorf("logsURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFanoutMirrorsRecords(t *testing.T) {
	var a, b strings.Builder
	h := fanout{
		slog.NewTextHandler(&a, &slog.HandlerOptions{Level: slog.LevelInfo}),
		slog.NewTextHandler(&b, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}
	l := slog.New(h).With("component", "test")
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected enabled")
	}
	l.Info("hello", "k", "v")
	for name, out := range map[string]string{"a": a.String(), "b": b.String()} {
		if !strings.Contains(out, "hello") || !strings.Contains(out, "k=v") ||
			!strings.Contains(out, "component=test") {
			t.Fatalf("handler %s missing record: %q", name, out)
		}
	}
	if g := h.WithGroup("g"); len(g.(fanout)) != 2 {
		t.Fatal("WithGroup must preserve fanout")
	}
}
