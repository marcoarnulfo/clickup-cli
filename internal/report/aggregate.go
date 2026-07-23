package report

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/duration"
)

// Supported grouping modes.
const (
	GroupByTask   = "task"
	GroupByList   = "list"
	GroupByDay    = "day"
	GroupByMember = "member"
	GroupByTag    = "tag"
	GroupByTotal  = "total"
)

// untaggedLabel is the bucket used by GroupByTag for entries with no tags.
const untaggedLabel = "(untagged)"

// dayFormat is the calendar-day layout used for day buckets and invoice lines.
const dayFormat = "2006-01-02"

// round2 rounds to 2 decimal places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// MonthRange returns the half-open interval [start, end) of the month in loc.
// loc == nil is treated as UTC.
func MonthRange(year int, month time.Month, loc *time.Location) (start, end time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	start = time.Date(year, month, 1, 0, 0, 0, 0, loc)
	end = start.AddDate(0, 1, 0)
	return start, end
}

// bucketRef is a (stable id, display label) pair for one report bucket.
type bucketRef struct{ key, label string }

// groupKeys returns the buckets an entry belongs to, given groupBy. Buckets are
// keyed by a stable ID (list/task/user id, calendar day, tag) and carry the
// human-readable name as label.
//
// GroupByTag is the only grouping that can return more than one bucket: an entry
// with N tags lands in all N of them (see the caveat on Build).
func groupKeys(e TimeEntry, groupBy string, loc *time.Location) []bucketRef {
	switch groupBy {
	case GroupByTask:
		return []bucketRef{{e.TaskID, e.TaskName}}
	case GroupByList:
		return []bucketRef{{e.ListID, e.ListName}}
	case GroupByDay:
		day := e.Start.In(loc).Format(dayFormat)
		return []bucketRef{{day, day}}
	case GroupByMember:
		return []bucketRef{{strconv.Itoa(e.UserID), e.UserName}}
	case GroupByTag:
		if len(e.Tags) == 0 {
			return []bucketRef{{untaggedLabel, untaggedLabel}}
		}
		refs := make([]bucketRef, 0, len(e.Tags))
		for _, t := range e.Tags {
			refs = append(refs, bucketRef{"tag|" + t, t})
		}
		return refs
	default: // GroupByTotal and anything unknown
		return []bucketRef{{"total", "Total"}}
	}
}

// bucketSignature joins an entry's bucket keys into a comparable string.
func bucketSignature(e TimeEntry, groupBy string, loc *time.Location) string {
	refs := groupKeys(e, groupBy, loc)
	keys := make([]string, 0, len(refs))
	for _, r := range refs {
		keys = append(keys, r.key)
	}
	return strings.Join(keys, "\x00")
}

// sameBucketForAll reports whether every entry of a billing unit maps to exactly
// the same bucket set, i.e. the grouping is not finer than the billing unit.
func sameBucketForAll(ents []int, entries []TimeEntry, groupBy string, loc *time.Location) bool {
	if len(ents) == 0 {
		return true
	}
	first := bucketSignature(entries[ents[0]], groupBy, loc)
	for _, i := range ents[1:] {
		if bucketSignature(entries[i], groupBy, loc) != first {
			return false
		}
	}
	return true
}

// currencyAmounts turns a currency->amount map into a deterministic, rounded slice.
func currencyAmounts(m map[string]float64) []CurrencyAmount {
	if len(m) == 0 {
		return nil
	}
	out := make([]CurrencyAmount, 0, len(m))
	for _, c := range slices.Sorted(maps.Keys(m)) {
		out = append(out, CurrencyAmount{Currency: c, Amount: round2(m[c])})
	}
	return out
}

// sortedKeys returns the sorted union of the given maps' keys.
func sortedKeys(ms ...map[string]float64) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range ms {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	slices.Sort(out)
	return out
}

// sortBuckets orders the report rows: chronological/alphabetical for day and tag
// groupings, otherwise billed hours desc (then raw hours desc, then label asc).
func sortBuckets(buckets []Bucket, groupBy string) {
	slices.SortStableFunc(buckets, func(a, b Bucket) int {
		if groupBy == GroupByDay || groupBy == GroupByTag {
			return cmp.Compare(a.Label, b.Label)
		}
		if c := cmp.Compare(b.BilledHours, a.BilledHours); c != 0 { // billed desc
			return c
		}
		if c := cmp.Compare(b.Hours, a.Hours); c != 0 { // raw hours desc
			return c
		}
		return cmp.Compare(a.Label, b.Label)
	})
}

