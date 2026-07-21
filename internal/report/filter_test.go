package report

import (
	"testing"
	"time"
)

func fe(list, status string, tags ...string) TimeEntry {
	return TimeEntry{ListName: list, Status: status, Tags: tags, Duration: time.Hour}
}

func names(entries []TimeEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ListName + "/" + e.Status
	}
	return out
}

func TestFilterEmptyReturnsAll(t *testing.T) {
	in := []TimeEntry{fe("A", "open"), fe("B", "done")}
	if got := Filter(in, FilterCriteria{}); len(got) != 2 {
		t.Fatalf("empty criteria should return all, got %d", len(got))
	}
}

func TestFilterListOR(t *testing.T) {
	in := []TimeEntry{fe("A", "open"), fe("B", "open"), fe("C", "open")}
	c := FilterCriteria{Lists: map[string]bool{"A": true, "C": true}}
	if got := Filter(in, c); len(got) != 2 {
		t.Fatalf("list OR should keep A and C, got %v", names(got))
	}
}

func TestFilterAcrossDimensionsAND(t *testing.T) {
	in := []TimeEntry{
		fe("A", "open", "urgent"),
		fe("A", "done", "urgent"),
		fe("B", "open", "urgent"),
	}
	c := FilterCriteria{Lists: map[string]bool{"A": true}, Statuses: map[string]bool{"open": true}}
	got := Filter(in, c)
	if len(got) != 1 || got[0].ListName != "A" || got[0].Status != "open" {
		t.Fatalf("list AND status should keep 1, got %v", names(got))
	}
}

func TestFilterTagAnyMatch(t *testing.T) {
	in := []TimeEntry{
		fe("A", "open", "frontend", "urgent"),
		fe("A", "open", "backend"),
	}
	c := FilterCriteria{Tags: map[string]bool{"urgent": true}}
	if got := Filter(in, c); len(got) != 1 {
		t.Fatalf("tag any-match should keep 1, got %v", names(got))
	}
}
