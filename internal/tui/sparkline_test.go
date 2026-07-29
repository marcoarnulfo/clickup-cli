package tui

import (
	"strings"
	"testing"
)

func TestSparkline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		values   []float64
		maxCells int
		want     string
	}{
		{"empty", nil, 10, ""},
		{"all zero renders as gaps", []float64{0, 0, 0}, 10, "   "},
		{"a single value is full height", []float64{5}, 10, "█"},
		{"equal values are all full height", []float64{4, 4, 4}, 10, "███"},
		{"an idle day is a gap, not a low bar", []float64{8, 0, 8}, 10, "█ █"},
		{"the maximum is full and the minimum non-zero is lowest", []float64{1, 8}, 10, "▁█"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sparkline(tc.values, tc.maxCells); got != tc.want {
				t.Errorf("sparkline(%v, %d) = %q, want %q", tc.values, tc.maxCells, got, tc.want)
			}
		})
	}
}

// A custom range of a year is 365 days and cannot be drawn one cell per day.
// Resampling keeps the line inside the terminal.
func TestSparklineResamples(t *testing.T) {
	t.Parallel()
	values := make([]float64, 365)
	for i := range values {
		values[i] = float64(i)
	}
	got := sparkline(values, 30)
	if n := len([]rune(got)); n != 30 {
		t.Errorf("got %d cells, want 30", n)
	}
	// The series rises monotonically, so the resampled line must too.
	runes := []rune(got)
	for i := 1; i < len(runes); i++ {
		if strings.IndexRune(sparkLevels, runes[i]) < strings.IndexRune(sparkLevels, runes[i-1]) {
			t.Errorf("cell %d (%q) is lower than cell %d (%q) in a rising series: %q",
				i, runes[i], i-1, runes[i-1], got)
		}
	}
}

// Fewer values than cells must not be stretched: 5 days is 5 cells.
func TestSparklineDoesNotStretch(t *testing.T) {
	t.Parallel()
	if got := sparkline([]float64{1, 2, 3, 4, 5}, 40); len([]rune(got)) != 5 {
		t.Errorf("sparkline of 5 values = %q (%d cells), want 5 cells", got, len([]rune(got)))
	}
}
