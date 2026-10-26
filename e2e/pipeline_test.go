// Package e2e is the pipeline integration suite (MPE-38): real HTTP
// against the compose stack — seeded tenant → POST /generate → status
// polling → presigned download → ZIP/PDF byte assertions, plus failure
// and authz flows. Run via `node tools/e2e.mjs` (brings the stack up) or
// `go test ./e2e/...` against an already-running stack.
package e2e

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var apiURL = envOr("API_URL", "http://localhost:8090")

const (
	demoKey      = "isppr_dev_demo"
	demoCustomer = "cust_demo"
	demoTemplate = "tmpl_invoice"
	demoClient   = "cli_demo"
)

var client = &http.Client{
	Timeout:   30 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

func envOr(k, fb string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fb
}

func TestMain(m *testing.M) {
	resp, err := client.Get(apiURL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("e2e: api unreachable at %s — skipping suite (run `node tools/e2e.mjs`)\n", apiURL)
		os.Exit(0) // skip-gated, same convention as compose integration tests
	}
	resp.Body.Close()
	os.Exit(m.Run())
}

// api issues a tenant request; returns code + decoded body + headers.
func api(t *testing.T, method, path, body, key, idem string) (int, map[string]any, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, apiURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m, resp.Header
}

// adminToken logs into the seeded admin account.
func adminToken(t *testing.T) string {
	t.Helper()
	code, m, _ := api(t, "POST", "/api/v1/admin/login",
		`{"email":"admin@isppr.local","password":"isppr-admin"}`, "", "")
	if code != 200 {
		t.Fatalf("admin login: %d %v", code, m)
	}
	return m["token"].(string)
}

// admin issues a portal request with the session token.
func admin(t *testing.T, tok, method, path, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, apiURL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m
}

func genBody(customer, pkg string, docs ...[2]string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `{"customer_id":%q,"package_name":%q,"documents":[`, customer, pkg)
	for i, d := range docs {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"template_id":%q,"payload":%s}`, d[0], d[1])
	}
	sb.WriteString("]}")
	return sb.String()
}

const invoicePayload = `{"title":"E2E","total":10,"due":"2026-01-01","items":[{"desc":"x","price":10}]}`

// waitDone polls the status endpoint until the request reaches a
// terminal state and returns the final body.
func waitDone(t *testing.T, key, rid string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		code, m, _ := api(t, "GET", "/api/v1/packages/"+rid, "", key, "")
		if code != 200 {
			t.Fatalf("status poll: %d %v", code, m)
		}
		if s := m["status"]; s == "COMPLETED" || s == "FAILED" {
			return m
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("request never reached terminal status")
	return nil
}

// mkTenant creates a full second tenant through the admin API —
// client + customer + template + api key — returning its credentials.
func mkTenant(t *testing.T, tok, sfx string) (customer, template, key string) {
	t.Helper()
	code, cli := admin(t, tok, "POST", "/api/v1/admin/clients",
		fmt.Sprintf(`{"name":"e2e-%s"}`, sfx))
	if code != 201 {
		t.Fatalf("create client: %d %v", code, cli)
	}
	cliID := cli["id"].(string)

	customer = "cust_e2e_" + sfx
	code, _ = admin(t, tok, "POST", "/api/v1/admin/clients/"+cliID+"/customers",
		fmt.Sprintf(`{"name":%q}`, customer))
	if code != 201 {
		t.Fatalf("create customer: %d", code)
	}

	template = "tmpl_e2e_" + sfx
	code, _ = admin(t, tok, "POST", "/api/v1/admin/clients/"+cliID+"/template-masters",
		fmt.Sprintf(`{"name":%q,"html":"<h1>{{title}}</h1>"}`, template))
	if code != 201 {
		t.Fatalf("create master: %d", code)
	}

	code, k := admin(t, tok, "POST", "/api/v1/admin/clients/"+cliID+"/api-keys",
		`{"label":"e2e"}`)
	if code != 201 || k["key"] == nil {
		t.Fatalf("create key: %d %v", code, k)
	}
	return customer, template, k["key"].(string)
}

// --- Flows ---

// Happy path: multi-doc package → 201 → presigned download → real ZIP
// with one PDF per document.
func TestGenerateMultiDoc201(t *testing.T) {
	code, body, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_Multi",
			[2]string{demoTemplate, invoicePayload},
			[2]string{demoTemplate, invoicePayload}),
		demoKey, "e2e-multi-"+time.Now().Format("150405"))
	rid := body["request_id"].(string)
	if code != 201 && code != 202 {
		t.Fatalf("generate: %d %v", code, body)
	}
	final := waitDone(t, demoKey, rid)
	if final["status"] != "COMPLETED" {
		t.Fatalf("final status: %v", final)
	}
	results := final["results"].(map[string]any)
	if results["output_type"] != "ZIP" {
		t.Fatalf("output_type: %v", results)
	}
	dl := results["download_url"].(string)
	if !strings.Contains(dl, "X-Amz-Signature") && !strings.Contains(dl, "/download") {
		t.Fatalf("download_url should be presigned or route: %q", dl)
	}
	resp, err := client.Get(dl)
	if err != nil {
		t.Fatal(err)
	}
	zb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("download fetch: %d", resp.StatusCode)
	}
	zr, err := zip.NewReader(bytes.NewReader(zb), int64(len(zb)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("want 2 docs, got %d", len(zr.File))
	}
	for _, f := range zr.File {
		rc, _ := f.Open()
		head := make([]byte, 4)
		io.ReadFull(rc, head)
		rc.Close()
		if string(head) != "%PDF" {
			t.Fatalf("%s is not a PDF", f.Name)
		}
	}
}

// Idempotent retry: same Idempotency-Key returns the same request_id.
func TestGenerateIdempotent(t *testing.T) {
	idem := "e2e-idem-" + time.Now().Format("150405.000000")
	code1, b1, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_Idem", [2]string{demoTemplate, invoicePayload}), demoKey, idem)
	code2, b2, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_Idem", [2]string{demoTemplate, invoicePayload}), demoKey, idem)
	if (code1 != 201 && code1 != 202) || code1 != code2 {
		t.Fatalf("codes: %d / %d", code1, code2)
	}
	if b1["request_id"] != b2["request_id"] {
		t.Fatalf("replay produced a new request: %v vs %v", b1["request_id"], b2["request_id"])
	}
	if b1["created_at"] != b2["created_at"] {
		t.Fatal("replay did not return the original row")
	}
}

// Render failure: a template with an unknown helper fails closed →
// request lands FAILED with an error.
func TestGenerateRenderFailure(t *testing.T) {
	tok := adminToken(t)
	name := "tmpl_broken_" + fmt.Sprint(time.Now().UnixNano()%1_000_000)
	code, _ := admin(t, tok, "POST", "/api/v1/admin/clients/"+demoClient+"/template-masters",
		fmt.Sprintf(`{"name":%q,"html":"{{bogushelper x}}"}`, name))
	if code != 201 {
		t.Fatalf("seed broken template: %d", code)
	}
	code, body, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_Broken", [2]string{name, `{"x":1}`}), demoKey, "")
	rid := body["request_id"].(string)
	// A job that dies inside the sync budget returns 200+FAILED outright.
	final := body
	if body["status"] != "FAILED" {
		if code != 201 && code != 202 {
			t.Fatalf("generate: %d %v", code, body)
		}
		final = waitDone(t, demoKey, rid)
	}
	if final["status"] != "FAILED" {
		t.Fatalf("want FAILED, got %v", final)
	}
	if final["error"] == nil || final["error"] == "" {
		t.Fatalf("FAILED request must carry error: %v", final)
	}
}

// Cross-tenant authz: tenant B's key cannot touch tenant A's request,
// customer, or template — and missing/bogus keys are rejected outright.
func TestGenerateAuthzDenials(t *testing.T) {
	// Missing + bogus keys → 401.
	for _, key := range []string{"", "not-a-real-key"} {
		code, m, _ := api(t, "POST", "/api/v1/packages/generate",
			genBody(demoCustomer, "x", [2]string{demoTemplate, "{}"}), key, "")
		if code != 401 {
			t.Fatalf("key %q: want 401, got %d %v", key, code, m)
		}
	}
	// Unknown customer/template → 403.
	code, m, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody("cust_nope", "x", [2]string{demoTemplate, "{}"}), demoKey, "")
	if code != 403 {
		t.Fatalf("unknown customer: want 403, got %d %v", code, m)
	}
	code, m, _ = api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "x", [2]string{"tmpl_nope", "{}"}), demoKey, "")
	if code != 403 {
		t.Fatalf("unknown template: want 403, got %d %v", code, m)
	}

	// Second tenant via the admin API; demo's request_id under B's key → 403.
	tok := adminToken(t)
	_, _, keyB := mkTenant(t, tok, fmt.Sprint(time.Now().UnixNano()%1_000_000))
	code, body, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_X", [2]string{demoTemplate, invoicePayload}), demoKey, "")
	rid := body["request_id"].(string)
	if code != 201 && code != 202 {
		t.Fatalf("generate: %d %v", code, body)
	}
	code, m, _ = api(t, "GET", "/api/v1/packages/"+rid, "", keyB, "")
	if code != 403 {
		t.Fatalf("cross-tenant status: want 403, got %d %v", code, m)
	}
	code, m, _ = api(t, "GET", "/api/v1/packages/"+rid+"/download", "", keyB, "")
	if code != 403 {
		t.Fatalf("cross-tenant download: want 403, got %d %v", code, m)
	}
}

// Page setup (MPE-50): a master carrying defaults.page renders through
// real Gotenberg with the requested orientation — asserted on the PDF's
// MediaBox (A5 landscape ≈ 595.92×420pt — wider than tall).
func TestGeneratePageSetup(t *testing.T) {
	tok := adminToken(t)
	name := "tmpl_page_" + fmt.Sprint(time.Now().UnixNano()%1_000_000)
	code, m := admin(t, tok, "POST", "/api/v1/admin/clients/"+demoClient+"/template-masters",
		fmt.Sprintf(`{"name":%q,"html":"<h1>{{title}}</h1>","defaults":{"page":{"size":"A5","orientation":"landscape","footer_html":"<div style=\"font-size:9px\">Page <span class=\"pageNumber\"></span></div>"}}}`, name))
	if code != 201 {
		t.Fatalf("create master: %d %v", code, m)
	}
	code, body, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_Page", [2]string{name, `{"title":"Landscape"}`}), demoKey, "")
	if code != 201 && code != 202 {
		t.Fatalf("generate: %d %v", code, body)
	}
	final := body
	if body["status"] != "COMPLETED" {
		final = waitDone(t, demoKey, body["request_id"].(string))
	}
	if final["status"] != "COMPLETED" {
		t.Fatalf("final: %v", final)
	}
	dl := final["results"].(map[string]any)["download_url"].(string)
	resp, err := client.Get(dl)
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	// The package is a ZIP — extract the lone PDF member.
	pdf := blob
	if bytes.HasPrefix(blob, []byte("PK")) {
		zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
		if err != nil || len(zr.File) != 1 {
			t.Fatalf("zip: %v files=%d", err, len(zr.File))
		}
		rc, _ := zr.File[0].Open()
		pdf, _ = io.ReadAll(rc)
		rc.Close()
	}
	// A5 landscape → MediaBox wider than tall (~595.92×420).
	var w, h float64
	if _, err := fmt.Sscanf(
		strings.TrimSpace(string(mediaBox(t, pdf))),
		"MediaBox [0 0 %f %f]", &w, &h); err != nil || w <= h {
		t.Fatalf("want landscape MediaBox, got %v err=%v", string(mediaBox(t, pdf)), err)
	}
}

// mediaBox finds the page box in the (uncompressed) PDF object stream.
func mediaBox(t *testing.T, pdf []byte) []byte {
	t.Helper()
	if string(pdf[:5]) != "%PDF-" {
		t.Fatalf("not a PDF: %q", pdf[:8])
	}
	i := bytes.Index(pdf, []byte("MediaBox"))
	if i < 0 {
		t.Fatal("no MediaBox in pdf")
	}
	end := bytes.IndexByte(pdf[i:], ']')
	return pdf[i : i+end+1]
}

// MPE-56: a print-ready template's generated PDF is post-processed
// through Ghostscript — CMYK color space, OutputIntent, embedded fonts.
func TestGeneratePrintReady(t *testing.T) {
	tok := adminToken(t)
	name := "tmpl_cmyk_" + fmt.Sprint(time.Now().UnixNano()%1_000_000)
	code, m := admin(t, tok, "POST", "/api/v1/admin/clients/"+demoClient+"/template-masters",
		fmt.Sprintf(`{"name":%q,"html":"<h1 style=\"color:#d00\">{{title}}</h1>","defaults":{"print":{"enabled":true}}}`, name))
	if code != 201 {
		t.Fatalf("create master: %d %v", code, m)
	}
	code, body, _ := api(t, "POST", "/api/v1/packages/generate",
		genBody(demoCustomer, "E2E_Print", [2]string{name, `{"title":"CMYK"}`}), demoKey, "")
	if code != 201 && code != 202 {
		t.Fatalf("generate: %d %v", code, body)
	}
	final := body
	if body["status"] != "COMPLETED" {
		final = waitDone(t, demoKey, body["request_id"].(string))
	}
	if final["status"] != "COMPLETED" {
		t.Fatalf("final: %v", final)
	}
	dl := final["results"].(map[string]any)["download_url"].(string)
	resp, err := client.Get(dl)
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	pdf := blob
	if bytes.HasPrefix(blob, []byte("PK")) {
		zr, _ := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
		rc, _ := zr.File[0].Open()
		pdf, _ = io.ReadAll(rc)
		rc.Close()
	}
	s := string(pdf)
	for _, marker := range []string{"/DeviceCMYK", "/OutputIntent", "/FontFile"} {
		if !strings.Contains(s, marker) {
			t.Fatalf("post-processed PDF missing %s", marker)
		}
	}
}
