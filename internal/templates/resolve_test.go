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

// ResolveFor needs a live DB — see resolve_integration_test.go.
