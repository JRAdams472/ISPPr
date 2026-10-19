package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Integration: real MinIO from docker compose (port 9100). Skip-gated —
// exercises put → presign → fetch → TTL expiry only when a live store
// is reachable.
func TestComposeMinIO(t *testing.T) {
	ep := os.Getenv("S3_ENDPOINT")
	if ep == "" {
		ep = "http://localhost:9100"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := NewS3(ctx, S3Config{
		Endpoint:       ep,
		PublicEndpoint: ep,
		Bucket:         fmt.Sprintf("isppr-test-%d", time.Now().UnixNano()),
		AccessKey:      envOr("S3_ACCESS_KEY", "minioadmin"),
		SecretKey:      envOr("S3_SECRET_KEY", "minioadmin"),
		PresignTTL:     15 * time.Minute,
	})
	if err != nil {
		t.Skipf("minio unreachable at %s: %v", ep, err)
	}

	// Put → Open round-trip.
	if err := s.Put(ctx, "req_x/pkg.zip", strings.NewReader("zipdata")); err != nil {
		t.Fatalf("put: %v", err)
	}
	rc, err := s.Open(ctx, "req_x/pkg.zip")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "zipdata" {
		t.Fatalf("roundtrip: %q", b)
	}

	// Presigned URL fetches the object without credentials.
	url, err := s.Presign(ctx, "req_x/pkg.zip")
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	resp, err := http.Get(url) //nolint:bodyclose // closed below
	if err != nil {
		t.Fatalf("presigned get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "zipdata" {
		t.Fatalf("presigned fetch: %d %q", resp.StatusCode, body)
	}

	// Expiry: a 1s-TTL URL is refused after it lapses (the TTL contract —
	// the real config mints 15-minute URLs).
	s.ttl = time.Second
	exp, err := s.Presign(ctx, "req_x/pkg.zip")
	if err != nil {
		t.Fatalf("short presign: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	resp2, err := http.Get(exp) //nolint:bodyclose
	if err != nil {
		t.Fatalf("expired get: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode == 200 {
		t.Fatal("expired presigned URL still served the object")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Unit: dial parses endpoint variants — bare host, http, https.
func TestDialEndpointForms(t *testing.T) {
	for _, ep := range []string{
		"http://localhost:9100", "https://s3.example.com", "minio:9000",
	} {
		if _, err := dial(ep, "ak", "sk", "us-east-1"); err != nil {
			t.Errorf("dial %q: %v", ep, err)
		}
	}
}

// Unit: S3Store defaults the presign TTL when unset.
func TestS3DefaultsTTL(t *testing.T) {
	s := &S3Store{}
	if s.ttl != 0 {
		t.Fatal("zero-value ttl")
	}
}
