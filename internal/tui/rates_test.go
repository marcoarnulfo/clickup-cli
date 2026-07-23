package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestRatesBOpensBrowser(t *testing.T) {
	m := Model{screen: screenRates, demo: true}
	m.ratesScreen = newRates(nil, config.Config{})
	u, _ := m.updateRates(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = u.(Model)
	if m.screen != screenListBrowser || m.browserScreen.origin != screenRates {
		t.Fatalf("'b' should open the browser for rates; screen=%v origin=%v", m.screen, m.browserScreen.origin)
	}
}

// billingEntries are the entries every editor test starts from: two lists and
// two members, so per-list, per-member and (list,member) rows all exist.
func billingEntries() []report.TimeEntry {
	start := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	return []report.TimeEntry{
		{ListID: "1", ListName: "Website", UserID: 7, UserName: "Alice", Start: start, Duration: time.Hour, Billable: true},
		{ListID: "2", ListName: "Internal", UserID: 8, UserName: "Bob", Start: start, Duration: time.Hour, Billable: true},
	}
}

// billingFixture points the config path at a temp dir (so 's' really saves)
// and opens the billing editor on the given config.
func billingFixture(t *testing.T, cfg config.Config) Model {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)            // os.UserConfigDir() derives from here (macOS)
	t.Setenv("XDG_CONFIG_HOME", dir) // ...or from here (Linux)
	t.Setenv("CLICKUP_TOKEN", "")
	if cfg.Token == "" {
		cfg.Token, cfg.WorkspaceID = "t", "1"
	}
	m := Model{
		screen: screenRates,
		cfg:    cfg,
		now:    time.Now,
		year:   2026,
		month:  time.July,
		preset: report.PresetThisMonth,
		loc:    time.UTC,
	}
	m.entries = billingEntries()
	m.ratesScreen = newRates(m.entries, cfg)
	return m
}

// press sends a named key ("tab", "enter", "esc", "down", …) or, for anything
// else, its runes.
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyMsg{Type: tea.KeyShiftTab}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		u, _ := m.updateRates(msg)
		m = u.(Model)
	}
	return m
}

// typeIn types free text into the currently open input field.
func typeIn(t *testing.T, m Model, s string) Model {
	t.Helper()
	u, _ := m.updateRates(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return u.(Model)
}

func TestRatesEditMemberRateSaves(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab")   // Lists -> Members
	m = press(t, m, "enter") // edit the first member's rate
	m = typeIn(t, m, "50")
	m = press(t, m, "enter")
	m = press(t, m, "s")

	if got := m.cfg.Billing.RatesByMember[7]; got != 50 {
		t.Fatalf("member rate not persisted: RatesByMember = %v", m.cfg.Billing.RatesByMember)
	}
	if m.screen != screenReport {
		t.Fatalf("a successful save should return to the report, got screen %v", m.screen)
	}
}

func TestRatesEditListCurrencyAndBudgetSave(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "c") // currency of the selected list
	m = typeIn(t, m, "usd")
	m = press(t, m, "enter")
	m = press(t, m, "g") // budget of the selected list
	m = typeIn(t, m, "2000")
	m = press(t, m, "enter")
	m = press(t, m, "s")

	if got := m.cfg.Billing.Currencies["1"]; got != "USD" {
		t.Fatalf("list currency not persisted (want USD): %v", m.cfg.Billing.Currencies)
	}
	if got := m.cfg.Billing.Budgets["1"]; got != 2000 {
		t.Fatalf("list budget not persisted (want 2000): %v", m.cfg.Billing.Budgets)
	}
}

func TestRatesAddListMemberOverrideSaves(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab", "tab") // -> Overrides
	m = press(t, m, "n")          // new (list,member) override
	m = press(t, m, "enter")      // list "Website" (id 1)
	m = press(t, m, "down")       // member Bob (id 8)
	m = press(t, m, "enter")
	m = typeIn(t, m, "60")
	m = press(t, m, "enter")
	m = press(t, m, "s")

	want := []config.Override{{List: "1", Member: 8, Rate: 60}}
	got := m.cfg.Billing.RateOverrides
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("(list,member) override not persisted: got %v, want %v", got, want)
	}
}

