package tui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestWheelKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   tea.MouseMsg
		want tea.KeyType
		ok   bool
	}{
		{"wheel up", tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress}, tea.KeyUp, true},
		{"wheel down", tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}, tea.KeyDown, true},
		{"wheel left", tea.MouseMsg{Button: tea.MouseButtonWheelLeft, Action: tea.MouseActionPress}, 0, false},
		{"wheel right", tea.MouseMsg{Button: tea.MouseButtonWheelRight, Action: tea.MouseActionPress}, 0, false},
		{"left click", tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}, 0, false},
		{"release", tea.MouseMsg{Button: tea.MouseButtonNone, Action: tea.MouseActionRelease}, 0, false},
		{"motion", tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}, 0, false},
	} {
		got, ok := wheelKey(tc.in)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && got.Type != tc.want {
			t.Errorf("%s: key type = %v, want %v", tc.name, got.Type, tc.want)
		}
	}
}

// The wheel must reach the active screen through the same path a key takes, so
// every screen that already handles up/down scrolls without being modified.
func TestWheelScrollsTheActiveScreen(t *testing.T) {
	m := filtersScreenFixture()
	if m.filtersScreen.row != 0 {
		t.Fatalf("fixture starts at row %d, want 0", m.filtersScreen.row)
	}

	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = u.(Model)
	if m.filtersScreen.row != 1 {
		t.Fatalf("row = %d after one wheel-down notch, want 1", m.filtersScreen.row)
	}

	u, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = u.(Model)
	if m.filtersScreen.row != 0 {
		t.Fatalf("row = %d after wheel-up, want back to 0", m.filtersScreen.row)
	}
}

// A click carries no meaning yet (#74 ships the wheel only), and must not be
// mistaken for a keystroke by the screen underneath.
//
// The cursor is moved off row 0 first, and deliberately: asserting "row 0
// before, row 0 after" would also pass if a click were translated into an UP
// key, because at row 0 the cursor is already at the top and cannot move. From
// row 1 the test sees both mistranslations.
func TestClickChangesNothing(t *testing.T) {
	m := filtersScreenFixture()
	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = u.(Model)
	if m.filtersScreen.row != 1 {
		t.Fatalf("setup: row = %d after one wheel-down, want 1", m.filtersScreen.row)
	}

	u, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := u.(Model).filtersScreen.row; got != 1 {
		t.Errorf("row = %d after a click, want it to stay 1 — clicks are ignored", got)
	}
}

// With an overlay open the overlay owns the wheel, exactly as it owns the
// keyboard: the screen underneath must not scroll behind it.
func TestWheelGoesToTheOverlayWhenOneIsOpen(t *testing.T) {
	m := openPaletteOn(newTestModelOnReport())
	if len(m.palette.items) < 2 {
		t.Fatalf("the fixture has %d actions; this test needs at least 2", len(m.palette.items))
	}

	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	got := u.(Model)
	if got.palette.idx != 1 {
		t.Errorf("palette idx = %d after wheel-down, want 1", got.palette.idx)
	}
}

// The error screen dismisses itself on any key. A wheel notch is not an answer
// to it: with mouse reporting on and a trackpad's inertia, one stray notch
// would wipe an error message before it had been read.
func TestWheelDoesNotDismissTheErrorScreen(t *testing.T) {
	m := newTestModel()
	m.screen = screenError
	m.err = errors.New("boom")

	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if got := u.(Model).screen; got != screenError {
		t.Errorf("screen = %v after a wheel notch, want it to stay on the error screen", got)
	}
}

// Same rule, second context: the delete confirmation answers "any key but y"
// with a cancel, and a wheel notch must not be that answer.
func TestWheelDoesNotCancelTheDeleteConfirmation(t *testing.T) {
	m := browserWithEntries(newTestModel(),
		report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now()})
	u, _ := m.Update(keyMsg("x"))
	m = u.(Model)
	if m.entriesScreen.mode != entriesConfirmDelete {
		t.Fatalf("setup: mode = %v, want the confirm dialog open", m.entriesScreen.mode)
	}

	u, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if got := u.(Model).entriesScreen.mode; got != entriesConfirmDelete {
		t.Errorf("mode = %v after a wheel notch, want the confirm dialog still open", got)
	}
}
