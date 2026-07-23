package report

import (
	"math"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/duration"
)

func d(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }

var (
	julStart = time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	julEnd   = julStart.AddDate(0, 1, 0)
	junStart = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	junEnd   = junStart.AddDate(0, 1, 0)
)

// eur is the simplest possible pricing: one flat rate, one currency, no rounding.
func eur(rate float64) Pricing {
	return Pricing{Rates: Rates{Default: rate}, DefaultCurrency: "EUR"}
}

// amountIn returns the bucket's amount in the given currency (0 when absent).
func amountIn(b Bucket, currency string) float64 {
	for _, a := range b.Amounts {
		if a.Currency == currency {
			return a.Amount
		}
	}
	return 0
}

func sampleEntries() []TimeEntry {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	return []TimeEntry{
		{ID: "1", TaskID: "t1", TaskName: "Bug fix", ListID: "l1", ListName: "Client A", UserID: 1, UserName: "me", Start: base, Duration: d(2), Billable: true},
		{ID: "2", TaskID: "t1", TaskName: "Bug fix", ListID: "l1", ListName: "Client A", UserID: 1, UserName: "me", Start: base.AddDate(0, 0, 1), Duration: d(1), Billable: true},
		{ID: "3", TaskID: "t2", TaskName: "Feature X", ListID: "l2", ListName: "Client B", UserID: 1, UserName: "me", Start: base, Duration: d(3), Billable: true},
	}
}

func TestBuildTotal(t *testing.T) {
	r := Build(sampleEntries(), GroupByTotal, eur(50), julStart, julEnd, nil)
	if r.TotalHours != 6 {
		t.Fatalf("total hours = %v, want 6", r.TotalHours)
	}
	if r.TotalAmount != 300 {
		t.Fatalf("total amount = %v, want 300", r.TotalAmount)
	}
	if len(r.Buckets) != 1 || r.Buckets[0].Label != "Total" {
		t.Fatalf("total should have one bucket labelled Total, got %+v", r.Buckets)
	}
	if r.Timezone != "UTC" {
		t.Errorf("nil loc should default to UTC, got %q", r.Timezone)
	}
}

func TestBuildByTaskSortedByHoursDesc(t *testing.T) {
	r := Build(sampleEntries(), GroupByTask, eur(0), julStart, julEnd, nil)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 task buckets, got %d", len(r.Buckets))
	}
	// "Bug fix" = 3h, "Feature X" = 3h -> tie, ordered by label asc
	if r.Buckets[0].Label != "Bug fix" || r.Buckets[0].Hours != 3 {
		t.Fatalf("bucket[0] = %+v", r.Buckets[0])
	}
	if r.Buckets[1].Label != "Feature X" || r.Buckets[1].Hours != 3 {
		t.Fatalf("bucket[1] = %+v", r.Buckets[1])
	}
	// Buckets are keyed by the stable id, not by the display label.
	if r.Buckets[0].Key != "t1" || r.Buckets[1].Key != "t2" {
		t.Errorf("task buckets must be keyed by task id, got %q / %q", r.Buckets[0].Key, r.Buckets[1].Key)
	}
}

func TestBuildByList(t *testing.T) {
	r := Build(sampleEntries(), GroupByList, eur(0), julStart, julEnd, nil)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 list buckets, got %d", len(r.Buckets))
	}
	m := map[string]float64{}
	keys := map[string]string{}
	for _, b := range r.Buckets {
		m[b.Label] = b.Hours
		keys[b.Label] = b.Key
	}
	if m["Client A"] != 3 || m["Client B"] != 3 {
		t.Fatalf("list hours wrong: %+v", m)
	}
	if keys["Client A"] != "l1" || keys["Client B"] != "l2" {
		t.Fatalf("list buckets must be keyed by list id, got %+v", keys)
	}
}

func TestBuildByDayChronological(t *testing.T) {
	r := Build(sampleEntries(), GroupByDay, eur(0), julStart, julEnd, nil)
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
	r := Build(nil, GroupByTask, eur(50), julStart, julEnd, nil)
	if r.TotalHours != 0 || len(r.Buckets) != 0 {
		t.Fatalf("empty report should be zero, got %+v", r)
	}
	if r.TotalAmount != 0 || len(r.CurrencySubtotals) != 0 || len(r.Lines) != 0 {
		t.Fatalf("empty report should carry no money, got %+v", r)
	}
}

