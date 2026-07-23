package tui

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
)

// ratesSection is one tab of the billing editor. The sections mirror the rate
// precedence ((list,member) > member > list > default) plus the workspace-wide
// rules that are not per-list.
type ratesSection int

const (
	secLists ratesSection = iota
	secMembers
	secOverrides
	secRules
	secCount
)

// Rows of the Rules section, in display order.
const (
	ruleDefaultCurrency = iota
	ruleIncrement
	ruleMode
	ruleScope
	ruleTimezone
	ruleCount
)

// editKind identifies which field the open input is editing, so commit knows
// how to validate and where to write the value.
type editKind int

const (
	editNone editKind = iota
	editListRate
	editListCurrency
	editListBudget
	editMemberRate
	editOverrideRate
	editDefaultCurrency
	editIncrement
	editTimezone
)

// numericEdit reports whether the field accepts only digits and a decimal
// separator (the others are free text: currency codes, "15m", "Europe/Rome").
func numericEdit(k editKind) bool {
	switch k {
	case editListRate, editListBudget, editMemberRate, editOverrideRate:
		return true
	}
	return false
}

// Steps of the "new (list,member) override" draft.
const (
	draftPickList = iota
	draftPickMember
	draftRate
)

// overrideDraft is the in-progress creation of a (list,member) override: pick
// a list, pick a member, then type the rate (which reuses the normal edit
// input with editOverrideRate).
type overrideDraft struct {
	active bool
	step   int
	idx    int
	listID string
	member int
}

// rateRow is a list shown in the rates screen.
type rateRow struct {
	listID string
	name   string
}

// memberRow is a workspace member shown in the Members section.
type memberRow struct {
	id   int
	name string
}

// overrideRow is one (list,member) rate override.
type overrideRow struct {
	listID string
	member int
	rate   float64
}

type ratesModel struct {
	sec ratesSection

	rows      []rateRow // lists
	idx       int       // selection in the Lists section
	members   []memberRow
	memIdx    int
	overrides []overrideRow
	ovIdx     int
	ruleIdx   int

	editing bool
	edit    editKind
	input   textinput.Model
	draft   overrideDraft

	// working copies of everything the screen persists; a failed save or a
	// rejected value never touches the config.
	rates       map[string]float64 // list_id -> rate
	memberRates map[int]float64    // user_id -> rate
	currencies  map[string]string  // list_id -> ISO currency
	budgets     map[string]float64 // list_id -> budget amount
	defCur      string             // billing.default_currency
	rounding    config.Rounding
	tz          string

	def float64 // default rate
	cur string  // top-level currency (fallback for display)
	msg string  // inline error message
}

// newRates builds the screen from the lists and members in the current report
// merged with those already present in config. Entities "only in config" are
// added in deterministic order (ascending id) so the view is stable and a
// configured value is never invisible (and therefore never silently dropped by
// the next save).
func newRates(entries []report.TimeEntry, cfg config.Config) ratesModel {
	rt := ratesModel{
		rates:       map[string]float64{},
		memberRates: map[int]float64{},
		currencies:  map[string]string{},
		budgets:     map[string]float64{},
		defCur:      cfg.Billing.DefaultCurrency,
		rounding:    cfg.Billing.Rounding,
		tz:          cfg.Timezone,
		def:         cfg.Rate,
		cur:         cfg.Currency,
	}
	for k, v := range cfg.Rates {
		rt.rates[k] = v
	}
	for k, v := range cfg.Billing.RatesByMember {
		rt.memberRates[k] = v
	}
	for k, v := range cfg.Billing.Currencies {
		rt.currencies[k] = v
	}
	for k, v := range cfg.Billing.Budgets {
		rt.budgets[k] = v
	}
	for _, o := range cfg.Billing.RateOverrides {
		rt.overrides = append(rt.overrides, overrideRow{listID: o.List, member: o.Member, rate: o.Rate})
	}
	sortOverrides(rt.overrides)

	rt.rows = listRowsFor(entries, cfg)
	rt.members = memberRowsFor(entries, cfg)
	return rt
}

