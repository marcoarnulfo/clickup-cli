package tui

import (
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestDemoModeSkipsSetup(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	m := New(config.Config{}) // config vuota: senza demo andrebbe al setup
	if !m.demo {
		t.Fatal("atteso m.demo = true con CLICKUP_DEMO impostata")
	}
	if m.screen != screenHome {
		t.Errorf("screen = %v, atteso screenHome (setup saltato)", m.screen)
	}
	if m.cfg.Rate == 0 || m.cfg.Currency == "" {
		t.Errorf("config demo non applicata: %+v", m.cfg)
	}
}

func TestReloadEntriesCmdUsesDemo(t *testing.T) {
	m := Model{demo: true, year: 2026, month: time.July}
	cmd := m.reloadEntriesCmd()
	if cmd == nil {
		t.Fatal("reloadEntriesCmd ha ritornato nil")
	}
	msg := cmd()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("atteso entriesMsg, ricevuto %T", msg)
	}
	if len(em.entries) == 0 {
		t.Error("attese voci demo, ricevute 0")
	}
}

func TestDemoEntriesBuildReport(t *testing.T) {
	entries := demoEntries(2026, time.July)
	rates := report.Rates{Default: 50, ByList: map[string]float64{"web": 65, "mobile": 45}}
	r := report.Build(entries, report.GroupByList, rates, "EUR", 2026, time.July)
	if r.TotalHours <= 0 || r.TotalAmount <= 0 {
		t.Errorf("report demo vuoto: ore=%v importo=%v", r.TotalHours, r.TotalAmount)
	}
	if len(r.Buckets) != 2 { // Website + Mobile app
		t.Errorf("bucket per lista = %d, attesi 2", len(r.Buckets))
	}
}
