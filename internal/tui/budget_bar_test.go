package tui

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// budgetBarFixtures pairs a percentage with its expected full-block count,
// HAND-MEASURED against the current (correct) renderBudgetBar and written down
// as literals — not recomputed here from budgetBarWidth/100 arithmetic. A
// wantFull expression built from the same formula as the implementation would
// share any bug in that formula and could never catch one: it would drift in
// lockstep with budget.go and still pass. Measured with:
//
//	go test ./internal/tui -run TestZZMeasureBarFixtures -v
//
// (a throwaway test that logged renderBudgetBar's output and its full-block
// count for each percentage below; the numbers here are copied from that log,
// not derived).
//
// Covers: zero (0 blocks), the two tick points either side of the first block
// (4.9/5.0) and of the twentieth (94.9/95.0), the near-100 cluster that used
// to disagree with its label (97.6/99.4/99.5/99.6/99.9, all 19 of 20 — the
// bug this task fixes), exact 100 (20, full), and a value past 100 (104.7,
// still 20 full — the fill clamps but the label must not).
var budgetBarFixtures = []struct {
	pct      float64
	wantFull int
}{
	{0, 0},
	{4.9, 0},
	{5.0, 1},
	{9.9, 1},
	{10.0, 2},
	{50, 10},
	{94.9, 18},
	{95.0, 19},
	{97.6, 19},
	{99.4, 19},
	{99.5, 19},
	{99.6, 19},
	{99.9, 19},
	{100, 20},
	{104.7, 20},
}

// The label rounded while the fill truncated, so from 99.5% up the label read
// "100%" over 19 of 20 blocks. Flooring the LABEL is the fix, not rounding the
// fill: a rounded fill would show a full bar from 97.5%, and a full bar means
// the budget is spent. The label stays unclamped above 100 — that is why this
// screen does not use bubbles/progress.
func TestBudgetBarLabelAgreesWithTheFill(t *testing.T) {
	th := testTheme(true)
	for _, tc := range budgetBarFixtures {
		bar := renderBudgetBar(th, tc.pct, 0)
		full := strings.Count(bar, string(gaugeFull))
		if full != tc.wantFull {
			t.Errorf("pct=%.1f: %d full blocks, want %d (%q)", tc.pct, full, tc.wantFull, bar)
		}
		// The label must state the floored percentage, so it can never claim a
		// milestone the bar has not reached.
		wantLabel := fmt.Sprintf("%.0f%%", math.Floor(tc.pct))
		if !strings.HasSuffix(bar, " "+wantLabel) {
			t.Errorf("pct=%.1f: %q does not end in %q", tc.pct, bar, wantLabel)
		}
	}
}