// listRowsFor merges the lists seen in the entries (in first-seen order) with
// every list id referenced by the config (sorted).
func listRowsFor(entries []report.TimeEntry, cfg config.Config) []rateRow {
	names := map[string]string{}
	var order []string
	remember := func(id, name string) {
		if id == "" {
			return
		}
		if _, ok := names[id]; !ok {
			order = append(order, id)
			names[id] = id // default label = id
		}
		if name != "" {
			names[id] = name
		}
	}
	for _, e := range entries {
		remember(e.ListID, e.ListName)
	}
	var cfgIDs []string
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := names[id]; !ok && !slices.Contains(cfgIDs, id) {
			cfgIDs = append(cfgIDs, id)
		}
	}
	for id := range cfg.Rates {
		add(id)
	}
	for id := range cfg.Billing.Currencies {
		add(id)
	}
	for id := range cfg.Billing.Budgets {
		add(id)
	}
	for _, o := range cfg.Billing.RateOverrides {
		add(o.List)
	}
	slices.Sort(cfgIDs)
	for _, id := range cfgIDs {
		remember(id, "")
	}

	rows := make([]rateRow, len(order))
	for i, id := range order {
		rows[i] = rateRow{listID: id, name: names[id]}
	}
	return rows
}

// memberRowsFor merges the members seen in the entries (in first-seen order)
// with every member id referenced by the config (sorted).
func memberRowsFor(entries []report.TimeEntry, cfg config.Config) []memberRow {
	names := map[int]string{}
	var order []int
	remember := func(id int, name string) {
		if id == 0 {
			return
		}
		if _, ok := names[id]; !ok {
			order = append(order, id)
			names[id] = fmt.Sprintf("user %d", id)
		}
		if name != "" {
			names[id] = name
		}
	}
	for _, e := range entries {
		remember(e.UserID, e.UserName)
	}
	var cfgIDs []int
	add := func(id int) {
		if id == 0 {
			return
		}
		if _, ok := names[id]; !ok && !slices.Contains(cfgIDs, id) {
			cfgIDs = append(cfgIDs, id)
		}
	}
	for id := range cfg.Billing.RatesByMember {
		add(id)
	}
	for _, o := range cfg.Billing.RateOverrides {
		add(o.Member)
	}
	slices.Sort(cfgIDs)
	for _, id := range cfgIDs {
		remember(id, "")
	}

	rows := make([]memberRow, len(order))
	for i, id := range order {
		rows[i] = memberRow{id: id, name: names[id]}
	}
	return rows
}

func sortOverrides(o []overrideRow) {
	slices.SortFunc(o, func(a, b overrideRow) int {
		if c := strings.Compare(a.listID, b.listID); c != 0 {
			return c
		}
		return a.member - b.member
	})
}