func TestRatesEditRoundingSaves(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab", "tab", "tab") // -> Rules
	m = press(t, m, "down")              // rounding increment
	m = press(t, m, "enter")
	m = typeIn(t, m, "15m")
	m = press(t, m, "enter")
	m = press(t, m, "down", "enter") // mode -> up
	m = press(t, m, "down", "enter") // scope -> day
	m = press(t, m, "s")

	want := config.Rounding{Increment: "15m", Mode: "up", Scope: "day"}
	if m.cfg.Billing.Rounding != want {
		t.Fatalf("rounding not persisted: got %+v, want %+v", m.cfg.Billing.Rounding, want)
	}
}

func TestRatesEditTimezoneAndDefaultCurrencySave(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab", "tab", "tab") // -> Rules
	m = press(t, m, "enter")             // default currency
	m = typeIn(t, m, "chf")
	m = press(t, m, "enter")
	m = press(t, m, "down", "down", "down", "down") // -> timezone
	m = press(t, m, "enter")
	m = typeIn(t, m, "Europe/Rome")
	m = press(t, m, "enter")
	m = press(t, m, "s")

	if m.cfg.Timezone != "Europe/Rome" {
		t.Fatalf("timezone not persisted: %q", m.cfg.Timezone)
	}
	if m.cfg.Billing.DefaultCurrency != "CHF" {
		t.Fatalf("default currency not persisted: %q", m.cfg.Billing.DefaultCurrency)
	}
}

// A full round trip: every dimension is edited through the UI, saved, and then
// the screen is re-entered through the real path ('p' from the report screen,
// which rebuilds the editor from the just-saved config).
func TestRatesReentryShowsPersistedValues(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})

	m = press(t, m, "enter") // Lists: rate
	m = typeIn(t, m, "45")
	m = press(t, m, "enter")
	m = press(t, m, "c")
	m = typeIn(t, m, "usd")
	m = press(t, m, "enter")
	m = press(t, m, "g")
	m = typeIn(t, m, "2000")
	m = press(t, m, "enter")

	m = press(t, m, "tab", "enter") // Members: Alice's rate
	m = typeIn(t, m, "50")
	m = press(t, m, "enter")

	m = press(t, m, "tab", "n", "enter", "down", "enter") // Overrides: (Website, Bob)
	m = typeIn(t, m, "60")
	m = press(t, m, "enter")

	m = press(t, m, "tab", "enter") // Rules: default currency
	m = typeIn(t, m, "chf")
	m = press(t, m, "enter")
	m = press(t, m, "down", "enter")
	m = typeIn(t, m, "15m")
	m = press(t, m, "enter")
	m = press(t, m, "down", "enter") // mode -> up
	m = press(t, m, "down", "enter") // scope -> day
	m = press(t, m, "down", "enter")
	m = typeIn(t, m, "Europe/Rome")
	m = press(t, m, "enter")

	m = press(t, m, "s")
	if m.screen != screenReport {
		t.Fatalf("save should return to the report, got %v (msg %q)", m.screen, m.ratesScreen.msg)
	}

	// Re-enter the editor the way a user does, from the report screen.
	u, _ := m.updateReport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("'p' should reopen the billing editor, got %v", m.screen)
	}

	lists := m.ratesScreen.view()
	for _, want := range []string{"45.00", "USD", "2000.00"} {
		if !strings.Contains(lists, want) {
			t.Errorf("Lists section should show %q, got:\n%s", want, lists)
		}
	}
	members := press(t, m, "tab").ratesScreen.view()
	if !strings.Contains(members, "50.00") {
		t.Errorf("Members section should show the persisted member rate, got:\n%s", members)
	}
	overrides := press(t, m, "tab", "tab").ratesScreen.view()
	if !strings.Contains(overrides, "60.00") {
		t.Errorf("Overrides section should show the persisted override, got:\n%s", overrides)
	}
	rules := press(t, m, "tab", "tab", "tab").ratesScreen.view()
	for _, want := range []string{"CHF", "15m", "up", "day", "Europe/Rome"} {
		if !strings.Contains(rules, want) {
			t.Errorf("Rules section should show %q, got:\n%s", want, rules)
		}
	}
}

func TestRatesRejectsUnknownTimezoneInline(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab", "tab", "tab")
	m = press(t, m, "down", "down", "down", "down", "enter") // timezone
	m = typeIn(t, m, "Mars/Olympus")
	m = press(t, m, "enter")

	rt := m.ratesScreen
	if rt.msg == "" {
		t.Fatal("an unknown timezone must be reported inline")
	}
	if !rt.editing {
		t.Fatal("a rejected timezone must keep the field open for correction")
	}
	if rt.tz != "" {
		t.Fatalf("a rejected timezone must not be accepted, got %q", rt.tz)
	}
}

