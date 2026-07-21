# docs/demo.tape — vhs script for the README demo GIF.
#
# Generate:  vhs docs/demo.tape   (writes docs/demo.gif)
# Install vhs: https://github.com/charmbracelet/vhs  (brew install vhs)
#
# NOTE: the app needs to reach the report to show a meaningful demo, so run this
# with a configured ~/.config/clickup-cli/config.yml (or CLICKUP_TOKEN set),
# which lands the TUI on the home screen with real data.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 18
Set Width 1100
Set Height 640
Set Padding 16
Set Theme "Dracula"

# Launch (hidden while it boots)
Hide
Type "clickup"
Enter
Sleep 2s
Show

# Home: pick a month, toggle scope
Sleep 1s
Left@500ms 2
Sleep 800ms
Type "t"
Sleep 900ms
Type "t"
Sleep 900ms

# Generate the report
Enter
Sleep 2500ms

# Cycle the grouping (total -> task -> list -> day)
Type "g"
Sleep 1500ms
Type "g"
Sleep 1500ms
Type "g"
Sleep 1500ms

# Peek at the Log hours screen, then back
Type "n"
Sleep 1800ms
Escape
Sleep 1000ms

# Quit
Type "q"
Sleep 800ms
