package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

func mustResolveInlineKeys(t *testing.T, overrides map[string]config.KeySpec) KeyTable {
	t.Helper()
	kt, err := ResolveKeys(overrides)
	if err != nil {
		t.Fatal(err)
	}
	return kt
}

func TestLogChoiceInstructionsUseLiveKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		step logStep
		want string
	}{
		{
			name: "mode select",
			step: logModeSelect,
			want: "  1) Guided — pick list and task\n  2) Task ID/URL — straight to the form\n  3) Timer — start/stop stopwatch\n",
		},
		{
			name: "timer pick",
			step: logTimerPick,
			want: "  1) Guided (list → task)\n  2) Task ID/URL\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := Model{screen: screenLog, theme: testTheme(true), now: time.Now, logScreen: logModel{step: tc.step}}
			if got := m.screenBody(); !strings.Contains(got, tc.want) {
				t.Errorf("default instructions changed:\n%s", got)
			}

			m.keys = mustResolveInlineKeys(t, map[string]config.KeySpec{
				"pick_guided": {"f1"},
				"pick_by_id":  {"f2"},
				"pick_timer":  {"f3"},
			})
			got := m.screenBody()
			for _, want := range []string{"  f1)", "  f2)"} {
				if !strings.Contains(got, want) {
					t.Errorf("remapped instructions do not contain %q:\n%s", want, got)
				}
			}
			if tc.step == logModeSelect && !strings.Contains(got, "  f3)") {
				t.Errorf("remapped instructions do not contain the timer key:\n%s", got)
			}
			for _, stale := range []string{"  1)", "  2)", "  3)"} {
				if strings.Contains(got, stale) {
					t.Errorf("remapped instructions still contain %q:\n%s", stale, got)
				}
			}
		})
	}
}

func TestBillableInstructionsUseLiveKeys(t *testing.T) {
	t.Parallel()
	models := []struct {
		name  string
		model Model
	}{
		{
			name:  "log form",
			model: Model{screen: screenLog, theme: testTheme(true), now: time.Now, logScreen: logModel{step: logForm, formField: 3}},
		},
		{
			name:  "entries edit",
			model: Model{screen: screenEntries, theme: testTheme(true), entriesScreen: entriesModel{mode: entriesEdit, editStep: 4}},
		},
	}
	for _, tc := range models {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.model.screenBody(); !strings.Contains(got, "Billable? [Y/n]   (Enter = yes)") {
				t.Errorf("default billable instruction changed:\n%s", got)
			}

			for _, remap := range []struct {
				name string
				over map[string]config.KeySpec
				want string
			}{
				{name: "yes", over: map[string]config.KeySpec{"yes": {"f4"}}, want: "Billable? [f4/n/N]"},
				{name: "no", over: map[string]config.KeySpec{"no": {"f5"}}, want: "Billable? [y/Y/enter/f5]"},
				{name: "both", over: map[string]config.KeySpec{"yes": {"f4"}, "no": {"f5"}}, want: "Billable? [f4/f5]"},
				{name: "space", over: map[string]config.KeySpec{"yes": {" "}, "toggle_item": {"f6"}}, want: "Billable? [space/n/N]"},
			} {
				t.Run(remap.name, func(t *testing.T) {
					m := tc.model
					m.keys = mustResolveInlineKeys(t, remap.over)
					got := m.screenBody()
					if !strings.Contains(got, remap.want) {
						t.Errorf("remapped billable instruction is not truthful:\n%s", got)
					}
					for _, stale := range []string{"[Y/n]", "Enter = yes"} {
						if strings.Contains(got, stale) {
							t.Errorf("remapped billable instruction still contains %q:\n%s", stale, got)
						}
					}
				})
			}
		})
	}
}