func TestRatesRejectsUnparseableRoundingIncrementInline(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab", "tab", "tab")
	m = press(t, m, "down", "enter") // increment
	m = typeIn(t, m, "banana")
	m = press(t, m, "enter")

	rt := m.ratesScreen
	if rt.msg == "" {
		t.Fatal("an unparseable rounding increment must be reported inline, never silently treated as off")
	}
	if rt.rounding.Increment != "" {
		t.Fatalf("a rejected increment must not be accepted, got %q", rt.rounding.Increment)
	}
}

func TestRatesEmptyRoundingIncrementMeansOff(t *testing.T) {
	cfg := config.Config{Rate: 30, Currency: "EUR"}
	cfg.Billing.Rounding = config.Rounding{Increment: "15m"}
	m := billingFixture(t, cfg)
	m = press(t, m, "tab", "tab", "tab")
	m = press(t, m, "down", "enter") // increment, leave empty
	m = press(t, m, "enter")
	m = press(t, m, "s")

	if m.ratesScreen.msg != "" {
		t.Fatalf("an empty increment is valid (rounding off), got error %q", m.ratesScreen.msg)
	}
	if m.cfg.Billing.Rounding.Increment != "" {
		t.Fatalf("an empty increment must turn rounding off, got %q", m.cfg.Billing.Rounding.Increment)
	}
}

func TestRatesRejectsNegativeRateInline(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab")   // Members
	m = press(t, m, "enter") // edit rate
	rt := m.ratesScreen
	rt = rt.commit("-5") // '-' cannot be typed (numericRune filters it); exercise the validator directly
	m.ratesScreen = rt

	if rt.msg == "" {
		t.Fatal("a negative rate must be reported inline")
	}
	if _, ok := rt.memberRates[7]; ok {
		t.Fatal("a rejected rate must not be stored")
	}
	if !rt.editing {
		t.Fatal("a rejected rate must keep the field open for correction")
	}
}

// A rate of 0 means "this list/member bills at zero", a distinct, deliberate
// billing outcome from clearing the value with 'd' (which falls back to the
// inherited rate). The UI must accept it exactly like the hand-written YAML
// already does.
func TestRatesZeroListRatePersists(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "enter") // Lists: edit the selected list's rate
	m = typeIn(t, m, "0")
	m = press(t, m, "enter")

	rt := m.ratesScreen
	if rt.msg != "" {
		t.Fatalf("a zero rate must be accepted, got inline error %q", rt.msg)
	}
	if v, ok := rt.rates["1"]; !ok || v != 0 {
		t.Fatalf("a zero list rate must be stored, got %v (ok=%v)", v, ok)
	}

	m = press(t, m, "s")
	if got, ok := m.cfg.Rates["1"]; !ok || got != 0 {
		t.Fatalf("a zero list rate must survive save, got Rates = %v", m.cfg.Rates)
	}

	// Re-enter the editor: the persisted zero must still be there and shown.
	m.ratesScreen = newRates(m.entries, m.cfg)
	if v, ok := m.ratesScreen.rates["1"]; !ok || v != 0 {
		t.Fatalf("a zero list rate must survive re-entry, got %v (ok=%v)", v, ok)
	}
	if !strings.Contains(m.ratesScreen.view(), "0.00") {
		t.Fatalf("the Lists view should show the persisted zero rate, got:\n%s", m.ratesScreen.view())
	}
}

func TestRatesZeroMemberRatePersists(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab")   // Members
	m = press(t, m, "enter") // edit rate
	m = typeIn(t, m, "0")
	m = press(t, m, "enter")

	rt := m.ratesScreen
	if rt.msg != "" {
		t.Fatalf("a zero rate must be accepted, got inline error %q", rt.msg)
	}
	if v, ok := rt.memberRates[7]; !ok || v != 0 {
		t.Fatalf("a zero member rate must be stored, got %v (ok=%v)", v, ok)
	}

	m = press(t, m, "s")
	if got, ok := m.cfg.Billing.RatesByMember[7]; !ok || got != 0 {
		t.Fatalf("a zero member rate must survive save, got RatesByMember = %v", m.cfg.Billing.RatesByMember)
	}
}

