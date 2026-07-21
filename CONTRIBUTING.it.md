[English](CONTRIBUTING.md) · **Italiano**

# Contribuire a clickup-cli

Grazie per l'interesse — i contributi di ogni dimensione sono benvenuti! Segnalazioni
di bug, documentazione e codice sono tutti apprezzati. Il progetto è libero e
open-source (MIT).

Partecipando accetti il nostro [Codice di Condotta](CODE_OF_CONDUCT.md).

## Modi per contribuire

- 🐛 **Segnala un bug** o 💡 **proponi una feature** dalle [Issue](https://github.com/marcoarnulfo/clickup-cli/issues) (ci sono i template).
- 🧑‍💻 **Apri una PR** — alle prime armi? Cerca la label [`good first issue`](https://github.com/marcoarnulfo/clickup-cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).
- 📖 Migliora la documentazione (il README è bilingue: inglese `README.md` + `README.it.md`).

Per qualcosa di non banale, **apri prima una issue** così ci allineiamo sull'approccio prima che tu ci investa tempo.

## Requisiti

- **[Go](https://go.dev/dl/) 1.26+** (`go version` per verificare).
- [`staticcheck`](https://staticcheck.dev) per il lint (opzionale in locale, gira in CI):
  `go install honnef.co/go/tools/cmd/staticcheck@latest`.

## Setup di sviluppo

```bash
git clone https://github.com/marcoarnulfo/clickup-cli.git
cd clickup-cli
go build ./...
go run ./cmd/clickup   # esecuzione locale
```

Per provarla su dati reali, passa un token via env (evita di scriverlo su disco):

```bash
CLICKUP_TOKEN=pk_xxx go run ./cmd/clickup
```

## Prima di aprire una PR

Lancia gli stessi controlli della CI — devono essere tutti puliti/verdi:

```bash
gofmt -l .                                          # nessun output = formattato
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go test ./... -race
go build ./...
```

## Struttura del progetto e convenzioni

```
cmd/clickup         entry point
internal/config     config (YAML + env CLICKUP_TOKEN)
internal/clickup    client ClickUp API v2 (solo net/http)
internal/report     DOMINIO PURO: aggregazione ore (nessun I/O, nessuna dip. esterna)
internal/duration   PURO: parser durata umana (2h30, 1.5h, 90m)
internal/export     export CSV/JSON/Markdown
internal/tui        TUI bubbletea: un file per schermata
```

- **`internal/report` e `internal/duration` restano puri** — nessun I/O, nessun import di `config`/`clickup`. La logica di dominio va lì e si testa senza mock.
- **La TUI segue il pattern Elm** (bubbletea): `Model` a value-receiver, `updateX`/`view` per schermata, write-back esplicito. Le chiamate API girano come `tea.Cmd` che ritornano messaggi tipizzati.
- **Gotcha API ClickUp:** header auth `Authorization: <token>` **senza** `Bearer`; durate/epoch in **millisecondi**.
- **Test:** table-driven; `httptest` per il client; la TUI si testa via `Update()` + messaggi simulati. Segui il **TDD** (test prima) dove possibile.

## Linee guida per commit e PR

- Usa i **[Conventional Commits](https://www.conventionalcommits.org)** (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`…).
- Mantieni le PR focalizzate; compila il template della PR e collega la issue (`Closes #N`).
- Assicurati che i controlli sopra passino prima di chiedere la review.
- Aggiorna la documentazione (entrambe le lingue del README, se rilevante) quando cambia il comportamento.

Grazie per aiutare a migliorare clickup-cli! 🙌
