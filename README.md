**English** · [Italiano](README.it.md)

# clickup — ClickUp Hours CLI

Terminal TUI for ClickUp monthly hours reports (self + team), with billable-amount
calculation and CSV/JSON/Markdown export.

## Installation

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clickup@latest
```

## Usage

Run `clickup`. On first launch a setup wizard asks, in sequence: your personal API
token (find it in ClickUp → Settings → Apps → API Token), the workspace to use
(chosen among those visible to the token), an optional hourly rate, and the currency
(default `EUR`). The result is saved to `~/.config/clickup-cli/config.yml` and reused
on subsequent launches.

From the home screen pick a month and scope, then `Enter` generates the report. In the
report you can change the grouping, re-export, or go back home. If the token becomes
invalid or is revoked while in use, the TUI automatically re-runs the setup wizard.

### TUI commands

| Key | Screen | Action |
|---|---|---|
| `◂` / `▸` (left/right arrows, also `h`/`l`) | Home | Change month |
| `t` | Home | Toggle scope `me` / `team` |
| `Enter` | Home | Generate the report for the selected month/scope |
| `g` | Report | Cycle grouping: total → task → list → day → total |
| `e` | Report | Open the export menu (CSV/JSON/Markdown) |
| `m` / `s` | Report | Go back home to change month/scope |
| `r` | Report | Reload the time entries from the API for the same month/scope |
| `p` | Report | Open the **Per-list rates** screen |
| `n` | Home / Report | Open the **Log hours** screen (record time on ClickUp) |
| `↑`/`↓` (also `k`/`j`) | Export | Select the format |
| `Enter` | Export | Save `clickup-report-YYYY-MM.<ext>` in the cwd |
| `Esc` | Export | Return to the report without exporting |
| `q` | Everywhere except setup | Quit the application |
| `Ctrl+C` | Always | Quit the application |

The setup screen has no `q`-to-quit, to avoid pressing it by mistake while typing the
token: use `Ctrl+C`.

#### Per-list rates screen

From the report screen, pressing `p` opens the **Per-list rates** screen, where you can
configure a specific hourly rate for each list (different from the default). Available
commands:

- `↑` / `↓` (also `k` / `j`): navigate the lists
- `Enter`: edit the selected list's rate (digits and decimal separator only)
- `d`: reset the list to the default rate
- `s`: save changes and return to the report
- `Esc`: cancel (discard unsaved changes) and return to the report

Since v1.1, each amount is computed from the list's real hours multiplied by its specific
rate (not from the rounded hours), so a single amount may differ by a few cents from
`shown_hours × list_rate`; however, the billing total is always the exact sum of the
displayed amounts.

#### Log hours screen

Pressing `n` (from Home or Report) opens **Log hours**, to record time on your own
ClickUp tasks. Three modes:

1. **Guided** — pick a list among the known ones (current report ∪ config), then a task
   of that list, then fill in the form.
2. **Task ID/URL** — paste the task ID or a ClickUp URL (e.g. `.../t/86abc`) and go
   straight to the form.
3. **Timer** — start a stopwatch on the chosen task (guided or ID); pressing `s` stops it
   and ClickUp records the time entry. If a timer is already running when you open the
   screen, it is shown and you can stop it right away.

In the form, **duration** accepts flexible formats: `2h30`, `2h30m`, `1.5h`, `1,5h`,
`90m`, `45` (bare number = hours). The **date** defaults to today (`YYYY-MM-DD`, editable)
and the **note** is optional. After saving, press `r` to reload the report and see the new
hours immediately. You always log **your own** hours.

### Team scope

For the `team` scope the token must have Owner/Admin permissions on the workspace: without
them the API call fails and the error is shown on the error screen. In v1.0 the `team`
scope aggregates the hours of **all** members of the configured workspace (no per-member
selection — that is planned for a future version, v1.3).

## Configuration

Configuration persists in `~/.config/clickup-cli/config.yml` (it follows
`os.UserConfigDir()`, so it respects `XDG_CONFIG_HOME` on Linux):

```yaml
token: pk_xxx...
workspace_id: "123456"
currency: EUR
rate: 45
rates:
  "111": 60
  "222": 30
```

- `token`: personal ClickUp API token.
- `workspace_id`: id of the workspace (ClickUp team) chosen during setup.
- `currency`: currency used in the report and exports.
- `rate`: default hourly rate used to compute the billable amount.
- `rates` (optional): a `list_id: rate` map with per-list hourly rates. Lists not listed
  use the default `rate`. The map is conveniently filled from the TUI by pressing `p` on
  the report screen.

The `CLICKUP_TOKEN` environment variable, when set, always overrides the `token` read from
the config file (handy for CI or to avoid saving the token to disk):

```bash
CLICKUP_TOKEN=pk_xxx clickup
```

## License

MIT
