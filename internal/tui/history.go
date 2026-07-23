package tui

import (
	"fmt"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

// entriesHistoryView renders the read-only change history opened with 'h'
// from the entries browser: one line per recorded change, chronological
// (oldest first, as returned by clickup.TimeEntryHistory). Read-only — no
// selection, no ownership gating, just Esc back to the list.
func entriesHistoryView(es entriesModel, loc *time.Location) string {
	b := styleTitle.Render("Entry history") + "\n\n"
	if len(es.historyChanges) == 0 {
		b += styleHelp.Render("No recorded changes for this entry.") + "\n"
	} else {
		for _, c := range es.historyChanges {
			b += historyLine(c, loc) + "\n"
		}
	}
	b += "\n" + styleHelp.Render("Esc: back to entries")
	return b
}

// historyLine formats one HistoryChange as "<when>  <field>: <before> -> <after>  (<user>)".
func historyLine(c clickup.HistoryChange, loc *time.Location) string {
	when := "unknown time"
	if !c.Date.IsZero() {
		when = c.Date.In(locOr(loc)).Format("02 Jan 15:04")
	}
	who := c.User
	if who == "" {
		who = "unknown user"
	}
	return fmt.Sprintf("%s  %s: %q -> %q  (%s)", when, styleAccent.Render(c.Field), c.Before, c.After, who)
}