func TestRatesInlineInstructionsUseLiveKeys(t *testing.T) {
	t.Parallel()
	remapped := mustResolveInlineKeys(t, map[string]config.KeySpec{
		"back":         {"f6"},
		"browse_list":  {"f7"},
		"clear_value":  {"f8"},
		"list_budget":  {"f9"},
		"next_section": {"f10"},
		"prev_section": {"f11"},
	})

	editCases := []struct {
		name        string
		rates       ratesModel
		placeholder string
	}{
		{
			name:        "list rate",
			rates:       ratesModel{sec: secLists, rows: []rateRow{{listID: "1"}}},
			placeholder: "new rate (Esc to cancel)",
		},
		{
			name:        "member rate",
			rates:       ratesModel{sec: secMembers, members: []memberRow{{id: 1}}},
			placeholder: "member rate (Esc to cancel)",
		},
		{
			name:        "existing override rate",
			rates:       ratesModel{sec: secOverrides, overrides: []overrideRow{{listID: "1", member: 1}}},
			placeholder: "override rate (Esc to cancel)",
		},
		{
			name: "draft override rate",
			rates: ratesModel{
				sec:     secOverrides,
				rows:    []rateRow{{listID: "1"}},
				members: []memberRow{{id: 1}},
				draft:   overrideDraft{active: true, step: draftPickMember, listID: "1"},
			},
			placeholder: "override rate (Esc to cancel)",
		},
	}
	for _, tc := range editCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			open := func(kt KeyTable) string {
				m := Model{screen: screenRates, keys: kt, ratesScreen: tc.rates}
				next, _ := m.updateRates(keyMsg("enter"))
				return next.(Model).ratesScreen.input.Placeholder
			}
			if got := open(KeyTable{}); got != tc.placeholder {
				t.Errorf("default placeholder = %q, want %q", got, tc.placeholder)
			}
			if got := open(remapped); got != strings.Replace(tc.placeholder, "Esc", "f6", 1) {
				t.Errorf("remapped placeholder = %q, want the live back key", got)
			}
		})
	}

	t.Run("empty draft guidance leads to browse", func(t *testing.T) {
		m := Model{screen: screenRates, demo: true, keys: remapped, ratesScreen: ratesModel{sec: secOverrides}}
		next, _ := m.updateRates(keyMsg("enter"))
		m = next.(Model)
		for _, want := range []string{"'f10/f11'", "'f7'"} {
			if !strings.Contains(m.ratesScreen.msg, want) {
				t.Fatalf("empty-draft guidance does not mention %s: %q", want, m.ratesScreen.msg)
			}
		}

		// Follow the advertised previous-section binding twice:
		// Overrides -> Members -> Lists, where BrowseList is enabled.
		for _, want := range []ratesSection{secMembers, secLists} {
			next, _ = m.updateRates(keyMsg("f11"))
			m = next.(Model)
			if m.ratesScreen.sec != want {
				t.Fatalf("f11 moved to section %v, want %v", m.ratesScreen.sec, want)
			}
		}
		next, _ = m.updateRates(keyMsg("f7"))
		if got := next.(Model).screen; got != screenListBrowser {
			t.Errorf("advertised browse route ended on screen %v, want list browser", got)
		}
	})

	t.Run("invalid existing rate", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			rt   ratesModel
		}{
			{name: "list", rt: ratesModel{editing: true, edit: editListRate, rows: []rateRow{{listID: "1"}}, rates: map[string]float64{}}},
			{name: "member", rt: ratesModel{editing: true, edit: editMemberRate, members: []memberRow{{id: 1}}, memberRates: map[int]float64{}}},
			{name: "override", rt: ratesModel{editing: true, edit: editOverrideRate, overrides: []overrideRow{{listID: "1", member: 1}}}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := tc.rt.commit("-1", remapped).msg
				for _, want := range []string{"'f6'", "'f8'", "cancel"} {
					if !strings.Contains(got, want) {
						t.Errorf("invalid existing-rate hint does not mention %q: %q", want, got)
					}
				}
			})
		}
	})

	t.Run("invalid draft rate", func(t *testing.T) {
		rt := ratesModel{editing: true, edit: editOverrideRate, draft: overrideDraft{active: true}}
		got := rt.commit("-1", remapped).msg
		if !strings.Contains(got, "'f6'") || !strings.Contains(got, "cancel") {
			t.Errorf("invalid draft-rate hint does not offer live cancellation: %q", got)
		}
		if strings.Contains(got, "'f8'") || strings.Contains(got, "clear") {
			t.Errorf("invalid draft-rate hint advertises clearing a nonexistent value: %q", got)
		}
	})

	t.Run("invalid budget stays in the open field", func(t *testing.T) {
		rt := ratesModel{editing: true, edit: editListBudget, rows: []rateRow{{listID: "1"}}, budgets: map[string]float64{}}
		got := rt.commit("-1", remapped).msg
		if !strings.Contains(got, "submit an empty value to remove the budget") {
			t.Errorf("invalid-budget hint is not actionable in the open field: %q", got)
		}
		if strings.Contains(got, "f9") || strings.Contains(got, "press") {
			t.Errorf("invalid-budget hint advertises a disabled binding: %q", got)
		}
	})

	t.Run("empty report browse hint", func(t *testing.T) {
		got := (ratesModel{sec: secLists}).view(testTheme(true), remapped)
		if !strings.Contains(got, "press 'f7' to browse") || strings.Contains(got, "press 'b' to browse") {
			t.Errorf("empty-report browse hint is not truthful:\n%s", got)
		}
	})

	t.Run("always-visible sentence remains unchanged", func(t *testing.T) {
		rt := ratesModel{sec: secLists}
		const want = "A rate of 0 bills at zero — to unset a value instead, submit an empty field."
		for _, kt := range []KeyTable{{}, remapped} {
			if got := rt.view(testTheme(true), kt); !strings.Contains(got, want) {
				t.Errorf("always-visible rates sentence changed:\n%s", got)
			}
		}
	})
}
