// Every function here dissolves an open overlay when it changes the screen:
// an overlay belongs to the screen it was raised over, so whatever swaps that
// screen out — a key handler, an async error, a 401 relaunching the setup
// wizard — takes the overlay with it. goTo, pop and resetTo always change (or
// re-affirm) the screen, so each calls closePalette() unconditionally, on
// every path, including their own no-op branches: a branch that silently kept
// the overlay open would be a latent trap for the next handler built on top of
// it. replace is the one exception, and deliberately so: several async
// handlers call replace(<the screen they are already on>) — statusesMsg on
// screenFilters, membersMsg on screenMembers, entriesReloadedMsg/
// entriesErrMsg on screenEntries, logDoneMsg/timerMsg on screenLog — and that
// is not a screen swap, so replace only dissolves the overlay when s !=
// m.screen. Otherwise an in-flight fetch of that kind would silently wipe a
// query the user is still typing into a palette opened while it was running.
// The palette's own actions are not an exception to any of this: they run
// after closePalette(), so the clearing here is a no-op for them.

package tui

import "slices"

// goTo navigates forward to s, remembering the current screen as its parent.
//
// The push truncates: if s is already in the chain, everything above it is
// dropped rather than appended to, so nav can never hold a duplicate and its
// depth is bounded by the number of screens. This is a structural invariant,
// not a rule anyone has to remember to apply.
func (m Model) goTo(s screen) Model {
	m = m.closePalette()
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
	if s != m.screen {
		m = m.closePalette()
	}
	m.screen = s
	return m
}

// pop returns to the parent screen. On an empty chain it is a no-op, which is
// exactly what Home needs — no special case.
func (m Model) pop() Model {
	m = m.closePalette()
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
	m = m.closePalette()
	m.nav = nil
	m.screen = s
	return m
}
