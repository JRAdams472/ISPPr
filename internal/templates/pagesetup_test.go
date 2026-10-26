package templates

import "testing"

func TestPageSetupValidate(t *testing.T) {
	cases := []struct {
		name    string
		page    *PageSetup
		wantErr bool
	}{
		{"nil", nil, false},
		{"empty", &PageSetup{}, false},
		{"a4 portrait", &PageSetup{Size: "A4", Orientation: "portrait"}, false},
		{"a4 landscape", &PageSetup{Size: "a4", Orientation: "landscape"}, false},
		{"ansi e", &PageSetup{Size: "ANSIE"}, false},
		{"legal", &PageSetup{Size: "legal"}, false},
		{"unknown size", &PageSetup{Size: "B9"}, true},
		{"bad orientation", &PageSetup{Orientation: "diagonal"}, true},
		{"negative margin", &PageSetup{Margins: &Margins{Top: -1}}, true},
		{"good margins", &PageSetup{Margins: &Margins{Top: 1, Bottom: 0.5}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.page.Validate()
			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v got %v", tc.wantErr, err)
			}
		})
	}
}

func TestPageSetupDimensions(t *testing.T) {
	w, h := (&PageSetup{Size: "A4"}).Dimensions()
	if w != 8.27 || h != 11.69 {
		t.Fatalf("a4 portrait: %vx%v", w, h)
	}
	// Landscape stays portrait dims — Chromium rotates via the flag.
	w, h = (&PageSetup{Size: "A4", Orientation: "landscape"}).Dimensions()
	if w != 8.27 || h != 11.69 {
		t.Fatalf("a4 landscape dims stay portrait: %vx%v", w, h)
	}
	w, h = (&PageSetup{Size: "ANSID"}).Dimensions()
	if w != 22 || h != 34 {
		t.Fatalf("ansid: %vx%v", w, h)
	}
	// Unknown size falls back to A4 rather than failing at render time.
	w, h = (&PageSetup{Size: "ZZZ"}).Dimensions()
	if w != 8.27 || h != 11.69 {
		t.Fatalf("fallback a4: %vx%v", w, h)
	}
	if !(&PageSetup{Orientation: "landscape"}).Landscape() {
		t.Fatal("landscape flag")
	}
	var nilPage *PageSetup
	if nilPage.Landscape() {
		t.Fatal("nil landscape")
	}
	if w, h := nilPage.Dimensions(); w != 8.27 || h != 11.69 {
		t.Fatalf("nil dims: %vx%v", w, h)
	}
}

func TestMergePageFieldwise(t *testing.T) {
	master := &PageSetup{
		Size: "A3", Orientation: "landscape",
		Margins:    &Margins{Top: 1, Left: 1},
		HeaderHTML: "<p>hdr</p>",
	}
	// Variant overrides only the footer — size/margins/header survive.
	ov := &PageSetup{FooterHTML: "<p>ftr</p>"}
	got := mergePage(master, ov)
	if got.Size != "A3" || got.Orientation != "landscape" ||
		got.Margins.Top != 1 || got.Margins.Left != 1 ||
		got.HeaderHTML != "<p>hdr</p>" || got.FooterHTML != "<p>ftr</p>" {
		t.Fatalf("fieldwise merge wrong: %+v", got)
	}
	if master.FooterHTML != "" {
		t.Fatal("merge mutated master")
	}
	// Override margins partially.
	ov2 := &PageSetup{Margins: &Margins{Right: 2}}
	got = mergePage(master, ov2)
	if got.Margins.Right != 2 || got.Margins.Top != 1 {
		t.Fatalf("margin merge wrong: %+v", got.Margins)
	}
	// nil src is a passthrough; nil dst copies.
	if mergePage(master, nil) != master {
		t.Fatal("nil src passthrough")
	}
	if cp := mergePage(nil, ov); cp == nil || cp.FooterHTML != "<p>ftr</p>" {
		t.Fatal("nil dst copy")
	}
}
