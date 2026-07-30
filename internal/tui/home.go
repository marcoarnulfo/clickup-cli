package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// homeModel is mostly stateless: month/year/scope live on the root Model
// (single source of truth); the view receives them as arguments. errText
// holds the message from a retryableErrMsg routed back to Home (#38), shown
// inline until the next load attempt clears it.
type homeModel struct {
	errText string
}

func newHome() homeModel { return homeModel{} }

// periodMode values (#4): periodModeMonth (the zero value) follows the active
// preset/month-year as before; periodModeWeek overrides it with the current
// ISO week (see Model.currentRange).
const (
	periodModeMonth = ""
	periodModeWeek  = "week"
)

func (m Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.PrevMonth):
		// The this_month/week-mode gate lives entirely in homeKeys' Enabled()
		// now: PrevMonth only matches when it holds, so there is no guard to
		// repeat here (see TestHomeMonthNavKeysMatchGuard).
		m.month--
		if m.month < time.January {
			m.month = time.December
			m.year--
		}
	case key.Matches(msg, k.NextMonth):
		m.month++
		if m.month > time.December {
			m.month = time.January
			m.year++
		}
	case key.Matches(msg, k.ToggleWeek):
		if m.periodMode == periodModeWeek {
			m.periodMode = periodModeMonth
		} else {
			m.periodMode = periodModeWeek
		}
	case key.Matches(msg, k.Range):
		m = m.openRange()
		return m, nil
	case key.Matches(msg, k.ToggleScope):
		if m.scope == "me" {
			m.scope = "team"
		} else {
			m.scope = "me"
		}
	case key.Matches(msg, k.LogHours):
		m = m.openLog()
	case key.Matches(msg, k.Timer):
		// The running-timer gate also lives entirely in homeKeys' Enabled()
		// now (see TestHomeTimerKeyMatchesGuard).
		m.logScreen = newLog(m.entries, m.cfg)
		m.logScreen.timer = m.runningTimer
		m.logScreen.step = logTimerRunning
		m = m.goTo(screenLog)
	case key.Matches(msg, k.Members):
		mm, cmd := m.openMembers(screenHome)
		return mm, cmd
	case key.Matches(msg, k.Generate):
		m.home.errText = "" // clear any previous inline error before retrying
		m = m.replace(screenLoading)
		// loadEntriesCmd derives the team assignees on its own when scope=="team".
		return m, m.reloadEntriesCmd(screenHome)
	}
	return m, nil
}

// homeMembersNote returns "Members: k/n" for the team scope when members are
// known, else "". An empty selection counts as all (k = n).
func (m Model) homeMembersNote() string {
	if m.scope != "team" || len(m.teamMembers) == 0 {
		return ""
	}
	k := len(m.selectedAssignees())
	if k == 0 {
		k = len(m.teamMembers) // empty selection = all
	}
	return fmt.Sprintf("Members: %d/%d", k, len(m.teamMembers))
}

// rangeLabel returns a short label for the active range shown on Home.
//
// Uses currentRange(), not activeRange(): Home changes month, week mode and
// preset without reloading, so a pinned label would freeze mid-navigation (#28).
func (m Model) rangeLabel() string {
	start, end := m.currentRange()
	return report.PeriodLabel(start, end)
}

func (m homeModel) view(th theme, rangeLabel, scope, membersNote, latestVersion, timerLine string) string {
	title := th.Title.Render("ClickUp Hours — Report")
	scopeStr := th.Accent.Render(scope)
	if membersNote != "" {
		scopeStr += " · " + membersNote
	}
	sel := th.Box.Render(fmt.Sprintf("Range: %s    Scope: %s",
		th.Accent.Render(rangeLabel), scopeStr))
	out := title + "\n\n" + sel
	if m.errText != "" {
		out += "\n\n" + th.Err.Render(m.errText)
	}
	if latestVersion != "" {
		// The module path is spelled out in full: an elided ".../cmd/clup" reads
		// fine but does not work when the user copies the line.
		out += "\n\n" + th.Help.Render(fmt.Sprintf(
			"clup %s available — go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest",
			latestVersion))
	}
	if timerLine != "" {
		out += "\n\n" + th.Help.Render(timerLine)
	}
	return out
}
