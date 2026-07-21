package report

import (
	"testing"
	"time"
)

func d(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }

func sampleEntries() []TimeEntry {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	return []TimeEntry{
		{TaskName: "Bug fix", ListName: "Cliente A", UserName: "me", Start: base, Duration: d(2)},
		{TaskName: "Bug fix", ListName: "Cliente A", UserName: "me", Start: base.AddDate(0, 0, 1), Duration: d(1)},
		{TaskName: "Feature X", ListName: "Cliente B", UserName: "me", Start: base, Duration: d(3)},
	}
}

func TestBuildTotal(t *testing.T) {
	r := Build(sampleEntries(), GroupByTotal, 50, "EUR", 2026, time.July)
	if r.TotalHours != 6 {
		t.Fatalf("total hours = %v, want 6", r.TotalHours)
	}
	if r.TotalAmount != 300 {
		t.Fatalf("total amount = %v, want 300", r.TotalAmount)
	}
	if len(r.Buckets) != 1 || r.Buckets[0].Label != "Totale" {
		t.Fatalf("total should have one bucket labelled Totale, got %+v", r.Buckets)
	}
}

func TestBuildByTaskSortedByHoursDesc(t *testing.T) {
	r := Build(sampleEntries(), GroupByTask, 0, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 task buckets, got %d", len(r.Buckets))
	}
	// "Bug fix" = 3h, "Feature X" = 3h -> pari merito, ordine per label asc
	if r.Buckets[0].Label != "Bug fix" || r.Buckets[0].Hours != 3 {
		t.Fatalf("bucket[0] = %+v", r.Buckets[0])
	}
}

func TestBuildByList(t *testing.T) {
	r := Build(sampleEntries(), GroupByList, 0, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 list buckets, got %d", len(r.Buckets))
	}
	m := map[string]float64{}
	for _, b := range r.Buckets {
		m[b.Label] = b.Hours
	}
	if m["Cliente A"] != 3 || m["Cliente B"] != 3 {
		t.Fatalf("list hours wrong: %+v", m)
	}
}

func TestBuildByDayChronological(t *testing.T) {
	r := Build(sampleEntries(), GroupByDay, 0, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 day buckets, got %d", len(r.Buckets))
	}
	if r.Buckets[0].Label != "2026-07-01" || r.Buckets[1].Label != "2026-07-02" {
		t.Fatalf("days not chronological: %+v", r.Buckets)
	}
	if r.Buckets[0].Hours != 5 || r.Buckets[1].Hours != 1 {
		t.Fatalf("day hours wrong: %+v", r.Buckets)
	}
}

func TestBuildEmpty(t *testing.T) {
	r := Build(nil, GroupByTask, 50, "EUR", 2026, time.July)
	if r.TotalHours != 0 || len(r.Buckets) != 0 {
		t.Fatalf("empty report should be zero, got %+v", r)
	}
}

func TestRoundingTwoDecimals(t *testing.T) {
	e := []TimeEntry{{TaskName: "x", Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Duration: d(1.0 / 3.0)}}
	r := Build(e, GroupByTask, 30, "EUR", 2026, time.July)
	if r.Buckets[0].Hours != 0.33 {
		t.Fatalf("hours should round to 0.33, got %v", r.Buckets[0].Hours)
	}
	if r.TotalAmount != 9.9 { // 0.33 * 30
		t.Fatalf("amount should be 9.9, got %v", r.TotalAmount)
	}
}

func TestMonthRange(t *testing.T) {
	start, end := MonthRange(2026, time.July)
	if !start.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v", start)
	}
	if !end.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("end = %v", end)
	}
}