func TestRoundingTwoDecimals(t *testing.T) {
	e := []TimeEntry{{ID: "1", TaskID: "t1", TaskName: "x", Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Duration: d(1.0 / 3.0), Billable: true}}
	r := Build(e, GroupByTask, eur(30), julStart, julEnd, nil)
	if r.Buckets[0].Hours != 0.33 {
		t.Fatalf("hours should round to 0.33, got %v", r.Buckets[0].Hours)
	}
	// 10.00: the money base is the exact billed duration (1/3 h * 30), never the
	// 2-decimal display value, which would misbill this as 0.33 * 30 = 9.90.
	if r.TotalAmount != 10 {
		t.Fatalf("amount should be 10, got %v", r.TotalAmount)
	}
}

func TestRatesFor(t *testing.T) {
	r := Rates{Default: 30, ByList: map[string]float64{"1": 50}}
	if r.For("1", 0) != 50 {
		t.Fatalf("override for list 1 should be 50, got %v", r.For("1", 0))
	}
	if r.For("999", 0) != 30 {
		t.Fatalf("list without override should use default 30, got %v", r.For("999", 0))
	}
}

func TestBuildPerListRates(t *testing.T) {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	entries := []TimeEntry{
		{ID: "1", TaskID: "a", TaskName: "A", ListID: "1", ListName: "Client A", Start: base, Duration: d(2), Billable: true},
		{ID: "2", TaskID: "b", TaskName: "B", ListID: "2", ListName: "Client B", Start: base, Duration: d(1), Billable: true},
	}
	p := Pricing{Rates: Rates{Default: 30, ByList: map[string]float64{"1": 50}}, DefaultCurrency: "EUR"}
	r := Build(entries, GroupByList, p, julStart, julEnd, nil)
	amt := map[string]float64{}
	for _, b := range r.Buckets {
		amt[b.Label] = amountIn(b, "EUR")
	}
	if amt["Client A"] != 100 { // 2h * 50
		t.Fatalf("Client A amount = %v, want 100", amt["Client A"])
	}
	if amt["Client B"] != 30 { // 1h * 30 (default)
		t.Fatalf("Client B amount = %v, want 30", amt["Client B"])
	}
	if r.TotalAmount != 130 {
		t.Fatalf("total amount = %v, want 130", r.TotalAmount)
	}
	if r.DefaultRate != 30 {
		t.Fatalf("Report.DefaultRate should carry the default rate, got %v", r.DefaultRate)
	}
}

func TestBuildMixedRatePerTask(t *testing.T) {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	// same task, two lists with different rates
	entries := []TimeEntry{
		{ID: "1", TaskID: "X", TaskName: "X", ListID: "1", Start: base, Duration: d(2), Billable: true},
		{ID: "2", TaskID: "X", TaskName: "X", ListID: "2", Start: base, Duration: d(1), Billable: true},
	}
	p := Pricing{Rates: Rates{ByList: map[string]float64{"1": 50, "2": 30}}, DefaultCurrency: "EUR"}
	r := Build(entries, GroupByTask, p, julStart, julEnd, nil)
	if len(r.Buckets) != 1 {
		t.Fatalf("want 1 task bucket, got %d", len(r.Buckets))
	}
	if r.Buckets[0].Hours != 3 {
		t.Fatalf("hours = %v, want 3", r.Buckets[0].Hours)
	}
	if got := amountIn(r.Buckets[0], "EUR"); got != 130 { // 2*50 + 1*30
		t.Fatalf("mixed-rate amount = %v, want 130", got)
	}
}

func TestMonthRange(t *testing.T) {
	start, end := MonthRange(2026, time.July, nil)
	if !start.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v", start)
	}
	if !end.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("end = %v", end)
	}
}

