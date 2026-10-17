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

func TestLoadBadInt(t *testing.T) {
	t.Setenv("S3_URL_TTL_MINUTES", "notanint")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-numeric S3_URL_TTL_MINUTES")
	}
}
