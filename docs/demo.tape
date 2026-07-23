# docs/demo.tape — vhs script for the README demo GIF.
#
# Generate:  vhs docs/demo.tape        (writes docs/demo.gif)
# Install vhs: https://github.com/charmbracelet/vhs   (brew install vhs)
#
# Uses the built-in demo mode (CLICKUP_DEMO=1): fixture data, no ClickUp
# account or token needed, so the GIF is fully reproducible. Requires the
# `clup` binary on PATH (go install ./cmd/clup).
#
# NOTE: the binary was renamed clickup -> clup (v1.6); this script and
# docs/demo.gif were both updated to match. Re-run `vhs docs/demo.tape`
# to refresh the GIF after any UI change (the TUI itself is unchanged by
# the rebrand — the visual redesign lands in v1.9).

Output docs/demo.gif

Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 660
Set Padding 18
Set Theme "Dracula"

# Launch in demo mode (hidden while it boots)
Hide
Type "CLICKUP_DEMO=1 clup"
Enter
Sleep 2s
Show

# Home: demo mode always starts with a fake timer already running, so the
# live ticking indicator (v1.8, #91) is visible on boot — pause to read it
Sleep 1800ms

# Home: pick an earlier month, toggle scope me/team
Left@450ms 2
Sleep 900ms
Type "t"
Sleep 900ms
Type "t"
Sleep 900ms

# Live timer (v1.8, #91): 'c' from Home jumps straight to the running timer,
# ticking live; 's' stops it, and the summary screen returns to Home on Enter
Type "c"
Sleep 1800ms
Type "s"
Sleep 1400ms
Enter
Sleep 700ms

# Generate the report
Enter
Sleep 2500ms

# Cycle the grouping: total -> task -> list -> day -> tag (v1.7: the summary
# line, billable/non-billable split and per-currency subtotals sit below the
# bucket rows on every grouping — the pause after the last "g" lets a viewer
# read them)
Type "g"
Sleep 1600ms
Type "g"
Sleep 1600ms
Type "g"
Sleep 1600ms
Type "g"
Sleep 2200ms

# Budget burn-down view (v1.7, #64): 'b' from the report
Type "b"
Sleep 2200ms
Escape
Sleep 700ms

# Billing settings editor (v1.7): 'p' from the report, glance at each tab
Type "p"
Sleep 1600ms
Tab
Sleep 1400ms
Tab
Sleep 1200ms
Tab
Sleep 1400ms
Escape
Sleep 700ms

# Time-entry browser (v1.8, #94): 'v' from the report lists this range's
# entries; glance at edit, at the delete confirm (declined, so nothing is
# actually removed), and at the read-only history, then back to the report
Type "v"
Sleep 1400ms
Type "e"
Sleep 1400ms
Escape
Sleep 700ms
Type "x"
Sleep 1200ms
Type "n"
Sleep 700ms
Type "h"
Sleep 1400ms
Escape
Sleep 700ms
Escape
Sleep 700ms

# Peek at Log hours: open, choose Guided, see the list picker, go back
Type "n"
Sleep 1400ms
Type "1"
Sleep 1600ms
Escape
Sleep 900ms

# Quit
Type "q"
Sleep 800ms
