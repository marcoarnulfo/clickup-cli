**English** · [Italiano](CONTRIBUTING.it.md)

# Contributing to clickup-cli

Thanks for your interest — contributions of all sizes are welcome! Bug reports,
docs, and code are all appreciated. This project is free and open-source (MIT).

By participating you agree to our [Code of Conduct](CODE_OF_CONDUCT.md).

## Ways to contribute

- 🐛 **Report a bug** or 💡 **propose a feature** via [Issues](https://github.com/marcoarnulfo/clickup-cli/issues) (templates provided).
- 🧑‍💻 **Send a PR** — new here? Look for the [`good first issue`](https://github.com/marcoarnulfo/clickup-cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) label.
- 📖 Improve the docs (the README is bilingual: English `README.md` + `README.it.md`).

For anything non-trivial, please **open an issue first** so we can align on the approach before you invest time.

## Prerequisites

- **[Go](https://go.dev/dl/) 1.26+** (`go version` to check).
- [`staticcheck`](https://staticcheck.dev) for linting (optional locally, run in CI):
  `go install honnef.co/go/tools/cmd/staticcheck@latest`.

## Development setup

```bash
git clone https://github.com/marcoarnulfo/clickup-cli.git
cd clickup-cli
go build ./...
go run ./cmd/clup   # run locally
```

To try it against real data, set a token via env (avoids writing it to disk):

```bash
CLICKUP_TOKEN=pk_xxx go run ./cmd/clup
```

## Before opening a PR

Run the same checks CI runs — all must be clean/green:

```bash
gofmt -l .                                          # no output = formatted
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go test ./... -race
go build ./...
```

## Project layout & conventions

```
cmd/clup            entry point (binary: clup)
cmd/clickup         deprecated shim, forwards to cmd/clup with a warning
internal/config     config (YAML + CLICKUP_TOKEN env)
internal/clickup    ClickUp API v2 client (net/http only)
internal/report     PURE domain: hours aggregation (no I/O, no external deps)
internal/duration   PURE: human-duration parser (2h30, 1.5h, 90m)
internal/export     CSV/JSON/Markdown export
internal/tui        bubbletea TUI: one file per screen
```

- **Keep `internal/report` and `internal/duration` pure** — no I/O, no imports of `config`/`clickup`. Domain logic goes there and is unit-tested without mocks.
- **TUI follows the Elm pattern** (bubbletea): value-receiver `Model`, per-screen `updateX`/`view`, explicit write-back. API calls run as `tea.Cmd` returning typed messages.
- **ClickUp API gotchas:** auth header is `Authorization: <token>` **without** `Bearer`; durations/epochs are **milliseconds**.
- **Tests:** table-driven; `httptest` for the client; TUI tested via `Update()` + simulated messages. Follow **TDD** (test first) where practical.

## Commit & PR guidelines

- Use **[Conventional Commits](https://www.conventionalcommits.org)** (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`…).
- Keep PRs focused; fill in the PR template and link the issue (`Closes #N`).
- Make sure the checks above pass before requesting review.
- Update docs (both README languages, if relevant) when behavior changes.

Thank you for helping make clickup-cli better! 🙌
