package templates

import (
	"fmt"
	"strings"
)

// PageSetup is the print-layout block carried in defaults.page (and
// overridable via overlay.page). Size is a symbolic name; margins are
// inches — the unit Gotenberg's Chromium route expects.
type PageSetup struct {
	Size        string   `json:"size,omitempty"`
	Orientation string   `json:"orientation,omitempty"` // portrait | landscape
	Margins     *Margins `json:"margins,omitempty"`
	// HeaderHTML/FooterHTML are Handlebars-bound page strips rendered by
	// Chromium's print header/footer mechanism — incrementing page
	// numbers come from the .pageNumber/.totalPages classes.
	HeaderHTML string `json:"header_html,omitempty"`
	FooterHTML string `json:"footer_html,omitempty"`
}

// Margins are inches; a zero value means "Chromium default for that side".
type Margins struct {
	Top    float64 `json:"top,omitempty"`
	Bottom float64 `json:"bottom,omitempty"`
	Left   float64 `json:"left,omitempty"`
	Right  float64 `json:"right,omitempty"`
}

// pageSizes maps a size name to portrait width×height in inches. A0–A8,
// the ANSI series, and the common US names are all reachable; unknown
// names fail validation rather than silently rendering A4.
var pageSizes = map[string][2]float64{
	"A0":      {33.11, 46.81},
	"A1":      {23.39, 33.11},
	"A2":      {16.54, 23.39},
	"A3":      {11.69, 16.54},
	"A4":      {8.27, 11.69},
	"A5":      {5.83, 8.27},
	"A6":      {4.13, 5.83},
	"A7":      {2.91, 4.13},
	"A8":      {2.05, 2.91},
	"ANSIA":   {8.5, 11},
	"ANSIB":   {11, 17},
	"ANSIC":   {17, 22},
	"ANSID":   {22, 34},
	"ANSIE":   {34, 44},
	"LETTER":  {8.5, 11},
	"LEGAL":   {8.5, 14},
	"TABLOID": {11, 17},
	"LEDGER":  {17, 11},
}

// Validate rejects unknown sizes, bad orientations, and negative margins.
// Header/footer Handlebars validity is checked by the write path (it
// needs render.Compile, which would be an import cycle from here).
func (p *PageSetup) Validate() error {
	if p == nil {
		return nil
	}
	if p.Size != "" {
		if _, ok := pageSizes[strings.ToUpper(p.Size)]; !ok {
			return fmt.Errorf("page.size %q is not a known size", p.Size)
		}
	}
	switch p.Orientation {
	case "", "portrait", "landscape":
	default:
		return fmt.Errorf("page.orientation %q must be portrait or landscape", p.Orientation)
	}
	if m := p.Margins; m != nil {
		for name, v := range map[string]float64{
			"top": m.Top, "bottom": m.Bottom, "left": m.Left, "right": m.Right,
		} {
			if v < 0 {
				return fmt.Errorf("page.margins.%s must be >= 0", name)
			}
		}
	}
	return nil
}

// Dimensions returns the paper's portrait width×height in inches —
// orientation is conveyed separately via Landscape() so Chromium can
// perform the rotation itself. An empty/unknown size resolves to A4:
// Validate is the write-path gate; this fallback keeps rendering alive on
// a stale row.
func (p *PageSetup) Dimensions() (w, h float64) {
	d := pageSizes["A4"]
	if p != nil {
		if v, found := pageSizes[strings.ToUpper(p.Size)]; found {
			d = v
		}
	}
	return d[0], d[1]
}

// Landscape reports whether the page renders rotated — forwarded to
// Gotenberg's `landscape` field in addition to the dimension swap.
func (p *PageSetup) Landscape() bool {
	return p != nil && p.Orientation == "landscape"
}

// mergePage overlays src onto dst field-wise: only fields set on src
// override, so a variant can patch just header_html without respecifying
// size/margins. A nil dst becomes a fresh PageSetup.
func mergePage(dst *PageSetup, src *PageSetup) *PageSetup {
	if src == nil {
		return dst
	}
	if dst == nil {
		cp := *src
		return &cp
	}
	out := *dst
	if src.Size != "" {
		out.Size = src.Size
	}
	if src.Orientation != "" {
		out.Orientation = src.Orientation
	}
	if src.Margins != nil {
		m := out.Margins
		if m == nil {
			m = &Margins{}
		}
		om := *m
		if src.Margins.Top != 0 {
			om.Top = src.Margins.Top
		}
		if src.Margins.Bottom != 0 {
			om.Bottom = src.Margins.Bottom
		}
		if src.Margins.Left != 0 {
			om.Left = src.Margins.Left
		}
		if src.Margins.Right != 0 {
			om.Right = src.Margins.Right
		}
		out.Margins = &om
	}
	if src.HeaderHTML != "" {
		out.HeaderHTML = src.HeaderHTML
	}
	if src.FooterHTML != "" {
		out.FooterHTML = src.FooterHTML
	}
	return &out
}
