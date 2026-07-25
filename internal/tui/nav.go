package tui

import "slices"

// goTo navigates forward to s, remembering the current screen as its parent.
//
// The push truncates: if s is already in the chain, everything above it is
// dropped rather than appended to, so nav can never hold a duplicate and its
// depth is bounded by the number of screens. This is a structural invariant,
// not a rule anyone has to remember to apply.
func (m Model) goTo(s screen) Model {
	if i := slices.Index(m.nav, s); i >= 0 {
		m.nav = m.nav[:i]
		m.screen = s
		return m
	}
	if m.screen == s {
		return m
	}
	m.nav = append(slices.Clone(m.nav), m.screen)
	m.screen = s
	return m
}

// replace swaps the current screen without touching the parent chain: the
// loading and error screens, and the async handlers that land on a result
// screen, are the same logical step as whatever launched them.
func (m Model) replace(s screen) Model {
	m.screen = s
	return m
}

// pop returns to the parent screen. On an empty chain it is a no-op, which is
// exactly what Home needs — no special case.
func (m Model) pop() Model {
	if len(m.nav) == 0 {
		return m
	}
	m.screen = m.nav[len(m.nav)-1]
	m.nav = m.nav[:len(m.nav)-1]
	return m
}

// resetTo clears the parent chain: Home, and the setup wizard relaunched by a
// 401.
func (m Model) resetTo(s screen) Model {
	m.nav = nil
	m.screen = s
	return m
}
