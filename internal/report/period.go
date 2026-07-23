package report

import "time"

// Range preset identifiers.
const (
	PresetThisMonth = "this_month"
	PresetLastMonth = "last_month"
	PresetLast7d    = "last_7d"
	PresetLast30d   = "last_30d"
	PresetThisWeek  = "this_week"
	PresetCustom    = "custom"
)

// midnightIn returns the start-of-day of t's calendar date in loc.
// loc == nil is treated as UTC.
func midnightIn(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// WeekRange returns the half-open interval [start, end) for the ISO-8601 week
// (Monday..Sunday) identified by isoYear/isoWeek, in loc. loc == nil is
// treated as UTC.
func WeekRange(isoYear, isoWeek int, loc *time.Location) (start, end time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	// Jan 4 is always in ISO week 1; walk back to that week's Monday, then add weeks.
	jan4 := time.Date(isoYear, 1, 4, 0, 0, 0, 0, loc)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7
	}
	week1Mon := jan4.AddDate(0, 0, -(wd - 1))
	start = week1Mon.AddDate(0, 0, (isoWeek-1)*7)
	end = start.AddDate(0, 0, 7)
	return start, end
}

// CustomRange returns the half-open interval [start, end) for a custom date
// range, where the "to" date is inclusive. Specifically, end = to + 1 day.
// from and to are taken as calendar dates in loc; loc == nil is treated as UTC.
func CustomRange(from, to time.Time, loc *time.Location) (start, end time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	return midnightIn(from, loc), midnightIn(to, loc).AddDate(0, 0, 1)
}

// RangeForPreset returns the half-open interval [start, end) for a non-custom
// preset. this_month uses the given year/month; the relative presets use now.
// An unknown preset falls back to this_month. loc == nil is treated as UTC.
func RangeForPreset(preset string, year int, month time.Month, now time.Time, loc *time.Location) (start, end time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	switch preset {
	case PresetLastMonth:
		n := now.In(loc)
		thisMonthStart, _ := MonthRange(n.Year(), n.Month(), loc)
		lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
		return lastMonthStart, thisMonthStart
	case PresetLast7d:
		e := midnightIn(now, loc).AddDate(0, 0, 1)
		return e.AddDate(0, 0, -7), e
	case PresetLast30d:
		e := midnightIn(now, loc).AddDate(0, 0, 1)
		return e.AddDate(0, 0, -30), e
	case PresetThisWeek:
		d := midnightIn(now, loc)
		wd := int(d.Weekday()) // Sunday=0..Saturday=6
		if wd == 0 {
			wd = 7 // treat Sunday as day 7 (ISO week ends Sunday)
		}
		mon := d.AddDate(0, 0, -(wd - 1))
		return mon, mon.AddDate(0, 0, 7)
	default: // PresetThisMonth and unknown
		return MonthRange(year, month, loc)
	}
}

// isCalendarMonth reports whether [start, end) is exactly one calendar month,
// in start's own location.
func isCalendarMonth(start, end time.Time) bool {
	if start.Day() != 1 || start.Hour() != 0 || start.Minute() != 0 ||
		start.Second() != 0 || start.Nanosecond() != 0 {
		return false
	}
	return end.Equal(start.AddDate(0, 1, 0))
}

// PeriodLabel renders [start, end) as a calendar month ("January 2006") when it
// is exactly one, else "2006-01-02 → 2006-01-02" with the second date inclusive.
// Formatting uses start's/end's own location (no forced UTC conversion).
func PeriodLabel(start, end time.Time) string {
	if isCalendarMonth(start, end) {
		return start.Format("January 2006")
	}
	return start.Format("2006-01-02") + " → " + end.AddDate(0, 0, -1).Format("2006-01-02")
}

// PeriodFileSlug renders the period for an export filename: "2006-01" for a
// calendar month, else "2006-01-02_2006-01-02" (second date inclusive).
// Formatting uses start's/end's own location (no forced UTC conversion).
func PeriodFileSlug(start, end time.Time) string {
	if isCalendarMonth(start, end) {
		return start.Format("2006-01")
	}
	return start.Format("2006-01-02") + "_" + end.AddDate(0, 0, -1).Format("2006-01-02")
}