func TestBuildGroupByMember(t *testing.T) {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	entries := []TimeEntry{
		{ID: "1", UserID: 1, UserName: "alice", ListID: "l1", Start: base, Duration: 2 * time.Hour, Billable: true},
		{ID: "2", UserID: 2, UserName: "bob", ListID: "l1", Start: base, Duration: 1 * time.Hour, Billable: true},
		{ID: "3", UserID: 1, UserName: "alice", ListID: "l1", Start: base, Duration: 30 * time.Minute, Billable: true},
	}
	r := Build(entries, GroupByMember, eur(10), julStart, julEnd, nil)
	if len(r.Buckets) != 2 {
		t.Fatalf("buckets = %d, want 2", len(r.Buckets))
	}
	if r.Buckets[0].Label != "alice" || r.Buckets[0].Hours != 2.5 {
		t.Errorf("bucket[0] = %+v, want alice 2.5", r.Buckets[0])
	}
	if r.Buckets[1].Label != "bob" || r.Buckets[1].Hours != 1 {
		t.Errorf("bucket[1] = %+v, want bob 1", r.Buckets[1])
	}
	if r.Buckets[0].Key != "1" || r.Buckets[1].Key != "2" {
		t.Errorf("member buckets must be keyed by user id, got %q / %q", r.Buckets[0].Key, r.Buckets[1].Key)
	}
}

// --- v1.7 billing engine ---------------------------------------------------

func TestBuildBillableSplitAndCurrency(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{
		{ID: "1", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: 2 * time.Hour, Billable: true, ListName: "Alpha"},
		{ID: "2", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 12, 0, 0, 0, loc), Duration: time.Hour, Billable: false, ListName: "Alpha"},
		{ID: "3", ListID: "B", UserID: 1, Start: time.Date(2026, 6, 2, 9, 0, 0, 0, loc), Duration: time.Hour, Billable: true, ListName: "Beta"},
	}
	p := Pricing{
		Rates:           Rates{Default: 100},
		Currencies:      map[string]string{"A": "EUR", "B": "USD"},
		DefaultCurrency: "EUR",
	}
	r := Build(entries, GroupByList, p, junStart, junEnd, loc)

	if r.TotalHours != 4 || r.BillableHours != 3 || r.NonBillableHours != 1 {
		t.Fatalf("hours: total=%v billable=%v non=%v", r.TotalHours, r.BillableHours, r.NonBillableHours)
	}
	// Two currencies -> TotalAmount must be 0; truth is in subtotals.
	if r.TotalAmount != 0 {
		t.Fatalf("mixed-currency TotalAmount must be 0, got %v", r.TotalAmount)
	}
	got := map[string]CurrencySubtotal{}
	for _, s := range r.CurrencySubtotals {
		got[s.Currency] = s
	}
	if got["EUR"].Amount != 200 || got["USD"].Amount != 100 { // 2h*100 EUR (billable), 1h*100 USD
		t.Fatalf("subtotal amounts = %+v", got)
	}
	// A5: Hours covers ALL entries in the currency, BillableHours only the billable ones.
	if got["EUR"].Hours != 3 || got["EUR"].BillableHours != 2 || got["EUR"].BilledHours != 2 {
		t.Fatalf("EUR subtotal = %+v, want hours=3 billable=2 billed=2", got["EUR"])
	}
	if got["USD"].Hours != 1 || got["USD"].BillableHours != 1 || got["USD"].BilledHours != 1 {
		t.Fatalf("USD subtotal = %+v, want hours=1 billable=1 billed=1", got["USD"])
	}
	if r.BilledHours != 3 {
		t.Fatalf("report billed hours = %v, want 3", r.BilledHours)
	}
	// Buckets carry the split too.
	byKey := map[string]Bucket{}
	for _, b := range r.Buckets {
		byKey[b.Key] = b
	}
	bA, ok := byKey["A"]
	if !ok {
		t.Fatalf("no bucket keyed A in %+v", r.Buckets)
	}
	if bA.Hours != 3 || bA.BillableHours != 2 || bA.BilledHours != 2 {
		t.Fatalf("bucket A = %+v, want hours=3 billable=2 billed=2", bA)
	}
}

