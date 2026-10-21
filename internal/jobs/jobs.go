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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

// Output types surfaced in results.output_type.
const (
	OutputZIP = "ZIP"
	OutputPDF = "PDF"
)

// Orchestrator owns the generation pipeline: enqueue, worker pool, sync
// fast-path wait.
type Orchestrator struct {
	q               *store.Queries
	pool            *render.Pool
	store           storage.Backend
	maxDocs         int
	budget          time.Duration
	workers         int
	jobTimeout      time.Duration
	maxBody         int64
	skipZipSingle   bool
	webhookSecret   string
	webhookAttempts int

	wake chan struct{}
	done sync.Map // requestID → chan struct{}

	wg       sync.WaitGroup // active workers, drained by Shutdown
	inflight sync.Map       // jobID → struct{} — requeued on shutdown
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
func WithJobTimeout(d time.Duration) Option { return func(o *Orchestrator) { o.jobTimeout = d } }
func WithMaxBody(n int64) Option            { return func(o *Orchestrator) { o.maxBody = n } }
func WithSingleDocSkipZip(on bool) Option   { return func(o *Orchestrator) { o.skipZipSingle = on } }
func WithWebhook(secret string, max int) Option {
	return func(o *Orchestrator) { o.webhookSecret, o.webhookAttempts = secret, max }
}

// New builds an orchestrator over the shared dependencies.
func New(q *store.Queries, pool *render.Pool, st storage.Backend, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		q: q, pool: pool, store: st,
		maxDocs: 25, budget: 8 * time.Second, workers: 4,
		jobTimeout: 2 * time.Minute, maxBody: 4 << 20,
		webhookAttempts: 5,
		wake:            make(chan struct{}, 1),
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
	ctx, enq := otel.Tracer("isppr/jobs").Start(ctx, "queue.enqueue",
		trace.WithAttributes(
			attribute.String("client.id", clientID),
			attribute.String("customer.id", req.CustomerID),
			attribute.String("package.name", req.PackageName),
			attribute.Int("document.count", len(req.Documents))))
	row, err := o.q.CreateGenerationRequest(ctx, store.CreateGenerationRequestParams{
		ID:             newID("req"),
		ClientID:       clientID,
		CustomerID:     req.CustomerID,
		IdempotencyKey: pgText(idem),
		PackageName:    req.PackageName,
	})
	if err != nil {
		enq.RecordError(err)
		enq.SetStatus(codes.Error, err.Error())
		enq.End()
		return store.GenerationRequest{}, false, err
	}
	enq.SetAttributes(attribute.String("request.id", row.ID))
	docs, _ := json.Marshal(req.Documents)
	if _, err := o.q.CreateJob(ctx, store.CreateJobParams{
		ID: newID("job"), RequestID: row.ID, ClientID: clientID, Payload: docs,
	}); err != nil {
		enq.RecordError(err)
		enq.SetStatus(codes.Error, err.Error())
		enq.End()
		return store.GenerationRequest{}, false, err
	}
	enq.End()
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
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
			o.work(ctx)
		}()
	}
}

// Shutdown drains the pipeline: workers stop claiming, in-flight jobs get
// up to ctx's deadline to finish, and anything still claimed is requeued
// so a stopped instance leaves no zombies in CLAIMED.
func (o *Orchestrator) Shutdown(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
	// Whatever still holds a claim belongs to a worker this deadline
	// abandoned — put it back so the next instance picks it up.
	o.inflight.Range(func(k, _ any) bool {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = o.q.RequeueJob(cctx, store.RequeueJobParams{
			ID: k.(string), Error: pgText("interrupted by shutdown"),
		})
		return true
	})
}