// Build aggregates entries into a Report over [start, end), grouped by groupBy
// and priced by p. loc == nil is treated as UTC; it defines the calendar day used
// for day buckets and for PerDay rounding (DST days are 23/25h, not a fixed 24h).
//
// Money ledger rule: the amount is rounded to 2 decimals at the smallest
// rate-homogeneous *billing unit* (PerEntry: each billable entry; PerDay: each
// (day-in-loc, listID, userID) group). Every aggregate — bucket, currency
// subtotal, total — is a sum of already-rounded unit amounts, so
// sum(Lines) == CurrencySubtotals exactly. Only billable entries are billed and
// only billable hours are rounded; non-billable hours stay raw.
//
// Caveat (indicative bucket allocation): when the grouping is finer than the
// billing unit (PerDay rounding with GroupByTask or GroupByTag), a unit's billed
// hours and amount are split across its buckets proportionally to raw hours. The
// per-bucket Amounts are therefore indicative and may drift a few cents (strictly
// less than one cent per bucket) from CurrencySubtotals, which — together with
// Lines — remain the authoritative totals.
//
// Caveat (GroupByTag double counting): an entry with N tags is counted in all N
// tag buckets, so per-tag hours and amounts can exceed the report total. The
// CurrencySubtotals stay the unique-entry truth.
func Build(entries []TimeEntry, groupBy string, p Pricing, start, end time.Time, loc *time.Location) Report {
	if loc == nil {
		loc = time.UTC
	}
	r := Report{
		Start: start, End: end, GroupBy: groupBy,
		Timezone:        loc.String(),
		DefaultCurrency: p.DefaultCurrency,
		DefaultRate:     p.Rates.Default,
	}

	// --- billing units: rate-homogeneous groups over BILLABLE entries ---
	// PerDay -> (day-in-loc, listID, userID); otherwise one unit per entry.
	type unit struct {
		day       string
		listID    string
		listName  string
		userID    int
		userName  string
		taskID    string
		taskName  string
		multiTask bool
		raw       time.Duration
		ents      []int // indexes into entries, for finer-than-unit grouping
	}
	units := map[string]*unit{}
	var unitOrder []string
	perDay := p.Rounding.Scope == PerDay && p.Rounding.Increment > 0
	for i, e := range entries {
		if !e.Billable {
			continue
		}
		day := e.Start.In(loc).Format(dayFormat)
		// PerEntry units are keyed by position, not by e.ID: an entry id is not
		// guaranteed to be present (and two entries with an empty id must never
		// collapse into one unit and inherit a single list's rate).
		k := "e|" + strconv.Itoa(i)
		if perDay {
			k = day + "|" + e.ListID + "|" + strconv.Itoa(e.UserID)
		}
		u := units[k]
		if u == nil {
			u = &unit{
				day: day, listID: e.ListID, listName: e.ListName,
				userID: e.UserID, userName: e.UserName,
				taskID: e.TaskID, taskName: e.TaskName,
			}
			units[k] = u
			unitOrder = append(unitOrder, k)
		} else if u.taskID != e.TaskID {
			u.multiTask = true
		}
		u.raw += e.Duration
		u.ents = append(u.ents, i)
	}

	// Per-currency accumulators (authoritative totals).
	curHours := map[string]float64{}    // ALL entries' hours, by currency
	curBillable := map[string]float64{} // raw billable hours, by currency
	curBilled := map[string]float64{}   // post-rounding billable hours
	curAmt := map[string]float64{}      // sum of rounded unit amounts

	// Per-bucket accumulators.
	type bacc struct {
		label                        string
		hours, billableHours, billed float64
		amt                          map[string]float64
	}
	buckets := map[string]*bacc{}
	var bOrder []string
	getBucket := func(key, label string) *bacc {
		b := buckets[key]
		if b == nil {
			b = &bacc{label: label, amt: map[string]float64{}}
			buckets[key] = b
			bOrder = append(bOrder, key)
		}
		return b
	}

	// 1) Raw hours: report totals, per-currency hours, per-bucket hours.
	for _, e := range entries {
		h := e.Duration.Hours()
		cur := p.currencyFor(e.ListID)
		r.TotalHours += h
		curHours[cur] += h
		if e.Billable {
			r.BillableHours += h
			curBillable[cur] += h
		} else {
			r.NonBillableHours += h
		}
		for _, gk := range groupKeys(e, groupBy, loc) {
			b := getBucket(gk.key, gk.label)
			b.hours += h
			if e.Billable {
				b.billableHours += h
			}
		}
	}

	// 2) Billed hours, amounts and invoice lines, from the billing units.
	for _, k := range unitOrder {
		u := units[k]
		billed := duration.Round(u.raw, p.Rounding.Increment, p.Rounding.Mode)
		rate := p.Rates.For(u.listID, u.userID)
		cur := p.currencyFor(u.listID)
		// The amount is computed from the hours the unit actually reports, not
		// from the unrounded duration: an invoice line must reconcile with its
		// own arithmetic (Hours × Rate == Amount) for whoever receives it. The
		// two agree exactly for the usual increments (15m, 30m, 1h); for
		// increments whose hour value is not exact to 2 decimals (5m, 10m, 20m,
		// 40m — and for unrounded reports) this deliberately bills the displayed
		// hours rather than the raw duration.
		billedH := round2(billed.Hours())
		amt := round2(billedH * rate)
		curBilled[cur] += billedH
		curAmt[cur] += amt

		desc := u.taskName
		if u.multiTask || desc == "" {
			desc = u.listName
		}
		r.Lines = append(r.Lines, InvoiceLine{
			Date: u.day, ListID: u.listID, ListName: u.listName,
			UserID: u.userID, UserName: u.userName,
			Description: desc, Hours: billedH, Rate: rate,
			Amount: amt, Currency: cur, Billable: true,
		})

		// Attribute billed hours + amount to buckets. If the grouping is not
		// finer than the unit, all the unit's entries share one bucket set;
		// otherwise split proportionally by raw hours (indicative, see above).
		if len(u.ents) == 1 || sameBucketForAll(u.ents, entries, groupBy, loc) {
			for _, gk := range groupKeys(entries[u.ents[0]], groupBy, loc) {
				b := getBucket(gk.key, gk.label)
				b.billed += billedH
				b.amt[cur] += amt
			}
		} else {
			for _, idx := range u.ents {
				e := entries[idx]
				share := 0.0
				if u.raw > 0 {
					share = e.Duration.Hours() / u.raw.Hours()
				}
				for _, gk := range groupKeys(e, groupBy, loc) {
					b := getBucket(gk.key, gk.label)
					b.billed += billedH * share
					b.amt[cur] += amt * share
				}
			}
		}
	}

	// --- assemble the report ---
	for _, cur := range sortedKeys(curHours, curBillable, curBilled, curAmt) {
		r.CurrencySubtotals = append(r.CurrencySubtotals, CurrencySubtotal{
			Currency:      cur,
			Hours:         round2(curHours[cur]),
			BillableHours: round2(curBillable[cur]),
			BilledHours:   round2(curBilled[cur]),
			Amount:        round2(curAmt[cur]),
		})
	}
	// No cross-currency totals: TotalAmount only makes sense single-currency.
	if len(r.CurrencySubtotals) == 1 {
		r.TotalAmount = r.CurrencySubtotals[0].Amount
	}

	for _, k := range bOrder {
		b := buckets[k]
		r.Buckets = append(r.Buckets, Bucket{
			Label: b.label, Key: k,
			Hours: round2(b.hours), BillableHours: round2(b.billableHours),
			BilledHours: round2(b.billed), Amounts: currencyAmounts(b.amt),
		})
	}
	sortBuckets(r.Buckets, groupBy)

	r.TotalHours = round2(r.TotalHours)
	r.BillableHours = round2(r.BillableHours)
	r.NonBillableHours = round2(r.NonBillableHours)
	for _, s := range r.CurrencySubtotals {
		r.BilledHours += s.BilledHours
	}
	r.BilledHours = round2(r.BilledHours)

	slices.SortStableFunc(r.Lines, func(a, b InvoiceLine) int {
		if c := cmp.Compare(a.Date, b.Date); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ListName, b.ListName); c != 0 {
			return c
		}
		return cmp.Compare(a.UserName, b.UserName)
	})
	return r
}
