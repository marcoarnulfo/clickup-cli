package report

// FilterCriteria selects entries by list, tag, status, and billable flag. Within
// a dimension the match is OR (any selected value); across dimensions it is AND.
// An empty/nil dimension imposes no constraint.
//
// Name-vs-ID: the list filter stays name-based (matches TimeEntry.ListName), so
// homonymous lists across workspaces both match a selected name. This is an
// accepted, documented limitation of the filter; Build still bucketizes by list
// ID for currency correctness. Do not change the list filter to ID-based here —
// that is a separate concern from this filter's job of narrowing which entries
// participate in the report.
type FilterCriteria struct {
	Lists    map[string]bool
	Tags     map[string]bool
	Statuses map[string]bool
	Billable *bool // nil = no constraint; otherwise only entries with this Billable value
}

// Empty reports whether no dimension constrains anything.
func (c FilterCriteria) Empty() bool {
	return countTrue(c.Lists) == 0 && countTrue(c.Tags) == 0 && countTrue(c.Statuses) == 0 && c.Billable == nil
}

func countTrue(m map[string]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}

// Filter returns the entries matching the criteria. An entry matches when, for
// every constrained dimension, it satisfies that dimension: its list is selected,
// at least one of its tags is selected, its status is selected, and (when
// Billable is set) its Billable value matches.
func Filter(entries []TimeEntry, c FilterCriteria) []TimeEntry {
	if c.Empty() {
		return entries
	}
	nL, nT, nS := countTrue(c.Lists), countTrue(c.Tags), countTrue(c.Statuses)
	out := make([]TimeEntry, 0, len(entries))
	for _, e := range entries {
		if nL > 0 && !c.Lists[e.ListName] {
			continue
		}
		if nT > 0 && !anyTagSelected(e.Tags, c.Tags) {
			continue
		}
		if nS > 0 && !c.Statuses[e.Status] {
			continue
		}
		if c.Billable != nil && e.Billable != *c.Billable {
			continue
		}
		out = append(out, e)
	}
	return out
}

func anyTagSelected(tags []string, sel map[string]bool) bool {
	for _, t := range tags {
		if sel[t] {
			return true
		}
	}
	return false
}
