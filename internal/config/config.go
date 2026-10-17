package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the DGE API. Every value is
// sourced from the environment; see .env.example for the local defaults.
type Config struct {
	Port         string
	DatabaseURL  string
	GotenbergURL string
	CORSOrigin   string

	S3Endpoint    string
	S3Bucket      string
	S3AccessKey   string
	S3SecretKey   string
	S3PresignTTL  time.Duration
	MaxDocsPerPkg int
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
	return Config{
		Port:          env("PORT", "8090"),
		DatabaseURL:   env("DATABASE_URL", "postgres://isppr:isppr@localhost:5433/isppr"),
		GotenbergURL:  env("GOTENBERG_URL", "http://localhost:3001"),
		CORSOrigin:    env("CORS_ORIGIN", "http://localhost:3000"),
		S3Endpoint:    env("S3_ENDPOINT", "http://localhost:9100"),
		S3Bucket:      env("S3_BUCKET", "isppr-docs"),
		S3AccessKey:   env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:   env("S3_SECRET_KEY", "minioadmin"),
		S3PresignTTL:  ttl,
		MaxDocsPerPkg: maxDocs,
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