// validRate accepts only a finite number > 0. A rate (or budget) of zero is
// not an edit but a removal, which has its own key ('d'), so accepting it here
// would only hide typos in a billing tool. The decimal comma is accepted as
// well as the dot (handy for the Italian keyboard).
func validRate(s string) (float64, bool) {
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// numericRune reports whether a rune is allowed in the rate field (digits and separator).
func numericRune(r rune) bool {
	return (r >= '0' && r <= '9') || r == '.' || r == ','
}

// ---------------------------------------------------------------- lookups --

func (rt ratesModel) listName(id string) string {
	for _, r := range rt.rows {
		if r.listID == id {
			return r.name
		}
	}
	return id
}

func (rt ratesModel) memberName(id int) string {
	for _, r := range rt.members {
		if r.id == id {
			return r.name
		}
	}
	return fmt.Sprintf("user %d", id)
}

// effectiveCurrency resolves the currency a list bills in: its own override,
// else the billing default, else the top-level currency.
func (rt ratesModel) effectiveCurrency(listID string) string {
	if c := rt.currencies[listID]; c != "" {
		return c
	}
	if rt.defCur != "" {
		return rt.defCur
	}
	return rt.cur
}

// rateBelowPair resolves the rate that would apply to (listID, member) if the
// (list,member) override did not exist, mirroring report.Rates.For's remaining
// precedence. The returned label names the level that wins.
func (rt ratesModel) rateBelowPair(listID string, member int) (float64, string) {
	if v, ok := rt.memberRates[member]; ok {
		return v, "member rate"
	}
	if v, ok := rt.rates[listID]; ok {
		return v, "list rate"
	}
	return rt.def, "default rate"
}

// pairsForList counts the (list,member) overrides that take precedence on a list.
func (rt ratesModel) pairsForList(listID string) int {
	n := 0
	for _, o := range rt.overrides {
		if o.listID == listID {
			n++
		}
	}
	return n
}

// listsForMember counts the (list,member) overrides that take precedence over
// a member's rate.
func (rt ratesModel) listsForMember(member int) int {
	n := 0
	for _, o := range rt.overrides {
		if o.member == member {
			n++
		}
	}
	return n
}

// ----------------------------------------------------------------- update --

func (m Model) updateRates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rt := m.ratesScreen

	if rt.editing {
		return m.updateRatesEditing(rt, msg)
	}
	if rt.draft.active {
		m.ratesScreen = rt.updateDraft(msg)
		return m, nil
	}

	switch msg.String() {
	case "tab", "right", "l":
		rt.sec = (rt.sec + 1) % secCount
		rt.msg = ""
	case "shift+tab", "left", "h":
		rt.sec = (rt.sec + secCount - 1) % secCount
		rt.msg = ""
	case "up", "k":
		rt = rt.move(-1)
	case "down", "j":
		rt = rt.move(+1)
	case "enter":
		rt = rt.startEdit()
	case "c":
		if rt.sec == secLists && len(rt.rows) > 0 {
			rt.editing, rt.edit, rt.msg = true, editListCurrency, ""
			rt.input = newTextInput("currency (e.g. EUR) — empty: use the default")
		}
	case "g":
		if rt.sec == secLists && len(rt.rows) > 0 {
			rt.editing, rt.edit, rt.msg = true, editListBudget, ""
			rt.input = newNumberInput("budget amount — empty: no budget")
		}
	case "n":
		if rt.sec == secOverrides {
			rt = rt.startDraft()
		}
	case "d":
		rt = rt.clearSelected()
	case "b":
		m.ratesScreen = rt
		return m.openListBrowser(screenRates)
	case "s":
		return m.saveRates(rt)
	case "esc":
		// Discard unsaved changes and return to the report.
		m.screen = screenReport
		return m, nil
	}
	m.ratesScreen = rt
	return m, nil
}

