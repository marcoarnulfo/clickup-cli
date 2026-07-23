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
		start, end := RangeForPreset(c.preset, 2026, time.July, now, nil)
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
			start, end := CustomRange(from, to, nil)
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

func TestMonthRangeInLocation(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	start, end := MonthRange(2026, time.March, rome)
	// March 1 2026 00:00 Rome (CET, +01) == Feb 28 23:00 UTC
	if !start.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, rome)) {
		t.Fatalf("start = %v", start)
	}
	if !end.Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, rome)) {
		t.Fatalf("end = %v", end)
	}
	// DST: March 29 2026 is the spring-forward day in Rome; the month still spans exactly one calendar month.
	if got := end.Sub(start); got < 30*24*time.Hour {
		t.Fatalf("March too short: %v", got)
	}
}

func TestMonthRangeNilIsUTC(t *testing.T) {
	s, e := MonthRange(2026, time.June, nil)
	if s.Location() != time.UTC || !s.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("nil loc not UTC: %v", s)
	}
	if !e.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("end = %v", e)
	}
}

func TestWeekRangeISO(t *testing.T) {
	// ISO week 1 of 2026 starts Mon 2025-12-29.
	s, e := WeekRange(2026, 1, time.UTC)
	if !s.Equal(time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("W1 start = %v", s)
	}
	if e.Sub(s) != 7*24*time.Hour {
		t.Fatalf("week not 7 days: %v", e.Sub(s))
	}
	// 2026 has 53 ISO weeks; week 53 is valid.
	s53, _ := WeekRange(2026, 53, time.UTC)
	if s53.IsZero() {
		t.Fatalf("W53 2026 should exist")
	}
}

func TestCustomRangeInLocation(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, rome)
	to := time.Date(2026, 5, 10, 0, 0, 0, 0, rome)
	s, e := CustomRange(from, to, rome)
	if !s.Equal(from) || !e.Equal(to.AddDate(0, 0, 1)) {
		t.Fatalf("custom = %v..%v", s, e)
	}
}

// TestPeriodLabelInLocation is the A10 amendment test: labels must not force
// UTC, so a calendar month in a non-UTC location still renders as a month
// label instead of falling back to the date-range format.
func TestPeriodLabelInLocation(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	start, end := MonthRange(2026, time.March, rome)
	if got := PeriodLabel(start, end); got != "March 2026" {
		t.Fatalf("label = %q, want %q", got, "March 2026")
	}
	if got := PeriodFileSlug(start, end); got != "2026-03" {
		t.Fatalf("slug = %q, want %q", got, "2026-03")
	}
}
