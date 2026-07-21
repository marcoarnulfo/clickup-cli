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

// midnightUTC returns the UTC midnight of t's date.
func midnightUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// RangeForPreset returns the half-open interval [start, end) for a non-custom
// preset. this_month uses the given year/month; the relative presets use now.
// An unknown preset falls back to this_month.
func RangeForPreset(preset string, year int, month time.Month, now time.Time) (start, end time.Time) {
	switch preset {
	case PresetLastMonth:
		m := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		return m, m.AddDate(0, 1, 0)
	case PresetLast7d:
		e := midnightUTC(now).AddDate(0, 0, 1)
		return e.AddDate(0, 0, -7), e
	case PresetLast30d:
		e := midnightUTC(now).AddDate(0, 0, 1)
		return e.AddDate(0, 0, -30), e
	case PresetThisWeek:
		d := midnightUTC(now)
		wd := int(d.Weekday()) // Sunday=0..Saturday=6
		if wd == 0 {
			wd = 7 // treat Sunday as day 7 (ISO week ends Sunday)
		}
		mon := d.AddDate(0, 0, -(wd - 1))
		return mon, mon.AddDate(0, 0, 7)
	default: // PresetThisMonth and unknown
		return MonthRange(year, month)
	}
}

// isCalendarMonth reports whether [start, end) is exactly one calendar month.
func isCalendarMonth(start, end time.Time) bool {
	start = start.UTC()
	if start.Day() != 1 || start.Hour() != 0 || start.Minute() != 0 ||
		start.Second() != 0 || start.Nanosecond() != 0 {
		return false
	}
	return end.Equal(start.AddDate(0, 1, 0))
}

// PeriodLabel renders [start, end) as a calendar month ("January 2006") when it
// is exactly one, else "2006-01-02 → 2006-01-02" with the second date inclusive.
func PeriodLabel(start, end time.Time) string {
	if isCalendarMonth(start, end) {
		return start.UTC().Format("January 2006")
	}
	return start.UTC().Format("2006-01-02") + " → " + end.AddDate(0, 0, -1).UTC().Format("2006-01-02")
}

// PeriodFileSlug renders the period for an export filename: "2006-01" for a
// calendar month, else "2006-01-02_2006-01-02" (second date inclusive).
func PeriodFileSlug(start, end time.Time) string {
	if isCalendarMonth(start, end) {
		return start.UTC().Format("2006-01")
	}
	return start.UTC().Format("2006-01-02") + "_" + end.AddDate(0, 0, -1).UTC().Format("2006-01-02")
}
