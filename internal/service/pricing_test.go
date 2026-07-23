package service

import (
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// TestPricingFromConfigFullMapping exercises every field PricingFromConfig
// wires: default rate, per-list rates, per-member rates, (list,member)
// overrides, per-list currencies, default currency and a rounding rule. It
// asserts through report.Rates.For (the precedence chain a caller actually
// exercises), not through map internals.
func TestPricingFromConfigFullMapping(t *testing.T) {
	cfg := config.Config{
		Currency: "USD", // fallback, overridden by Billing.DefaultCurrency below
		Rate:     40,
		Rates:    map[string]float64{"list-1": 60},
		Billing: config.Billing{
			DefaultCurrency: "EUR",
			RatesByMember:   map[int]float64{7: 70},
			RateOverrides: []config.Override{
				{List: "list-1", Member: 7, Rate: 90},
			},
			Currencies: map[string]string{"list-2": "GBP"},
			Rounding: config.Rounding{
				Increment: "15m",
				Mode:      "up",
				Scope:     "day",
			},
		},
	}

	p, err := PricingFromConfig(cfg)
	if err != nil {
		t.Fatalf("PricingFromConfig: unexpected error: %v", err)
	}

	// Precedence: (list,member) > member > list > default.
	if got := p.Rates.For("list-1", 7); got != 90 {
		t.Errorf("Rates.For(list-1, 7) = %v, want 90 (list+member override)", got)
	}
	if got := p.Rates.For("list-3", 7); got != 70 {
		t.Errorf("Rates.For(list-3, 7) = %v, want 70 (per-member rate)", got)
	}
	if got := p.Rates.For("list-1", 1); got != 60 {
		t.Errorf("Rates.For(list-1, 1) = %v, want 60 (per-list rate)", got)
	}
	if got := p.Rates.For("list-9", 1); got != 40 {
		t.Errorf("Rates.For(list-9, 1) = %v, want 40 (default rate)", got)
	}

	if got := p.Currencies["list-2"]; got != "GBP" {
		t.Errorf("Currencies[list-2] = %q, want GBP", got)
	}
	if p.DefaultCurrency != "EUR" {
		t.Errorf("DefaultCurrency = %q, want EUR (Billing.DefaultCurrency wins over Currency)", p.DefaultCurrency)
	}

	if p.Rounding.Increment != 15*time.Minute {
		t.Errorf("Rounding.Increment = %v, want 15m", p.Rounding.Increment)
	}
	if p.Rounding.Mode != duration.RoundUp {
		t.Errorf("Rounding.Mode = %v, want RoundUp", p.Rounding.Mode)
	}
	if p.Rounding.Scope != report.PerDay {
		t.Errorf("Rounding.Scope = %v, want PerDay", p.Rounding.Scope)
	}
}

// TestPricingFromConfigDefaultCurrencyFallback confirms Currency is used when
// Billing.DefaultCurrency is unset.
func TestPricingFromConfigDefaultCurrencyFallback(t *testing.T) {
	cfg := config.Config{Currency: "USD", Rate: 10}
	p, err := PricingFromConfig(cfg)
	if err != nil {
		t.Fatalf("PricingFromConfig: unexpected error: %v", err)
	}
	if p.DefaultCurrency != "USD" {
		t.Errorf("DefaultCurrency = %q, want USD (fallback to top-level Currency)", p.DefaultCurrency)
	}
}

// TestPricingFromConfigEmptyRoundingIsOff pins the "not configured" case: an
// empty Increment means rounding is off (Increment == 0), with no error and
// default Mode/Scope.
func TestPricingFromConfigEmptyRoundingIsOff(t *testing.T) {
	cfg := config.Config{Rate: 10}
	p, err := PricingFromConfig(cfg)
	if err != nil {
		t.Fatalf("PricingFromConfig: unexpected error: %v", err)
	}
	if p.Rounding.Increment != 0 {
		t.Errorf("Rounding.Increment = %v, want 0 (off)", p.Rounding.Increment)
	}
	if p.Rounding.Mode != duration.RoundNearest {
		t.Errorf("Rounding.Mode = %v, want RoundNearest (default)", p.Rounding.Mode)
	}
	if p.Rounding.Scope != report.PerEntry {
		t.Errorf("Rounding.Scope = %v, want PerEntry (default)", p.Rounding.Scope)
	}
}

// TestPricingFromConfigUnparseableIncrementErrors is the M2 amendment: a
// non-empty but unparseable increment must fail loudly rather than silently
// turning rounding off, which would under-bill the client.
func TestPricingFromConfigUnparseableIncrementErrors(t *testing.T) {
	cfg := config.Config{
		Rate: 10,
		Billing: config.Billing{
			Rounding: config.Rounding{Increment: "not-a-duration"},
		},
	}
	_, err := PricingFromConfig(cfg)
	if err == nil {
		t.Fatal("PricingFromConfig: expected an error for an unparseable increment, got nil")
	}
	if !strings.Contains(err.Error(), "not-a-duration") {
		t.Errorf("error = %q, want it to name the offending value %q", err.Error(), "not-a-duration")
	}
}

func TestBudgetsFromConfig(t *testing.T) {
	cfg := config.Config{Billing: config.Billing{Budgets: map[string]float64{"list-1": 500}}}
	got := BudgetsFromConfig(cfg)
	if got["list-1"] != 500 {
		t.Errorf("BudgetsFromConfig()[list-1] = %v, want 500", got["list-1"])
	}
}

func TestLoadLocationEmptyNameReturnsFallback(t *testing.T) {
	fallback := time.UTC
	loc, err := LoadLocation("", fallback)
	if err != nil {
		t.Fatalf("LoadLocation: unexpected error: %v", err)
	}
	if loc != fallback {
		t.Errorf("LoadLocation(\"\", fallback) = %v, want the fallback pointer", loc)
	}
}

func TestLoadLocationValidName(t *testing.T) {
	loc, err := LoadLocation("America/New_York", time.UTC)
	if err != nil {
		t.Fatalf("LoadLocation: unexpected error: %v", err)
	}
	if loc.String() != "America/New_York" {
		t.Errorf("LoadLocation name = %q, want America/New_York", loc.String())
	}
}

func TestLoadLocationInvalidNameErrors(t *testing.T) {
	_, err := LoadLocation("Not/A_Real_Zone", time.UTC)
	if err == nil {
		t.Fatal("LoadLocation: expected an error for an invalid zone name, got nil")
	}
	if !strings.Contains(err.Error(), "Not/A_Real_Zone") {
		t.Errorf("error = %q, want it to name the offending value", err.Error())
	}
}
