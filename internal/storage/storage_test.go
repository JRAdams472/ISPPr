package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalStoreRoundTrip(t *testing.T) {
	s := &LocalStore{Dir: t.TempDir()}
	ctx := context.Background()
	if err := s.Put(ctx, "req_1/pkg.zip", strings.NewReader("zipdata")); err != nil {
		t.Fatal(err)
	}
	rc, err := s.Open(ctx, "req_1/pkg.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "zipdata" {
		t.Fatalf("roundtrip: %q", b)
	}
}

func TestLocalStoreTraversal(t *testing.T) {
	s := &LocalStore{Dir: t.TempDir()}
	for _, key := range []string{"../escape", "req/../../etc/passwd", "/abs/path"} {
		if err := s.Put(context.Background(), key, strings.NewReader("x")); err == nil {
			t.Errorf("key %q must be rejected", key)
		}
	}
	if _, err := s.Open(context.Background(), "../escape"); err == nil {
		t.Error("traversal open must be rejected")
	}
}

func TestLocalStoreUnconfigured(t *testing.T) {
	s := &LocalStore{}
	if err := s.Put(context.Background(), "k", strings.NewReader("x")); err == nil {
		t.Error("unconfigured dir must error")
	}
}