func TestBuildSingleCurrencyTotalAmount(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{{ID: "1", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: time.Hour, Billable: true}}
	p := Pricing{Rates: Rates{Default: 80}, DefaultCurrency: "EUR"}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	if r.TotalAmount != 80 {
		t.Fatalf("single-currency TotalAmount = %v", r.TotalAmount)
	}
}

func TestBuildNonBillableEntriesAreNotBilled(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{{ID: "1", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: time.Hour, Billable: false}}
	p := Pricing{
		Rates: Rates{Default: 80}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundUp, Scope: PerEntry},
	}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	if r.TotalAmount != 0 || r.BilledHours != 0 || r.BillableHours != 0 {
		t.Fatalf("non-billable entry must not be billed: %+v", r)
	}
	if r.NonBillableHours != 1 || r.TotalHours != 1 {
		t.Fatalf("non-billable hours must stay raw: total=%v non=%v", r.TotalHours, r.NonBillableHours)
	}
	if len(r.Lines) != 0 {
		t.Fatalf("non-billable entries must not produce invoice lines, got %+v", r.Lines)
	}
}

func TestBuildRoundingPerEntry(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{
		{ID: "1", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: 50 * time.Minute, Billable: true},
		{ID: "2", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 11, 0, 0, 0, loc), Duration: 50 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 60}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundNearest, Scope: PerEntry},
	}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	// each 50m -> nearest hour = 1h; billed 2h; amount 120
	if r.BilledHours != 2 || r.TotalAmount != 120 {
		t.Fatalf("perEntry: billed=%v amt=%v", r.BilledHours, r.TotalAmount)
	}
	if r.BillableHours != round2(100.0/60.0) {
		t.Fatalf("raw billable preserved: %v", r.BillableHours)
	}
	if len(r.Lines) != 2 {
		t.Fatalf("PerEntry must emit one line per entry, got %d", len(r.Lines))
	}
}

func TestBuildRoundingPerDayGroupsAcrossEntries(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{
		{ID: "1", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: 50 * time.Minute, Billable: true},
		{ID: "2", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 11, 0, 0, 0, loc), Duration: 50 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 60}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundNearest, Scope: PerDay},
	}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	// per-day: 100m summed -> nearest hour = 2h; amount 120
	if r.BilledHours != 2 || r.TotalAmount != 120 {
		t.Fatalf("perDay: billed=%v amt=%v", r.BilledHours, r.TotalAmount)
	}
	if len(r.Lines) != 1 {
		t.Fatalf("PerDay must collapse the day into one line, got %d", len(r.Lines))
	}
}

func TestBuildRoundingPerDaySeparatesMembersAndLists(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	entries := []TimeEntry{
		{ID: "1", ListID: "A", UserID: 1, Start: at, Duration: 30 * time.Minute, Billable: true},
		{ID: "2", ListID: "A", UserID: 2, Start: at, Duration: 30 * time.Minute, Billable: true},
		{ID: "3", ListID: "B", UserID: 1, Start: at, Duration: 30 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 60}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundUp, Scope: PerDay},
	}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	// Three rate-homogeneous units (day|list|user), each rounded up to 1h.
	if len(r.Lines) != 3 || r.BilledHours != 3 || r.TotalAmount != 180 {
		t.Fatalf("units must be (day,list,user): lines=%d billed=%v amt=%v", len(r.Lines), r.BilledHours, r.TotalAmount)
	}
}

