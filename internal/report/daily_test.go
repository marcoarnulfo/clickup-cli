package report

import (
	"testing"
	"time"
)

func TestDailyHours(t *testing.T) {
	t.Parallel()
	utc := time.UTC
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, utc)
	}
	entry := func(start time.Time, h float64) TimeEntry {
		return TimeEntry{Start: start, Duration: time.Duration(h * float64(time.Hour))}
	}

	for _, tc := range []struct {
		name       string
		entries    []TimeEntry
		start, end time.Time
		want       []float64
	}{
		{
			name:  "empty range yields nil",
			start: day(2026, time.July, 5), end: day(2026, time.July, 5),
			want: nil,
		},
		{
			name:  "end before start yields nil",
			start: day(2026, time.July, 5), end: day(2026, time.July, 1),
			want: nil,
		},
		{
			name:  "no entries still yields one zero per day",
			start: day(2026, time.July, 1), end: day(2026, time.July, 4),
			want: []float64{0, 0, 0},
		},
		{
			name: "idle days stay zero rather than disappearing",
			entries: []TimeEntry{
				entry(day(2026, time.July, 1).Add(9*time.Hour), 2),
				entry(day(2026, time.July, 3).Add(9*time.Hour), 1.5),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 4),
			want: []float64{2, 0, 1.5},
		},
		{
			name: "several entries on one day are summed",
			entries: []TimeEntry{
				entry(day(2026, time.July, 2).Add(9*time.Hour), 2),
				entry(day(2026, time.July, 2).Add(14*time.Hour), 3),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 3),
			want: []float64{0, 5},
		},
		{
			name: "entries outside the range are ignored",
			entries: []TimeEntry{
				entry(day(2026, time.June, 30).Add(9*time.Hour), 8),
				entry(day(2026, time.July, 2).Add(9*time.Hour), 1),
				entry(day(2026, time.July, 9).Add(9*time.Hour), 8),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 3),
			want: []float64{0, 1},
		},
		{
			name: "an entry is credited to the day it starts on",
			entries: []TimeEntry{
				// Starts at 23:00 on the 1st and runs three hours into the 2nd.
				entry(day(2026, time.July, 1).Add(23*time.Hour), 3),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 3),
			want: []float64{3, 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := DailyHours(tc.entries, tc.start, tc.end, utc)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d days %v, want %d days %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("day %d = %v, want %v (full series %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// A day is not 24 hours across a DST transition. Rome springs forward on
// 2026-03-29, making that day 23 hours long: a series built by adding 24h
// would drift into the next day and mis-credit every entry after it.
func TestDailyHoursSurvivesDST(t *testing.T) {
	t.Parallel()
	rome, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	start := time.Date(2026, time.March, 28, 0, 0, 0, 0, rome)
	end := time.Date(2026, time.March, 31, 0, 0, 0, 0, rome)
	entries := []TimeEntry{
		{Start: time.Date(2026, time.March, 28, 10, 0, 0, 0, rome), Duration: time.Hour},
		{Start: time.Date(2026, time.March, 29, 10, 0, 0, 0, rome), Duration: 2 * time.Hour},
		{Start: time.Date(2026, time.March, 30, 10, 0, 0, 0, rome), Duration: 3 * time.Hour},
	}
	got := DailyHours(entries, start, end, rome)
	want := []float64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %d days %v, want 3", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("day %d = %v, want %v (full series %v)", i, got[i], want[i], got)
		}
	}
}