// updateRatesEditing handles keys while an input field is open.
func (m Model) updateRatesEditing(rt ratesModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		rt = rt.commit(strings.TrimSpace(rt.input.Value()))
		m.ratesScreen = rt
		return m, nil
	case tea.KeyEsc:
		rt.editing, rt.edit, rt.msg = false, editNone, ""
		rt.draft = overrideDraft{} // an abandoned rate abandons the whole draft
		m.ratesScreen = rt
		return m, nil
	}
	// Numeric-only field: ignore characters that aren't allowed.
	if numericEdit(rt.edit) && msg.Type == tea.KeyRunes {
		for _, r := range msg.Runes {
			if !numericRune(r) {
				m.ratesScreen = rt
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	rt.input, cmd = rt.input.Update(msg)
	m.ratesScreen = rt
	return m, cmd
}

// updateDraft handles the list/member pickers of a new override.
func (rt ratesModel) updateDraft(msg tea.KeyMsg) ratesModel {
	n := len(rt.rows)
	if rt.draft.step == draftPickMember {
		n = len(rt.members)
	}
	switch msg.String() {
	case "up", "k":
		if rt.draft.idx > 0 {
			rt.draft.idx--
		}
	case "down", "j":
		if rt.draft.idx < n-1 {
			rt.draft.idx++
		}
	case "enter":
		if rt.draft.step == draftPickList {
			rt.draft.listID = rt.rows[rt.draft.idx].listID
			rt.draft.step, rt.draft.idx = draftPickMember, 0
			return rt
		}
		rt.draft.member = rt.members[rt.draft.idx].id
		rt.draft.step = draftRate
		rt.editing, rt.edit, rt.msg = true, editOverrideRate, ""
		rt.input = newNumberInput("override rate (Esc to cancel)")
	case "esc":
		rt.draft = overrideDraft{}
	}
	return rt
}

// startDraft begins a new (list,member) override.
func (rt ratesModel) startDraft() ratesModel {
	if len(rt.rows) == 0 || len(rt.members) == 0 {
		rt.msg = "No lists or members known yet: browse a list ('b') or run a team report first"
		return rt
	}
	rt.draft = overrideDraft{active: true, step: draftPickList}
	rt.msg = ""
	return rt
}

// selCount is the number of selectable rows in the active section (the
// Overrides section has one extra row: "new override").
func (rt ratesModel) selCount() int {
	switch rt.sec {
	case secLists:
		return len(rt.rows)
	case secMembers:
		return len(rt.members)
	case secOverrides:
		return len(rt.overrides) + 1
	default:
		return ruleCount
	}
}

// move shifts the selection of the active section by delta, clamped.
func (rt ratesModel) move(delta int) ratesModel {
	cur := rt.idx
	switch rt.sec {
	case secMembers:
		cur = rt.memIdx
	case secOverrides:
		cur = rt.ovIdx
	case secRules:
		cur = rt.ruleIdx
	}
	next := cur + delta
	if next < 0 || next > rt.selCount()-1 {
		return rt
	}
	switch rt.sec {
	case secLists:
		rt.idx = next
	case secMembers:
		rt.memIdx = next
	case secOverrides:
		rt.ovIdx = next
	case secRules:
		rt.ruleIdx = next
	}
	return rt
}

// startEdit opens the input (or applies the toggle) for the selected row.
func (rt ratesModel) startEdit() ratesModel {
	rt.msg = ""
	switch rt.sec {
	case secLists:
		if len(rt.rows) == 0 {
			return rt
		}
		rt.editing, rt.edit = true, editListRate
		rt.input = newNumberInput("new rate (Esc to cancel)")
	case secMembers:
		if len(rt.members) == 0 {
			return rt
		}
		rt.editing, rt.edit = true, editMemberRate
		rt.input = newNumberInput("member rate (Esc to cancel)")
	case secOverrides:
		if rt.ovIdx >= len(rt.overrides) {
			return rt.startDraft()
		}
		rt.editing, rt.edit = true, editOverrideRate
		rt.input = newNumberInput("override rate (Esc to cancel)")
	case secRules:
		switch rt.ruleIdx {
		case ruleDefaultCurrency:
			rt.editing, rt.edit = true, editDefaultCurrency
			rt.input = newTextInput("default currency (e.g. EUR)")
		case ruleIncrement:
			rt.editing, rt.edit = true, editIncrement
			rt.input = newTextInput("rounding increment (e.g. 15m) — empty: off")
		case ruleMode:
			if rt.rounding.Mode == "up" {
				rt.rounding.Mode = ""
			} else {
				rt.rounding.Mode = "up"
			}
		case ruleScope:
			if rt.rounding.Scope == "day" {
				rt.rounding.Scope = ""
			} else {
				rt.rounding.Scope = "day"
			}
		case ruleTimezone:
			rt.editing, rt.edit = true, editTimezone
			rt.input = newTextInput("timezone (e.g. Europe/Rome) — empty: system local")
		}
	}
	return rt
}

// commit validates and applies the typed value. On a rejected value the field
// stays open with an inline message, so nothing else the user has edited is lost.
func (rt ratesModel) commit(v string) ratesModel {
	done := func() ratesModel {
		rt.editing, rt.edit, rt.msg = false, editNone, ""
		return rt
	}
	const badRate = "Invalid rate: enter a number > 0 ('d' clears the value)"

	switch rt.edit {
	case editListRate:
		if v == "" {
			return done() // empty = no change (to clear an override, use 'd')
		}
		f, ok := validRate(v)
		if !ok {
			rt.msg = badRate
			return rt
		}
		rt.rates[rt.rows[rt.idx].listID] = f
		return done()

	case editListCurrency:
		id := rt.rows[rt.idx].listID
		if v == "" {
			delete(rt.currencies, id)
		} else {
			rt.currencies[id] = strings.ToUpper(v)
		}
		return done()

	case editListBudget:
		id := rt.rows[rt.idx].listID
		if v == "" {
			delete(rt.budgets, id)
			return done()
		}
		f, ok := validRate(v)
		if !ok {
			rt.msg = "Invalid budget: enter an amount > 0 ('d' clears the value)"
			return rt
		}
		rt.budgets[id] = f
		return done()

	case editMemberRate:
		if v == "" {
			return done() // empty = no change (to clear, use 'd')
		}
		f, ok := validRate(v)
		if !ok {
			rt.msg = badRate
			return rt
		}
		rt.memberRates[rt.members[rt.memIdx].id] = f
		return done()

	case editOverrideRate:
		f, ok := validRate(v)
		if !ok {
			rt.msg = badRate
			return rt
		}
		if rt.draft.active {
			rt.overrides = upsertOverride(rt.overrides, overrideRow{listID: rt.draft.listID, member: rt.draft.member, rate: f})
			rt.ovIdx = indexOfOverride(rt.overrides, rt.draft.listID, rt.draft.member)
			rt.draft = overrideDraft{}
		} else {
			rt.overrides[rt.ovIdx].rate = f
		}
		return done()

	case editDefaultCurrency:
		rt.defCur = strings.ToUpper(v)
		return done()

	case editIncrement:
		// An empty increment means rounding off; a non-empty unparseable one
		// is an error, never a silent off (silent-off over-bills).
		if v != "" {
			if _, err := duration.Parse(v); err != nil {
				rt.msg = "Invalid rounding increment: use e.g. 15m, 0.25h, 90 (empty: rounding off)"
				return rt
			}
		}
		rt.rounding.Increment = v
		return done()

	case editTimezone:
		if _, err := service.LoadLocation(v, time.Local); err != nil {
			rt.msg = "Unknown timezone: use an IANA name like Europe/Rome (empty: system local)"
			return rt
		}
		rt.tz = v
		return done()
	}
	return done()
}

func upsertOverride(list []overrideRow, o overrideRow) []overrideRow {
	for i, x := range list {
		if x.listID == o.listID && x.member == o.member {
			list[i].rate = o.rate
			return list
		}
	}
	list = append(list, o)
	sortOverrides(list)
	return list
}

func indexOfOverride(list []overrideRow, listID string, member int) int {
	for i, x := range list {
		if x.listID == listID && x.member == member {
			return i
		}
	}
	return 0
}

// clearSelected ('d') removes the value of the selected row, reverting it to
// the next level of the precedence (or to "not configured" for the rules).
func (rt ratesModel) clearSelected() ratesModel {
	rt.msg = ""
	switch rt.sec {
	case secLists:
		if len(rt.rows) > 0 {
			delete(rt.rates, rt.rows[rt.idx].listID) // revert to the default rate
		}
	case secMembers:
		if len(rt.members) > 0 {
			delete(rt.memberRates, rt.members[rt.memIdx].id)
		}
	case secOverrides:
		if rt.ovIdx < len(rt.overrides) {
			// The selection stays valid: after the delete the freed index is
			// either another override or the trailing "new override" row.
			rt.overrides = slices.Delete(rt.overrides, rt.ovIdx, rt.ovIdx+1)
		}
	case secRules:
		switch rt.ruleIdx {
		case ruleDefaultCurrency:
			rt.defCur = ""
		case ruleIncrement:
			rt.rounding.Increment = ""
		case ruleMode:
			rt.rounding.Mode = ""
		case ruleScope:
			rt.rounding.Scope = ""
		case ruleTimezone:
			rt.tz = ""
		}
	}
	return rt
}

// saveRates validates the whole editor state, persists it into the config and
// rebuilds the report. Validation is repeated here (not only per field) so a
// value that would make the next config load fail can never reach the disk.
func (m Model) saveRates(rt ratesModel) (tea.Model, tea.Cmd) {
	if rt.rounding.Increment != "" {
		if _, err := duration.Parse(rt.rounding.Increment); err != nil {
			rt.msg = "Invalid rounding increment: use e.g. 15m, 0.25h, 90 (empty: rounding off)"
			m.ratesScreen = rt
			return m, nil
		}
	}
	if _, err := service.LoadLocation(rt.tz, time.Local); err != nil {
		rt.msg = "Unknown timezone: use an IANA name like Europe/Rome (empty: system local)"
		m.ratesScreen = rt
		return m, nil
	}

	// Build on a copy: if saving fails, config and working copy stay intact.
	// Per-list rates equal to the default are redundant and not persisted.
	toSave := map[string]float64{}
	for id, v := range rt.rates {
		if v != rt.def {
			toSave[id] = v
		}
	}
	overrides := make([]config.Override, len(rt.overrides))
	for i, o := range rt.overrides {
		overrides[i] = config.Override{List: o.listID, Member: o.member, Rate: o.rate}
	}

	cfg := m.cfg
	cfg.Rates = toSave
	cfg.Timezone = rt.tz
	cfg.Billing.DefaultCurrency = rt.defCur
	cfg.Billing.RatesByMember = maps.Clone(rt.memberRates)
	cfg.Billing.RateOverrides = overrides
	cfg.Billing.Currencies = maps.Clone(rt.currencies)
	cfg.Billing.Budgets = maps.Clone(rt.budgets)
	cfg.Billing.Rounding = rt.rounding

	if err := config.Save(cfg); err != nil {
		rt.msg = "Error saving config: " + err.Error()
		m.ratesScreen = rt
		return m, nil
	}
	m.cfg = cfg
	rt.rates = toSave // update the working copy only after a successful save
	rt.msg = ""
	m.ratesScreen = rt

	g := m.report.GroupBy
	if g == "" {
		g = report.GroupByTotal
	}
	if _, ok := m.locOrErr(); !ok {
		return m, nil
	}
	p, ok := m.pricingOrErr()
	if !ok {
		return m, nil
	}
	start, end := m.currentRange()
	m.report = report.Build(m.visibleEntries(), g, p, start, end, m.loc)
	m.report.Scope = m.scope
	m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
	m.screen = screenReport
	return m, nil
}

// ------------------------------------------------------------------- view --

func (rt ratesModel) view() string {
	b := styleTitle.Render("Billing settings") + "\n" + rt.tabs() + "\n\n"

	if rt.draft.active && rt.draft.step != draftRate {
		b += rt.draftView()
	} else {
		switch rt.sec {
		case secLists:
			b += rt.listsView()
		case secMembers:
			b += rt.membersView()
		case secOverrides:
			b += rt.overridesView()
		default:
			b += rt.rulesView()
		}
	}
	if rt.editing {
		b += "\n" + rt.input.View() + "\n"
	}
	if rt.msg != "" {
		b += "\n" + styleErr.Render(rt.msg) + "\n"
	}
	b += "\n" + styleHelp.Render(rt.help())
	b += "\n" + styleHelp.Render("Rate precedence: (list,member) > member > list > default")
	return b
}

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
		return "Tab: section · ↑/↓ select · Enter: rate · c: currency · g: budget · d: use default · b: browse lists · s: save · Esc: cancel"
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

// rateFor is the list-level rate (no member in play).
func (rt ratesModel) rateFor(listID string) float64 {
	if v, ok := rt.rates[listID]; ok {
		return v
	}
	return rt.def
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