// M4: the PerDay unit key uses the calendar day in loc, not in UTC.
func TestBuildPerDayUsesCalendarDayInLocation(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*60*60)
	// 2026-06-01 23:30 UTC and 2026-06-02 01:00 UTC are different UTC days but
	// the same calendar day (2026-06-02) in JST.
	entries := []TimeEntry{
		{ID: "1", ListID: "A", ListName: "Alpha", UserID: 1, Start: time.Date(2026, 6, 1, 23, 30, 0, 0, time.UTC), Duration: 30 * time.Minute, Billable: true},
		{ID: "2", ListID: "A", ListName: "Alpha", UserID: 1, Start: time.Date(2026, 6, 2, 1, 0, 0, 0, time.UTC), Duration: 30 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 60}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundUp, Scope: PerDay},
	}
	r := Build(entries, GroupByDay, p, junStart, junEnd, tokyo)
	if len(r.Lines) != 1 {
		t.Fatalf("both entries fall on 2026-06-02 in JST: want 1 line, got %d (%+v)", len(r.Lines), r.Lines)
	}
	if r.Lines[0].Date != "2026-06-02" {
		t.Errorf("line date = %q, want 2026-06-02 (day in loc)", r.Lines[0].Date)
	}
	if r.BilledHours != 1 || r.TotalAmount != 60 {
		t.Errorf("one day-unit of 1h: billed=%v amt=%v", r.BilledHours, r.TotalAmount)
	}
	if len(r.Buckets) != 1 || r.Buckets[0].Key != "2026-06-02" {
		t.Errorf("day buckets must use loc too, got %+v", r.Buckets)
	}
	if r.Timezone != "JST" {
		t.Errorf("Timezone = %q, want JST", r.Timezone)
	}
	// The same input in UTC splits into two days / two units.
	rUTC := Build(entries, GroupByDay, p, junStart, junEnd, time.UTC)
	if len(rUTC.Lines) != 2 {
		t.Errorf("in UTC the two entries are on different days: want 2 lines, got %d", len(rUTC.Lines))
	}
}

// --- grouping by tag -------------------------------------------------------

func TestBuildGroupByTagDoubleCountsMultiTagEntries(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{
		{ID: "1", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: 2 * time.Hour, Billable: true, Tags: []string{"frontend", "qa"}},
		{ID: "2", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 1, 12, 0, 0, 0, loc), Duration: time.Hour, Billable: true, Tags: []string{"qa"}},
		{ID: "3", ListID: "A", UserID: 1, Start: time.Date(2026, 6, 2, 9, 0, 0, 0, loc), Duration: time.Hour, Billable: true},
	}
	r := Build(entries, GroupByTag, eur(10), junStart, junEnd, loc)

	byLabel := map[string]Bucket{}
	for _, b := range r.Buckets {
		byLabel[b.Label] = b
	}
	if len(byLabel) != 3 {
		t.Fatalf("want frontend/qa/(untagged) buckets, got %+v", r.Buckets)
	}
	if byLabel["frontend"].Hours != 2 || byLabel["qa"].Hours != 3 || byLabel[untaggedLabel].Hours != 1 {
		t.Fatalf("tag hours wrong: %+v", byLabel)
	}
	if byLabel["frontend"].Key != "tag|frontend" {
		t.Errorf("tag bucket key = %q, want tag|frontend", byLabel["frontend"].Key)
	}
	// Intended: the 2h entry is counted under both its tags, so the per-tag
	// hours (2+3+1=6) exceed the report total (4h).
	var sum float64
	for _, b := range r.Buckets {
		sum += b.Hours
	}
	if sum != 6 || r.TotalHours != 4 {
		t.Fatalf("tag buckets must double-count multi-tag entries: sum=%v total=%v", sum, r.TotalHours)
	}
	// The currency subtotals stay the unique-entry truth: 4h * 10.
	if len(r.CurrencySubtotals) != 1 || r.CurrencySubtotals[0].Amount != 40 {
		t.Fatalf("currency subtotals must not double-count: %+v", r.CurrencySubtotals)
	}
	if r.TotalAmount != 40 {
		t.Fatalf("TotalAmount = %v, want 40", r.TotalAmount)
	}
	// Tag buckets are sorted by label asc.
	if r.Buckets[0].Label != untaggedLabel || r.Buckets[1].Label != "frontend" || r.Buckets[2].Label != "qa" {
		t.Errorf("tag buckets must be label-sorted, got %+v", r.Buckets)
	}
}

// --- proportional allocation (A7) -----------------------------------------