func (o *Orchestrator) work(ctx context.Context) {
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		job, err := o.q.ClaimJob(ctx)
		switch {
		case err == nil:
			o.inflight.Store(job.ID, struct{}{})
			o.execute(ctx, job)
			o.inflight.Delete(job.ID)
			continue
		case errors.Is(err, pgx.ErrNoRows):
		case errors.Is(err, context.Canceled):
			return
		default:
			slog.WarnContext(ctx, "job claim failed", "err", err)
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
	// Global ceiling for one pipeline run — a hung dependency can't hold a
	// claim forever (the lease index is the crash-recovery backstop).
	ctx, cancel := context.WithTimeout(ctx, o.jobTimeout)
	defer cancel()
	// The worker picks up async — there is no inbound request context to
	// continue, so this is the root of the generation trace tree.
	ctx, span := otel.Tracer("isppr/jobs").Start(ctx, "job.execute",
		trace.WithAttributes(
			attribute.String("request.id", job.RequestID),
			attribute.String("job.id", job.ID),
			attribute.Int("job.attempt", int(job.Attempts))))
	defer span.End()
	fail := func(err error, transient bool) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.WarnContext(ctx, "generation failed", "request", job.RequestID, "err", err)
		// Cleanup must outlive a cancelled pipeline ctx — otherwise a
		// job interrupted mid-flight would be stranded as CLAIMED.
		cctx, ccancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer ccancel()
		if transient && job.Attempts < maxAttempts {
			_ = o.q.RequeueJob(cctx, store.RequeueJobParams{
				ID: job.ID, Error: pgText(err.Error()),
			})
			return
		}
		_ = o.q.SetRequestStatus(cctx, store.SetRequestStatusParams{
			ID: job.RequestID, Status: StatusFailed, Error: pgText(err.Error()),
		})
		_ = o.q.FinishJob(cctx, store.FinishJobParams{
			ID: job.ID, Status: StatusFailed, Error: pgText(err.Error()),
		})
		o.signal(job.RequestID)
		go o.deliver(cctx, job.RequestID) // terminal failure still notifies
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
		_, dspan := otel.Tracer("isppr/jobs").Start(ctx, "doc.bind",
			trace.WithAttributes(attribute.String("template.id", d.Template)))
		resolved, err := templates.ResolveFor(ctx, o.q, req.ClientID, req.CustomerID, d.Template)
		if err != nil {
			dspan.RecordError(err)
			dspan.SetStatus(codes.Error, err.Error())
			dspan.End()
			fail(fmt.Errorf("resolve %s: %w", d.Template, err), false)
			return
		}
		html, err := render.Render(resolved, d.Payload)
		if err != nil {
			dspan.RecordError(err)
			dspan.SetStatus(codes.Error, err.Error())
			dspan.End()
			fail(fmt.Errorf("render %s: %w", d.Template, err), false)
			return
		}
		dspan.End()
		rdocs[i] = render.Doc{ID: d.Template, HTML: html, CSS: resolved.CSS}
	}
	parsingMs := time.Since(parsing).Milliseconds()

	// Gotenberg fan-out (rendering stage).
	rendering := time.Now()
	rctx, rspan := otel.Tracer("isppr/jobs").Start(ctx, "render.batch",
		trace.WithAttributes(attribute.Int("document.count", len(rdocs))))
	results := o.pool.PDFBatch(rctx, rdocs)
	rspan.End()
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

	// Package + store (compression stage). SINGLE_DOC_SKIP_ZIP uploads a
	// lone PDF raw — output_type PDF per the ticket's acceptance.
	compressing := time.Now()
	raw := o.skipZipSingle && len(results) == 1
	filename := ArtifactName(req.PackageName, req.ID, raw)
	key := req.ID + "/" + filename
	var data []byte
	if raw {
		data = results[0].PDF
	} else {
		_, zspan := otel.Tracer("isppr/jobs").Start(ctx, "zip.compress")
		if data, err = zipResults(results); err != nil {
			zspan.RecordError(err)
			zspan.SetStatus(codes.Error, err.Error())
			zspan.End()
			fail(fmt.Errorf("zip: %w", err), false)
			return
		}
		zspan.SetAttributes(attribute.Int("package.bytes", len(data)))
		zspan.End()
	}
	_, uspan := otel.Tracer("isppr/jobs").Start(ctx, "storage.put",
		trace.WithAttributes(attribute.String("output.key", key)))
	if err := o.store.Put(ctx, key, bytes.NewReader(data)); err != nil {
		uspan.RecordError(err)
		uspan.SetStatus(codes.Error, err.Error())
		uspan.End()
		fail(fmt.Errorf("store output: %w", err), true)
		return
	}
	uspan.End()
	compressionMs := time.Since(compressing).Milliseconds()

	m := Metrics{
		TotalDocuments: len(docs),
		ParsingMs:      parsingMs,
		RenderingMs:    renderingMs,
		CompressionMs:  compressionMs,
		TotalMs:        time.Since(start).Milliseconds(),
	}
	span.SetAttributes(
		attribute.Int("metrics.total_documents", m.TotalDocuments),
		attribute.Int64("metrics.parsing_ms", m.ParsingMs),
		attribute.Int64("metrics.rendering_ms", m.RenderingMs),
		attribute.Int64("metrics.compression_ms", m.CompressionMs),
		attribute.Int64("metrics.total_ms", m.TotalMs))
	metrics, _ := json.Marshal(m)
	if err := o.q.CompleteRequest(ctx, store.CompleteRequestParams{
		ID: req.ID, Metrics: metrics, OutputKey: pgText(key),
	}); err != nil {
		fail(fmt.Errorf("complete: %w", err), false)
		return
	}
	_ = o.q.FinishJob(ctx, store.FinishJobParams{ID: job.ID, Status: "DONE"})
	o.signal(req.ID)
	go o.deliver(ctx, req.ID) // signed callback when the customer has a webhook_url
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

// ArtifactName builds "{Package_Name}_{request_id}.{zip|pdf}" — the
// spec's naming convention for stored artifacts.
func ArtifactName(pkg, requestID string, rawPDF bool) string {
	ext := ".zip"
	if rawPDF {
		ext = ".pdf"
	}
	return sanitize(pkg) + "_" + requestID + ext
}

// artifactBase strips the request prefix from a stored output key.
func artifactBase(key string) string {
	if i := strings.LastIndex(key, "/"); i != -1 {
		return key[i+1:]
	}
	return key
}

// outputTypeFor derives the response's output_type from the stored
// artifact extension.
func outputTypeFor(key string) string {
	if strings.HasSuffix(key, ".pdf") {
		return OutputPDF
	}
	return OutputZIP
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
