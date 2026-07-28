package report

import "time"

// DailyHours returns one element per day in [start, end): the total hours of
// the entries that STARTED that day, read in loc. It returns nil when the
// range is empty or inverted.
//
// It counts ALL hours, billable and not — the same total Bucket.Hours carries.
// The series answers "when did I work", not "when did I bill"; the billable
// share has its own line under the report table.
//
// The hours of an entry land entirely on its start day, which is the same rule
// groupKeys uses for GroupByDay — the two views cannot disagree about which
// day an overnight entry belongs to.
//
// This exists instead of reusing GroupByDay buckets because Build creates no
// bucket for a day with no entries. A series built from buckets would close
// the gaps, drawing a full month of work for three days of it.
//
// Days are advanced with AddDate and identified by their formatted key, never
// by adding 24 hours: across a daylight-saving transition a day is 23 or 25
// hours long, and second-arithmetic would silently drop or duplicate one.
func DailyHours(entries []TimeEntry, start, end time.Time, loc *time.Location) []float64 {
	loc = normLoc(loc)
	day := midnightIn(start, loc)
	last := midnightIn(end, loc)
	if !day.Before(last) {
		return nil
	}

	index := map[string]int{}
	var out []float64
	for d := day; d.Before(last); d = d.AddDate(0, 0, 1) {
		index[d.Format(dayFormat)] = len(out)
		out = append(out, 0)
	}

	for _, e := range entries {
		if i, ok := index[e.Start.In(loc).Format(dayFormat)]; ok {
			out[i] += e.Duration.Hours()
		}
	}
	return out
}
