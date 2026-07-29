// Every function here dissolves an open overlay. An overlay belongs to the
// screen it was raised over, so whatever swaps that screen out — a key handler,
// an async error, a 401 relaunching the setup wizard — takes the overlay with
// it. The palette's own actions are not an exception: they run after
// closePalette(), so the clearing here is a no-op for them.
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
		m.overlay = overlayNone
		m.palette = paletteModel{}
		return m
	}
	if m.screen == s {
		m.overlay = overlayNone
		m.palette = paletteModel{}
		return m
	}
	m.nav = append(slices.Clone(m.nav), m.screen)
	m.screen = s
	m.overlay = overlayNone
	m.palette = paletteModel{}
	return m
}

// replace swaps the current screen without touching the parent chain: the
// loading and error screens, and the async handlers that land on a result
// screen, are the same logical step as whatever launched them.
func (m Model) replace(s screen) Model {
	m.screen = s
	m.overlay = overlayNone
	m.palette = paletteModel{}
	return m
}

// pop returns to the parent screen. On an empty chain it is a no-op, which is
// exactly what Home needs — no special case.
func (m Model) pop() Model {
	if len(m.nav) == 0 {
		m.overlay = overlayNone
		m.palette = paletteModel{}
		return m
	}
	m.screen = m.nav[len(m.nav)-1]
	m.nav = m.nav[:len(m.nav)-1]
	m.overlay = overlayNone
	m.palette = paletteModel{}
	return m
}

// resetTo clears the parent chain: Home, and the setup wizard relaunched by a
// 401.
func (m Model) resetTo(s screen) Model {
	m.nav = nil
	m.screen = s
	m.overlay = overlayNone
	m.palette = paletteModel{}
	return m
}
