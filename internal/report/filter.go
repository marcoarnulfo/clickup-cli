package report

// FilterCriteria selects entries by list, tag, and status. Within a dimension the
// match is OR (any selected value); across dimensions it is AND. An empty
// dimension imposes no constraint.
type FilterCriteria struct {
	Lists    map[string]bool
	Tags     map[string]bool
	Statuses map[string]bool
}

// Empty reports whether no dimension constrains anything.
func (c FilterCriteria) Empty() bool {
	return countTrue(c.Lists) == 0 && countTrue(c.Tags) == 0 && countTrue(c.Statuses) == 0
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
// at least one of its tags is selected, and its status is selected.
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
