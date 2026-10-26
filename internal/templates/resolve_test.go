package templates

import (
	"encoding/json"
	"testing"

	"github.com/jradams472/isppr/internal/store"
)

func master() *store.TemplateMaster {
	return &store.TemplateMaster{
		ID:   "tmpl_1",
		Html: "<h1>{{title}}</h1>{{#if blocks.header}}H{{/if}}{{#if blocks.promo}}P{{/if}}",
		Css:  "h1{color:blue}",
		Defaults: json.RawMessage(
			`{"toggles":{"header":true},"branding":{"logo":"b/logo.png"}}`),
	}
}

func TestResolveLayers(t *testing.T) {
	base := Resolved{HTML: "<base>", CSS: "base{css}"}

	// Master only: html replaced, css concatenated, defaults applied.
	got, err := Resolve(base, master(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.HTML != master().Html {
		t.Fatalf("master html not used: %q", got.HTML)
	}
	if got.CSS != "base{css}\nh1{color:blue}" {
		t.Fatalf("css layers wrong: %q", got.CSS)
	}
	ctx := got.Context
	if ctx["blocks"].(map[string]bool)["header"] != true ||
		ctx["branding"].(map[string]string)["logo"] != "b/logo.png" {
		t.Fatalf("defaults not merged: %+v", ctx)
	}

	// Variant: css appended, toggle/branding override per key.
	ov := &Overlay{
		CSSPatch: "h1{color:red}",
		Toggles:  map[string]bool{"header": false, "promo": true},
		Branding: map[string]string{"logo": "v/logo.png"},
	}
	got, err = Resolve(base, master(), ov)
	if err != nil {
		t.Fatal(err)
	}
	if got.CSS != "base{css}\nh1{color:blue}\nh1{color:red}" {
		t.Fatalf("patch not appended: %q", got.CSS)
	}
	b := got.Context["blocks"].(map[string]bool)
	if b["header"] != false || b["promo"] != true {
		t.Fatalf("toggles not overlaid: %+v", b)
	}
	if got.Context["branding"].(map[string]string)["logo"] != "v/logo.png" {
		t.Fatalf("branding not overridden: %+v", got.Context["branding"])
	}
}

func TestResolveNilMasterFallsBackToBase(t *testing.T) {
	base := Resolved{HTML: "<base>", CSS: "base{css}"}
	got, err := Resolve(base, nil, &Overlay{CSSPatch: "x{}"})
	if err != nil {
		t.Fatal(err)
	}
	if got.HTML != "<base>" || got.CSS != "base{css}\nx{}" {
		t.Fatalf("base fallback wrong: %+v", got)
	}
}

func TestBaseEmbeds(t *testing.T) {
	r, err := Base()
	if err != nil {
		t.Fatal(err)
	}
	if r.HTML == "" || r.CSS == "" {
		t.Fatal("embedded base template empty")
	}
}

func TestParseOverlayEmpty(t *testing.T) {
	ov, err := ParseOverlay(nil)
	if err != nil || ov.CSSPatch != "" || ov.Toggles != nil {
		t.Fatalf("empty overlay: %+v %v", ov, err)
	}
}

func TestResolveBadMasterJSON(t *testing.T) {
	m := master()
	m.Defaults = json.RawMessage(`{not json`)
	if _, err := Resolve(Resolved{}, m, nil); err == nil {
		t.Fatal("expected error on malformed master defaults")
	}
}

// Variant-introduced keys (absent from master defaults) still land in the
// resolved context — overlays may declare toggles the master never seeded.
func TestVariantIntroducesKeys(t *testing.T) {
	ov := &Overlay{
		Toggles:  map[string]bool{"banner": true},
		Branding: map[string]string{"watermark": "v/wm.png"},
	}
	got, err := Resolve(Resolved{HTML: "b", CSS: "b{}"}, master(), ov)
	if err != nil {
		t.Fatal(err)
	}
	b := got.Context["blocks"].(map[string]bool)
	if !b["banner"] || !b["header"] {
		t.Fatalf("introduced/inherited toggles: %+v", b)
	}
	if got.Context["branding"].(map[string]string)["watermark"] != "v/wm.png" {
		t.Fatalf("introduced branding: %+v", got.Context["branding"])
	}
}

// Empty layers contribute nothing: no stray separators, no empty context keys.
func TestEmptyLayersSkipped(t *testing.T) {
	base := Resolved{HTML: "b", CSS: "base{}"}
	m := master()
	m.Css = ""
	got, err := Resolve(base, m, &Overlay{})
	if err != nil {
		t.Fatal(err)
	}
	if got.CSS != "base{}" {
		t.Fatalf("empty css layers joined: %q", got.CSS)
	}
}

// A base that already carries context keeps it — blocks/branding merge in,
// not over.
func TestBaseContextPreserved(t *testing.T) {
	base := Resolved{
		HTML:    "b",
		CSS:     "b{}",
		Context: map[string]any{"engine": "dge"},
	}
	got, err := Resolve(base, master(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Context["engine"] != "dge" || got.Context["blocks"] == nil {
		t.Fatalf("base context lost: %+v", got.Context)
	}
}

func TestParseOverlayBadJSON(t *testing.T) {
	if _, err := ParseOverlay(json.RawMessage(`{nope`)); err == nil {
		t.Fatal("expected error on malformed overlay")
	}
}

func TestParseDefaultsEmpty(t *testing.T) {
	d, err := ParseDefaults(nil)
	if err != nil || d.Toggles != nil || d.Branding != nil {
		t.Fatalf("empty defaults: %+v %v", d, err)
	}
}

// The Handlebars-stage contract: resolved context merges under caller data,
// caller keys win.
func TestContextWith(t *testing.T) {
	r := Resolved{Context: map[string]any{
		"blocks":   map[string]bool{"header": true},
		"branding": map[string]string{"logo": "m/l.png"},
	}}
	merged := r.ContextWith(map[string]any{"title": "Hi", "blocks": "caller"})
	if merged["title"] != "Hi" || merged["blocks"] != "caller" {
		t.Fatalf("caller precedence: %+v", merged)
	}
	if merged["branding"] == nil {
		t.Fatalf("resolved key lost: %+v", merged)
	}
	// nil data / nil context both safe.
	if got := (Resolved{}).ContextWith(nil); len(got) != 0 {
		t.Fatalf("empty merge: %+v", got)
	}
}

// ResolveFor needs a live DB — see resolve_integration_test.go.

// Page setup merges through the same base→master→variant chain: master
// fields land on Resolved.Page, overlay fields override field-wise.
func TestResolvePageMerge(t *testing.T) {
	m := master()
	m.Defaults = json.RawMessage(`{"page":{"size":"A3","orientation":"landscape","footer_html":"<p>F</p>"}}`)
	got, err := Resolve(Resolved{HTML: "b"}, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Page == nil || got.Page.Size != "A3" || !got.Page.Landscape() || got.Page.FooterHTML != "<p>F</p>" {
		t.Fatalf("master page not resolved: %+v", got.Page)
	}
	ov := &Overlay{Page: &PageSetup{Size: "A4"}}
	got, err = Resolve(Resolved{HTML: "b"}, m, ov)
	if err != nil {
		t.Fatal(err)
	}
	if got.Page.Size != "A4" || got.Page.Orientation != "landscape" || got.Page.FooterHTML != "<p>F</p>" {
		t.Fatalf("overlay page merge wrong: %+v", got.Page)
	}
}
