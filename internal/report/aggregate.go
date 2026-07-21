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

// Build aggrega le entry in un Report secondo groupBy, applicando la tariffa.
func Build(entries []TimeEntry, groupBy string, rate float64, currency string, year int, month time.Month) Report {
	r := Report{
		Year:     year,
		Month:    month,
		GroupBy:  groupBy,
		Currency: currency,
		Rate:     rate,
	}

	sums := map[string]float64{}
	var order []string
	for _, e := range entries {
		k := keyFor(e, groupBy)
		if _, seen := sums[k]; !seen {
			order = append(order, k)
		}
		sums[k] += e.Duration.Hours()
	}

	for _, k := range order {
		h := round2(sums[k])
		r.Buckets = append(r.Buckets, Bucket{Label: k, Hours: h, Amount: round2(h * rate)})
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

	var total float64
	for _, b := range r.Buckets {
		total += b.Hours
	}
	r.TotalHours = round2(total)
	r.TotalAmount = round2(r.TotalHours * rate)
	return r
}
