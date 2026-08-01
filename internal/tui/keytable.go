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
	d   keyDefaults
	set bool
}

// DefaultKeyTable is the built-in table, for callers that want it explicitly.
func DefaultKeyTable() KeyTable { return KeyTable{d: defaultKeys(), set: true} }

func (kt KeyTable) bindings() keyDefaults {
	if !kt.set {
		return defaultKeys()
	}
	return kt.d
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
	}
	return KeyTable{d: d, set: true}, nil
}
