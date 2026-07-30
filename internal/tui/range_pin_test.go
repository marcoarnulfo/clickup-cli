package tui

import (
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// A relative preset re-resolved time.Now() on every rebuild, so regrouping after
// midnight relabeled the report with a range the loaded entries never covered.
// The range is pinned by the load and carried in the message, so a rebuild
// cannot drift away from its own data (#28).
//
// m.loc is pinned to UTC on purpose: with time.Local the two instants below fall
// on the same local day east of Greenwich, the fixture stops exercising the
// drift, and the test would pass in CI while failing on a developer's machine.
// cfg.Timezone is pinned alongside it: entriesMsg's handler calls locOrErr
// before every rebuild (#83), which re-resolves m.loc from cfg.Timezone
// (falling back to time.Local, not to whatever m.loc already holds) — without
// this, m.loc set above would be clobbered back to the host's zone on the
// very first Update, one assertion before the drift this test is about (found
// by running this test on a UTC+2 machine, where it failed with CEST offsets
// instead of the intended one-day drift).
func TestRebuildKeepsTheRangeTheEntriesWereLoadedFor(t *testing.T) {
	before := time.Date(2026, time.July, 24, 23, 59, 59, 0, time.UTC)
	after := time.Date(2026, time.July, 25, 0, 0, 1, 0, time.UTC)
	now := before
	m := newTestModel()
	m.cfg.Timezone = "UTC"
	m.loc = time.UTC
	m.preset = report.PresetLast7d
	m.now = func() time.Time { return now }

	start, end := m.currentRange()
	mm, _ := m.Update(entriesMsg{entries: goldenEntries(), start: start, end: end})
	m = mm.(Model)
	if !m.report.Start.Equal(start) {
		t.Fatalf("report.Start = %v, want the loaded range start %v", m.report.Start, start)
	}

	now = after // midnight passes; last_7d would now resolve one day later
	if newStart, _ := m.currentRange(); newStart.Equal(start) {
		t.Fatal("the fixture does not exercise the drift: currentRange did not move across midnight")
	}
	if !m.applyReport() {
		t.Fatal("applyReport returned false")
	}
	if !m.report.Start.Equal(start) {
		t.Errorf("after midnight report.Start = %v, want the pinned %v", m.report.Start, start)
	}
}

// Home's label describes the range the NEXT load will fetch, so it must stay
// fresh: month, week mode and preset all change on Home WITHOUT a reload, and a
// pinned label would freeze while the user navigates.
func TestHomeLabelFollowsTheSelectionNotThePin(t *testing.T) {
	m := newTestModel()
	m.cfg.Timezone = "UTC" // see TestRebuildKeepsTheRangeTheEntriesWereLoadedFor: locOrErr re-resolves m.loc from this on every Update
	m.loc = time.UTC
	m.now = func() time.Time { return time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC) }
	start, end := m.currentRange()
	mm, _ := m.Update(entriesMsg{entries: goldenEntries(), start: start, end: end})
	m = mm.(Model)
	pinned := m.rangeLabel()

	m.month-- // Home's PrevMonth, which triggers no reload
	if got := m.rangeLabel(); got == pinned {
		t.Errorf("rangeLabel stayed %q after changing month: the label must describe the next load", got)
	}
}

// Nothing is pinned before the first load, and the fallback is reachable in the
// live flow: the rates screen opens from Home and rebuilds the report there.
func TestActiveRangeFallsBackBeforeAnyLoad(t *testing.T) {
	m := newTestModel()
	m.loc = time.UTC
	wantStart, wantEnd := m.currentRange()
	gotStart, gotEnd := m.activeRange()
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Errorf("activeRange() = (%v, %v), want currentRange() (%v, %v)", gotStart, gotEnd, wantStart, wantEnd)
	}
}

// A browser reload is a load: it must refresh the pin, not leave a stale one.
func TestBrowserReloadRepinsTheRange(t *testing.T) {
	m := newTestModel()
	m.cfg.Timezone = "UTC" // see TestRebuildKeepsTheRangeTheEntriesWereLoadedFor: locOrErr re-resolves m.loc from this on every Update
	m.loc = time.UTC
	old := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	m.loadedStart, m.loadedEnd = old, old.AddDate(0, 1, 0)
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	mm, _ := m.Update(entriesReloadedMsg{entries: goldenEntries(), status: "ok", start: start, end: end})
	m = mm.(Model)
	if got, _ := m.activeRange(); !got.Equal(start) {
		t.Errorf("activeRange() = %v after a browser reload, want the reloaded %v", got, start)
	}
}
