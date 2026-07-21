package report

import (
	"cmp"
	"math"
	"slices"
	"time"
)

// Supported grouping modes.
const (
	GroupByTask  = "task"
	GroupByList  = "list"
	GroupByDay   = "day"
	GroupByTotal = "total"
)

// round2 rounds to 2 decimal places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// MonthRange returns the half-open interval [start, end) of the month in UTC.
func MonthRange(year int, month time.Month) (start, end time.Time) {
	start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return start, end
}

// keyFor determines the bucket label for an entry, given groupBy.
func keyFor(e TimeEntry, groupBy string) string {
	switch groupBy {
	case GroupByTask:
		return e.TaskName
	case GroupByList:
		return e.ListName
	case GroupByDay:
		return e.Start.Format("2006-01-02")
	default:
		return "Total"
	}
}

// Build aggregates the entries into a Report according to groupBy. Each bucket's amount
// is the sum, over the bucket's entries, of actual_hours × list_rate (Rates.For),
// rounded to 2 decimal places. Report.Rate carries the default rate (for export).
func Build(entries []TimeEntry, groupBy string, rates Rates, currency string, year int, month time.Month) Report {
	r := Report{
		Year:     year,
		Month:    month,
		GroupBy:  groupBy,
		Currency: currency,
		Rate:     rates.Default,
	}

	sumsH := map[string]float64{}
	sumsA := map[string]float64{}
	var order []string
	for _, e := range entries {
		k := keyFor(e, groupBy)
		if _, seen := sumsH[k]; !seen {
			order = append(order, k)
		}
		h := e.Duration.Hours()
		sumsH[k] += h
		sumsA[k] += h * rates.For(e.ListID)
	}

	for _, k := range order {
		r.Buckets = append(r.Buckets, Bucket{
			Label:  k,
			Hours:  round2(sumsH[k]),
			Amount: round2(sumsA[k]),
		})
	}

	// Sorting: chronological by day (label asc); otherwise hours desc, tie label asc.
	slices.SortStableFunc(r.Buckets, func(a, b Bucket) int {
		if groupBy == GroupByDay {
			return cmp.Compare(a.Label, b.Label)
		}
		if c := cmp.Compare(b.Hours, a.Hours); c != 0 { // hours desc
			return c
		}
		return cmp.Compare(a.Label, b.Label)
	})

	var th, ta float64
	for _, b := range r.Buckets {
		th += b.Hours
		ta += b.Amount
	}
	r.TotalHours = round2(th)
	r.TotalAmount = round2(ta)
	return r
}
