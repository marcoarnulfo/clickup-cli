package tui

import tea "github.com/charmbracelet/bubbletea"

// wheelKey translates a wheel notch into the arrow key it stands for, and
// reports false for every other mouse event.
//
// Going through a synthetic KeyMsg is what makes the wheel work on every
// screen without touching any of them: the message is replayed through the
// same routing a real key takes, so each screen's existing cursor handling —
// and the tests that cover it — apply unchanged. actions.go does the same for
// the command palette.
//
// It deliberately does not go through keyMsgFor: that function's key set is
// closed on purpose for the palette's benefit (see its doc comment), and
// widening it for a different caller would betray that.
//
// There is no check on msg.Action, and it would be dead code: measured against
// bubbletea v1.3.10, a wheel notch always arrives as a single press. The SGR
// parser excludes IsWheel() from the release branch (mouse.go:186-191, with
// the comment "Wheel buttons don't have release events"), and the X10 parser
// assigns a release only in the ordinary-button branch and never applies the
// motion bit to a wheel event (mouse.go:249-257).
//
// Horizontal wheel and the side buttons are dropped: nothing in the TUI scrolls
// sideways, and a key that means nothing is worse than no key at all.
func wheelKey(msg tea.MouseMsg) (tea.KeyMsg, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return tea.KeyMsg{Type: tea.KeyUp}, true
	case tea.MouseButtonWheelDown:
		return tea.KeyMsg{Type: tea.KeyDown}, true
	}
	return tea.KeyMsg{}, false
}

// anyKeyIsAnAnswer reports whether the active context treats every keystroke as
// an answer to a question rather than as a movement. A wheel notch is not an
// answer, so it is dropped there instead of being replayed.
//
// Measured, these are the two contexts where replaying a notch does damage: on
// screenError any key resets to Home (app.go), so a stray notch wipes an error
// message before it has been read; in entriesConfirmDelete any key but "y"
// cancels (entries.go), so a notch dismisses the confirmation. They are the
// same two places where the app already had to leave "?" unassigned.
//
// The name states the property rather than the mechanism on purpose: it gives
// the list an admission rule — a context belongs here if and only if every key
// is an answer in it — instead of leaving it to rot as a list of special cases.
func (m Model) anyKeyIsAnAnswer() bool {
	return m.screen == screenError ||
		(m.screen == screenEntries && m.entriesScreen.mode == entriesConfirmDelete)
}