func TestRatesZeroOverrideRatePersists(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "tab", "tab") // -> Overrides
	m = press(t, m, "n")          // new (list,member) override
	m = press(t, m, "enter")      // list "Website" (id 1)
	m = press(t, m, "down")       // member Bob (id 8)
	m = press(t, m, "enter")
	m = typeIn(t, m, "0")
	m = press(t, m, "enter")

	rt := m.ratesScreen
	if rt.msg != "" {
		t.Fatalf("a zero override rate must be accepted, got inline error %q", rt.msg)
	}

	m = press(t, m, "s")
	want := []config.Override{{List: "1", Member: 8, Rate: 0}}
	got := m.cfg.Billing.RateOverrides
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("a zero override rate must survive save, got %v, want %v", got, want)
	}
}

// A budget of 0 is not a meaningful value (it would render a 0% or divide-by-
// zero burn-down bar): unlike a rate, it must stay rejected.
func TestRatesRejectsZeroBudgetInline(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "g") // budget of the selected list
	m = typeIn(t, m, "0")
	m = press(t, m, "enter")

	rt := m.ratesScreen
	if rt.msg == "" {
		t.Fatal("a zero budget must be reported inline")
	}
	if _, ok := rt.budgets["1"]; ok {
		t.Fatal("a rejected budget must not be stored")
	}
	if !rt.editing {
		t.Fatal("a rejected budget must keep the field open for correction")
	}
}

// A rejected value must not cost the user the rest of their edits.
func TestRatesInvalidValueKeepsOtherEdits(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "c") // set a list currency first
	m = typeIn(t, m, "usd")
	m = press(t, m, "enter")
	m = press(t, m, "tab", "tab", "tab")
	m = press(t, m, "down", "down", "down", "down", "enter")
	m = typeIn(t, m, "Mars/Olympus")
	m = press(t, m, "enter") // rejected
	m = press(t, m, "esc")   // give up on the timezone
	m = press(t, m, "s")

	if got := m.cfg.Billing.Currencies["1"]; got != "USD" {
		t.Fatalf("earlier edits must survive a rejected value: %v", m.cfg.Billing.Currencies)
	}
}

// The editor must make the effective rate visible: with a (list,member)
// override in play the Overrides section shows what it wins over.
func TestRatesOverrideShowsWhatItOverrides(t *testing.T) {
	cfg := config.Config{Rate: 30, Currency: "EUR"}
	cfg.Rates = map[string]float64{"1": 45}
	cfg.Billing.RatesByMember = map[int]float64{8: 50}
	cfg.Billing.RateOverrides = []config.Override{{List: "1", Member: 8, Rate: 60}}
	m := billingFixture(t, cfg)
	v := press(t, m, "tab", "tab").ratesScreen.view()
	if !strings.Contains(v, "60.00") || !strings.Contains(v, "50.00") {
		t.Fatalf("the Overrides section must show the override and the rate it wins over, got:\n%s", v)
	}
	if !strings.Contains(v, "member rate") {
		t.Fatalf("the Overrides section must name the precedence level it wins over, got:\n%s", v)
	}
}

// Demo mode is zero-I/O: 's' must update the in-memory state (so the rebuilt
// report reflects the edits) without ever writing the user's real config —
// this screen owns cfg.Timezone and the whole cfg.Billing block, so a stray
// write would poison every later report with demo values.
func TestRatesSaveInDemoModeWritesNoConfig(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m.demo = true
	m = press(t, m, "tab", "enter") // Members: Alice's rate
	m = typeIn(t, m, "50")
	m = press(t, m, "enter")
	m = press(t, m, "s")

	if m.ratesScreen.msg != "" {
		t.Fatalf("demo save should not error, got %q", m.ratesScreen.msg)
	}
	if m.cfg.Billing.RatesByMember[7] != 50 {
		t.Fatalf("demo save must still update the in-memory config: %v", m.cfg.Billing.RatesByMember)
	}
	if m.screen != screenReport {
		t.Fatalf("demo save should return to the report, got %v", m.screen)
	}
	p, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("demo mode must not write %s (stat err = %v)", p, err)
	}
}

// Arbitrary text must not reach Pricing.Currencies: it would be printed as the
// currency of every invoice line for that list.
func TestRatesRejectsInvalidCurrencyInline(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "c")
	m = typeIn(t, m, "banana")
	m = press(t, m, "enter")
	if m.ratesScreen.msg == "" || !m.ratesScreen.editing {
		t.Fatalf("an invalid list currency must be rejected inline; msg=%q editing=%v", m.ratesScreen.msg, m.ratesScreen.editing)
	}
	if c, ok := m.ratesScreen.currencies["1"]; ok {
		t.Fatalf("a rejected currency must not be stored, got %q", c)
	}

	m = press(t, m, "esc")
	m = press(t, m, "tab", "tab", "tab", "enter") // Rules: default currency
	m = typeIn(t, m, "banana")
	m = press(t, m, "enter")
	if m.ratesScreen.msg == "" || m.ratesScreen.defCur != "" {
		t.Fatalf("an invalid default currency must be rejected inline; msg=%q defCur=%q", m.ratesScreen.msg, m.ratesScreen.defCur)
	}
}

