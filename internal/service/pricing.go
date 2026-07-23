package service

import (
	"fmt"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// PricingFromConfig builds a report.Pricing from the persisted config: the
// rate precedence table (default / per-list / per-member / per-list-member),
// per-list currencies (falling back to the top-level Currency when
// Billing.DefaultCurrency is unset) and the invoicing rounding rule. It is
// the single source of truth both front ends (TUI and CLI) build reports
// from — see #57.
//
// An empty Billing.Rounding.Increment means rounding is off (Increment: 0),
// which is not an error. A non-empty but unparseable Increment IS an error:
// silently falling back to "off" would round hours away instead of up/down
// as configured, which over- or under-bills the client — exactly the
// failure a billing tool must not have.
func PricingFromConfig(cfg config.Config) (report.Pricing, error) {
	rounding, err := roundRuleFromConfig(cfg.Billing.Rounding)
	if err != nil {
		return report.Pricing{}, err
	}

	currency := cfg.Billing.DefaultCurrency
	if currency == "" {
		currency = cfg.Currency
	}

	return report.Pricing{
		Rates: report.Rates{
			Default:      cfg.Rate,
			ByList:       cfg.Rates,
			ByMember:     cfg.Billing.RatesByMember,
			ByListMember: listMemberRatesFromOverrides(cfg.Billing.RateOverrides),
		},
		Currencies:      cfg.Billing.Currencies,
		DefaultCurrency: currency,
		Rounding:        rounding,
	}, nil
}

// listMemberRatesFromOverrides converts the config's flat (list, member,
// rate) override list into the map report.Rates.ByListMember expects. An
// empty input returns a nil map (Rates.For handles nil maps fine).
func listMemberRatesFromOverrides(overrides []config.Override) map[report.ListMember]float64 {
	if len(overrides) == 0 {
		return nil
	}
	m := make(map[report.ListMember]float64, len(overrides))
	for _, o := range overrides {
		m[report.ListMember{ListID: o.List, UserID: o.Member}] = o.Rate
	}
	return m
}

// roundRuleFromConfig parses the config's string-based rounding rule into
// report.RoundRule. An empty Increment leaves rounding off (Increment stays
// 0); any other value that duration.Parse rejects is a hard error naming the
// offending string, per the M2 amendment.
func roundRuleFromConfig(r config.Rounding) (report.RoundRule, error) {
	var rule report.RoundRule
	if r.Increment != "" {
		d, err := duration.Parse(r.Increment)
		if err != nil {
			return report.RoundRule{}, fmt.Errorf("billing.rounding.increment %q: %w", r.Increment, err)
		}
		rule.Increment = d
	}
	if r.Mode == "up" {
		rule.Mode = duration.RoundUp
	} else {
		rule.Mode = duration.RoundNearest
	}
	if r.Scope == "day" {
		rule.Scope = report.PerDay
	} else {
		rule.Scope = report.PerEntry
	}
	return rule, nil
}

// BudgetsFromConfig returns the per-list budget amounts (listID -> amount)
// configured for burn-down tracking.
func BudgetsFromConfig(cfg config.Config) map[string]float64 {
	return cfg.Billing.Budgets
}

// LoadLocation resolves the named IANA timezone, returning fallback unchanged
// when name is empty — that emptiness is how "not configured" is expressed,
// letting each front end supply its own default (the CLI passes time.UTC,
// the TUI passes time.Local or the configured timezone).
func LoadLocation(name string, fallback *time.Location) (*time.Location, error) {
	if name == "" {
		return fallback, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return loc, nil
}
