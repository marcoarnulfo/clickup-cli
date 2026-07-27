package tui

import (
	"slices"
	"testing"
)

// TestSetupKeyLabelsPerStep pins the label set each wizard step of
// setup.go's updateSetup accepts today. The steps accept different labels
// (stepWorkspace has a list to move through; the others only ever react to
// Confirm, forwarding everything else to the focused textinput), so each
// gets its own case rather than one test per screen. Quit stays off in every
// step (see TestQuitBindingPerScreen).
func TestSetupKeyLabelsPerStep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		step setupStep
		want []string
	}{
		{"token", stepToken, []string{"enter"}},
		{"workspace", stepWorkspace, []string{"down", "enter", "j", "k", "up"}},
		{"rate", stepRate, []string{"enter"}},
		{"currency", stepCurrency, []string{"enter"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{screen: screenSetup, setup: setupModel{step: c.step}}
			if got := enabledLabels(keysFor(m)); !slices.Equal(got, c.want) {
				t.Errorf("setup %s labels = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
