package tui

import (
	"slices"
	"testing"
)

func TestBindingName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"Quit", "quit"},
		{"LogHours", "log_hours"},
		{"PaletteUp", "palette_up"},
		{"ForceQuit", "force_quit"},
		// An acronym run stays together. The naive rule — underscore before
		// every capital — turns this into "pick_by_i_d", which would have
		// shipped as a config key.
		{"PickByID", "pick_by_id"},
		{"ID", "id"},
		{"HTTPServer", "http_server"},
	} {
		if got := bindingName(tc.in); got != tc.want {
			t.Errorf("bindingName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBindingNameTreatsTitlecaseAsCapital(t *testing.T) {
	t.Parallel()
	if got, want := bindingName("Log\u01c5name"), "log_\u01c6name"; got != want {
		t.Errorf("bindingName with a titlecase rune = %q, want %q", got, want)
	}
}

// This is the test that matters most in this task. The config keys a user
// writes are derived from Go field names, so renaming a field silently renames
// a key in every config file in the wild. Spelling the list out here means the
// rename fails a test instead of failing a user.
func TestBindingNamesArePinned(t *testing.T) {
	t.Parallel()
	want := []string{
		"back", "browse_list", "budget", "change_range", "clear_value",
		"confirm", "confirm_delete", "delete", "down", "edit",
		"export", "filters", "force_quit", "generate", "group_by",
		"help", "history", "list_budget", "list_currency", "log_hours",
		"members", "new_override", "new_tag", "next_field", "next_month",
		"next_section", "no", "open_entries", "palette", "palette_down",
		"palette_up", "pick_by_id", "pick_guided", "pick_timer", "prev_field",
		"prev_month", "prev_section", "quit", "range", "rates",
		"reload", "save", "select_all", "stop_timer", "tags",
		"timer", "toggle_item", "toggle_scope", "toggle_week", "up",
		"yes",
	}
	got := BindingNames()
	if !slices.Equal(got, want) {
		t.Errorf("BindingNames() = %v\nwant %v", got, want)
	}
}

func TestBindingNamesAreUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, n := range BindingNames() {
		if seen[n] {
			t.Errorf("duplicate derived name %q — two fields collapse to the same config key", n)
		}
		seen[n] = true
	}
}
