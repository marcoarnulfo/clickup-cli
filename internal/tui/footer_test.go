package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
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

// A handler that acts on the absence of a match — the delete confirmation's
// "any other key cancels" — has no binding to advertise, so the footer needs a
// display-only one. It must render like any other item and must be enabled, or
// help would drop it.
func TestAnyKeyHelpRendersAsAnItem(t *testing.T) {
	t.Parallel()
	b := anyKeyHelp("cancel")
	if !b.Enabled() {
		t.Fatal("display-only binding is disabled, so the footer would drop it")
	}
	k := keyMap{}
	k.short = []key.Binding{b}
	if got := footerView(testTheme(true), 0, false, k); got != "any key cancel" {
		t.Errorf("footer = %q, want %q", got, "any key cancel")
	}
}

// TestFooterNeverExceedsWidth sweeps the widths a real terminal can have. The
// library's own truncation is not enough: bubbles/help returns the untruncated
// 74-column report footer at widths 65-67, where it would wrap (#134). The
// footer must always be one line, and never wider than the terminal it is
// drawn in — a wrapped footer pushes the screen body around, which is worse
// than a visibly cut hint.
func TestFooterNeverExceedsWidth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		m    func() Model
	}{
		{"report", newTestModelOnReport},
		{"filters", func() Model { m := newTestModel(); m.screen = screenFilters; return m }},
		{"home", func() Model { m := newTestModel(); m.screen = screenHome; return m }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := keysFor(tc.m())
			for w := 20; w <= 120; w++ {
				got := footerView(testTheme(true), w, false, k)
				if strings.Contains(got, "\n") {
					t.Fatalf("width %d: footer wrapped:\n%s", w, got)
				}
				if n := lipgloss.Width(got); n > w {
					t.Fatalf("width %d: footer is %d columns wide: %q", w, n, got)
				}
			}
		})
	}
}

// clampWidth must not build a style on lipgloss's default renderer: that is the
// one thing the injected-renderer discipline exists to prevent, and this file's
// own opening comment says so about help.New().
//
// This is invisible to the suite by construction: TestMain pins the default
// renderer to termenv.Ascii, so a style built on it renders identically to one
// built on the injected renderer, and no golden or assertion can tell them
// apart. Do not add a test that claims otherwise — pin the truncation instead,
// and keep the grep in the checklist.
func TestClampWidthNeverExceedsTheTerminal(t *testing.T) {
	th := testTheme(true)
	long := "↑/↓ move · enter run · esc close · ctrl+c force quit · ? help · g group · e export"
	for _, width := range []int{20, 40, 66, 80} {
		got := clampWidth(th, long, width)
		if w := lipgloss.Width(got); w > width {
			t.Errorf("clampWidth(width=%d) rendered %d columns", width, w)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("clampWidth(width=%d) = %q, want a visible ellipsis", width, got)
		}
	}
}

// The footer's input comes from help.View, which closes its own styles. If a
// caller ever passes an unterminated one, ansi.Truncate will NOT close it and
// the style bleeds past the cut — this pins the boundary of what clampWidth
// promises, so the discovery happens here and not on a user's terminal.
func TestClampWidthDoesNotCloseAnOpenStyle(t *testing.T) {
	th := testTheme(true)
	got := clampWidth(th, "\x1b[31m"+strings.Repeat("x", 40), 10)
	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected the caller's escape to survive, got %q", got)
	}
	if strings.Count(got, "\x1b[0m") != 0 {
		t.Errorf("clampWidth closed a style it did not open: %q", got)
	}
}
