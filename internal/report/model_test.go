package report

import "testing"

func TestRatesForPrecedence(t *testing.T) {
	r := Rates{
		Default:      50,
		ByList:       map[string]float64{"L": 60},
		ByMember:     map[int]float64{7: 70},
		ByListMember: map[ListMember]float64{{"L", 7}: 90},
	}
	if got := r.For("L", 7); got != 90 { // most specific: (list,member)
		t.Errorf("list+member: %v", got)
	}
	if got := r.For("L", 9); got != 60 { // list only
		t.Errorf("list: %v", got)
	}
	if got := r.For("X", 7); got != 70 { // member only
		t.Errorf("member: %v", got)
	}
	if got := r.For("X", 9); got != 50 { // default
		t.Errorf("default: %v", got)
	}
}

func TestRatesForZeroValueNilMapsDoesNotPanic(t *testing.T) {
	var r Rates // all maps nil, Default zero
	if got := r.For("L", 7); got != 0 {
		t.Errorf("zero-value Rates should resolve to Default 0, got %v", got)
	}

	r2 := Rates{Default: 42} // maps nil, Default set
	if got := r2.For("anything", 123); got != 42 {
		t.Errorf("nil maps should fall through to Default 42, got %v", got)
	}
}

// TestCurrencyForExported pins the single exported currency resolver shared by
// the pricing path, the budget lines and the TUI: a per-list override wins, an
// empty or missing mapping falls back to DefaultCurrency.
func TestCurrencyForExported(t *testing.T) {
	p := Pricing{Currencies: map[string]string{"A": "USD", "B": ""}, DefaultCurrency: "EUR"}
	for listID, want := range map[string]string{"A": "USD", "B": "EUR", "Z": "EUR"} {
		if got := p.CurrencyFor(listID); got != want {
			t.Errorf("CurrencyFor(%q) = %q, want %q", listID, got, want)
		}
	}
}
