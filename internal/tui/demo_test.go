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
	m := Model{demo: true, year: 2026, month: time.July}
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

func TestDemoEntriesBuildReport(t *testing.T) {
	entries := demoEntries(2026, time.July)
	rates := report.Rates{Default: 50, ByList: map[string]float64{"web": 65, "mobile": 45}}
	r := report.Build(entries, report.GroupByList, rates, "EUR", 2026, time.July)
	if r.TotalHours <= 0 || r.TotalAmount <= 0 {
		t.Errorf("empty demo report: hours=%v amount=%v", r.TotalHours, r.TotalAmount)
	}
	if len(r.Buckets) != 2 { // Website + Mobile app
		t.Errorf("buckets per list = %d, expected 2", len(r.Buckets))
	}
}
