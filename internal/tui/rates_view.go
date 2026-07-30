package tui

import (
	"fmt"
	"strings"
)

// Rendering of the billing editor. Split out of rates.go purely for size:
// these are pure functions of a ratesModel value and share no mutable state
// with the update side.

func (rt ratesModel) tabs(th theme) string {
	labels := []string{"Lists", "Members", "Overrides", "Rules"}
	parts := make([]string, len(labels))
	for i, l := range labels {
		if ratesSection(i) == rt.sec {
			parts[i] = th.Accent.Render("[" + l + "]")
		} else {
			parts[i] = " " + l + " "
		}
	}
	return strings.Join(parts, " ")
}

// moneyOrDash renders a money value, or an em dash when unset.
func moneyOrDash(v float64, set bool) string {
	if !set {
		return "—"
	}
	return fmt.Sprintf("%.2f", v)
}

// billingRow renders one selectable row of the editor.
func billingRow(th theme, sel bool, line string) string {
	if sel {
		return "▸ " + th.Accent.Render(line) + "\n"
	}
	return "  " + line + "\n"
}

func (rt ratesModel) listsView(th theme) string {
	// "List", "Rate", "Cur", "Budget" and "Source" are ASCII literals, where
	// runes and columns coincide by construction, so the %-Ns verbs in the
	// header stay aligned with the data rows' cell(...,N) columns below.
	//
	// The Cur column is the one data column here that is NOT a %-Ns verb: the
	// currency comes from effectiveCurrency, which traces back to the user's
	// own config (currencies/default_currency/currency in config.go) with no
	// ISO check enforced — Valid() never looks at it. A wide-rune currency
	// (e.g. "円") would misalign every column after it under %-5s, which pads
	// by counting runes; cell(cur, 5) pads by display columns instead, the
	// same fix this whole file's non-ASCII columns already use.
	b := th.Help.Render(fmt.Sprintf("  %-24s %10s %-5s %10s  %s", "List", "Rate", "Cur", "Budget", "Source")) + "\n"
	if len(rt.rows) == 0 {
		return b + th.Help.Render("  No lists in the current report — press 'b' to browse the workspace.") + "\n"
	}
	for i, r := range rt.rows {
		rate, tag := rt.def, "default"
		if v, ok := rt.rates[r.listID]; ok {
			rate, tag = v, "list rate"
		}
		bud, hasBud := rt.budgets[r.listID]
		line := fmt.Sprintf("%s %10.2f %s %10s  %s",
			cell(r.name, 24), rate, cell(rt.effectiveCurrency(r.listID), 5), moneyOrDash(bud, hasBud), tag)
		b += billingRow(th, i == rt.sel[secLists], line)
	}
	sel := rt.rows[rt.sel[secLists]]
	note := fmt.Sprintf("Effective for %s: %.2f %s", truncateWidth(sel.name, 24), rt.rateFor(sel.listID), rt.effectiveCurrency(sel.listID))
	if n := rt.pairsForList(sel.listID); n > 0 {
		note += fmt.Sprintf(" · %d (list,member) override(s) take precedence here", n)
	} else if len(rt.memberRates) > 0 {
		note += fmt.Sprintf(" · %d member rate(s) take precedence here", len(rt.memberRates))
	}
	return b + "\n" + th.Help.Render(note) + "\n"
}

func (rt ratesModel) membersView(th theme) string {
	// "Member" is an ASCII literal, where runes and columns coincide by
	// construction, so %-30s stays aligned with cell(...,30) in the data rows.
	b := th.Help.Render(fmt.Sprintf("  %-30s %10s  %s", "Member", "Rate", "Source")) + "\n"
	if len(rt.members) == 0 {
		return b + th.Help.Render("  No members in the current report — run a team-scope report first.") + "\n"
	}
	for i, mr := range rt.members {
		rate, tag := rt.def, "default"
		if v, ok := rt.memberRates[mr.id]; ok {
			rate, tag = v, "member rate"
		}
		line := fmt.Sprintf("%s %10.2f  %s", cell(fmt.Sprintf("%s (%d)", mr.name, mr.id), 30), rate, tag)
		b += billingRow(th, i == rt.sel[secMembers], line)
	}
	sel := rt.members[rt.sel[secMembers]]
	note := "A member rate wins over any per-list rate, on every list."
	if n := rt.listsForMember(sel.id); n > 0 {
		note = fmt.Sprintf("%s is overridden on %d list(s) by a (list,member) rate.", truncateWidth(sel.name, 24), n)
	}
	return b + "\n" + th.Help.Render(note) + "\n"
}

func (rt ratesModel) overridesView(th theme) string {
	// "List" and "Member" are ASCII literals, where runes and columns coincide
	// by construction, so %-20s and %-22s stay aligned with the cell(...,20)
	// and cell(...,22) data rows below.
	b := th.Help.Render(fmt.Sprintf("  %-20s %-22s %10s  %s", "List", "Member", "Rate", "Instead of")) + "\n"
	for i, o := range rt.overrides {
		below, src := rt.rateBelowPair(o.listID, o.member)
		line := fmt.Sprintf("%s %s %10.2f  %.2f (%s)",
			cell(rt.listName(o.listID), 20),
			cell(fmt.Sprintf("%s (%d)", rt.memberName(o.member), o.member), 22),
			o.rate, below, src)
		b += billingRow(th, i == rt.sel[secOverrides], line)
	}
	b += billingRow(th, rt.sel[secOverrides] >= len(rt.overrides), "+ new (list,member) override")
	if len(rt.overrides) == 0 {
		b += "\n" + th.Help.Render("No (list,member) overrides — the most specific level of the precedence.") + "\n"
	}
	return b
}

func (rt ratesModel) rulesView(th theme) string {
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
		b += billingRow(th, i == rt.sel[secRules], fmt.Sprintf("%-22s %s", f[0], f[1]))
	}
	return b + "\n" + th.Help.Render("The default currency and rounding rule apply to every list without its own currency.") + "\n"
}

func (rt ratesModel) draftView(th theme) string {
	if rt.draft.step == draftPickList {
		b := th.Help.Render("New override — choose the list:") + "\n"
		for i, r := range rt.rows {
			b += billingRow(th, i == rt.draft.idx, truncateWidth(r.name, 40))
		}
		return b
	}
	b := th.Help.Render(fmt.Sprintf("New override on %s — choose the member:", truncateWidth(rt.listName(rt.draft.listID), 24))) + "\n"
	for i, mr := range rt.members {
		b += billingRow(th, i == rt.draft.idx, truncateWidth(fmt.Sprintf("%s (%d)", mr.name, mr.id), 40))
	}
	return b
}
