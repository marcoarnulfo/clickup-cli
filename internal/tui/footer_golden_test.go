package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// TestGoldenFooters pins the short and full footer for every label set the
// keymap parity tests cover — same granularity, different assertion. Where a
// parity test is table-driven (log, entries, rates, range, setup), every row
// gets its own case here too, mirroring exactly how that row builds its
// Model. screenLoading and screenError have no parity test of their own
// (keysFor handles them inline rather than delegating to a per-screen
// function) but are covered here since they are real states of keysFor's
// switch.
//
// This is the only check on Task 2's `full` columns: a binding named in a
// `full` group but never assigned in that branch is a zero key.Binding, which
// help.FullHelpView silently drops — no compile error, no failing parity
// test, nothing else would ever notice.
func TestGoldenFooters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		model func() Model
	}{
		// screenHome (keys_test.go:34 TestHomeKeyLabels).
		{"home", func() Model {
			m := newTestModel()
			m.screen = screenHome
			m.scope = "me"
			return m
		}},

		// screenLoading / screenError: no delegating function, handled inline
		// in keysFor's switch.
		{"loading", func() Model { return Model{screen: screenLoading} }},
		{"error", func() Model { return Model{screen: screenError} }},

		// screenBudget (budget_test.go:53 TestBudgetKeyLabels).
		{"budget", func() Model { return Model{screen: screenBudget} }},

		// screenExport (export_test.go:14 TestExportKeyLabels).
		{"export", func() Model { return Model{screen: screenExport} }},

		// screenReport (report_test.go:17 TestReportKeyLabels asserts on a
		// bare Model{screen: screenReport}; reportKeys(d) never reads m, so
		// newTestModelOnReport's richer fixture — used verbatim, per this
		// task's own template — produces the identical keyMap).
		{"report", newTestModelOnReport},

		// screenFilters (filters_test.go:176 TestFiltersKeyLabels).
		{"filters", filtersFixture},

		// screenMembers (members_test.go:89 TestMembersKeyLabels).
		{"members", membersFixture},

		// screenListBrowser (listbrowser_test.go:14 TestListBrowserKeyLabels).
		{"listbrowser", func() Model { return browserFixture(screenLog) }},

		// screenEntries (entries_test.go): one case per mode/step the parity
		// tests cover.
		{"entries_list", func() Model { return entriesFixture(true) }},
		{"entries_list_empty", func() Model {
			m := newTestModel()
			m.screen = screenEntries
			m.entriesScreen = entriesModel{}
			return m
		}},
		{"entries_confirm_delete", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesConfirmDelete
			return m
		}},
		{"entries_history", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesHistory
			return m
		}},
		{"entries_tags", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesTags
			return m
		}},
		{"entries_tags_new_mode", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesTags
			m.entriesScreen.tagNewMode = true
			return m
		}},
		{"entries_edit_duration", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesEdit
			m.entriesScreen.editStep = 0
			return m
		}},
		{"entries_edit_date", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesEdit
			m.entriesScreen.editStep = 1
			return m
		}},
		{"entries_edit_time", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesEdit
			m.entriesScreen.editStep = 2
			return m
		}},
		{"entries_edit_note", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesEdit
			m.entriesScreen.editStep = 3
			return m
		}},
		{"entries_edit_billable", func() Model {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesEdit
			m.entriesScreen.editStep = 4
			return m
		}},

		// screenLog (log_test.go:594 TestLogKeyLabelsPerStep), one case per
		// step/formField row, built exactly like that table.
		{"log_mode_select", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logModeSelect, formField: 0}}
		}},
		{"log_timer_pick", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logTimerPick, formField: 0}}
		}},
		{"log_list_pick", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logListPick, formField: 0}}
		}},
		{"log_task_pick", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logTaskPick, formField: 0}}
		}},
		{"log_id_input", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logIDInput, formField: 0}}
		}},
		{"log_form_duration", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logForm, formField: 0}}
		}},
		{"log_form_date", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logForm, formField: 1}}
		}},
		{"log_form_note", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logForm, formField: 2}}
		}},
		{"log_form_billable", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logForm, formField: 3}}
		}},
		{"log_timer_running", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logTimerRunning, formField: 0}}
		}},
		{"log_done", func() Model {
			return Model{screen: screenLog, logScreen: logModel{step: logDone, formField: 0}}
		}},

		// screenSetup (setup_test.go:14 TestSetupKeyLabelsPerStep), one case
		// per wizard step.
		{"setup_token", func() Model {
			return Model{screen: screenSetup, setup: setupModel{step: stepToken}}
		}},
		{"setup_workspace", func() Model {
			return Model{screen: screenSetup, setup: setupModel{step: stepWorkspace}}
		}},
		{"setup_rate", func() Model {
			return Model{screen: screenSetup, setup: setupModel{step: stepRate}}
		}},
		{"setup_currency", func() Model {
			return Model{screen: screenSetup, setup: setupModel{step: stepCurrency}}
		}},

		// screenRates (rates_test.go), one case per section/mode the parity
		// tests cover.
		{"rates_normal", func() Model {
			return Model{screen: screenRates, ratesScreen: ratesModel{sec: secRules}}
		}},
		{"rates_draft_pick_list", func() Model {
			return Model{screen: screenRates, ratesScreen: ratesModel{draft: overrideDraft{active: true, step: draftPickList}}}
		}},
		{"rates_draft_pick_member", func() Model {
			return Model{screen: screenRates, ratesScreen: ratesModel{draft: overrideDraft{active: true, step: draftPickMember}}}
		}},
		{"rates_draft_rate_step", func() Model {
			return Model{screen: screenRates, ratesScreen: ratesModel{
				editing: true,
				edit:    editOverrideRate,
				draft:   overrideDraft{active: true, step: draftRate},
			}}
		}},
		{"rates_editing", func() Model {
			return Model{screen: screenRates, ratesScreen: ratesModel{editing: true, edit: editListRate}}
		}},

		// screenRange (range_test.go:15,27), list mode and the custom-date
		// editor.
		{"range_list_mode", func() Model {
			return Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
		}},
		{"range_editing_mode", func() Model {
			m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
			m.rangeScreen.editing = true
			return m
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := keysFor(tc.model())
			golden(t, "footer_"+tc.name+"_short", footerView(testTheme(true), 0, false, k))
			golden(t, "footer_"+tc.name+"_full", footerView(testTheme(true), 0, true, k))
		})
	}
}

// TestShortFootersFitEightyColumns is the rule the curation of short exists to
// serve. help.ShortHelpView walks the slice left to right and stops at the
// first item that would overflow, so display order IS survival order: with the
// globals last by convention, a short footer wider than the terminal drops the
// exit and the ? that reaches full help — the two things a stuck user needs.
// Eighty columns is the floor worth holding: it is the default width of a bare
// terminal and of a split tmux pane.
//
// full is deliberately unbounded. It is what ? exists to show.
func TestShortFootersFitEightyColumns(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("testdata/footer_*_short.golden")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no short-footer goldens found — the glob or the naming changed")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if w := lipgloss.Width(string(b)); w > 80 {
			t.Errorf("%s is %d columns wide; at 80 it would drop its tail:\n%s",
				filepath.Base(f), w, string(b))
		}
	}
}
