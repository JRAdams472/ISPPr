package render

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/aymerick/raymond"
	"github.com/jradams472/isppr/internal/templates"
)

// init registers the fixed helper surface documented in
// docs/rendering.md. raymond helpers are process-global — the surface is
// frozen here once, no user-registered helpers exist.
func init() {
	raymond.RegisterHelpers(map[string]any{
		"eq":    func(a, b any) bool { return compare(a, b) == 0 },
		"ne":    func(a, b any) bool { return compare(a, b) != 0 },
		"lt":    func(a, b any) bool { return compare(a, b) < 0 },
		"le":    func(a, b any) bool { return compare(a, b) <= 0 },
		"gt":    func(a, b any) bool { return compare(a, b) > 0 },
		"ge":    func(a, b any) bool { return compare(a, b) >= 0 },
		"and":   func(a, b any) bool { return truthy(a) && truthy(b) },
		"or":    func(a, b any) bool { return truthy(a) || truthy(b) },
		"not":   func(a any) bool { return !truthy(a) },
		"upper": func(s any) string { return strings.ToUpper(str(s)) },
		"lower": func(s any) string { return strings.ToLower(str(s)) },
		"title": titleCase,
		"join": func(list any, sep string) string {
			v := reflect.ValueOf(list)
			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return str(list)
			}
			parts := make([]string, v.Len())
			for i := range v.Len() {
				parts[i] = str(v.Index(i).Interface())
			}
			return strings.Join(parts, sep)
		},
		// date formats a value (RFC3339 string, unix number, or time.Time)
		// with a Go layout — e.g. {{date due "2006-01-02"}}.
		"date": func(v any, layout string) string {
			if t, ok := toTime(v); ok {
				return t.Format(layout)
			}
			return ""
		},
		// currency formats a number as "<amount> <code>" with grouped
		// thousands — e.g. {{currency total "USD"}} → "1,234.56 USD".
		"currency": func(v any, code string) string {
			f, ok := toFloat(v)
			if !ok {
				return ""
			}
			return fmt.Sprintf("%s %s", groupThousands(f), code)
		},
	})
}

// Template is a compiled Handlebars template, safe for concurrent Exec.
type Template struct {
	tpl *raymond.Template
}

// Compile parses resolved HTML into a reusable template, rejecting calls to
// helpers outside the frozen surface (fail closed — see docs/rendering.md).
func Compile(html string) (*Template, error) {
	if err := validateHelpers(html); err != nil {
		return nil, err
	}
	t, err := raymond.Parse(html)
	if err != nil {
		return nil, err
	}
	return &Template{tpl: t}, nil
}

// Exec renders with the given data map. Use Resolved.ContextWith(payload)
// to bind a generation request's payload under tenant context.
func (t *Template) Exec(data map[string]any) (string, error) {
	return t.tpl.Exec(data)
}

// Render resolves one generation: compile + exec in a single call. For
// hot paths compiling once and Exec-ing many times, split the calls.
func Render(resolved templates.Resolved, payload map[string]any) (string, error) {
	tpl, err := Compile(resolved.HTML)
	if err != nil {
		return "", err
	}
	return tpl.Exec(resolved.ContextWith(payload))
}

// BindPageParts execs a resolved PageSetup's header/footer strips through
// raymond with the same context as the body — payloads and branding can
// appear in page strips. A nil PageSetup returns empty strings.
func BindPageParts(resolved templates.Resolved, payload map[string]any) (header, footer string, err error) {
	if resolved.Page == nil {
		return "", "", nil
	}
	ctx := resolved.ContextWith(payload)
	if resolved.Page.HeaderHTML != "" {
		t, err := Compile(resolved.Page.HeaderHTML)
		if err != nil {
			return "", "", fmt.Errorf("header: %w", err)
		}
		if header, err = t.Exec(ctx); err != nil {
			return "", "", fmt.Errorf("header exec: %w", err)
		}
	}
	if resolved.Page.FooterHTML != "" {
		t, err := Compile(resolved.Page.FooterHTML)
		if err != nil {
			return "", "", fmt.Errorf("footer: %w", err)
		}
		if footer, err = t.Exec(ctx); err != nil {
			return "", "", fmt.Errorf("footer exec: %w", err)
		}
	}
	return header, footer, nil
}
