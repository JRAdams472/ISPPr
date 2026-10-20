package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// ErrorKind classifies render failures for job status + metrics.
type ErrorKind int

const (
	// KindRenderFailed — Gotenberg rejected the document (4xx). Permanent.
	KindRenderFailed ErrorKind = iota
	// KindPoolUnavailable — every attempt failed transiently (network,
	// 429, 5xx). The request may be retried by the caller.
	KindPoolUnavailable
	// KindTimeout — a render attempt exceeded the per-render deadline.
	KindTimeout
)

func (k ErrorKind) String() string {
	switch k {
	case KindRenderFailed:
		return "render_failed"
	case KindPoolUnavailable:
		return "pool_unavailable"
	case KindTimeout:
		return "timeout"
	}
	return "unknown"
}

// RenderError is the structured failure every pool call returns — never a
// panic, never a bare string.
type RenderError struct {
	Kind     ErrorKind
	Endpoint string
	Status   int // last HTTP status, 0 for transport failures
	cause    error
}

func (e *RenderError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("render %s (%s, http %d): %v", e.Kind, e.Endpoint, e.Status, e.cause)
	}
	return fmt.Sprintf("render %s (%s): %v", e.Kind, e.Endpoint, e.cause)
}

func (e *RenderError) Unwrap() error { return e.cause }

// Pool is a round-robin client over a set of Gotenberg endpoints.
type Pool struct {
	endpoints []string
	timeout   time.Duration
	retries   int
	backoff   time.Duration
	maxConc   int
	http      *http.Client
	next      atomic.Uint64
}

// PoolOption tunes Pool construction; sensible defaults apply otherwise.
type PoolOption func(*Pool)

func WithTimeout(d time.Duration) PoolOption   { return func(p *Pool) { p.timeout = d } }
func WithRetries(n int) PoolOption             { return func(p *Pool) { p.retries = n } }
func WithBackoff(d time.Duration) PoolOption   { return func(p *Pool) { p.backoff = d } }
func WithMaxConcurrent(n int) PoolOption       { return func(p *Pool) { p.maxConc = n } }
func WithHTTPClient(c *http.Client) PoolOption { return func(p *Pool) { p.http = c } }

// NewPool builds a render-pool client. endpoints must be non-empty.
func NewPool(endpoints []string, opts ...PoolOption) (*Pool, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("render pool needs at least one endpoint")
	}
	p := &Pool{
		endpoints: endpoints,
		timeout:   30 * time.Second,
		retries:   2,
		backoff:   200 * time.Millisecond,
		http:      http.DefaultClient,
	}
	for _, o := range opts {
		o(p)
	}
	if p.maxConc <= 0 {
		p.maxConc = 2 * len(endpoints)
	}
	return p, nil
}

// PDF renders one document (rendered HTML + resolved CSS) to a PDF via
// /forms/chromium/convert/html, retrying transient failures with backoff
// across the pool.
func (p *Pool) PDF(ctx context.Context, html, css string) ([]byte, error) {
	var last *RenderError
	backoff := p.backoff
	for attempt := 0; attempt <= p.retries; attempt++ {
		ep := p.endpoint()
		out, err := p.convert(ctx, ep, html, css)
		if err == nil {
			return out, nil
		}
		var rerr *RenderError
		if errors.As(err, &rerr) && rerr.Kind == KindRenderFailed {
			return nil, rerr // permanent — no retry
		}
		last = rerr
		if attempt < p.retries {
			select {
			case <-ctx.Done():
				return nil, &RenderError{Kind: KindTimeout, cause: ctx.Err()}
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
	if last != nil {
		return nil, last
	}
	return nil, &RenderError{Kind: KindPoolUnavailable, cause: errors.New("no endpoints")}
}

// Doc is one item of a multi-document package.
type Doc struct {
	ID   string
	HTML string
	CSS  string
}

// Result pairs a doc ID with its PDF or its error — batch callers get
// independent outcomes, never a single failure swallowing the batch.
type Result struct {
	ID  string
	PDF []byte
	Err error
}

// PDFBatch fans a document set across the pool, bounded to maxConc
// in-flight renders.
func (p *Pool) PDFBatch(ctx context.Context, docs []Doc) []Result {
	results := make([]Result, len(docs))
	var g errgroup.Group
	g.SetLimit(p.maxConc)
	for i, d := range docs {
		results[i].ID = d.ID
		g.Go(func() error {
			pdf, err := p.PDF(ctx, d.HTML, d.CSS)
			results[i].PDF, results[i].Err = pdf, err
			return nil // per-doc errors ride in the Result, not the group
		})
	}
	_ = g.Wait()
	return results
}

func (p *Pool) endpoint() string {
	i := p.next.Add(1)
	return p.endpoints[int(i)%len(p.endpoints)]
}

// convert does a single attempt against one endpoint.
func (p *Pool) convert(ctx context.Context, ep, html, css string) ([]byte, error) {
	ctx, span := otel.Tracer("isppr/render").Start(ctx, "gotenberg.convert",
		trace.WithAttributes(
			attribute.String("gotenberg.endpoint", ep),
			attribute.String("http.route", "/forms/chromium/convert/html")))
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	body, ctype, err := multipartBody(html, css)
	if err != nil {
		return nil, &RenderError{Kind: KindRenderFailed, Endpoint: ep, cause: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ep+"/forms/chromium/convert/html", body)
	if err != nil {
		return nil, &RenderError{Kind: KindPoolUnavailable, Endpoint: ep, cause: err}
	}
	req.Header.Set("Content-Type", ctype)

	resp, err := p.http.Do(req)
	if err != nil {
		span.RecordError(err)
		kind := KindPoolUnavailable
		if errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(ctx.Err(), context.DeadlineExceeded) || isTimeout(err) {
			kind = KindTimeout
		}
		return nil, &RenderError{Kind: kind, Endpoint: ep, cause: err}
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("http %d", resp.StatusCode))
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, &RenderError{Kind: KindPoolUnavailable, Endpoint: ep, cause: err}
		}
		return out, nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	kind := KindRenderFailed
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		kind = KindPoolUnavailable
	}
	return nil, &RenderError{
		Kind: kind, Endpoint: ep, Status: resp.StatusCode,
		cause: fmt.Errorf("%s", strings.TrimSpace(string(snippet))),
	}
}

// multipartBody builds the Gotenberg payload: index.html with the resolved
// CSS injected as a <style> element, plus style.css for completeness.
func multipartBody(html, css string) (*bytes.Buffer, string, error) {
	doc := injectCSS(html, css)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := writePart(w, "files", "index.html", "text/html", []byte(doc)); err != nil {
		return nil, "", err
	}
	if css != "" {
		if err := writePart(w, "files", "style.css", "text/css", []byte(css)); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// injectCSS drops a <style> block into the document head (or prepends one
// if the fragment has no head).
func injectCSS(html, css string) string {
	if css == "" {
		return html
	}
	tag := "<style>\n" + css + "\n</style>"
	if i := strings.Index(strings.ToLower(html), "</head>"); i >= 0 {
		return html[:i] + tag + "\n" + html[i:]
	}
	return tag + "\n" + html
}

func writePart(w *multipart.Writer, field, name, ctype string, data []byte) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, name))
	h.Set("Content-Type", ctype)
	part, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
