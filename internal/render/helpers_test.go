package render

import (
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b any
		want int
	}{
		{1, 2, -1}, {2, 1, 1}, {2, 2, 0},
		{1.5, 1.4, 1}, {"3", "4", -1},
		{"a", "b", -1}, {"b", "a", 1}, {"a", "a", 0},
		{false, true, -1}, {true, false, 1}, {true, true, 0},
		{"x", "x", 0}, // string fallback equality
	}
	for _, c := range cases {
		if got := compare(c.a, c.b); got != c.want {
			t.Errorf("compare(%v,%v)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestTruthy(t *testing.T) {
	falsy := []any{nil, false, "", 0, int64(0), uint(0), 0.0, []any{}, map[string]any{}}
	for _, v := range falsy {
		if truthy(v) {
			t.Errorf("truthy(%v) = true, want false", v)
		}
	}
	truthyVals := []any{true, "x", 1, -1.5, []any{1}, map[string]any{"a": 1}, struct{}{}}
	for _, v := range truthyVals {
		if !truthy(v) {
			t.Errorf("truthy(%v) = false, want true", v)
		}
	}
}

func TestToFloat(t *testing.T) {
	for _, tc := range []struct {
		v    any
		want float64
		ok   bool
	}{
		{1, 1, true}, {int64(2), 2, true}, {float32(1.5), 1.5, true},
		{2.5, 2.5, true}, {"3.5", 3.5, true}, {" ", 0, false},
		{"abc", 0, false}, {true, 0, false}, {nil, 0, false},
	} {
		got, ok := toFloat(tc.v)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("toFloat(%v)=%v,%v want %v,%v", tc.v, got, ok, tc.want, tc.ok)
		}
	}
}

func TestToTime(t *testing.T) {
	if tm, ok := toTime("2026-06-15"); !ok || tm.Year() != 2026 {
		t.Errorf("date string: %v %v", tm, ok)
	}
	if tm, ok := toTime("2026-06-15 10:30:00"); !ok || tm.Hour() != 10 {
		t.Errorf("datetime string: %v %v", tm, ok)
	}
	if tm, ok := toTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)); !ok || !tm.Equal(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("time.Time: %v %v", tm, ok)
	}
	if _, ok := toTime("garbage"); ok {
		t.Error("garbage string parsed")
	}
	if _, ok := toTime(nil); ok {
		t.Error("nil parsed")
	}
	// Unix is interpreted in UTC — deterministic across host timezones.
	if tm, ok := toTime(int64(1767225600)); !ok || tm.Format("2006") != "2026" {
		t.Errorf("unix utc: %v %v", tm, ok)
	}
}

func TestGroupThousands(t *testing.T) {
	for _, tc := range []struct {
		f    float64
		want string
	}{
		{0, "0.00"}, {1, "1.00"}, {1234.5, "1,234.50"},
		{1234567.891, "1,234,567.89"}, {-42.1, "-42.10"},
		{999.999, "1,000.00"}, // carry on rounding
		{12, "12.00"},
	} {
		if got := groupThousands(tc.f); got != tc.want {
			t.Errorf("groupThousands(%v)=%q want %q", tc.f, got, tc.want)
		}
	}
}

func TestStr(t *testing.T) {
	if str(nil) != "" || str("x") != "x" || str(42) != "42" {
		t.Fatal("str conversions")
	}
}

func TestTitleCase(t *testing.T) {
	if got := titleCase("john DOE"); got != "John Doe" {
		t.Fatalf("title: %q", got)
	}
	if got := titleCase(nil); got != "" {
		t.Fatalf("title nil: %q", got)
	}
}

// Validation: call-position callee must be whitelisted; sub-expressions and
// block inlines are walked too.
func TestValidateHelpers(t *testing.T) {
	ok := []string{
		"{{foo}} {{a.b.c}}",                       // path lookups, not calls
		"{{#if x}}y{{/if}}",                       // built-in block
		"{{#each items as |i|}}{{i}}{{/each}}",    // block params fine
		"{{upper name}} {{#if (eq a b)}}x{{/if}}", // custom + sub-expr
		"{{> partial}}",                           // partials not call-checked
	}
	for _, src := range ok {
		if err := validateHelpers(src); err != nil {
			t.Errorf("valid source rejected: %q: %v", src, err)
		}
	}
	bad := []string{
		"{{nosuch x}}",
		"{{#nosuch x}}y{{/nosuch}}",
		"{{#if (nosuch a)}}x{{/if}}",  // nested sub-expr call
		"{{lookup m key=(nosuch b)}}", // hash-position sub-expr
		"{{a.b x}}",                   // call on non-simple path
	}
	for _, src := range bad {
		if err := validateHelpers(src); err == nil {
			t.Errorf("invalid source accepted: %q", src)
		}
	}
	if _, err := Compile("{{#nosuch x}}y{{/nosuch}}"); err == nil {
		t.Error("Compile must fail closed on unknown block helper")
	}
	if _, err := Compile("{{nosuch x}}"); err == nil {
		t.Error("Compile must fail closed on unknown helper")
	}
	if _, err := Compile("{{{"); err == nil {
		t.Error("Compile must surface parse errors")
	}
}
