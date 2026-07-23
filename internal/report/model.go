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
}

// Report is the aggregated result ready for presentation/export.
//
// Money: CurrencySubtotals (and Lines) are the authoritative totals. TotalAmount
// is populated ONLY when the report is single-currency, otherwise it stays 0 —
// amounts in different currencies are never summed (no FX).
type Report struct {
	Start   time.Time `json:"start"` // period [Start, End)
	End     time.Time `json:"end"`
	Scope   string    `json:"scope"` // "me" | "team"
	GroupBy string    `json:"group_by"`

	// Timezone is loc.String(): "UTC", an IANA name, or "Local".
	Timezone string `json:"timezone"`
	// DefaultCurrency / DefaultRate mirror the Pricing used to build the report.
	// They exist so presentation layers (and the CLI's deprecated JSON "currency"
	// and "rate" keys) keep working now that per-list currencies and rates are
	// the real model.
	DefaultCurrency string  `json:"default_currency"`
	DefaultRate     float64 `json:"default_rate"`

	Buckets           []Bucket           `json:"buckets"`
	Lines             []InvoiceLine      `json:"lines"`
	CurrencySubtotals []CurrencySubtotal `json:"currency_subtotals"`

	TotalHours       float64 `json:"total_hours"`        // all entries, raw
	BillableHours    float64 `json:"billable_hours"`     // billable entries, raw
	NonBillableHours float64 `json:"non_billable_hours"` // never rounded
	BilledHours      float64 `json:"billed_hours"`       // billable after rounding
	TotalAmount      float64 `json:"total_amount"`       // 0 unless single-currency
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

// currencyFor resolves the currency billed for a list: its per-list override, or
// DefaultCurrency when the list has none.
func (p Pricing) currencyFor(listID string) string {
	if c, ok := p.Currencies[listID]; ok && c != "" {
		return c
	}
	return p.DefaultCurrency
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

// InvoiceLine is one billing unit (see the money-ledger rule on Build). Emitted
// by Build; defined here so later tasks share one definition.
type InvoiceLine struct {
	Date        string `json:"date"` // the unit's day in loc, "2006-01-02"
	ListID      string `json:"list_id"`
	ListName    string `json:"list_name"`
	UserID      int    `json:"user_id"`
	UserName    string `json:"user_name"`
	Description string `json:"description"`
	// Hours is the unit's billed (post-rounding) hours at 4 decimals — more
	// precision than the 2-decimal aggregates elsewhere, so that the row
	// reconciles to its own amount at cent precision:
	// round2(Hours × Rate) == Amount for rates up to about 120/h (see Build).
	// Exporters must render it with 4 decimals, not 2.
	Hours float64 `json:"hours"`
	Rate  float64 `json:"rate"`
	// Amount is round2(exact billed hours × Rate) — the money base is the exact
	// billed duration, not the 4-decimal Hours above.
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Billable bool    `json:"billable"`
}
