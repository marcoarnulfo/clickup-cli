// Package fuzzy scores a query against a candidate string the way a command
// palette needs it: case-insensitive subsequence matching that also reports
// which runes matched, so the caller can highlight them.
//
// The package is pure — no I/O, nothing outside the standard library — for the
// same reason internal/report and internal/duration are: the interesting part
// is an algorithm, and an algorithm is worth testing without a Model.
package fuzzy

import (
	"math"
	"unicode"
)

// Scoring weights. See the spec's section 7.2.
const (
	// consecutiveBonus rewards a rune that immediately follows the previous match.
	consecutiveBonus = 10
	// boundaryBonus rewards a match that starts a word.
	boundaryBonus = 8
	// maxLeadPenalty caps "how far into the target the match starts". Without a
	// cap, a long label loses to a worse match that merely starts further left.
	maxLeadPenalty = 10
)

// noMatch marks an unreachable cell of the table. It is halved so that adding a
// bonus to it can never wrap around into a plausible score.
const noMatch = math.MinInt / 2

// Match reports whether query matches target as a case-insensitive
// subsequence. When ok, score ranks the match and idx holds the rune indices of
// target that were matched, in ascending order, one per query rune.
//
// An empty query always matches, with score 0 and a nil idx.
//
// The search is exhaustive rather than greedy because idx is not only a ranking
// input: it decides which runes the caller highlights. Greedy matching of "rt"
// against "report table" yields [0, 5] — repor[t] — where the right answer is
// [0, 7], [r]eport [t]able.
func Match(query, target string) (score int, idx []int, ok bool) {
	q := lowerRunes(query)
	if len(q) == 0 {
		return 0, nil, true
	}
	t := lowerRunes(target)
	if len(q) > len(t) {
		return 0, nil, false
	}

	// best[j][i] is the highest score for matching q[:j+1] with q[j] landing on
	// t[i]. from[j][i] is the target index q[j-1] landed on along that path,
	// which is what lets idx be reconstructed at the end.
	best := make([][]int, len(q))
	from := make([][]int, len(q))
	for j := range best {
		best[j] = make([]int, len(t))
		from[j] = make([]int, len(t))
		for i := range best[j] {
			best[j][i] = noMatch
			from[j][i] = -1
		}
	}

	for i := range t {
		if t[i] != q[0] {
			continue
		}
		best[0][i] = boundaryScore(t, i) - min(i, maxLeadPenalty)
	}
	for j := 1; j < len(q); j++ {
		for i := j; i < len(t); i++ {
			if t[i] != q[j] {
				continue
			}
			for p := j - 1; p < i; p++ {
				if best[j-1][p] == noMatch {
					continue
				}
				s := best[j-1][p] + boundaryScore(t, i)
				if p == i-1 {
					s += consecutiveBonus
				}
				// Strictly greater keeps the leftmost path on a tie, which is
				// what makes the result deterministic.
				if s > best[j][i] {
					best[j][i], from[j][i] = s, p
				}
			}
		}
	}

	last := len(q) - 1
	end := -1
	score = noMatch
	for i := range t {
		if best[last][i] > score {
			end, score = i, best[last][i]
		}
	}
	if end < 0 {
		return 0, nil, false
	}
	idx = make([]int, len(q))
	for j, i := last, end; j >= 0; j-- {
		idx[j] = i
		i = from[j][i]
	}
	return score, idx, true
}

// boundaryScore is the word-start bonus for t[i]: index 0, or a rune that
// follows a separator.
func boundaryScore(t []rune, i int) int {
	if i == 0 {
		return boundaryBonus
	}
	switch t[i-1] {
	case ' ', '-', '_', '/', '.':
		return boundaryBonus
	}
	return 0
}

// lowerRunes decomposes s into lower-cased runes. Indices into the result are
// rune indices, which is what Match reports and what a caller highlighting
// characters needs — byte offsets would be wrong for any accented label.
func lowerRunes(s string) []rune {
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i])
	}
	return r
}
