// Package report contains the domain model and the hour-aggregation logic.
// It is pure: no I/O, no external dependencies (it may depend on internal/duration,
// itself pure).
package report

import (
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/duration"
)

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
	Tags     []string
	Status   string
	Billable bool
}

// Bucket is an aggregated row of the report. The JSON tags serve the export
// (the package stays pure: the tags don't introduce dependencies).
type Bucket struct {
	Label         string           `json:"label"`
	Key           string           `json:"key"`            // stable ID (listID/taskID/userID/day/tag)
	Hours         float64          `json:"hours"`          // all hours (billable+non), raw
	BillableHours float64          `json:"billable_hours"` // raw, billable only
	BilledHours   float64          `json:"billed_hours"`   // billable after rounding
	Amounts       []CurrencyAmount `json:"amounts"`        // per-currency subtotals inside the bucket
	Amount        float64          `json:"amount"`         // Deprecated: single-currency amount, removed in the Build rewrite
}

// Report is the aggregated result ready for presentation/export.
type Report struct {
	Start       time.Time // period [Start, End)
	End         time.Time
	Scope       string // "me" | "team"
	GroupBy     string
	Currency    string
	Rate        float64
	Buckets     []Bucket
	TotalHours  float64
	TotalAmount float64
}

// ListMember identifies a (list, member) pair, the most specific rate override.
type ListMember struct {
	ListID string
	UserID int
}

// Rates resolves the hourly rate for an entry, applying the precedence
// (list,member) > member > list > default. The zero value is legitimate: all
// lookups fall through nil maps to Default (0).
type Rates struct {
	Default      float64
	ByList       map[string]float64     // listID -> rate
	ByMember     map[int]float64        // userID -> rate
	ByListMember map[ListMember]float64 // (listID,userID) -> rate
}

// For applies the precedence (list,member) > member > list > default. It never
// panics on nil maps: a lookup on a nil Go map returns the zero value with
// ok == false, so a zero-value Rates simply resolves to Default (0) everywhere.
func (r Rates) For(listID string, userID int) float64 {
	if v, ok := r.ByListMember[ListMember{listID, userID}]; ok {
		return v
	}
	if v, ok := r.ByMember[userID]; ok {
		return v
	}
	if v, ok := r.ByList[listID]; ok {
		return v
	}
	return r.Default
}

// RoundScope selects the granularity at which billable hours are rounded.
type RoundScope int

const (
	PerEntry RoundScope = iota
	PerDay
)

// RoundRule configures how billable hours are rounded before invoicing.
type RoundRule struct {
	Increment time.Duration // 0 => rounding off (default)
	Mode      duration.RoundMode
	Scope     RoundScope
}

// Pricing bundles the rate table, per-list currencies and rounding rule that
// together determine how a report is billed.
type Pricing struct {
	Rates           Rates
	Currencies      map[string]string // listID -> ISO currency (e.g. "EUR")
	DefaultCurrency string            // fallback when the list is not in Currencies
	Rounding        RoundRule
}

// CurrencyAmount is an amount expressed in a given currency.
type CurrencyAmount struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

// CurrencySubtotal is a per-currency rollup of hours and amount for a report.
type CurrencySubtotal struct {
	Currency      string  `json:"currency"`
	Hours         float64 `json:"hours"`
	BillableHours float64 `json:"billable_hours"`
	BilledHours   float64 `json:"billed_hours"`
	Amount        float64 `json:"amount"`
}

// InvoiceLine is one billing unit (see the money-ledger rule). Emitted by Build in
// Task 3b; defined here so later tasks share one definition.
type InvoiceLine struct {
	Date        string  `json:"date"` // the unit's day in loc, "2006-01-02"
	ListID      string  `json:"list_id"`
	ListName    string  `json:"list_name"`
	UserID      int     `json:"user_id"`
	UserName    string  `json:"user_name"`
	Description string  `json:"description"`
	Hours       float64 `json:"hours"`
	Rate        float64 `json:"rate"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Billable    bool    `json:"billable"`
}
