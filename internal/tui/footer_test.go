package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func sampleKeys() keyMap {
	d := defaultKeys()
	k := keyMap{Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back, Quit: d.Quit}
	k.short = []key.Binding{pairHelp(k.Up, k.Down, "↑/↓/j/k", "move"), k.Confirm, k.Back, k.Quit}
	k.full = [][]key.Binding{
		{pairHelp(k.Up, k.Down, "↑/↓/j/k", "move"), k.Confirm},
		{k.Back, k.Quit},
	}
	return k
}

func TestFooterShortRendersOneLine(t *testing.T) {
	t.Parallel()
	got := footerView(testTheme(true), 0, false, sampleKeys())
	if strings.Contains(got, "\n") {
		t.Errorf("short footer spans several lines:\n%s", got)
	}
	for _, want := range []string{"↑/↓/j/k", "move", "enter", "esc", "q"} {
		if !strings.Contains(got, want) {
			t.Errorf("short footer missing %q:\n%s", want, got)
		}
	}
}

func TestFooterUsesMiddleDotSeparator(t *testing.T) {
	t.Parallel()
	got := footerView(testTheme(true), 0, false, sampleKeys())
	if !strings.Contains(got, " · ") {
		t.Errorf("footer does not use the house separator:\n%s", got)
	}
	if strings.Contains(got, " • ") {
		t.Errorf("footer still uses the bubbles default separator:\n%s", got)
	}
}

func TestFooterFullRendersColumns(t *testing.T) {
	t.Parallel()
	got := footerView(testTheme(true), 0, true, sampleKeys())
	if !strings.Contains(got, "\n") {
		t.Errorf("full footer is a single line, expected stacked columns:\n%s", got)
	}
}

func TestFooterWidthTruncates(t *testing.T) {
	t.Parallel()
	wide := footerView(testTheme(true), 0, false, sampleKeys())
	narrow := footerView(testTheme(true), 20, false, sampleKeys())
	if narrow == wide {
		t.Errorf("width 20 did not truncate:\n%s", narrow)
	}
	if !strings.Contains(narrow, "…") {
		t.Errorf("truncated footer has no ellipsis:\n%s", narrow)
	}
}

func TestFooterDisabledBindingIsNotShown(t *testing.T) {
	t.Parallel()
	k := sampleKeys()
	k.short[1].SetEnabled(false) // Confirm
	if got := footerView(testTheme(true), 0, false, k); strings.Contains(got, "confirm") {
		t.Errorf("disabled binding still advertised:\n%s", got)
	}
}

func TestPairHelpEnabledWhenEitherIs(t *testing.T) {
	t.Parallel()
	d := defaultKeys()
	a, b := d.Up, d.Down
	a.SetEnabled(false)
	if !pairHelp(a, b, "↑/↓", "move").Enabled() {
		t.Error("pair disabled although one half is enabled")
	}
	b.SetEnabled(false)
	if pairHelp(a, b, "↑/↓", "move").Enabled() {
		t.Error("pair enabled although both halves are disabled")
	}
}

func TestGoldenFooterSample(t *testing.T) {
	t.Parallel()
	golden(t, "footer_sample_short", footerView(testTheme(true), 0, false, sampleKeys()))
	golden(t, "footer_sample_full", footerView(testTheme(true), 0, true, sampleKeys()))
}
