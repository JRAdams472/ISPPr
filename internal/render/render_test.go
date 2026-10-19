package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jradams472/isppr/internal/templates"
)

// Golden tests: each testdata/<name>.hbs renders with <name>.json (the full
// merged context, as ContextWith produces) and must byte-match
// <name>.golden.html (modulo line endings).
func TestGolden(t *testing.T) {
	pairs, err := filepath.Glob("testdata/*.hbs")
	if err != nil || len(pairs) == 0 {
		t.Fatalf("no golden fixtures: %v", err)
	}
	for _, hbs := range pairs {
		name := strings.TrimSuffix(filepath.Base(hbs), ".hbs")
		t.Run(name, func(t *testing.T) {
			tpl, err := Compile(read(t, hbs))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			var data map[string]any
			if err := json.Unmarshal(
				[]byte(read(t, "testdata/"+name+".json")), &data); err != nil {
				t.Fatalf("payload: %v", err)
			}
			got, err := tpl.Exec(data)
			if err != nil {
				t.Fatalf("exec: %v", err)
			}
			want := read(t, "testdata/"+name+".golden.html")
			if norm(got) != norm(want) {
				t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func norm(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Render applies the Resolved→payload merge contract (ContextWith).
func TestRenderMergesContext(t *testing.T) {
	resolved := templates.Resolved{
		HTML: "{{greeting}} {{#if blocks.promo}}PROMO{{/if}} {{branding.logo}}",
		Context: map[string]any{
			"blocks":   map[string]bool{"promo": true},
			"branding": map[string]string{"logo": "v/l.png"},
		},
	}
	got, err := Render(resolved, map[string]any{"greeting": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hi PROMO v/l.png" {
		t.Fatalf("rendered %q", got)
	}
	// Caller data wins on collisions.
	got, err = Render(resolved, map[string]any{"blocks": map[string]bool{}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "  v/l.png" {
		t.Fatalf("caller did not shadow blocks: %q", got)
	}
}

func TestCompileError(t *testing.T) {
	if _, err := Compile("{{#if}}"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := Compile("{{unclosed"); err == nil {
		t.Fatal("expected parse error")
	}
}

// Unknown helpers fail closed; unknown paths render empty.
func TestUnknownHelperAndPath(t *testing.T) {
	if _, err := Compile("{{nosuchhelper x}}"); err == nil {
		if tpl, cerr := Compile("{{nosuchhelper x}}"); cerr == nil {
			if _, eerr := tpl.Exec(map[string]any{}); eerr == nil {
				t.Fatal("unknown helper must error at parse or exec")
			}
		}
	}
	tpl, err := Compile("{{deep.missing.path}}")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.Exec(map[string]any{})
	if err != nil || got != "" {
		t.Fatalf("missing path: %q %v", got, err)
	}
}
