package templates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	isppr "github.com/jradams472/isppr"
	"github.com/jradams472/isppr/internal/store"
)

// Overlay is the customer-variant payload (template_variants.overlay).
// css_patch is appended to the master CSS; toggles/branding override the
// master's defaults key-by-key. Unknown keys are ignored on read so older
// servers tolerate newer overlays.
type Overlay struct {
	CSSPatch string            `json:"css_patch,omitempty"`
	Toggles  map[string]bool   `json:"toggles,omitempty"`
	Branding map[string]string `json:"branding,omitempty"`
	Page     *PageSetup        `json:"page,omitempty"`
}

// Defaults is the master-owned map a variant overlays (template_masters.
// defaults) — same shape minus css_patch.
type Defaults struct {
	Toggles  map[string]bool   `json:"toggles,omitempty"`
	Branding map[string]string `json:"branding,omitempty"`
	Page     *PageSetup        `json:"page,omitempty"`
}

// Resolved is the effective template handed to the renderer (MPE-33): final
// HTML/CSS plus a Context the renderer merges under caller data — `blocks`
// drives {{#if blocks.<name>}} regions, `branding` swaps asset paths.
// Page is the print-layout block (size/margins/header/footer) forwarded
// to the render pool.
type Resolved struct {
	HTML    string
	CSS     string
	Context map[string]any
	Page    *PageSetup
}

// ParseOverlay decodes a template_variants.overlay row.
func ParseOverlay(raw json.RawMessage) (Overlay, error) {
	var ov Overlay
	if len(raw) == 0 {
		return ov, nil
	}
	return ov, json.Unmarshal(raw, &ov)
}

// ParseDefaults decodes a template_masters.defaults row.
func ParseDefaults(raw json.RawMessage) (Defaults, error) {
	var d Defaults
	if len(raw) == 0 {
		return d, nil
	}
	return d, json.Unmarshal(raw, &d)
}

// Base is the built-in fallback used when no client master resolves a name.
func Base() (Resolved, error) {
	html, err := isppr.Assets.ReadFile("templates/base/default.html")
	if err != nil {
		return Resolved{}, err
	}
	css, err := isppr.Assets.ReadFile("templates/base/default.css")
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{HTML: string(html), CSS: string(css)}, nil
}

// Merge applies the documented order — base <- master <- variant overlay —
// to produce the effective template. CSS layers concatenate (later rules
// win the cascade); toggles and branding merge key-by-key.
func Resolve(base Resolved, master *store.TemplateMaster, ov *Overlay) (Resolved, error) {
	out := base
	if out.Context == nil {
		out.Context = map[string]any{}
	}
	blocks := map[string]bool{}
	branding := map[string]string{}

	if master != nil {
		out.HTML = master.Html
		if master.Css != "" {
			out.CSS = joinCSS(out.CSS, master.Css)
		}
		d, err := ParseDefaults(master.Defaults)
		if err != nil {
			return Resolved{}, fmt.Errorf("master %s defaults: %w", master.ID, err)
		}
		mergeInto(blocks, d.Toggles)
		mergeInto(branding, d.Branding)
		out.Page = mergePage(out.Page, d.Page)
	}
	if ov != nil {
		out.CSS = joinCSS(out.CSS, ov.CSSPatch)
		mergeInto(blocks, ov.Toggles)
		mergeInto(branding, ov.Branding)
		out.Page = mergePage(out.Page, ov.Page)
	}
	if len(blocks) > 0 {
		out.Context["blocks"] = blocks
	}
	if len(branding) > 0 {
		out.Context["branding"] = branding
	}
	return out, nil
}

// ContextWith returns the render context for the Handlebars stage (MPE-33):
// resolved context (blocks/branding) merged under the caller's data —
// caller keys win per top-level key, so payload fields can always shadow
// resolved context.
func (r Resolved) ContextWith(data map[string]any) map[string]any {
	merged := make(map[string]any, len(r.Context)+len(data))
	for k, v := range r.Context {
		merged[k] = v
	}
	for k, v := range data {
		merged[k] = v
	}
	return merged
}

// ResolveFor runs the full lookup chain for (client, customer, template
// name): latest master version, then the customer's variant (attached to
// any version of that name), falling back to the embedded base when the
// client has no master at all.
func ResolveFor(ctx context.Context, q *store.Queries, clientID, customerID, name string) (Resolved, error) {
	base, err := Base()
	if err != nil {
		return Resolved{}, err
	}
	master, err := q.GetLatestMaster(ctx, store.GetLatestMasterParams{
		ClientID: clientID, Name: name,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return base, nil
	}
	if err != nil {
		return Resolved{}, err
	}
	var ov *Overlay
	row, err := q.ResolveVariant(ctx, store.ResolveVariantParams{
		ClientID: clientID, Name: name, CustomerID: customerID,
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return Resolved{}, err
	default:
		parsed, perr := ParseOverlay(row.Overlay)
		if perr != nil {
			return Resolved{}, fmt.Errorf("variant %s overlay: %w", row.ID, perr)
		}
		ov = &parsed
	}
	return Resolve(base, &master, ov)
}

func joinCSS(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += "\n"
		}
		out += p
	}
	return out
}

func mergeInto[T any](dst map[string]T, src map[string]T) {
	for k, v := range src {
		dst[k] = v
	}
}
