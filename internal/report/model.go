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
	ID        string
	TaskID    string
	TaskName  string
	ListID    string
	ListName  string // the "project"
	UserID    int
	UserName  string
	Start     time.Time
	Duration  time.Duration
	Tags      []string
	EntryTags []string // the entry's own time-tracking tags (distinct from Tags = task tags)
	Status    string
	Billable  bool
	// Description is the ClickUp time entry's free-text note. Carried through
	// (not just aggregated over) so an edit (#94) can prefill and preserve it —
	// without this, resubmitting a blank note would wipe the entry's ClickUp
	// description on every edit.
	Description string
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
// is populated ONLY when exactly one currency carries a non-zero amount (other
// currencies may still appear with non-billable hours only), otherwise it stays
// 0 — amounts in different currencies are never summed (no FX).
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
	TotalAmount      float64 `json:"total_amount"`       // 0 unless one currency carries money
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

// CurrencyFor resolves the currency billed for a list: its per-list override,
// or DefaultCurrency when the list has none (an empty mapping counts as none).
// It is the single currency resolver: the aggregation, BudgetLines and the TUI
// all go through it, so they can never disagree on a list's currency.
func (p Pricing) CurrencyFor(listID string) string {
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
	// Hours is the unit's billed (post-rounding) hours at 6 decimals — more
	// precision than the 2-decimal aggregates elsewhere, so that the row
	// reconciles to its own amount at cent precision:
	// round2(Hours × Rate) == Amount for any rate up to 1000/h (see Build).
	// Exporters must render it at this precision and must not re-round it to 2
	// decimals, which would break the row's arithmetic.
	Hours float64 `json:"hours"`
	Rate  float64 `json:"rate"`
	// Amount is round2(exact billed hours × Rate) — the money base is the exact
	// billed duration, not the 6-decimal Hours above.
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	// Billable is always true: only billable entries become billing units. It is
	// kept for schema stability (exports and scripts already read the column),
	// not as a live discriminator.
	Billable bool `json:"billable"`
}
