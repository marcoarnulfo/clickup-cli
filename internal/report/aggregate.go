package report

import (
	"math"
	"sort"
	"time"
)

// Modalità di raggruppamento supportate.
const (
	GroupByTask  = "task"
	GroupByList  = "list"
	GroupByDay   = "day"
	GroupByTotal = "total"
)

// round2 arrotonda a 2 decimali.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// MonthRange ritorna l'intervallo half-open [start, end) del mese in UTC.
func MonthRange(year int, month time.Month) (start, end time.Time) {
	start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return start, end
}

// keyFor determina l'etichetta del bucket per una entry, dato il groupBy.
func keyFor(e TimeEntry, groupBy string) string {
	switch groupBy {
	case GroupByTask:
		return e.TaskName
	case GroupByList:
		return e.ListName
	case GroupByDay:
		return e.Start.Format("2006-01-02")
	default:
		return "Totale"
	}
}

// Build aggrega le entry in un Report secondo groupBy. L'importo di ogni bucket
// è la somma, sulle entry del bucket, di ore_reali × tariffa_della_lista (Rates.For),
// arrotondata a 2 decimali. Report.Rate riporta la tariffa di default (per l'export).
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

	// Ordinamento: per giorno cronologico (label asc); altrimenti ore desc, tie label asc.
	sort.SliceStable(r.Buckets, func(i, j int) bool {
		a, b := r.Buckets[i], r.Buckets[j]
		if groupBy == GroupByDay {
			return a.Label < b.Label
		}
		if a.Hours != b.Hours {
			return a.Hours > b.Hours
		}
		return a.Label < b.Label
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
