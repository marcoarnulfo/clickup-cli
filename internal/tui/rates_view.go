package tui

import (
	"fmt"
	"strings"
)

// Rendering of the billing editor. Split out of rates.go purely for size:
// these are pure functions of a ratesModel value and share no mutable state
// with the update side.

func (rt ratesModel) tabs() string {
	labels := []string{"Lists", "Members", "Overrides", "Rules"}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if ratesSection(i) == rt.sec {
			parts[i] = styleAccent.Render("[" + l + "]")
		} else {
			parts[i] = " " + l + " "
		}
	}
	return strings.Join(parts, " ")
}

func (rt ratesModel) help() string {
	if rt.draft.active && rt.draft.step != draftRate {
		return "↑/↓ select · Enter: confirm · Esc: cancel"
	}
	switch rt.sec {
	case secLists:
		// 'd' only clears the rate; a currency or a budget is cleared by
		// reopening its own field ('c'/'g') and submitting an empty value.
		return "Tab: section · ↑/↓ select · Enter: rate · c: currency · g: budget (submit empty to clear either) · d: use default rate · b: browse lists · s: save · Esc: cancel"
	case secMembers:
		return "Tab: section · ↑/↓ select · Enter: rate · d: use default · s: save · Esc: cancel"
	case secOverrides:
		return "Tab: section · ↑/↓ select · Enter: edit · n: new override · d: delete · s: save · Esc: cancel"
	default:
		return "Tab: section · ↑/↓ select · Enter: edit/toggle · d: clear · s: save · Esc: cancel"
	}
}

// moneyOrDash renders a money value, or an em dash when unset.
func moneyOrDash(v float64, set bool) string {
	if !set {
		return "—"
	}
	return fmt.Sprintf("%.2f", v)
}

// billingRow renders one selectable row of the editor.
func billingRow(sel bool, line string) string {
	if sel {
		return "▸ " + styleAccent.Render(line) + "\n"
	}
	return "  " + line + "\n"
}

func (rt ratesModel) listsView() string {
	b := styleHelp.Render(fmt.Sprintf("  %-24s %10s %-5s %10s  %s", "List", "Rate", "Cur", "Budget", "Source")) + "\n"
	if len(rt.rows) == 0 {
		return b + styleHelp.Render("  No lists in the current report — press 'b' to browse the workspace.") + "\n"
	}
	for i, r := range rt.rows {
		rate, tag := rt.def, "default"
		if v, ok := rt.rates[r.listID]; ok {
			rate, tag = v, "list rate"
		}
		bud, hasBud := rt.budgets[r.listID]
		line := fmt.Sprintf("%-24s %10.2f %-5s %10s  %s",
			truncate(r.name, 24), rate, rt.effectiveCurrency(r.listID), moneyOrDash(bud, hasBud), tag)
		b += billingRow(i == rt.idx, line)
	}
	sel := rt.rows[rt.idx]
	note := fmt.Sprintf("Effective for %s: %.2f %s", truncate(sel.name, 24), rt.rateFor(sel.listID), rt.effectiveCurrency(sel.listID))
	if n := rt.pairsForList(sel.listID); n > 0 {
		note += fmt.Sprintf(" · %d (list,member) override(s) take precedence here", n)
	} else if len(rt.memberRates) > 0 {
		note += fmt.Sprintf(" · %d member rate(s) take precedence here", len(rt.memberRates))
	}
	return b + "\n" + styleHelp.Render(note) + "\n"
}

func (rt ratesModel) membersView() string {
	b := styleHelp.Render(fmt.Sprintf("  %-30s %10s  %s", "Member", "Rate", "Source")) + "\n"
	if len(rt.members) == 0 {
		return b + styleHelp.Render("  No members in the current report — run a team-scope report first.") + "\n"
	}
	for i, mr := range rt.members {
		rate, tag := rt.def, "default"
		if v, ok := rt.memberRates[mr.id]; ok {
			rate, tag = v, "member rate"
		}
		line := fmt.Sprintf("%-30s %10.2f  %s", truncate(fmt.Sprintf("%s (%d)", mr.name, mr.id), 30), rate, tag)
		b += billingRow(i == rt.memIdx, line)
	}
	sel := rt.members[rt.memIdx]
	note := "A member rate wins over any per-list rate, on every list."
	if n := rt.listsForMember(sel.id); n > 0 {
		note = fmt.Sprintf("%s is overridden on %d list(s) by a (list,member) rate.", truncate(sel.name, 24), n)
	}
	return b + "\n" + styleHelp.Render(note) + "\n"
}

func (rt ratesModel) overridesView() string {
	b := styleHelp.Render(fmt.Sprintf("  %-20s %-22s %10s  %s", "List", "Member", "Rate", "Instead of")) + "\n"
	for i, o := range rt.overrides {
		below, src := rt.rateBelowPair(o.listID, o.member)
		line := fmt.Sprintf("%-20s %-22s %10.2f  %.2f (%s)",
			truncate(rt.listName(o.listID), 20),
			truncate(fmt.Sprintf("%s (%d)", rt.memberName(o.member), o.member), 22),
			o.rate, below, src)
		b += billingRow(i == rt.ovIdx, line)
	}
	b += billingRow(rt.ovIdx >= len(rt.overrides), "+ new (list,member) override")
	if len(rt.overrides) == 0 {
		b += "\n" + styleHelp.Render("No (list,member) overrides — the most specific level of the precedence.") + "\n"
	}
	return b
}

func (rt ratesModel) rulesView() string {
	mode := "nearest"
	if rt.rounding.Mode == "up" {
		mode = "up"
	}
	scope := "per entry"
	if rt.rounding.Scope == "day" {
		scope = "per day"
	}
	inc := rt.rounding.Increment
	if inc == "" {
		inc = "— (rounding off)"
	}
	cur := rt.defCur
	if cur == "" {
		cur = fmt.Sprintf("— (using %s)", rt.cur)
	}
	tz := rt.tz
	if tz == "" {
		tz = "— (system local)"
	}
	fields := [ruleCount][2]string{
		{"Default currency", cur},
		{"Rounding increment", inc},
		{"Rounding mode", mode},
		{"Rounding scope", scope},
		{"Timezone", tz},
	}
	b := ""
	for i, f := range fields {
		b += billingRow(i == rt.ruleIdx, fmt.Sprintf("%-22s %s", f[0], f[1]))
	}
	return b + "\n" + styleHelp.Render("The default currency and rounding rule apply to every list without its own currency.") + "\n"
}

func (rt ratesModel) draftView() string {
	if rt.draft.step == draftPickList {
		b := styleHelp.Render("New override — choose the list:") + "\n"
		for i, r := range rt.rows {
			b += billingRow(i == rt.draft.idx, truncate(r.name, 40))
		}
		return b
	}
	b := styleHelp.Render(fmt.Sprintf("New override on %s — choose the member:", truncate(rt.listName(rt.draft.listID), 24))) + "\n"
	for i, mr := range rt.members {
		b += billingRow(i == rt.draft.idx, truncate(fmt.Sprintf("%s (%d)", mr.name, mr.id), 40))
	}
	return b
}
