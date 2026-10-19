package jobs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jradams472/isppr/internal/render"
	"github.com/jradams472/isppr/internal/storage"
	"github.com/jradams472/isppr/internal/store"
	"github.com/jradams472/isppr/internal/templates"
)

// DocPayload is one document in a generation request.
type DocPayload struct {
	Template string         `json:"template_id"`
	Payload  map[string]any `json:"payload"`
}

// GenerateRequest is the POST /packages/generate body.
type GenerateRequest struct {
	ClientID    string       `json:"client_id"`
	CustomerID  string       `json:"customer_id"`
	PackageName string       `json:"package_name"`
	Documents   []DocPayload `json:"documents"`
}

// Metrics mirrors the spec's egress metrics block (durations in ms).
type Metrics struct {
	TotalDocuments int   `json:"total_documents"`
	ParsingMs      int64 `json:"parsing_duration_ms"`
	RenderingMs    int64 `json:"rendering_duration_ms"`
	CompressionMs  int64 `json:"compression_duration_ms"`
	TotalMs        int64 `json:"total_duration_ms"`
}

// Results mirrors the spec's results block.
type Results struct {
	OutputType  string `json:"output_type"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
}

// Orchestrator owns the generation pipeline: enqueue, worker pool, sync
// fast-path wait.
type Orchestrator struct {
	q       *store.Queries
	pool    *render.Pool
	store   storage.Backend
	maxDocs int
	budget  time.Duration
	workers int

	wake chan struct{}
	done sync.Map // requestID → chan struct{}
}

// errForbidden marks tenant-ownership failures — handlers map it to 403.
type forbiddenError string

func (e forbiddenError) Error() string { return string(e) }

func errForbidden(msg string) error { return forbiddenError(msg) }

// IsForbidden reports whether err is a tenant-ownership denial.
func IsForbidden(err error) bool {
	var f forbiddenError
	return errors.As(err, &f)
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// Option tunes the orchestrator.
type Option func(*Orchestrator)

func WithMaxDocs(n int) Option              { return func(o *Orchestrator) { o.maxDocs = n } }
func WithSyncBudget(d time.Duration) Option { return func(o *Orchestrator) { o.budget = d } }
func WithWorkers(n int) Option              { return func(o *Orchestrator) { o.workers = n } }

// New builds an orchestrator over the shared dependencies.
func New(q *store.Queries, pool *render.Pool, st storage.Backend, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		q: q, pool: pool, store: st,
		maxDocs: 25, budget: 8 * time.Second, workers: 4,
		wake: make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Status lifecycle — surfaced verbatim in API responses.
const (
	StatusQueued      = "QUEUED"
	StatusRendering   = "RENDERING"
	StatusCompressing = "COMPRESSING"
	StatusCompleted   = "COMPLETED"
	StatusFailed      = "FAILED"
	maxAttempts       = 3
)

// Submit validates ownership, creates the request + job rows, and wakes a
// worker. Idempotent replay: when key matches an existing request the
// stored row is returned with replayed=true (no new work enqueued).
func (o *Orchestrator) Submit(ctx context.Context, clientID string, req GenerateRequest, idem string) (store.GenerationRequest, bool, error) {
	if idem != "" {
		if prev, err := o.q.GetRequestByIdempotency(ctx, store.GetRequestByIdempotencyParams{
			ClientID: clientID, IdempotencyKey: pgText(idem),
		}); err == nil {
			return prev, true, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return store.GenerationRequest{}, false, err
		}
	}
	// Ownership: client must own the customer and every template name —
	// global deny on any mismatch (docs/authz-matrix.md).
	if _, err := o.q.GetCustomerForClient(ctx, store.GetCustomerForClientParams{
		ID: req.CustomerID, ClientID: clientID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return store.GenerationRequest{}, false, errForbidden("customer_id not owned by tenant")
	} else if err != nil {
		return store.GenerationRequest{}, false, err
	}
	for _, d := range req.Documents {
		if _, err := o.q.GetLatestMaster(ctx, store.GetLatestMasterParams{
			ClientID: clientID, Name: d.Template,
		}); errors.Is(err, pgx.ErrNoRows) {
			return store.GenerationRequest{}, false, errForbidden("template_id not owned by tenant: " + d.Template)
		} else if err != nil {
			return store.GenerationRequest{}, false, err
		}
	}
	row, err := o.q.CreateGenerationRequest(ctx, store.CreateGenerationRequestParams{
		ID:             newID("req"),
		ClientID:       clientID,
		CustomerID:     req.CustomerID,
		IdempotencyKey: pgText(idem),
		PackageName:    req.PackageName,
	})
	if err != nil {
		return store.GenerationRequest{}, false, err
	}
	docs, _ := json.Marshal(req.Documents)
	if _, err := o.q.CreateJob(ctx, store.CreateJobParams{
		ID: newID("job"), RequestID: row.ID, ClientID: clientID, Payload: docs,
	}); err != nil {
		return store.GenerationRequest{}, false, err
	}
	o.done.Store(row.ID, make(chan struct{}))
	select {
	case o.wake <- struct{}{}:
	default:
	}
	return row, false, nil
}

// Await blocks until the worker signals completion or the budget elapses.
func (o *Orchestrator) Await(ctx context.Context, requestID string) bool {
	ch, ok := o.done.Load(requestID)
	if !ok {
		return true // nothing in flight locally (replayed/foreign request)
	}
	timer := time.NewTimer(o.budget)
	defer timer.Stop()
	select {
	case <-ch.(chan struct{}):
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Start launches the worker pool. Workers poll the queue table, claim rows
// FOR UPDATE SKIP LOCKED, and run the full pipeline.
func (o *Orchestrator) Start(ctx context.Context) {
	for range o.workers {
		go o.work(ctx)
	}
}

func (o *Orchestrator) work(ctx context.Context) {
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		job, err := o.q.ClaimJob(ctx)
		switch {
		case err == nil:
			o.execute(ctx, job)
			continue
		case errors.Is(err, pgx.ErrNoRows):
		case errors.Is(err, context.Canceled):
			return
		default:
			slog.Warn("job claim failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-o.wake:
		case <-tick.C:
		}
	}
}

// execute runs one job through resolve → render → pdf → zip → store.
func (o *Orchestrator) execute(ctx context.Context, job store.GenerationJob) {
	start := time.Now()
	fail := func(err error, transient bool) {
		slog.Warn("generation failed", "request", job.RequestID, "err", err)
		if transient && job.Attempts < maxAttempts {
			_ = o.q.RequeueJob(ctx, store.RequeueJobParams{
				ID: job.ID, Error: pgText(err.Error()),
			})
			return
		}
		_ = o.q.SetRequestStatus(ctx, store.SetRequestStatusParams{
			ID: job.RequestID, Status: StatusFailed, Error: pgText(err.Error()),
		})
		_ = o.q.FinishJob(ctx, store.FinishJobParams{
			ID: job.ID, Status: StatusFailed, Error: pgText(err.Error()),
		})
		o.signal(job.RequestID)
	}
	defer func() {
		if r := recover(); r != nil {
			fail(fmt.Errorf("panic: %v", r), false)
		}
	}()

	req, err := o.q.GetRequest(ctx, job.RequestID)
	if err != nil {
		fail(fmt.Errorf("load request: %w", err), false)
		return
	}
	var docs []DocPayload
	if err := json.Unmarshal(job.Payload, &docs); err != nil {
		fail(fmt.Errorf("job payload: %w", err), false)
		return
	}
	if err := o.setStatus(ctx, req.ID, StatusRendering); err != nil {
		fail(err, false)
		return
	}

	// Resolve + bind (parsing stage).
	parsing := time.Now()
	rdocs := make([]render.Doc, len(docs))
	for i, d := range docs {
		resolved, err := templates.ResolveFor(ctx, o.q, req.ClientID, req.CustomerID, d.Template)
		if err != nil {
			fail(fmt.Errorf("resolve %s: %w", d.Template, err), false)
			return
		}
		html, err := render.Render(resolved, d.Payload)
		if err != nil {
			fail(fmt.Errorf("render %s: %w", d.Template, err), false)
			return
		}
		rdocs[i] = render.Doc{ID: d.Template, HTML: html, CSS: resolved.CSS}
	}
	parsingMs := time.Since(parsing).Milliseconds()

	// Gotenberg fan-out (rendering stage).
	rendering := time.Now()
	results := o.pool.PDFBatch(ctx, rdocs)
	renderingMs := time.Since(rendering).Milliseconds()
	for _, r := range results {
		if r.Err != nil {
			var rerr *render.RenderError
			transient := errors.As(r.Err, &rerr) && rerr.Kind != render.KindRenderFailed
			fail(fmt.Errorf("pdf %s: %w", r.ID, r.Err), transient)
			return
		}
	}
	if err := o.setStatus(ctx, req.ID, StatusCompressing); err != nil {
		fail(err, false)
		return
	}

	// Zip + store (compression stage).
	compressing := time.Now()
	filename := filenameFor(req.PackageName, req.ID)
	key := req.ID + "/" + filename
	zipBuf, err := zipResults(results)
	if err != nil {
		fail(fmt.Errorf("zip: %w", err), false)
		return
	}
	if err := o.store.Put(ctx, key, bytes.NewReader(zipBuf)); err != nil {
		fail(fmt.Errorf("store output: %w", err), true)
		return
	}
	compressionMs := time.Since(compressing).Milliseconds()

	metrics, _ := json.Marshal(Metrics{
		TotalDocuments: len(docs),
		ParsingMs:      parsingMs,
		RenderingMs:    renderingMs,
		CompressionMs:  compressionMs,
		TotalMs:        time.Since(start).Milliseconds(),
	})
	if err := o.q.CompleteRequest(ctx, store.CompleteRequestParams{
		ID: req.ID, Metrics: metrics, OutputKey: pgText(key),
	}); err != nil {
		fail(fmt.Errorf("complete: %w", err), false)
		return
	}
	_ = o.q.FinishJob(ctx, store.FinishJobParams{ID: job.ID, Status: "DONE"})
	o.signal(req.ID)
}

func (o *Orchestrator) setStatus(ctx context.Context, id, status string) error {
	return o.q.SetRequestStatus(ctx, store.SetRequestStatusParams{ID: id, Status: status})
}

func (o *Orchestrator) signal(requestID string) {
	if ch, ok := o.done.Load(requestID); ok {
		close(ch.(chan struct{}))
		o.done.Delete(requestID)
	}
}

// zipResults streams PDFs into a single archive, named by doc template.
func zipResults(results []render.Result) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, r := range results {
		name := fmt.Sprintf("%02d_%s.pdf", i+1, sanitize(r.ID))
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(r.PDF); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var unsafeNameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// filenameFor builds "{Package_Name}_{request_id}.zip".
func filenameFor(pkg, requestID string) string {
	return sanitize(pkg) + "_" + requestID + ".zip"
}

func sanitize(s string) string {
	s = unsafeNameChars.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}
