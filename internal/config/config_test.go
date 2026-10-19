package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8090" || cfg.S3PresignTTL != 15*time.Minute || cfg.MaxDocsPerPkg != 25 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.RunSeed {
		t.Fatal("RunSeed should default to false")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("S3_URL_TTL_MINUTES", "30")
	t.Setenv("MAX_DOCS_PER_PACKAGE", "5")
	t.Setenv("RUN_SEED", "1")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9999" || cfg.S3PresignTTL != 30*time.Minute ||
		cfg.MaxDocsPerPkg != 5 || !cfg.RunSeed {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

func TestGotenbergEndpoints(t *testing.T) {
	t.Setenv("GOTENBERG_URL", "http://gb:3000")
	t.Setenv("GOTENBERG_ENDPOINTS", "http://gb1:3000, http://gb2:3000 ,,http://gb3:3000")
	t.Setenv("RENDER_TIMEOUT_SECONDS", "45")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"http://gb1:3000", "http://gb2:3000", "http://gb3:3000"}
	if len(cfg.GotenbergEndpoints) != 3 || cfg.GotenbergEndpoints[0] != want[0] ||
		cfg.GotenbergEndpoints[2] != want[2] {
		t.Fatalf("endpoints: %v", cfg.GotenbergEndpoints)
	}
	if cfg.RenderTimeout != 45*time.Second {
		t.Fatalf("timeout: %v", cfg.RenderTimeout)
	}
}

func TestGotenbergFallbackToSingleURL(t *testing.T) {
	t.Setenv("GOTENBERG_URL", "http://gb:3000")
	t.Setenv("GOTENBERG_ENDPOINTS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GotenbergEndpoints) != 1 || cfg.GotenbergEndpoints[0] != "http://gb:3000" {
		t.Fatalf("fallback: %v", cfg.GotenbergEndpoints)
	}
	if cfg.RenderTimeout != 30*time.Second {
		t.Fatalf("default timeout: %v", cfg.RenderTimeout)
	}
}

func TestLoadBadInt(t *testing.T) {
	t.Setenv("S3_URL_TTL_MINUTES", "notanint")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric S3_URL_TTL_MINUTES")
	}
}
