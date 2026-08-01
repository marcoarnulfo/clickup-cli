package tui

import (
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// bindingName turns a keyDefaults field name into the key a user writes in
// their config: LogHours -> log_hours.
//
// An acronym run stays together — PickByID becomes pick_by_id, not
// pick_by_i_d — because the underscore goes in only where a case boundary is a
// word boundary: after a lowercase letter, or before the last capital of a run.
// Measured against every field in keyDefaults; the naive rule produced
// "pick_by_i_d", which would have shipped as a config key.
func bindingName(field string) string {
	r := []rune(field)
	var b strings.Builder
	for i, c := range r {
		if !unicode.IsUpper(c) {
			b.WriteRune(c)
			continue
		}
		prevLower := i > 0 && unicode.IsLower(r[i-1])
		nextLower := i+1 < len(r) && unicode.IsLower(r[i+1])
		if i > 0 && (prevLower || nextLower) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(c))
	}
	return b.String()
}

// BindingNames lists every binding a config may remap, sorted, for error
// messages and for the docs.
//
// Derived rather than written out so that a binding added to keyDefaults is
// remappable with no further work — the maintenance multiplier #82 warned about
// does not exist. The cost is moved instead: renaming a Go field would rename a
// user's config key, which is what TestBindingNamesArePinned exists to catch.
func BindingNames() []string {
	ty := reflect.TypeOf(keyDefaults{})
	out := make([]string, 0, ty.NumField())
	for i := range ty.NumField() {
		out = append(out, bindingName(ty.Field(i).Name))
	}
	slices.Sort(out)
	return out
}
