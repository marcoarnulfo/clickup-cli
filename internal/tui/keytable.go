package tui

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

// forceQuitName is the one binding a config may not remap: ctrl+c is the way
// out of a TUI whose other keys the user has moved somewhere unreachable.
const forceQuitName = "force_quit"

// KeyTable is the resolved binding table the TUI routes and renders with.
//
// The zero value means the built-in defaults, which is what a Model built by
// hand carries — the tests construct 108 of those without going through New,
// and a zero table read literally would leave every binding disabled.
type KeyTable struct {
	d    keyDefaults
	over map[string]bool
	set  bool
}

// DefaultKeyTable is the built-in table, for callers that want it explicitly.
func DefaultKeyTable() KeyTable { return KeyTable{d: defaultKeys(), set: true} }

func (kt KeyTable) bindings() keyDefaults {
	if !kt.set {
		return defaultKeys()
	}
	return kt.d
}

// keysOf returns the keys currently bound to a binding, by config name.
func (kt KeyTable) keysOf(name string) []string {
	d := kt.bindings()
	v := reflect.ValueOf(d)
	ty := v.Type()
	for i := range v.NumField() {
		if bindingName(ty.Field(i).Name) == name {
			return v.Field(i).Interface().(key.Binding).Keys()
		}
	}
	panic(fmt.Sprintf("unknown binding name %q", name))
}

func (kt KeyTable) remapped(names ...string) bool {
	for _, name := range names {
		if kt.over[name] {
			return true
		}
	}
	return false
}

// label returns lit unless one of the named bindings was remapped, in which
// case it derives the label from the keys actually bound.
//
// The literal labels carry typography the defaults earned, such as "↑/↓/j/k"
// and "tab/⇧tab". Deriving them unconditionally would make the default footer
// longer and uglier, so the literal wins until it would be a lie.
func (kt KeyTable) label(lit string, names ...string) string {
	var keys []string
	for _, name := range names {
		keys = append(keys, kt.keysOf(name)...)
	}
	if !kt.remapped(names...) {
		return lit
	}
	return strings.Join(keys, "/")
}

// setHelp is label's counterpart for single-binding SetHelp call sites.
func (kt KeyTable) setHelp(b *key.Binding, name, lit, desc string) {
	b.SetHelp(kt.label(lit, name), desc)
}

// claims maps every physical key to the sorted names of the bindings that want
// it. The defaults are deliberately overloaded — measured, 20 keys have more
// than one claimant — because screenKeys enables only a subset per screen.
func claims(d keyDefaults) map[string][]string {
	out := map[string][]string{}
	v := reflect.ValueOf(d)
	ty := v.Type()
	for i := range v.NumField() {
		b := v.Field(i).Interface().(key.Binding)
		name := bindingName(ty.Field(i).Name)
		for _, k := range b.Keys() {
			out[k] = append(out[k], name)
		}
	}
	for k := range out {
		slices.Sort(out[k])
	}
	return out
}

// checkCollisions rejects a post-override key with multiple claimants unless
// all of them already claimed that key in the defaults.
//
// Detecting real conflicts would mean asking, per screen, which bindings are
// enabled at once — 14 screens plus the sub-modes of entries, log, rates and
// setup, a table to keep in sync with every screen ever added. This rule is
// computed from the key table alone, and is deliberately conservative: it
// refuses taking a key that another binding still claims, even when those
// bindings never share a screen. A free key and a clean swap remain allowed.
func checkCollisions(before, after keyDefaults) error {
	was, now := claims(before), claims(after)
	for _, k := range slices.Sorted(maps.Keys(now)) {
		names := now[k]
		if len(names) < 2 {
			continue
		}
		for _, n := range names {
			if slices.Contains(was[k], n) {
				continue
			}
			others := slices.DeleteFunc(slices.Clone(names), func(s string) bool { return s == n })
			return fmt.Errorf("binding %q cannot take key %q: it is already claimed by %s",
				n, k, strings.Join(others, ", "))
		}
	}
	return nil
}

// ResolveKeys applies a config's overrides to the built-in table.
//
// Every failure is an error rather than a fallback: a key that cannot be
// honored would leave the user in front of a TUI where a command simply does
// not answer, with nothing to go on. Same rule as billing.rounding.increment
// and as the theme resolution.
func ResolveKeys(overrides map[string]config.KeySpec) (KeyTable, error) {
	d := defaultKeys()
	v := reflect.ValueOf(&d).Elem()
	ty := v.Type()
	over := map[string]bool{}

	index := map[string]int{}
	for i := range ty.NumField() {
		index[bindingName(ty.Field(i).Name)] = i
	}

	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		if name == forceQuitName {
			return KeyTable{}, fmt.Errorf(
				"binding %q cannot be remapped: ctrl+c is the way out of a TUI whose other keys have moved", name)
		}
		i, ok := index[name]
		if !ok {
			// force_quit is filtered out of the suggestion list: offering the
			// one name that is then rejected would be a message that argues
			// with itself.
			valid := slices.DeleteFunc(BindingNames(), func(s string) bool { return s == forceQuitName })
			return KeyTable{}, fmt.Errorf("unknown binding %q; valid names: %s",
				name, strings.Join(valid, ", "))
		}
		ks := slices.Clone(overrides[name])
		if len(ks) == 0 {
			return KeyTable{}, fmt.Errorf("binding %q needs at least one key", name)
		}
		seen := map[string]bool{}
		for _, k := range ks {
			if k == "" {
				return KeyTable{}, fmt.Errorf("binding %q has an empty key in its list", name)
			}
			canonical := ""
			switch k {
			case "ctrl+i":
				canonical = "tab"
			case "ctrl+m":
				canonical = "enter"
			}
			if canonical != "" {
				return KeyTable{}, fmt.Errorf("binding %q uses noncanonical key %q; use %q instead", name, k, canonical)
			}
			// Caught here rather than by the collision rule, which would report
			// the binding colliding with itself and name no other claimant.
			if seen[k] {
				return KeyTable{}, fmt.Errorf("binding %q lists key %q twice", name, k)
			}
			seen[k] = true
		}
		old := v.Field(i).Interface().(key.Binding)
		// The help string carries the key inside it, so it has to be rebuilt or
		// the footer would advertise a key that no longer does anything. The
		// typographic arrows the defaults use (↑/k, tab/▸) are lost for
		// remapped bindings: the user chose these keys, and inventing a
		// prettier rendering for them would be guessing.
		v.Field(i).Set(reflect.ValueOf(key.NewBinding(
			key.WithKeys(ks...),
			key.WithHelp(strings.Join(ks, "/"), old.Help().Desc),
		)))
		over[name] = true
	}
	if err := checkCollisions(defaultKeys(), d); err != nil {
		return KeyTable{}, err
	}
	return KeyTable{d: d, over: over, set: true}, nil
}
