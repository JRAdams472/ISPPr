package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jradams472/isppr/internal/config"
)

func TestHealth(t *testing.T) {
	e := New(config.Config{CORSOrigin: "http://localhost:3000"}, nil, nil, nil)

	for _, path := range []string{"/healthz", "/api/health", "/api/v1/status"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: got %d, want 200", path, rec.Code)
		}
	}
}
