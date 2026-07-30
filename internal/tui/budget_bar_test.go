package tui

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// The label rounded while the fill truncated, so from 99.5% up the label read
// "100%" over 19 of 20 blocks. Flooring the LABEL is the fix, not rounding the
// fill: a rounded fill would show a full bar from 97.5%, and a full bar means
// the budget is spent. The label stays unclamped above 100 — that is why this
// screen does not use bubbles/progress.
func TestBudgetBarLabelAgreesWithTheFill(t *testing.T) {
	th := testTheme(true)
	for _, pct := range []float64{0, 50, 97.6, 99.4, 99.5, 99.6, 99.9, 100, 104.7} {
		bar := renderBudgetBar(th, pct)
		full := strings.Count(bar, string(gaugeFull))
		wantFull := int(min(max(pct, 0), 100) / 100 * budgetBarWidth)
		if full != wantFull {
			t.Errorf("pct=%.1f: %d full blocks, want %d (%q)", pct, full, wantFull, bar)
		}
		// The label must state the floored percentage, so it can never claim a
		// milestone the bar has not reached.
		wantLabel := fmt.Sprintf("%.0f%%", math.Floor(pct))
		if !strings.HasSuffix(bar, " "+wantLabel) {
			t.Errorf("pct=%.1f: %q does not end in %q", pct, bar, wantLabel)
		}
	}
}
