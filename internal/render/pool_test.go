package render

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jradams472/isppr/internal/templates"
)

const gotenbergPath = "/forms/chromium/convert/html"

// fakeGotenberg returns a handler that records multipart parts and answers
// 200 with a canned PDF.
func fakeGotenberg(t *testing.T, parts *map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != gotenbergPath {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if parts != nil {
			_ = r.ParseMultipartForm(1 << 20)
			for _, f := range r.MultipartForm.File["files"] {
				rc, _ := f.Open()
				b, _ := io.ReadAll(rc)
				(*parts)[f.Filename] = string(b)
				rc.Close()
			}
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-fake"))
	}
}

func TestPDFHappyPath(t *testing.T) {
	parts := map[string]string{}
	srv := httptest.NewServer(fakeGotenberg(t, &parts))
	defer srv.Close()

	p, err := NewPool([]string{srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := p.PDF(context.Background(), "<html><head></head><body>hi</body></html>", "h1{color:red}", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf) != "%PDF-fake" {
		t.Fatalf("pdf body: %q", pdf)
	}
	doc, ok := parts["index.html"]
	if !ok {
		t.Fatalf("index.html missing: %v", parts)
	}
	if !strings.Contains(doc, "<style>\nh1{color:red}\n</style>\n</head>") {
		t.Fatalf("css not injected into head: %q", doc)
	}
	if _, ok := parts["style.css"]; !ok {
		t.Fatal("style.css part missing")
	}
}

func TestPDFNoCSS(t *testing.T) {
	parts := map[string]string{}
	srv := httptest.NewServer(fakeGotenberg(t, &parts))
	defer srv.Close()
	p, _ := NewPool([]string{srv.URL})
	if _, err := p.PDF(context.Background(), "<p>x</p>", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := parts["style.css"]; ok {
		t.Fatal("style.css sent for empty css")
	}
	if !strings.HasPrefix(parts["index.html"], "<p>x</p>") {
		t.Fatalf("doc mutated: %q", parts["index.html"])
	}
}

func TestRenderFailed4xxNoRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "bad html", http.StatusBadRequest)
	}))
	defer srv.Close()
	p, _ := NewPool([]string{srv.URL}, WithRetries(3))
	_, err := p.PDF(context.Background(), "x", "", nil)
	var rerr *RenderError
	if !errors.As(err, &rerr) || rerr.Kind != KindRenderFailed || rerr.Status != 400 {
		t.Fatalf("want RenderFailed/400: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("4xx must not retry, got %d calls", calls.Load())
	}
}

func TestTransientRetriesAcrossPool(t *testing.T) {
	var calls atomic.Int32
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer flaky.Close()
	var goodCalls atomic.Int32
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodCalls.Add(1)
		w.Write([]byte("%PDF-good"))
	}))
	defer good.Close()

	p, _ := NewPool([]string{flaky.URL, good.URL},
		WithRetries(2), WithBackoff(time.Millisecond))
	pdf, err := p.PDF(context.Background(), "x", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf) != "%PDF-good" {
		t.Fatalf("pdf: %q", pdf)
	}
	// first attempt hits an endpoint; retry eventually lands on the good one
	if goodCalls.Load() < 1 {
		t.Fatal("retry never reached healthy endpoint")
	}
}

func TestPoolUnavailableAfterRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()
	p, _ := NewPool([]string{srv.URL}, WithRetries(1), WithBackoff(time.Millisecond))
	_, err := p.PDF(context.Background(), "x", "", nil)
	var rerr *RenderError
	if !errors.As(err, &rerr) || rerr.Kind != KindPoolUnavailable {
		t.Fatalf("want PoolUnavailable: %v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()
	p, _ := NewPool([]string{srv.URL},
		WithTimeout(50*time.Millisecond), WithRetries(0))
	_, err := p.PDF(context.Background(), "x", "", nil)
	var rerr *RenderError
	if !errors.As(err, &rerr) || rerr.Kind != KindTimeout {
		t.Fatalf("want Timeout: %v", err)
	}
	// Exhausted retries preserve the underlying kind — a slow pool is a
	// timeout, not pool_unavailable (transient 5xx keeps its own kind).
	p2, _ := NewPool([]string{srv.URL},
		WithTimeout(50*time.Millisecond), WithRetries(1), WithBackoff(time.Millisecond))
	_, err = p2.PDF(context.Background(), "x", "", nil)
	var rerr2 *RenderError
	if !errors.As(err, &rerr2) || rerr2.Kind != KindTimeout {
		t.Fatalf("exhausted timeouts stay Timeout, got %v", err)
	}
}

func TestBatchIndependentResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		f, _ := r.MultipartForm.File["files"][0].Open()
		b, _ := io.ReadAll(f)
		f.Close()
		if strings.Contains(string(b), "BAD") {
			http.Error(w, "rejected", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "%PDF:"+strings.TrimSpace(string(b)))
	}))
	defer srv.Close()

	p, _ := NewPool([]string{srv.URL}, WithMaxConcurrent(4))
	docs := []Doc{
		{ID: "a", HTML: "ok-a"},
		{ID: "b", HTML: "BAD-b"},
		{ID: "c", HTML: "ok-c"},
	}
	res := p.PDFBatch(context.Background(), docs)
	if len(res) != 3 {
		t.Fatalf("results: %d", len(res))
	}
	if res[0].Err != nil || !strings.Contains(string(res[0].PDF), "ok-a") {
		t.Fatalf("doc a: %v %q", res[0].Err, res[0].PDF)
	}
	var rerr *RenderError
	if !errors.As(res[1].Err, &rerr) || rerr.Kind != KindRenderFailed {
		t.Fatalf("doc b must fail independently: %v", res[1].Err)
	}
	if res[2].Err != nil {
		t.Fatalf("doc c must succeed despite b: %v", res[2].Err)
	}
}

