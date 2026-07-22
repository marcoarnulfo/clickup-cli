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

# Home: pick an earlier month, toggle scope me/team
Sleep 1s
Left@450ms 2
Sleep 900ms
Type "t"
Sleep 900ms
Type "t"
Sleep 900ms

# Generate the report
Enter
Sleep 2500ms

# Cycle the grouping: total -> task -> list -> day
Type "g"
Sleep 1600ms
Type "g"
Sleep 1600ms
Type "g"
Sleep 1600ms

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
