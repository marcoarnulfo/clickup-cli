// Package service orchestrates the I/O pipeline that turns a ClickUp API
// client + a scope/range/assignee selection into report.TimeEntry values.
// It is the shared entry point for both the TUI and the (future) headless
// report command; report.Build itself stays pure and lives with its callers.
package service

import (
	"context"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// LoadEntries fetches the time entries for the given scope/range/assignees
// and enriches them with human-readable list names.
//
// For scope "team" with an empty assignees slice it derives ALL workspace
// members (via TeamMembers) and filters on them; a non-empty assignees slice
// is used as-is (skipping the members lookup). For scope "me" no assignee
// filter is applied.
func LoadEntries(ctx context.Context, c *clickup.Client, teamID string, start, end time.Time, scope string, assignees []int) ([]report.TimeEntry, error) {
	if scope == "team" && len(assignees) == 0 {
		members, err := c.TeamMembers(ctx, teamID)
		if err != nil {
			return nil, err
		}
		for _, mem := range members {
			assignees = append(assignees, mem.ID)
		}
	}

	entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
	if err != nil {
		return nil, err
	}
	// Resolve human-readable list names ONCE per unique list_id, fetched
	// concurrently (bounded) to avoid the timeout when a report spans many
	// distinct lists.
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ListID)
	}
	resolved := c.ListNames(ctx, ids)
	for i := range entries {
		if name := resolved[entries[i].ListID]; name != "" {
			entries[i].ListName = name
		}
	}
	return entries, nil
}
