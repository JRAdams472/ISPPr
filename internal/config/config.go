package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the DGE API. Every value is
// sourced from the environment; see .env.example for the local defaults.
type Config struct {
	Port         string
	DatabaseURL  string
	GotenbergURL string
	// GotenbergEndpoints is the render pool: GOTENBERG_ENDPOINTS (comma
	// list) wins, else the single GOTENBERG_URL.
	GotenbergEndpoints []string
	RenderTimeout      time.Duration
	CORSOrigin         string

	S3Endpoint    string
	S3Bucket      string
	S3AccessKey   string
	S3SecretKey   string
	S3PresignTTL  time.Duration
	MaxDocsPerPkg int
	RunSeed       bool
}

func Load() (Config, error) {
	ttl, err := envMinutes("S3_URL_TTL_MINUTES", 15)
	if err != nil {
		return Config{}, err
	}
	maxDocs, err := envInt("MAX_DOCS_PER_PACKAGE", 25)
	if err != nil {
		return Config{}, err
	}
	renderSecs, err := envInt("RENDER_TIMEOUT_SECONDS", 30)
	if err != nil {
		return Config{}, err
	}
	gbURL := env("GOTENBERG_URL", "http://localhost:3001")
	endpoints := []string{gbURL}
	if list := os.Getenv("GOTENBERG_ENDPOINTS"); list != "" {
		endpoints = nil
		for _, e := range strings.Split(list, ",") {
			if e = strings.TrimSpace(e); e != "" {
				endpoints = append(endpoints, e)
			}
		}
	}
	return Config{
		Port:               env("PORT", "8090"),
		DatabaseURL:        env("DATABASE_URL", "postgres://isppr:isppr@localhost:5433/isppr?sslmode=disable"),
		GotenbergURL:       gbURL,
		GotenbergEndpoints: endpoints,
		RenderTimeout:      time.Duration(renderSecs) * time.Second,
		CORSOrigin:         env("CORS_ORIGIN", "http://localhost:3000"),
		S3Endpoint:         env("S3_ENDPOINT", "http://localhost:9100"),
		S3Bucket:           env("S3_BUCKET", "isppr-docs"),
		S3AccessKey:        env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:        env("S3_SECRET_KEY", "minioadmin"),
		S3PresignTTL:       ttl,
		MaxDocsPerPkg:      maxDocs,
		RunSeed:            env("RUN_SEED", "0") == "1",
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func envMinutes(key string, fallback int) (time.Duration, error) {
	n, err := envInt(key, fallback)
	if err != nil {
		return 0, err
	}
	return time.Duration(n) * time.Minute, nil
}
