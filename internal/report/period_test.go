package report

import (
	"testing"
	"time"
)

func TestRangeForPreset(t *testing.T) {
	now := time.Date(2026, time.July, 15, 13, 0, 0, 0, time.UTC) // a Wednesday
	cases := []struct {
		preset    string
		wantStart string
		wantEnd   string
	}{
		{PresetThisMonth, "2026-07-01", "2026-08-01"},
		{PresetLastMonth, "2026-06-01", "2026-07-01"},
		{PresetLast7d, "2026-07-09", "2026-07-16"},
		{PresetLast30d, "2026-06-16", "2026-07-16"},
		{PresetThisWeek, "2026-07-13", "2026-07-20"}, // Monday..next Monday
		{"unknown", "2026-07-01", "2026-08-01"},      // falls back to this_month
	}
	for _, c := range cases {
		start, end := RangeForPreset(c.preset, 2026, time.July, now)
		if start.Format("2006-01-02") != c.wantStart || end.Format("2006-01-02") != c.wantEnd {
			t.Errorf("%s: got [%s,%s), want [%s,%s)", c.preset,
				start.Format("2006-01-02"), end.Format("2006-01-02"), c.wantStart, c.wantEnd)
		}
	}
}

func TestCustomRange(t *testing.T) {
	cases := []struct {
		name      string
		from      string
		to        string
		wantStart string
		wantEnd   string
	}{
		{
			name:      "month span",
			from:      "2026-06-01",
			to:        "2026-06-30",
			wantStart: "2026-06-01",
			wantEnd:   "2026-07-01",
		},
		{
			name:      "single day",
			from:      "2026-06-15",
			to:        "2026-06-15",
			wantStart: "2026-06-15",
			wantEnd:   "2026-06-16",
		},
		{
			name:      "multi-month span",
			from:      "2026-01-15",
			to:        "2026-03-10",
			wantStart: "2026-01-15",
			wantEnd:   "2026-03-11",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, _ := time.Parse("2006-01-02", c.from)
			to, _ := time.Parse("2006-01-02", c.to)
			start, end := CustomRange(from, to)
			if start.Format("2006-01-02") != c.wantStart || end.Format("2006-01-02") != c.wantEnd {
				t.Errorf("got [%s,%s), want [%s,%s)",
					start.Format("2006-01-02"), end.Format("2006-01-02"),
					c.wantStart, c.wantEnd)
			}
		})
	}
}

func TestPeriodLabelAndSlug(t *testing.T) {
	jul1 := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	aug1 := jul1.AddDate(0, 1, 0)
	if got := PeriodLabel(jul1, aug1); got != "July 2026" {
		t.Errorf("month label = %q", got)
	}
	if got := PeriodFileSlug(jul1, aug1); got != "2026-07" {
		t.Errorf("month slug = %q", got)
	}
	// custom [Jul 1, Jul 16) -> inclusive "to" is Jul 15
	mid := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	if got := PeriodLabel(jul1, mid); got != "2026-07-01 → 2026-07-15" {
		t.Errorf("custom label = %q", got)
	}
	if got := PeriodFileSlug(jul1, mid); got != "2026-07-01_2026-07-15" {
		t.Errorf("custom slug = %q", got)
	}
}