// 'd' in the Lists section clears the rate only. The currency and the budget
// are cleared by reopening their own field and submitting an empty value —
// which is what their help text and error messages now say.
func TestRatesListClearingIsPerField(t *testing.T) {
	m := billingFixture(t, config.Config{Rate: 30, Currency: "EUR"})
	m = press(t, m, "enter")
	m = typeIn(t, m, "45")
	m = press(t, m, "enter")
	m = press(t, m, "c")
	m = typeIn(t, m, "usd")
	m = press(t, m, "enter")
	m = press(t, m, "g")
	m = typeIn(t, m, "2000")
	m = press(t, m, "enter")

	m = press(t, m, "d") // clears the rate only
	rt := m.ratesScreen
	if _, ok := rt.rates["1"]; ok {
		t.Fatal("'d' should clear the per-list rate")
	}
	if rt.currencies["1"] != "USD" {
		t.Fatalf("'d' must not touch the per-list currency, got %q", rt.currencies["1"])
	}
	if _, ok := rt.budgets["1"]; !ok {
		t.Fatal("'d' must not touch the per-list budget")
	}

	m = press(t, m, "g", "enter") // empty budget clears it
	m = press(t, m, "c", "enter") // empty currency clears it
	rt = m.ratesScreen
	if _, ok := rt.budgets["1"]; ok {
		t.Fatal("submitting an empty budget should remove it")
	}
	if _, ok := rt.currencies["1"]; ok {
		t.Fatal("submitting an empty currency should remove it")
	}
}

// overrideFixture opens the editor with one (list,member) override already
// configured, with the Overrides section selected on it.
func overrideFixture(t *testing.T) Model {
	t.Helper()
	cfg := config.Config{Rate: 30, Currency: "EUR"}
	cfg.Billing.RateOverrides = []config.Override{{List: "1", Member: 8, Rate: 60}}
	m := billingFixture(t, cfg)
	return press(t, m, "tab", "tab")
}

func TestRatesEditExistingOverrideRate(t *testing.T) {
	m := overrideFixture(t)
	m = press(t, m, "enter") // edits the selected override, not a new draft
	if m.ratesScreen.draft.active {
		t.Fatal("Enter on an existing override must edit it, not start a new draft")
	}
	m = typeIn(t, m, "70")
	m = press(t, m, "enter")
	m = press(t, m, "s")

	want := []config.Override{{List: "1", Member: 8, Rate: 70}}
	if got := m.cfg.Billing.RateOverrides; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("edited override not persisted: got %v, want %v", got, want)
	}
}

func TestRatesDeleteOverride(t *testing.T) {
	m := overrideFixture(t)
	m = press(t, m, "d")
	if len(m.ratesScreen.overrides) != 0 {
		t.Fatalf("'d' should delete the selected override, got %v", m.ratesScreen.overrides)
	}
	m = press(t, m, "s")
	if len(m.cfg.Billing.RateOverrides) != 0 {
		t.Fatalf("the deletion must be persisted, got %v", m.cfg.Billing.RateOverrides)
	}
	// The selection landed on the trailing "new override" row and still works.
	if v := m.ratesScreen.view(); !strings.Contains(v, "new (list,member) override") {
		t.Fatalf("the new-override row should still be offered, got:\n%s", v)
	}
}

// Re-creating a pair that already has an override updates it instead of
// appending a second, contradictory entry (report.Rates.ByListMember is a map:
// a duplicate would resolve non-deterministically).
func TestRatesNewOverrideOnExistingPairUpdatesIt(t *testing.T) {
	m := overrideFixture(t)
	m = press(t, m, "n", "enter", "down", "enter") // same pair: (Website, Bob)
	m = typeIn(t, m, "70")
	m = press(t, m, "enter")

	if got := m.ratesScreen.overrides; len(got) != 1 || got[0].rate != 70 {
		t.Fatalf("re-creating an existing pair must update it in place, got %v", got)
	}
	m = press(t, m, "s")
	want := []config.Override{{List: "1", Member: 8, Rate: 70}}
	if got := m.cfg.Billing.RateOverrides; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("upserted override not persisted: got %v, want %v", got, want)
	}
}
