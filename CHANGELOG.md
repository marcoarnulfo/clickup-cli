# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Billing engine: per-list, per-member and per-(list, member) hourly rate overrides,
  with precedence (list, member) > member > list > default.
- Per-list budgets (`billing.budgets`), checked against billed amounts, with a
  **Budget burn-down** view in the TUI (`b` from the report screen).
- Billable/non-billable split: entries carry a billable flag; the report can be
  filtered to one or the other (Filters screen, `--billable`/`--billable=false`).
- Tag filtering and grouping: filter by tag (Filters screen, `--tag`, repeatable) and
  group the report by tag (new `tag` step in the `g` grouping cycle, `--group tag`).
- Multi-currency billing: bill each list in its own currency (`billing.currencies`),
  with per-currency subtotals (no FX conversion; a single overall total is shown only
  when the whole report is single-currency).
- Configurable rounding of billable hours before invoicing (`billing.rounding`:
  increment, up/nearest mode, per-entry or per-day scope); non-billable hours are
  never rounded.
- Timezone-aware reporting: an optional `timezone` config field and a `--tz` flag
  anchor day/week/month boundaries to an IANA zone (headless `clup report` still
  defaults to UTC unless overridden).
- ISO-8601 week reporting: `--week YYYY-Www` and a `w` week toggle in the TUI.
- New **Billing settings** screen (`p` from the report screen), replacing the old
  per-list rates screen, with four tabs: Lists (rate, currency, budget), Members
  (per-member rate), Overrides (per-(list, member) rate) and Rules (default currency,
  rounding, timezone).
- Export to self-contained HTML (`--format html`), suitable for printing to PDF, and
  to a line-item CSV invoice (`--format csv-invoice`, one row per billing unit, hours
  rendered at 6 decimal places so `qty_hours × rate` reconciles to `amount` exactly).
- Demo mode (`CLICKUP_DEMO=1`) covers the new billing model with no API calls:
  billable/non-billable split, multiple currencies, tags and a budget.

### Changed
- **Config schema v2 (additive):** a new `billing` block (rate overrides, currencies,
  budgets, rounding) and a `timezone` field are added alongside the existing
  `rate`/`rates`/`currency` fields, which keep their current meaning. A schema v1
  config file is still read as-is and is stamped to `schema_version: 2` the next
  time it is saved — nothing needs to change by hand to keep using it.
- `clup report`'s JSON output gains `schema_version`, `timezone`,
  `currency_subtotals`, `billable_hours`, `non_billable_hours`, `billed_hours` and
  `lines` (per-billing-unit invoice rows); it stays additive and backward-compatible.

### Deprecated
- `clup report`'s JSON top-level `rate` and `currency` fields are kept for
  compatibility but deprecated in favor of `currency_subtotals`/`lines`; new scripts
  should read the latter.

## [1.6.0] - 2026-07-22

### Changed
- **Rebranded the binary to `clup`** (`go install .../cmd/clup@latest`); this is the
  release everyone upgrading from an older `clickup` install should read.
- Config moved to a `clup` config directory, with a **permanent read-fallback** to
  the legacy `clickup-cli` path — existing configs keep working and are migrated on
  first save; a `schema_version` field was added.
- Unified the HTTP client into a single `do()` with a shared rate limiter (90/min,
  burst 30) that honors `Retry-After` and retries 429s on all verbs.
- Extracted the report I/O pipeline into `internal/service`, shared by the TUI and
  the new headless report.
- API errors now route back to Home with an inline message instead of a dead-end
  error screen; a 401 still relaunches the setup wizard.

### Added
- Headless report: `clup report --month YYYY-MM --scope me|team --format
  json|csv|md` prints a report to stdout for cron jobs, scripts and agents; the
  JSON output is a stable, `jq`-friendly scripting schema. `CLICKUP_DEMO` is
  ignored by `report`.
- `FilteredTeamTasks` (paginated `GET /team/{id}/task`) with parallelized status
  enrichment.

### Fixed
- Demo mode no longer builds the client with the real token instead of the demo
  token.
- The setup wizard no longer writes an empty token to disk when the typed token
  equals `CLICKUP_TOKEN`.
- The log-hours flow in demo mode is now fully I/O-free (no more accidental real
  API calls under `CLICKUP_DEMO=1`).

### Deprecated
- The old `clickup` binary is kept as a **deprecated shim**: it still works,
  forwards to `clup`, and prints a warning. It will be removed in a future release.

## [1.5.0] - 2026-07-21

### Added
- Workspace list browser: pick any list in the workspace, not just recent/config
  ones, via a lazy Space → Folder → List drill-down. Opened from the guided log
  picker or with `b` on the per-list rates screen.

### Changed
- List-name resolution during report load is now done concurrently (bounded),
  instead of one sequential `GET /list/{id}` per list.
- The custom date-range editor prefills From/To and supports `Tab`/`Shift+Tab`;
  report filters automatically adjust when the date range changes.

## [1.4.0] - 2026-07-21

### Added
- Team member selection (`f` on Home, team scope): multi-select which workspace
  members the report covers, instead of always aggregating everyone; new
  per-member grouping in the `g` cycle.
- Custom date range (`d` on Home): pick a preset (This month, Last month, Last 7
  days, Last 30 days, This week) or a custom From/To range; the report is no
  longer limited to a whole calendar month.
- Report filters (`f` on the Report): a sectioned Lists / Tags / Statuses filter
  (OR within a dimension, AND across dimensions), composable with member selection
  and the date range.

### Changed
- Per-list rates, amounts and grouping now recompute over the selected/filtered
  set. Demo mode honors ranges, filters and statuses with no API calls.

## [1.3.0] - 2026-07-21

### Changed
- The whole terminal UI, and error/export output, is now in English.
- Modernized parts of the codebase to use `slices`/`cmp` instead of `sort`; added
  `staticcheck` to CI.

### Added
- Demo mode: `CLICKUP_DEMO=1 clickup` runs the TUI on fixture data, no ClickUp
  account needed; powers the README demo GIF.
- Revamped bilingual README, `CONTRIBUTING`, `CODE_OF_CONDUCT`, issue/PR templates
  and `good first issue` labels.

## [1.2.0] - 2026-07-21

### Added
- Log hours from the TUI (`n` from Home and Report): create time entries on
  ClickUp directly from the terminal, in three modes — Guided (list → task),
  Task ID/URL, and Timer (start/stop, with an in-progress timer shown on open).
- A flexible duration parser (`2h30`, `2h30m`, `1.5h`, `1,5h`, `90m`, `45` = hours)
  and a today-by-default, editable date for the log-hours form.
- Bilingual documentation: English-primary `README.md` plus `README.it.md`.

---

Releases before 1.2.0 predate this changelog; see the
[GitHub releases](https://github.com/marcoarnulfo/clickup-cli/releases) page.
