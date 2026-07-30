package tui

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// openBase is a Model on the report screen with entries and a built report —
// the state every one of these methods is reached from in the real app.
func openBase() Model {
	m := newTestModelOnReport()
	m.report = goldenReport()
	return m
}

func TestOpenMethodsBuildTheirScreen(t *testing.T) {
	t.Parallel()
	t.Run("export", func(t *testing.T) {
		t.Parallel()
		m := openBase().openExport()
		if m.screen != screenExport {
			t.Errorf("screen = %v, want screenExport", m.screen)
		}
		if reflect.DeepEqual(m.export, exportModel{}) {
			t.Error("openExport left the export sub-model at its zero value")
		}
		if len(m.nav) == 0 || m.nav[len(m.nav)-1] != screenReport {
			t.Errorf("nav = %v, want screenReport on top so esc goes back", m.nav)
		}
	})
	t.Run("rates", func(t *testing.T) {
		t.Parallel()
		m := openBase().openRates()
		if m.screen != screenRates {
			t.Errorf("screen = %v, want screenRates", m.screen)
		}
		if reflect.DeepEqual(m.ratesScreen, ratesModel{}) {
			t.Error("openRates left the rates sub-model at its zero value")
		}
	})
	t.Run("log", func(t *testing.T) {
		t.Parallel()
		m := openBase().openLog()
		if m.screen != screenLog {
			t.Errorf("screen = %v, want screenLog", m.screen)
		}
		if reflect.DeepEqual(m.logScreen, logModel{}) {
			t.Error("openLog left the log sub-model at its zero value")
		}
	})
	t.Run("range", func(t *testing.T) {
		t.Parallel()
		// PresetLast7d, not the default: newRange sets only rm.idx, and
		// this_month is rangePresets[0], so newRange(PresetThisMonth) IS the
		// zero rangeModel (range.go:37-45). A zero-value assertion would fail
		// against a perfectly correct openRange.
		m := openBase()
		m.preset = report.PresetLast7d
		m = m.openRange()
		if m.screen != screenRange {
			t.Errorf("screen = %v, want screenRange", m.screen)
		}
		if m.rangeScreen.idx == 0 {
			t.Error("openRange did not seed the cursor from m.preset")
		}
	})
	t.Run("entries", func(t *testing.T) {
		t.Parallel()
		m, _ := openBase().openEntries()
		if m.screen != screenEntries {
			t.Errorf("screen = %v, want screenEntries", m.screen)
		}
	})
}

// openEntries absorbed the lazy user fetch that used to live in updateReport's
// OpenEntries case. Without it here, the palette's "Go to entries" would have
// to copy that branch.
func TestOpenEntriesFetchesTheUserWhenUnknown(t *testing.T) {
	t.Parallel()
	m := openBase()
	m.userID = 0
	if _, cmd := m.openEntries(); cmd == nil {
		t.Error("openEntries returned no command with an unknown user; ownership gating stays off forever")
	}
	m.userID = 42
	if _, cmd := m.openEntries(); cmd != nil {
		t.Error("openEntries re-fetched a user it already knows")
	}
}

func TestOpenFiltersOpensImmediatelyWhenStatusesAreCached(t *testing.T) {
	t.Parallel()
	m := openBase()
	m.taskStatus = map[string]string{}
	for _, e := range m.entries {
		m.taskStatus[e.TaskID] = "open"
	}
	got, cmd := m.openFilters()
	if got.screen != screenFilters {
		t.Errorf("screen = %v, want screenFilters", got.screen)
	}
	if cmd != nil {
		t.Error("openFilters issued an enrichment command although every status was cached")
	}
}

// openMembers takes an origin because that value decides where a load failure
// lands. Pinned to screenHome, a failure raised from the palette on another
// screen would teleport the user Home and blame a screen they were not on.
func TestOpenMembersRoutesFailuresBackToItsOrigin(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := openBase()
	m.client = clickup.New("tok")
	m.client.BaseURL = srv.URL
	m.teamMembers = nil // force the fetch

	_, cmd := m.openMembers(screenRates)
	if cmd == nil {
		t.Fatal("openMembers issued no command with no members cached")
	}
	msg, ok := cmd().(retryableErrMsg)
	if !ok {
		t.Fatalf("command produced %T, want retryableErrMsg", cmd())
	}
	if msg.origin != screenRates {
		t.Errorf("origin = %v, want screenRates — the caller's screen, not a hardcoded Home", msg.origin)
	}
}

// The key handlers must go through the same methods, or the palette and the
// keyboard drift apart.
func TestReportAndHomeKeysReachTheSameScreens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  string
		from func() Model
		want screen
	}{
		{"report export", "e", openBase, screenExport},
		{"report rates", "p", openBase, screenRates},
		{"report log hours", "n", openBase, screenLog},
		{"home range", "d", func() Model { m := newTestModel(); m.screen = screenHome; return m }, screenRange},
		{"home log hours", "n", func() Model { m := newTestModel(); m.screen = screenHome; return m }, screenLog},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := tc.from()
			var got tea.Model
			if m.screen == screenReport {
				got, _ = m.updateReport(keyMsg(tc.key))
			} else {
				got, _ = m.updateHome(keyMsg(tc.key))
			}
			if s := got.(Model).screen; s != tc.want {
				t.Errorf("screen = %v, want %v", s, tc.want)
			}
		})
	}
}
