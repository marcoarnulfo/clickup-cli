package tui

import (
	"os"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestDemoModeSkipsSetup(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	m := New(config.Config{}) // empty config: without demo it would go to setup
	if !m.demo {
		t.Fatal("expected m.demo = true with CLICKUP_DEMO set")
	}
	if m.screen != screenHome {
		t.Errorf("screen = %v, expected screenHome (setup skipped)", m.screen)
	}
	if m.cfg.Rate == 0 || m.cfg.Currency == "" {
		t.Errorf("demo config not applied: %+v", m.cfg)
	}
}

func TestReloadEntriesCmdUsesDemo(t *testing.T) {
	m := Model{demo: true, year: 2026, month: time.July, now: time.Now}
	cmd := m.reloadEntriesCmd(screenHome)
	if cmd == nil {
		t.Fatal("reloadEntriesCmd returned nil")
	}
	msg := cmd()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	if len(em.entries) == 0 {
		t.Error("expected demo entries, got 0")
	}
}

func TestDemoMembers(t *testing.T) {
	if len(demoMembers()) != 3 {
		t.Fatalf("demoMembers = %d, want 3", len(demoMembers()))
	}
	if _, ok := demoMembersCmd()().(membersMsg); !ok {
		t.Fatalf("demoMembersCmd should produce membersMsg")
	}
}

func TestDemoEntriesMultipleUsers(t *testing.T) {
	users := map[string]bool{}
	for _, e := range demoEntries(2026, time.July) {
		users[e.UserName] = true
	}
	if len(users) < 2 {
		t.Errorf("expected multiple demo users, got %v", users)
	}
}

func TestReloadDemoFiltersMembers(t *testing.T) {
	// Team scope, only alice (id 1) selected: the demo report must exclude bob/carol.
	m := Model{demo: true, year: 2026, month: time.July, scope: "team", selectedMembers: map[int]bool{1: true}, now: time.Now}
	em, ok := m.reloadEntriesCmd(screenHome)().(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg")
	}
	if len(em.entries) == 0 {
		t.Fatal("expected alice's demo entries, got 0")
	}
	for _, e := range em.entries {
		if e.UserID != 1 {
			t.Errorf("demo filter leaked user %d (%s)", e.UserID, e.UserName)
		}
	}
}

func TestReloadDemoMeScopeIsSingleSelfUser(t *testing.T) {
	// Real "me" scope is server-side filtered to the authenticated caller
	// (one user); demo must mirror that instead of summing all demo users.
	m := Model{demo: true, year: 2026, month: time.July, scope: "me", now: time.Now}
	em, ok := m.reloadEntriesCmd(screenHome)().(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg")
	}
	if len(em.entries) == 0 {
		t.Fatal("expected self demo entries, got 0")
	}
	for _, e := range em.entries {
		if e.UserID != demoSelfID {
			t.Errorf("demo me scope leaked user %d (%s), want only demoSelfID", e.UserID, e.UserName)
		}
	}
}

func TestDemoEntriesBuildReport(t *testing.T) {
	entries := demoEntries(2026, time.July)
	pricing := report.Pricing{
		Rates:           report.Rates{Default: 50, ByList: map[string]float64{"web": 65, "mobile": 45}},
		DefaultCurrency: "EUR",
	}
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	r := report.Build(entries, report.GroupByList, pricing, start, start.AddDate(0, 1, 0), nil)
	// Demo parity: the fake entries are billable, so demo mode shows real money.
	if r.TotalHours <= 0 || r.TotalAmount <= 0 || r.BilledHours <= 0 {
		t.Errorf("empty demo report: hours=%v billed=%v amount=%v", r.TotalHours, r.BilledHours, r.TotalAmount)
	}
	if len(r.Buckets) != 2 { // Website + Mobile app
		t.Errorf("buckets per list = %d, expected 2", len(r.Buckets))
	}
}

// TestDemoConfigHasBillingFields pins the v1.7 demo-parity additions to
// demoConfig() (#51,#6,#53,#64): a pinned timezone (M6 -- deterministic
// output across machines, no time.Local fallback) and a fully populated
// Billing block, so every v1.7 billing feature has something real to show.
func TestDemoConfigHasBillingFields(t *testing.T) {
	cfg := demoConfig()

	if cfg.Timezone != "UTC" {
		t.Errorf("demo Timezone = %q, want %q (pinned for deterministic output, M6)", cfg.Timezone, "UTC")
	}
	if cfg.Billing.DefaultCurrency == "" {
		t.Error("demo Billing.DefaultCurrency is empty")
	}
	if len(cfg.Billing.RatesByMember) == 0 {
		t.Error("demo Billing.RatesByMember is empty")
	}
	if len(cfg.Billing.Budgets) == 0 {
		t.Error("demo Billing.Budgets is empty, want at least one demo budget")
	}

	distinct := map[string]bool{}
	for _, c := range cfg.Billing.Currencies {
		distinct[c] = true
	}
	if len(distinct) < 2 {
		t.Errorf("demo Billing.Currencies = %v, want at least 2 lists mapped to different currencies", cfg.Billing.Currencies)
	}
}

// TestDemoEntriesHaveBillableMix pins the mix the billable/non-billable split
// needs: earlier demo entries were all Billable:true (so the report wasn't
// stuck at zero); this task adds the mix so the split is visibly non-trivial.
func TestDemoEntriesHaveBillableMix(t *testing.T) {
	entries := demoEntries(2026, time.July)
	var billable, nonBillable int
	for _, e := range entries {
		if e.Billable {
			billable++
		} else {
			nonBillable++
		}
	}
	if billable == 0 || nonBillable == 0 {
		t.Errorf("expected a mix of billable/non-billable demo entries, got billable=%d non-billable=%d", billable, nonBillable)
	}
}

// TestDemoReportShowsBillingDepth is the v1.7 demo-parity smoke test
// (#51,#6,#53,#64). It drives the same Model methods the real TUI uses
// (reloadEntriesCmd/applyReport/openBudgetView) to check that a demo session
// actually shows: a visible billable/non-billable split, at least two
// currency subtotals (with TotalAmount staying 0 -- no cross-currency total),
// at least one tag bucket, and at least one budget line with a sensible burn
// percentage.
func TestDemoReportShowsBillingDepth(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	m := New(config.Config{})
	m.year, m.month = 2026, time.July

	msg := m.reloadEntriesCmd(screenHome)()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	m.entries = em.entries

	if !m.applyReport() {
		t.Fatalf("applyReport failed: %v", m.err)
	}
	r := m.report

	if r.BillableHours <= 0 || r.NonBillableHours <= 0 {
		t.Errorf("expected a visible billable/non-billable split, got billable=%v non-billable=%v", r.BillableHours, r.NonBillableHours)
	}
	if len(r.CurrencySubtotals) < 2 {
		t.Errorf("expected at least 2 currency subtotals, got %v", r.CurrencySubtotals)
	}
	if r.TotalAmount != 0 {
		t.Errorf("cross-currency TotalAmount must stay 0 (no FX), got %v", r.TotalAmount)
	}

	// Tag grouping (#6): at least one bucket once grouped by tag.
	m.report.GroupBy = report.GroupByTag
	if !m.applyReport() {
		t.Fatalf("applyReport (tag) failed: %v", m.err)
	}
	if len(m.report.Buckets) == 0 {
		t.Error("expected at least one tag bucket in the demo report")
	}

	// Budget burn-down (#64): at least one budget line, with a burn
	// percentage that is neither 0% nor wildly over 100%.
	if !m.openBudgetView() {
		t.Fatalf("openBudgetView failed: %v", m.err)
	}
	if len(m.budgetScreen.lines) == 0 {
		t.Fatal("expected at least one budget line in the demo data")
	}
	for _, l := range m.budgetScreen.lines {
		if l.PercentUsed <= 0 || l.PercentUsed > 150 {
			t.Errorf("budget line %s percent used = %.2f, want a sensible burn in (0,150]", l.ListName, l.PercentUsed)
		}
	}
}

// TestDemoModeBillingEditorNeverWritesConfig extends the zero-I/O guarantee
// already covering rates.go's config.Save (see TestRatesSaveInDemoModeWritesNoConfig)
// to the now fully populated demo Billing config: even with real values in
// every field, saving from the billing editor in demo mode must never touch
// disk. Removing the "if !m.demo" guard around config.Save in saveRates makes
// this test fail.
func TestDemoModeBillingEditorNeverWritesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CLICKUP_DEMO", "1")

	m := New(config.Config{})
	m.entries = demoEntries(2026, time.July)
	m.ratesScreen = newRates(m.entries, m.cfg)
	m.screen = screenRates

	m = press(t, m, "s")

	p, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("demo mode must not write %s (stat err = %v)", p, err)
	}
}