func TestNewPoolNoEndpoints(t *testing.T) {
	if _, err := NewPool(nil); err == nil {
		t.Fatal("expected error for empty endpoints")
	}
}

// Integration: real Gotenberg from docker compose (port 3001). Skip-gated —
// CI exercises this only when GOTENBERG_URL points at a live pool.
func TestComposeGotenberg(t *testing.T) {
	ep := os.Getenv("GOTENBERG_URL")
	if ep == "" {
		ep = "http://localhost:3001"
	}
	p, err := NewPool([]string{ep}, WithTimeout(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := p.PDF(ctx, "<html><body>probe</body></html>", "", nil); err != nil {
		t.Skipf("gotenberg unreachable at %s: %v", ep, err)
	}
	cancel()

	// Concurrent renders against the real pool.
	docs := make([]Doc, 6)
	for i := range docs {
		docs[i] = Doc{
			ID:   fmt.Sprintf("d%d", i),
			HTML: fmt.Sprintf("<html><head></head><body><h1>doc %d</h1></body></html>", i),
			CSS:  "h1{color:#123456}",
		}
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()
	start := time.Now()
	res := p.PDFBatch(ctx2, docs)
	for _, r := range res {
		if r.Err != nil {
			t.Fatalf("%s: %v", r.ID, r.Err)
		}
		if !strings.HasPrefix(string(r.PDF), "%PDF") {
			t.Fatalf("%s: not a pdf (%d bytes)", r.ID, len(r.PDF))
		}
	}
	t.Logf("6 concurrent renders via real gotenberg in %v", time.Since(start))
}

// PageOptions emit the Gotenberg print fields plus header/footer file
// parts; nil page sends none of them.
func TestPDFPageOptions(t *testing.T) {
	var fields map[string][]string
	parts := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		fields = r.MultipartForm.Value
		for _, f := range r.MultipartForm.File["files"] {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			parts[f.Filename] = string(b)
			rc.Close()
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-fake"))
	}))
	defer srv.Close()
	p, _ := NewPool([]string{srv.URL})

	pg := &PageOptions{
		WidthIn: 16.54, HeightIn: 11.69, Landscape: true,
		MarginTop: 0.8, MarginBottom: 0.8,
		Header: "<p>hdr</p>", Footer: "<span class=\"pageNumber\"></span>",
	}
	if _, err := p.PDF(context.Background(), "<p>x</p>", "", pg); err != nil {
		t.Fatal(err)
	}
	if fields["paperWidth"][0] != "16.54" || fields["paperHeight"][0] != "11.69" {
		t.Fatalf("paper size: %v", fields)
	}
	if fields["landscape"][0] != "true" {
		t.Fatalf("landscape missing: %v", fields)
	}
	if fields["marginTop"][0] != "0.8" || fields["marginBottom"][0] != "0.8" {
		t.Fatalf("margins: %v", fields)
	}
	if _, ok := fields["marginLeft"]; ok {
		t.Fatal("zero margin should not be sent")
	}
	if parts["header.html"] != "<p>hdr</p>" || parts["footer.html"] == "" {
		t.Fatalf("header/footer parts: %v", parts)
	}

	// nil page → no print fields.
	fields = nil
	parts = map[string]string{}
	if _, err := p.PDF(context.Background(), "<p>x</p>", "", nil); err != nil {
		t.Fatal(err)
	}
	if fields["paperWidth"] != nil || parts["header.html"] != "" {
		t.Fatalf("nil page emitted fields: %v %v", fields, parts)
	}
}

func TestPageOptionsFrom(t *testing.T) {
	pg := PageOptionsFrom(nil, "", "")
	if pg != nil {
		t.Fatal("nil setup must yield nil options")
	}
	o := PageOptionsFrom(&templates.PageSetup{
		Size: "A4", Orientation: "landscape",
		Margins: &templates.Margins{Top: 1, Right: 0.5},
	}, "<p>h</p>", "<p>f</p>")
	// Portrait dims + flag — Chromium rotates, we don't swap.
	if o.WidthIn != 8.27 || o.HeightIn != 11.69 || !o.Landscape {
		t.Fatalf("dims: %+v", o)
	}
	if o.MarginTop != 1 || o.MarginRight != 0.5 || o.MarginBottom != 0 {
		t.Fatalf("margins: %+v", o)
	}
	if o.Header != "<p>h</p>" || o.Footer != "<p>f</p>" {
		t.Fatalf("parts: %+v", o)
	}
}