func TestBuildPerDayWithFinerGroupingDistributesProportionally(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	entries := []TimeEntry{
		{ID: "1", TaskID: "t1", TaskName: "One", ListID: "A", UserID: 1, Start: at, Duration: 20 * time.Minute, Billable: true},
		{ID: "2", TaskID: "t2", TaskName: "Two", ListID: "A", UserID: 1, Start: at.Add(time.Hour), Duration: 40 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 100}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundNearest, Scope: PerDay},
	}
	r := Build(entries, GroupByTask, p, junStart, junEnd, loc)
	// One billing unit of 1h -> 100.00, split 1/3 - 2/3 across the two tasks.
	byKey := map[string]Bucket{}
	for _, b := range r.Buckets {
		byKey[b.Key] = b
	}
	if byKey["t1"].BilledHours != 0.33 || byKey["t2"].BilledHours != 0.67 {
		t.Fatalf("billed hours not distributed: %+v", byKey)
	}
	if amountIn(byKey["t1"], "EUR") != 33.33 || amountIn(byKey["t2"], "EUR") != 66.67 {
		t.Fatalf("amounts not distributed: %+v", byKey)
	}
	if r.TotalAmount != 100 {
		t.Fatalf("authoritative total = %v, want 100", r.TotalAmount)
	}
}

// A7: bucket allocation is indicative; the drift against the authoritative
// currency subtotal must stay strictly below one cent per bucket.
func TestBuildBucketAllocationDriftIsBounded(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	var entries []TimeEntry
	for i := range 3 {
		entries = append(entries, TimeEntry{
			ID: string(rune('1' + i)), TaskID: "t" + string(rune('1'+i)), TaskName: "T",
			ListID: "A", UserID: 1, Start: at.Add(time.Duration(i) * time.Hour),
			Duration: 20 * time.Minute, Billable: true,
		})
	}
	p := Pricing{
		Rates: Rates{Default: 100}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: time.Hour, Mode: duration.RoundNearest, Scope: PerDay},
	}
	r := Build(entries, GroupByTask, p, junStart, junEnd, loc)
	var sum float64
	for _, b := range r.Buckets {
		sum += amountIn(b, "EUR")
	}
	want := r.CurrencySubtotals[0].Amount
	drift := math.Abs(sum - want)
	if drift >= float64(len(r.Buckets))*0.01 {
		t.Fatalf("bucket drift %v must stay below %d cents (sum=%v subtotal=%v)", drift, len(r.Buckets), sum, want)
	}
	if drift == 0 {
		t.Log("no drift in this fixture; the bound still holds")
	}
}

// --- invoice lines (A4) ----------------------------------------------------

func TestBuildLinesPerEntry(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{
		{ID: "1", TaskID: "t1", TaskName: "Task One", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice",
			Start: time.Date(2026, 6, 2, 9, 0, 0, 0, loc), Duration: 90 * time.Minute, Billable: true},
		{ID: "2", TaskID: "t2", TaskName: "Task Two", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice",
			Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: time.Hour, Billable: true},
		{ID: "3", TaskID: "t3", TaskName: "Unbilled", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice",
			Start: time.Date(2026, 6, 1, 14, 0, 0, 0, loc), Duration: time.Hour, Billable: false},
	}
	p := Pricing{Rates: Rates{Default: 100}, Currencies: map[string]string{"A": "EUR"}, DefaultCurrency: "USD"}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)

	if len(r.Lines) != 2 {
		t.Fatalf("want 2 lines (billable only), got %d: %+v", len(r.Lines), r.Lines)
	}
	// Sorted by (date asc, list asc, user asc).
	want := []InvoiceLine{
		{Date: "2026-06-01", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice",
			Description: "Task Two", Hours: 1, Rate: 100, Amount: 100, Currency: "EUR", Billable: true},
		{Date: "2026-06-02", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice",
			Description: "Task One", Hours: 1.5, Rate: 100, Amount: 150, Currency: "EUR", Billable: true},
	}
	for i, w := range want {
		if r.Lines[i] != w {
			t.Errorf("line[%d] = %+v, want %+v", i, r.Lines[i], w)
		}
	}
}

