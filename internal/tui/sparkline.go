package tui

import "math"

// sparkLevels are the eight bar heights, lowest first. Index 0 is the lowest
// NON-ZERO level: an exact zero renders as a space instead, because a day with
// no work must read as a gap and ▁ would read as a little work.
const sparkLevels = "▁▂▃▄▅▆▇█"

// sparkline renders values as block glyphs, one cell per value, resampled to
// at most maxCells when there are more values than cells.
//
// Heights are relative to the largest value in the series, so the shape shows
// the period's rhythm rather than an absolute scale.
func sparkline(values []float64, maxCells int) string {
	if len(values) == 0 {
		return ""
	}
	if maxCells > 0 && len(values) > maxCells {
		values = resample(values, maxCells)
	}

	peak := 0.0
	for _, v := range values {
		peak = max(peak, v)
	}

	levels := []rune(sparkLevels)
	out := make([]rune, 0, len(values))
	for _, v := range values {
		if v <= 0 || peak == 0 {
			out = append(out, ' ')
			continue
		}
		lvl := int(math.Ceil(v / peak * float64(len(levels))))
		out = append(out, levels[min(max(lvl, 1), len(levels))-1])
	}
	return string(out)
}

// resample averages values into k contiguous buckets. Bucket i covers
// values[i*n/k : (i+1)*n/k], which distributes the remainder deterministically
// and leaves no bucket empty as long as k <= n.
func resample(values []float64, k int) []float64 {
	n := len(values)
	out := make([]float64, k)
	for i := range out {
		lo, hi := i*n/k, (i+1)*n/k
		if hi <= lo {
			hi = lo + 1
		}
		sum := 0.0
		for _, v := range values[lo:hi] {
			sum += v
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}
