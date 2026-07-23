**English** · [Italiano](README.it.md)

# clup — ClickUp Hours CLI

[![CI](https://github.com/marcoarnulfo/clickup-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/marcoarnulfo/clickup-cli/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/marcoarnulfo/clickup-cli)](https://github.com/marcoarnulfo/clickup-cli/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/marcoarnulfo/clickup-cli)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

> A fast, colorful terminal TUI to pull your **monthly ClickUp hours** — self or team — compute the **billable amount**, and log time back to ClickUp. Free and open-source (MIT).

## Features

- 📊 **Monthly hours report** (self or whole team), grouped by total / task / list / day / member / tag.
- 💶 **Billing engine**: default, per-list, per-member and per-(list,member) hourly rate overrides, a billable/non-billable split, configurable rounding, and per-currency subtotals (multi-currency, no FX).
- 🎯 **Per-list budgets** with a burn-down view, so you can see at a glance how much of each project's budget is already billed.
- ⏱️ **Log hours** back to ClickUp from the TUI: guided (list → task), by task ID/URL, or with a start/stop timer.
- ⏲️ **Live timer & entry management**: a ticking Home indicator for a running timer, and a browser to edit, delete or inspect the history of past entries.
- 📤 **Export** to CSV / JSON / Markdown / self-contained HTML (print to PDF) / line-item CSV invoice.
- ⌨️ Fully interactive, keyboard-driven TUI (built with [Charm](https://charm.sh) bubbletea).
- 🔒 Token stays local (config file or `CLICKUP_TOKEN` env var).

## Demo

![clup demo](docs/demo.gif)

Try it yourself without a ClickUp account: **`CLICKUP_DEMO=1 clup`** runs a demo mode with
fixture data — including the billing model: a billable/non-billable split, two invoicing
currencies, tagged entries, and a per-list budget. The GIF is recorded with
[vhs](https://github.com/charmbracelet/vhs) from [`docs/demo.tape`](docs/demo.tape) (run
`vhs docs/demo.tape` to regenerate).

## Requirements

- **[Go](https://go.dev/dl/) 1.26 or newer** — only needed to install/build from source.
  - macOS: `brew install go` · Linux: [official install](https://go.dev/doc/install) · check with `go version`.
- A **ClickUp personal API token** (ClickUp → Settings → Apps → API Token).

## Installation

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest
```

This installs the `clup` binary into `$(go env GOPATH)/bin` (make sure it's on your `PATH`).

<details>
<summary>Build from source</summary>

```bash
git clone https://github.com/marcoarnulfo/clickup-cli.git
cd clickup-cli
go build -o clup ./cmd/clup
./clup
```
</details>

## Quick start

1. **Install** (see above) and run `clup`.
2. On first launch, the **setup wizard** asks for your API token, workspace, an optional hourly rate, and currency — saved to your config file (see [Configuration](#configuration) for the exact path).
3. Pick a **range** (`d`) and **scope** (`me`/`team`) on the home screen, press `Enter` → your report. Press `n` to log hours, `e` to export, `p` for billing settings (rates, currencies, budgets, rounding), `b` for the budget burn-down view.

## Usage

Run `clup`. On first launch a setup wizard asks, in sequence: your personal API
token (find it in ClickUp → Settings → Apps → API Token), the workspace to use
(chosen among those visible to the token), an optional hourly rate, and the currency
(default `EUR`). The result is saved to your config file (see
[Configuration](#configuration)) and reused on subsequent launches.

From the home screen pick a range and scope, then `Enter` generates the report. The
report is no longer limited to a calendar month: press `d` on the home screen to open
the **range picker**, which offers presets (this month, last month, last 7 days, last
30 days, this week) plus a **custom** `From`/`To` range (dates as `YYYY-MM-DD`). In the
report you can change the grouping, re-export, or go back home. If the token becomes
invalid or is revoked while in use, the TUI automatically re-runs the setup wizard.

### TUI commands

| Key | Screen | Action |
|---|---|---|
| `d` | Home | Open the **report range** picker (presets + custom from/to) |
| `◂` / `▸` (left/right arrows, also `h`/`l`) | Home | Change month (only while the `this month` range is active) |
| `w` | Home | Toggle the current ISO week |
| `t` | Home | Toggle scope `me` / `team` |
| `f` | Home | Open **member selection** (team scope): multi-select which members the report covers |
| `Enter` | Home | Generate the report for the selected range/scope |
| `g` | Report | Cycle grouping: total → task → list → day → tag → member (team) → total |
| `e` | Report | Open the export menu (CSV/JSON/Markdown/HTML/CSV invoice) |
| `m` / `s` | Report | Go back home to change range/scope |
| `r` | Report | Reload the time entries from the API for the same range/scope |
| `p` | Report | Open the **Billing settings** screen (rates, currencies, budgets, rounding, timezone) |
| `b` | Report | Open the **Budget burn-down** view |
| `f` | Report | Open the **Filters** screen (list/tag/status/billable) |
| `v` | Report | Open the **time-entry browser** (edit/delete/history) |
| `n` | Home / Report | Open the **Log hours** screen (record time on ClickUp) |
| `c` | Home | Jump to the running timer (shown only while one is active) |
| `↑`/`↓` (also `k`/`j`) | Export | Select the format |
| `Enter` | Export | Save `clickup-report-<period>.<ext>` in the cwd (the CSV invoice is saved as `clickup-invoice-<period>.csv`; `<period>` is `YYYY-MM` for a calendar month, or `YYYY-MM-DD_YYYY-MM-DD` for a custom range) |
| `Esc` | Export | Return to the report without exporting |
| `q` | Everywhere except setup / rates / range | Quit the application |
| `Ctrl+C` | Always | Quit the application |

The setup screen has no `q`-to-quit, to avoid pressing it by mistake while typing the
token: use `Ctrl+C`.

#### Billing settings screen

From the report screen, pressing `p` opens the **Billing settings** screen, with four
tabs (`Tab`/`Shift+Tab` to switch): **Lists** (per-list rate, currency and budget),
**Members** (per-member rate), **Overrides** (per-(list,member) rate — the most
specific level of the precedence) and **Rules** (default currency, rounding
increment/mode/scope, and timezone). Rate precedence, most specific first:
**(list, member) > member > list > default**. Available commands:

- `Tab` / `Shift+Tab`: switch tab
- `↑` / `↓` (also `k` / `j`): navigate the rows
- `Enter`: edit the selected row's rate (in Rules: edit the field, or toggle it for
  rounding mode/scope)
- `c` (Lists): edit the list's currency; `g` (Lists): edit the list's budget (submit an
  empty value to clear either)
- `n` (Overrides): create a new (list,member) override — pick the list, then the
  member, then type the rate
- `d`: clear the selected value, reverting to the next level of the precedence (a
  list's currency or budget is instead cleared via `c`/`g` with an empty value)
- Typing `0` for a rate is a different action from clearing it: `0` bills the list,
  member or pair at zero, while `d` clears the override so the inherited rate applies.
  A budget of `0` has no such meaning and stays rejected.
- `b` (Lists): open the **workspace list browser** to add a list not yet tracked
- `s`: save changes and return to the report
- `Esc`: cancel (discard unsaved changes) and return to the report

Since v1.1, each amount is computed from the exact billed duration multiplied by its
effective rate, never from an already-rounded hours value — see
[How billed amounts are computed](#how-billed-amounts-are-computed) for the full rule.

#### Budget burn-down view

Pressing `b` from the report screen opens the **Budget burn-down** view: one text
progress bar per list with a `billing.budgets` entry, sorted most-burned first. Each
bar shows the amount billed against the budget in the list's own currency (money, not
hours). Press `b` or `Esc` to return to the report.

#### Filters screen

From the report screen, pressing `f` opens the **Filters** screen, with four
sections: Lists, Tags, Statuses and Billable. The first three list the distinct
values found in the loaded entries; selecting one or more values in a section keeps
only the matching entries (OR within a section, AND across sections); leaving a
section empty means "no filter" for that dimension. Billable is different — a
single-choice toggle (**All** / **Billable only** / **Non-billable only**), one value
active at a time. Task statuses are not included in the initial API load, so the
first time you open Filters in a session the app fetches each loaded task's current
status from ClickUp (shown as "Loading statuses…"); after that it is cached for the
rest of the session. Filters compose with the team member selection and the active
date range — they only narrow what is already loaded. When the date range changes,
filter selections automatically adjust to the new entries: any selected value that no
longer occurs is dropped, so the report never gets stuck empty because of a stale
filter. Available commands:

- `Tab` / `Shift+Tab`: switch section
- `↑` / `↓` (also `k` / `j`): move within the section
- `Space`: toggle the highlighted value
- `a`: select/deselect all values in the section
- `Enter`: apply the filter and return to the report
- `Esc`: discard changes and return to the report

#### Live timer & entry management

When a timer is running (started from **Log hours**, see below), the Home screen
shows a live, ticking indicator — `⏱  running on <task> — HH:MM:SS  (X.XXh)` —
regardless of which screen started it, so you never lose track of it. Press `c`
on Home to jump straight to it and stop it.

From the report screen, pressing `v` opens the **time-entry browser**: the
current range's entries, newest first, navigable with `↑`/`↓` (also `k`/`j`).
Available commands:

- `e`: edit the highlighted entry's duration, date/time, note and billable
  flag — **only on your own entries**
- `x`: delete the highlighted entry, with a `[y/N]` confirmation — **only on
  your own entries**
- `h`: view the entry's change history (read-only) — available on **any**
  entry, not just your own
- `Esc`: return to the report

Edit and delete are ownership-gated: an entry logged by a teammate shows in the
browser (team scope) but `e`/`x` do nothing on it — only `h` works. Editing
tags from the browser is not yet supported.

#### Log hours screen

Pressing `n` (from Home or Report) opens **Log hours**, to record time on your own
ClickUp tasks. Three modes:

1. **Guided** — pick a list among the known ones (current report ∪ config), then a task
   of that list, then fill in the form. The list picker includes a "**Browse all workspace
   lists…**" entry that opens the workspace list browser, allowing you to navigate all
   spaces, folders, and lists in your workspace (not only recent or configured ones).
2. **Task ID/URL** — paste the task ID or a ClickUp URL (e.g. `.../t/86abc`) and go
   straight to the form.
3. **Timer** — start a stopwatch on the chosen task (guided or ID); pressing `s` stops it
   and ClickUp records the time entry. If a timer is already running when you open the
   screen, it is shown and you can stop it right away.

In the form, **duration** accepts flexible formats: `2h30`, `2h30m`, `1.5h`, `1,5h`,
`90m`, `45` (bare number = hours). The **date** defaults to today (`YYYY-MM-DD`, editable)
and the **note** is optional. Finally you set whether the entry is **billable** (`Y`/`n`,
default yes). After saving, press `r` to reload the report and see the new hours immediately.
You always log **your own** hours.

#### Workspace list browser

The workspace list browser (opened from **Log hours** guided mode or the **Billing settings** screen)
shows all spaces, folders, and lists in your workspace as a hierarchical drill-down:
start at the workspace root → select a space → drill down into folders within that space → pick a list.
Each space's folders and lists are fetched on first visit and cached for the session; opening a folder
needs no extra request (its lists come inline). Available commands:

- `↑` / `↓` (also `k` / `j`): move up/down in the current level
- `Enter`: enter/expand the highlighted space or folder; select the highlighted list
- `Esc`: go back one level (or return to the origin screen at the root level)

### Team scope

For the `team` scope the token must have Owner/Admin permissions on the workspace: without
them the API call fails and the error is shown on the error screen. The `team` scope
aggregates the hours of the workspace members; by default **all** members are included, but
you can press `f` from Home to open the member selection screen and pick individual members
(a partial selection shows a `(k/n members)` note in the report title).

### Headless report

`clup report` prints an hours report to stdout without starting the TUI — meant for
scripts, cron jobs, and agents. It reuses the same range/scope/grouping/billing logic
as the interactive report, but never touches the terminal UI.

```sh
clup report --month 2026-06 --scope me --format json
clup report --week 2026-W30 --billable --format csv-invoice > invoice.csv
```

Flags:

- `--month YYYY-MM` — report a calendar month (default: current month if no other range flag is given).
- `--week YYYY-Www` — report an ISO-8601 week (e.g. `2026-W30`); rejects a malformed
  value or a week number outside 1–53.
- `--from YYYY-MM-DD --to YYYY-MM-DD` — custom range, inclusive (given together).
- `--preset this_month|last_month|last_7d|last_30d|this_week` — same presets as the TUI's range picker.
- Range priority when more than one is given: `--month` > `--week` > `--from`/`--to` >
  `--preset` > current month (default).
- `--scope me|team` (default `me`).
- `--group total|task|list|day|member|tag` (default `total`).
- `--billable` — filter to billable entries only; pass `--billable=false` to keep only
  non-billable entries. Omitting the flag applies no filter.
- `--tag TAG` — filter to entries carrying this tag; repeatable (`--tag a --tag b`
  matches entries carrying *either* tag).
- `--tz IANA` — timezone for range boundaries and the report's `timezone` field
  (default: the config's `timezone`, else UTC — see [Configuration](#configuration)).
- `--format json|csv|md|html|csv-invoice` (default `json`).

All formats write to stdout — use shell redirection to save (e.g. `clup report --format csv > report.csv`).

Note: `CLICKUP_DEMO=1` is **ignored** by `report` — it always loads the real config and
calls the real API; demo mode is TUI-only.

The `--format json` output is a **stable scripting schema** (snake_case keys, RFC3339
timestamps) — safe to parse with `jq` and pin in scripts. It's additive and
non-breaking: the pre-v1.7 `rate` and `currency` fields are kept, now **deprecated**,
alongside the v1.7 additions `schema_version`, `timezone`, `currency_subtotals`,
`billable_hours`, `non_billable_hours`, `billed_hours` and `lines` (the
per-billing-unit invoice rows). New scripts should read `currency_subtotals`/`lines`
rather than the deprecated single-value `rate`/`currency`.

`--format html` writes a self-contained report: inline CSS, no external stylesheets,
fonts, scripts, or images. Open it in a browser and print to PDF for a shareable
document.

`--format csv-invoice` writes one row per billing unit (not per bucket), with columns
`date, list_id, client, user, description, qty_hours, rate, amount, currency, billable`
— `client` holds the ClickUp list's name (the closest a list-based tool has to a
client/project field). `qty_hours` is rendered at 6 decimal places on purpose, so every
row's `qty_hours × rate` reconciles to `amount` at cent precision — a 20-minute unit at
30/h invoices exactly 10.00, not 9.90.

## Configuration

Configuration persists under `os.UserConfigDir()` (so it respects
`XDG_CONFIG_HOME` on Linux): `~/Library/Application Support/clup/config.yml`
on macOS, `~/.config/clup/config.yml` on Linux. If that file doesn't exist yet,
the legacy pre-rebrand path (`~/.config/clickup-cli/config.yml` and its
per-OS equivalent) is still read as a fallback, so upgrading from an older
`clickup` install doesn't lose your settings.

```yaml
schema_version: 2
token: pk_xxx...
workspace_id: "123456"
currency: EUR
rate: 45
rates:
  "111": 60
  "222": 30
timezone: Europe/Rome
billing:
  default_currency: EUR
  rates_by_member:
    42: 60
  rate_overrides:
    - list: "111"
      member: 42
      rate: 70
  currencies:
    "111": EUR
    "222": USD
  budgets:
    "111": 2000
  rounding:
    increment: 15m
    mode: up
    scope: day
```

- `token`: personal ClickUp API token.
- `workspace_id`: id of the workspace (ClickUp team) chosen during setup.
- `currency`: currency used in the report and exports.
- `rate`: default hourly rate used to compute the billable amount.
- `rates` (optional): a `list_id: rate` map with per-list hourly rates. Lists not listed
  use the default `rate`. The map is conveniently filled from the TUI's Billing
  settings screen (`p` on the report screen). A rate of `0` (here or in `rates_by_member`/
  `rate_overrides` below) means the list/member/pair bills at zero — a deliberate value,
  not the same as omitting the entry (which falls back to the next level of the
  precedence).
- `schema_version`: written automatically on save — you never edit it by hand. A
  config file from before v1.7 (schema v1) is still read as-is, its existing
  `rate`/`rates`/`currency` values untouched, and gets stamped to v2 the next time it
  is saved.
- `timezone` (optional): an IANA zone name (e.g. `Europe/Rome`) anchoring the report's
  day/week/month boundaries. Two tracks: the **TUI** uses it, falling back to the
  machine's local zone when unset (and then shows its zone as `Local`, not an IANA
  name); the headless `clup report` always defaults to **UTC** unless overridden by
  `--tz` or this field. Setting it explicitly is recommended; it can also be edited
  from the TUI's Billing settings screen.
- `billing` (optional, v1.7): additive to `rate`/`rates`/`currency` above — none of
  those change meaning.
  - `default_currency`: fallback ISO currency for lists not listed in `currencies`
    (falls back further to the top-level `currency` if unset).
  - `rates_by_member`: `user_id: rate` — a per-member hourly rate.
  - `rate_overrides`: a list of `{list, member, rate}` — the most specific rate, for
    one member on one list. Rate precedence, most specific first:
    **(list, member) > member > list > default**.
  - `currencies`: `list_id: ISO code` — bill each list in its own currency. Subtotals
    are always per currency and never summed across currencies (no FX); a single
    overall total is shown only when exactly one currency carries money (other
    currencies may still appear with non-billable hours only).
  - `budgets`: `list_id: amount` — a money budget per list, checked against **billed
    amounts** (not hours) and rendered as a burn-down bar in the TUI (`b` from the
    report screen).
  - `rounding`: rounds billable hours before invoicing; non-billable hours are never
    rounded.
    - `increment`: a human duration (`15m`, `1h`, `2h30`); empty (the default) means
      rounding is off. **A non-empty value that fails to parse is a hard error**, not
      a silent "off" — a typo here must never quietly under-round and over-bill.
    - `mode`: `up` rounds up; any other value (including empty/omitted) rounds to the
      nearest increment.
    - `scope`: `day` rounds the total per (day, list, member) instead of per entry;
      any other value rounds each entry individually.
- `update_check` (optional): set to `false` to turn off the update check described
  below. Omitting the key (or setting `true`) leaves it enabled.

### How billed amounts are computed

The amount of a billing unit — one billable entry, or one (day, list, member) group
when `rounding.scope: day` — is rounded to 2 decimals from its *exact* billed duration
times its rate, never from an already-rounded hours value. Every total (a bucket, a
currency subtotal, an invoice line) is then a sum of already-rounded unit amounts, so
the invoice CSV, the JSON `currency_subtotals`, and the HTML export always agree to the
cent. The one place this isn't true is a report grouped *finer* than the billing unit
(e.g. per-task with day-scoped rounding): a bucket's own amount there is an
**indicative** proportional split of its unit(s) and may drift a cent or two — the
currency subtotals and invoice lines (`--format csv-invoice`, or the `lines` field in
the JSON output) are always the authoritative totals.

The `CLICKUP_TOKEN` environment variable, when set, always overrides the `token` read from
the config file (handy for CI or to avoid saving the token to disk):

```bash
CLICKUP_TOKEN=pk_xxx clup
```

### Update check

Once a day, `clup` asks GitHub whether a newer release exists and, if so, shows a
short notice. It is deliberately narrow in what it does:

- **Anonymous.** It's a single, 2-second-timeout GET to the public
  `https://api.github.com/repos/marcoarnulfo/clickup-cli/releases/latest` endpoint,
  sending only `Accept` and `User-Agent` headers. There is no `Authorization` header
  — your ClickUp token never travels to GitHub.
- **No self-update.** `clup` never downloads or replaces its own binary; the notice
  only tells you a newer version exists and points at
  `go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest`.
- **Cached.** The result is stored at `os.UserCacheDir()/clup/update.json` and
  reused for 24 hours, so most runs make no network call at all.
- **Most source builds are exempt.** If you built `clup` yourself with a plain
  `go build`, the binary reports a pseudo-version rather than a numbered release
  and the check never runs — unless the checkout is clean and sitting exactly on
  a release tag, in which case it reports that exact version and the check
  behaves as it would for any release build. Extra commits past the tag, or a
  dirty tree (`+dirty`), are what keep it silent.
- **Where it shows up:** as an extra line on the TUI's home screen, and for
  `clup report`, as a line on **stderr** printed after the report body — never on
  stdout, so `clup report --format json` stays parsable by downstream tools.
- **Opt out** with `CLUP_NO_UPDATE_CHECK=1` (any non-empty value) or with
  `update_check: false` in the config; the environment variable always wins over
  the config. Omitting the key leaves the check enabled. Demo mode
  (`CLICKUP_DEMO=1`) also disables it — but for the **TUI only**; `clup report`
  ignores `CLICKUP_DEMO` and checks like any other run.

## Contributing

Contributions are very welcome — this is a free, open-source project. See
**[CONTRIBUTING.md](CONTRIBUTING.md)** for how to set up the dev environment, run the
tests, and open a PR. New here? Look for the
[`good first issue`](https://github.com/marcoarnulfo/clickup-cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
label. Please also read the [Code of Conduct](CODE_OF_CONDUCT.md).

## Roadmap

The north star is to grow from a monthly hours-reporting tool into a **complete, beautiful
ClickUp terminal client** — keeping **time tracking & billing as the flagship** (no other
tool offers per-list/member rates, budgets and report export in a TUI).

The full plan lives in **[GitHub Issues](https://github.com/marcoarnulfo/clickup-cli/issues)**,
tracked by the **[🗺️ Roadmap epic #33](https://github.com/marcoarnulfo/clickup-cli/issues/33)**
and organized into milestones:

| Milestone | Focus |
|---|---|
| [v1.6 — Rebrand & foundations](https://github.com/marcoarnulfo/clickup-cli/milestone/4) | rebrand to `clup`, service layer, rate limiter, `report --json` |
| [v1.7 — Billing depth](https://github.com/marcoarnulfo/clickup-cli/milestone/5) | billable split, per-member & per-pair rates, rounding, multi-currency, budgets & burn-down, HTML/CSV-invoice export |
| [v1.8 — Live time tracking](https://github.com/marcoarnulfo/clickup-cli/milestone/6) | live timer, edit/delete entries |
| [v1.9 — TUI design system](https://github.com/marcoarnulfo/clickup-cli/milestone/7) | themes, tables, command palette, accessibility |
| [v1.10 — Task context & accounts](https://github.com/marcoarnulfo/clickup-cli/milestone/8) | search, my-tasks, task detail, keychain, profiles |
| [v1.11 — Task management](https://github.com/marcoarnulfo/clickup-cli/milestone/9) | create/update tasks, comments, checklists |
| [v1.12 — Navigation, views & presets](https://github.com/marcoarnulfo/clickup-cli/milestone/10) | spaces/lists, saved views, report presets |
| [v1.13 — Docs, Goals & Sprints](https://github.com/marcoarnulfo/clickup-cli/milestone/11) | ClickUp Docs, goals, sprints |
| [v2.0 — Git & AI](https://github.com/marcoarnulfo/clickup-cli/milestone/3) | git integration, `--jq`/`--template`, MCP, skill files |
| [Distribution & packaging](https://github.com/marcoarnulfo/clickup-cli/milestone/12) | goreleaser, Homebrew, completions, man page |
| [Docs & website](https://github.com/marcoarnulfo/clickup-cli/milestone/13) | landing page, docs site, screenshots |

**Out of scope:** fiscal invoicing (VAT, invoice numbering, legal PDF) — too country-specific;
the tool produces shareable pre-invoice reports instead.

## License

[MIT](LICENSE)
