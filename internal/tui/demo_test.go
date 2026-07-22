package tui

import (
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
	cmd := m.reloadEntriesCmd()
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
	em, ok := m.reloadEntriesCmd()().(entriesMsg)
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
	em, ok := m.reloadEntriesCmd()().(entriesMsg)
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
	rates := report.Rates{Default: 50, ByList: map[string]float64{"web": 65, "mobile": 45}}
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	r := report.Build(entries, report.GroupByList, rates, "EUR", start, start.AddDate(0, 1, 0))
	if r.TotalHours <= 0 || r.TotalAmount <= 0 {
		t.Errorf("empty demo report: hours=%v amount=%v", r.TotalHours, r.TotalAmount)
	}
	if len(r.Buckets) != 2 { // Website + Mobile app
		t.Errorf("buckets per list = %d, expected 2", len(r.Buckets))
	}
}