func TestBuildLinesPerDayDescriptionFallsBackToList(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	entries := []TimeEntry{
		{ID: "1", TaskID: "t1", TaskName: "One", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice", Start: at, Duration: time.Hour, Billable: true},
		{ID: "2", TaskID: "t2", TaskName: "Two", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice", Start: at.Add(time.Hour), Duration: time.Hour, Billable: true},
		{ID: "3", TaskID: "t3", TaskName: "Three", ListID: "B", ListName: "Beta", UserID: 1, UserName: "alice", Start: at, Duration: time.Hour, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 50}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: 30 * time.Minute, Mode: duration.RoundNearest, Scope: PerDay},
	}
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	if len(r.Lines) != 2 {
		t.Fatalf("want 2 day units (one per list), got %d: %+v", len(r.Lines), r.Lines)
	}
	// Same date -> sorted by list name; Alpha spans two tasks, so it falls back
	// to the list name; Beta has a single task and keeps the task name.
	if r.Lines[0].ListName != "Alpha" || r.Lines[0].Description != "Alpha" {
		t.Errorf("multi-task unit should describe itself with the list name, got %+v", r.Lines[0])
	}
	if r.Lines[1].ListName != "Beta" || r.Lines[1].Description != "Three" {
		t.Errorf("single-task unit should use the task name, got %+v", r.Lines[1])
	}
}

func TestBuildLinesSortOrder(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	entries := []TimeEntry{
		{ID: "1", ListID: "B", ListName: "Beta", UserID: 2, UserName: "bob", Start: at.AddDate(0, 0, 1), Duration: time.Hour, Billable: true},
		{ID: "2", ListID: "A", ListName: "Alpha", UserID: 2, UserName: "bob", Start: at, Duration: time.Hour, Billable: true},
		{ID: "3", ListID: "A", ListName: "Alpha", UserID: 1, UserName: "alice", Start: at, Duration: time.Hour, Billable: true},
		{ID: "4", ListID: "B", ListName: "Beta", UserID: 1, UserName: "alice", Start: at, Duration: time.Hour, Billable: true},
	}
	r := Build(entries, GroupByTotal, eur(10), junStart, junEnd, loc)
	var got []string
	for _, l := range r.Lines {
		got = append(got, l.Date+"/"+l.ListName+"/"+l.UserName)
	}
	want := []string{
		"2026-06-01/Alpha/alice",
		"2026-06-01/Alpha/bob",
		"2026-06-01/Beta/alice",
		"2026-06-02/Beta/bob",
	}
	if len(got) != len(want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lines = %v, want %v", got, want)
		}
	}
}

// The money-ledger rule: sum(line amounts) == currency subtotal, exactly.
func TestBuildLinesSumMatchesCurrencySubtotals(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	entries := []TimeEntry{
		{ID: "1", ListID: "A", ListName: "Alpha", UserID: 1, Start: at, Duration: 100 * time.Minute, Billable: true},
		{ID: "2", ListID: "A", ListName: "Alpha", UserID: 1, Start: at.AddDate(0, 0, 1), Duration: 50 * time.Minute, Billable: true},
		{ID: "3", ListID: "B", ListName: "Beta", UserID: 1, Start: at, Duration: 25 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates:           Rates{Default: 77},
		Currencies:      map[string]string{"B": "USD"},
		DefaultCurrency: "EUR",
	}
	r := Build(entries, GroupByList, p, junStart, junEnd, loc)
	sums := map[string]float64{}
	for _, l := range r.Lines {
		sums[l.Currency] = round2(sums[l.Currency] + l.Amount)
	}
	for _, s := range r.CurrencySubtotals {
		if sums[s.Currency] != s.Amount {
			t.Errorf("%s: sum(lines) = %v, subtotal = %v", s.Currency, sums[s.Currency], s.Amount)
		}
	}
}

// The amount bills the EXACT billed duration, and InvoiceLine.Hours carries 4
// decimals so the row still reconciles at cent precision:
// round2(Hours * Rate) == Amount, even for increments whose hour value is not
// exact to 2 decimals (5m -> 0.0833h).
func TestBuildLinesReconcileHoursTimesRate(t *testing.T) {
	loc := time.UTC
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, loc)
	entries := []TimeEntry{
		{ID: "1", ListID: "A", ListName: "Alpha", UserID: 1, Start: at, Duration: 7 * time.Minute, Billable: true},
		{ID: "2", ListID: "A", ListName: "Alpha", UserID: 1, Start: at.Add(time.Hour), Duration: 23 * time.Minute, Billable: true},
		{ID: "3", ListID: "A", ListName: "Alpha", UserID: 1, Start: at.Add(2 * time.Hour), Duration: 41 * time.Minute, Billable: true},
	}
	p := Pricing{
		Rates: Rates{Default: 60}, DefaultCurrency: "EUR",
		Rounding: RoundRule{Increment: 5 * time.Minute, Mode: duration.RoundNearest, Scope: PerEntry},
	}
	r := Build(entries, GroupByList, p, junStart, junEnd, loc)
	if len(r.Lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(r.Lines))
	}
	var sum float64
	for _, l := range r.Lines {
		if got := round2(l.Hours * l.Rate); got != l.Amount {
			t.Errorf("line %+v does not reconcile: round2(%v * %v) = %v, Amount = %v", l, l.Hours, l.Rate, got, l.Amount)
		}
		sum = round2(sum + l.Amount)
	}
	// 7m -> 5m (0.0833h) at 60/h bills the exact 5 minutes: 5.00, not 4.80.
	if r.Lines[0].Hours != 0.0833 || r.Lines[0].Amount != 5 {
		t.Errorf("line[0] = %+v, want 0.0833h / 5.00", r.Lines[0])
	}
	// The ledger identity still holds: sum(lines) == currency subtotal.
	if len(r.CurrencySubtotals) != 1 || sum != r.CurrencySubtotals[0].Amount {
		t.Errorf("sum(lines) = %v, subtotals = %+v", sum, r.CurrencySubtotals)
	}
	if sum != r.TotalAmount {
		t.Errorf("sum(lines) = %v, TotalAmount = %v", sum, r.TotalAmount)
	}
}

