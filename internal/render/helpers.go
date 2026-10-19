package render

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// compare orders two values: numbers numerically, strings lexically, bools
// as false<true; mixed/unknown kinds fall back to string equality.
// Returns -1/0/1.
func compare(a, b any) int {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			}
			return 0
		}
	}
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
			return strings.Compare(as, bs)
		}
	}
	if ab, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			if ab == bb {
				return 0
			}
			if bb {
				return -1
			}
			return 1
		}
	}
	if str(a) == str(b) {
		return 0
	}
	return -1
}

// truthy mirrors Handlebars truthiness: false, nil, 0, "", empty
// collections are falsy; everything else is truthy.
func truthy(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.String:
		return rv.Len() > 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len() > 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0
	}
	return true
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func titleCase(s any) string {
	return cases.Title(language.English).String(str(s))
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func toTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC(), true
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed.UTC(), true
			}
		}
	case int64:
		return time.Unix(t, 0).UTC(), true
	case float64:
		return time.Unix(int64(t), 0).UTC(), true
	}
	return time.Time{}, false
}

// groupThousands renders a float with comma grouping and two decimals —
// good enough for document display; no locale machinery.
func groupThousands(f float64) string {
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	intPart := int64(f)
	frac := int64((f-float64(intPart))*100 + 0.5)
	if frac == 100 {
		intPart++
		frac = 0
	}
	digits := strconv.FormatInt(intPart, 10)
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return fmt.Sprintf("%s%s.%02d", sign, b.String(), frac)
}
