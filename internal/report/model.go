// Package report contains the domain model and the hour-aggregation logic.
// It is pure: no I/O, no external dependencies.
package report

import "time"

// TimeEntry is a single time entry normalized from the ClickUp domain.
type TimeEntry struct {
	ID       string
	TaskID   string
	TaskName string
	ListID   string
	ListName string // the "project"
	UserID   int
	UserName string
	Start    time.Time
	Duration time.Duration
}

// Bucket is an aggregated row of the report. The JSON tags serve the export
// (the package stays pure: the tags don't introduce dependencies).
type Bucket struct {
	Label  string  `json:"label"`
	Hours  float64 `json:"hours"`
	Amount float64 `json:"amount"`
}

// Report is the aggregated result ready for presentation/export.
type Report struct {
	Year        int
	Month       time.Month
	Scope       string // "me" | "team"
	GroupBy     string
	Currency    string
	Rate        float64
	Buckets     []Bucket
	TotalHours  float64
	TotalAmount float64
}

// Rates resolves the hourly rate per list, falling back to the default rate.
type Rates struct {
	Default float64
	ByList  map[string]float64
}

// For returns the list's rate if present, otherwise the default rate.
func (r Rates) For(listID string) float64 {
	if v, ok := r.ByList[listID]; ok {
		return v
	}
	return r.Default
}