// Regression guard: with no rounding configured, a 20-minute unit at 30/h must
// bill the exact 20 minutes (10.00) and report 0.3333 h — never 0.33 h / 9.90.
func TestBuildLineUnroundedKeepsExactAmount(t *testing.T) {
	loc := time.UTC
	entries := []TimeEntry{
		{ID: "1", ListID: "A", ListName: "Alpha", UserID: 1,
			Start: time.Date(2026, 6, 1, 9, 0, 0, 0, loc), Duration: 20 * time.Minute, Billable: true},
	}
	p := Pricing{Rates: Rates{Default: 30}, DefaultCurrency: "EUR"} // no RoundRule
	r := Build(entries, GroupByTotal, p, junStart, junEnd, loc)
	if len(r.Lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(r.Lines))
	}
	l := r.Lines[0]
	if l.Hours != 0.3333 {
		t.Errorf("line hours = %v, want 0.3333 (4 decimals)", l.Hours)
	}
	if l.Amount != 10 {
		t.Errorf("line amount = %v, want 10 (exact billed duration * rate)", l.Amount)
	}
	if got := round2(l.Hours * l.Rate); got != l.Amount {
		t.Errorf("line must reconcile: round2(%v * %v) = %v, Amount = %v", l.Hours, l.Rate, got, l.Amount)
	}
	if r.TotalAmount != 10 {
		t.Errorf("TotalAmount = %v, want 10", r.TotalAmount)
	}
	// The 2-decimal aggregates keep their coarser display precision.
	if r.BilledHours != 0.33 {
		t.Errorf("BilledHours = %v, want 0.33 (2 decimals)", r.BilledHours)
	}
}

func TestRound4(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{1.0 / 3.0, 0.3333},
		{1.0 / 12.0, 0.0833}, // 5 minutes
		{2.0 / 3.0, 0.6667},
		{1, 1},
		{0, 0},
	}
	for _, c := range cases {
		if got := round4(c.in); got != c.want {
			t.Errorf("round4(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPricingCurrencyFor(t *testing.T) {
	p := Pricing{Currencies: map[string]string{"A": "USD"}, DefaultCurrency: "EUR"}
	if got := p.currencyFor("A"); got != "USD" {
		t.Errorf("currencyFor(A) = %q, want USD", got)
	}
	if got := p.currencyFor("Z"); got != "EUR" {
		t.Errorf("currencyFor(Z) = %q, want EUR (default)", got)
	}
}
