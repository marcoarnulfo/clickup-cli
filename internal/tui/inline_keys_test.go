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
		"back":        {"f6"},
		"browse_list": {"f7"},
		"clear_value": {"f8"},
		"list_budget": {"f9"},
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

	t.Run("browse list error", func(t *testing.T) {
		m := Model{screen: screenRates, keys: remapped, ratesScreen: ratesModel{sec: secOverrides}}
		next, _ := m.updateRates(keyMsg("enter"))
		got := next.(Model).ratesScreen.msg
		if !strings.Contains(got, "('f7')") || strings.Contains(got, "('b')") {
			t.Errorf("browse-list error is not truthful: %q", got)
		}
	})

	t.Run("invalid rate", func(t *testing.T) {
		rt := ratesModel{editing: true, edit: editListRate, rows: []rateRow{{listID: "1"}}, rates: map[string]float64{}}
		got := rt.commit("-1", remapped).msg
		if !strings.Contains(got, "'f8'") || strings.Contains(got, "'d'") {
			t.Errorf("invalid-rate hint is not truthful: %q", got)
		}
	})

	t.Run("invalid budget", func(t *testing.T) {
		rt := ratesModel{editing: true, edit: editListBudget, rows: []rateRow{{listID: "1"}}, budgets: map[string]float64{}}
		got := rt.commit("-1", remapped).msg
		if !strings.Contains(got, "'f9'") || strings.Contains(got, "'g'") {
			t.Errorf("invalid-budget hint is not truthful: %q", got)
		}
	})

	t.Run("empty report browse hint", func(t *testing.T) {
		got := (ratesModel{sec: secLists}).view(testTheme(true), remapped)
		if !strings.Contains(got, "press 'f7' to browse") || strings.Contains(got, "press 'b' to browse") {
			t.Errorf("empty-report browse hint is not truthful:\n%s", got)
		}
	})

	t.Run("always-visible unset hint", func(t *testing.T) {
		rt := ratesModel{sec: secLists}
		for _, tc := range []struct {
			name string
			kt   KeyTable
			key  string
		}{
			{name: "defaults", kt: KeyTable{}, key: "d"},
			{name: "remapped", kt: remapped, key: "f8"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := rt.view(testTheme(true), tc.kt)
				if !strings.Contains(got, "empty list/member rate makes no change") || !strings.Contains(got, "press '"+tc.key+"' to unset") {
					t.Errorf("unset-rate hint is not truthful:\n%s", got)
				}
			})
		}
	})
}
