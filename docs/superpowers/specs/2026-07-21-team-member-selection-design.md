# Design — Selezione puntuale membri team (#3, milestone v1.4)

> Spec di progetto. Documento di design in italiano (convenzione `docs/superpowers/`).
> Tutto ciò che finisce nel codice (identificatori, commenti, stringhe UI, test) va in **inglese**.

## Obiettivo

Nello scope `team`, permettere all'utente di selezionare **quali** membri del
workspace includere nel report (multi-select), invece di aggregare sempre tutti.
Come complemento, aggiungere il raggruppamento del report **per membro**.

Issue: [#3](https://github.com/marcoarnulfo/clickup-cli/issues/3) — milestone `v1.4`.

## Contesto (stato attuale)

- Lo scope vive sul root `Model` (`me` | `team`), scelto nella Home con `t`.
- `Enter` in Home → `screenLoading` → `loadEntriesCmd`.
- `loadEntriesCmd` (in `internal/tui/app.go`): quando `scope=="team"` chiama
  `Client.Teams(ctx)`, trova il team con `cfg.WorkspaceID`, raccoglie **tutti**
  gli id dei membri e li passa come `assignees` a `Client.TimeEntries`.
- `Client.TimeEntries(ctx, teamID, start, end, assignees)` (in
  `internal/clickup/timeentries.go`): se `assignees` è non vuoto, filtra
  server-side via query param `assignee` (lista di id separati da virgola).
- Il dominio `internal/report` è **puro**. `Build` raggruppa via `keyFor` sui modi
  `total | task | list | day`. Ogni `TimeEntry` ha già `UserID` e `UserName`
  (oggi non usati per il raggruppamento).
- `internal/clickup/teams.go`: `Team{ID, Name, Members []Member}`,
  `Member{ID int, Username string}`.
- Demo mode (`internal/tui/demo.go`): `demoEntries` genera 6 entry tutte con
  `UserID: 1, UserName: "demo"`; nessuna API.

## Decisioni di design (dal brainstorming)

1. **Flusso**: schermata dedicata raggiunta dalla Home (non picker inline).
2. **Group-by member**: sì, aggiungere `GroupByMember` al ciclo `g`.
3. **Default / vuoto**: all'apertura tutti selezionati; se l'utente deseleziona
   tutti, il report considera comunque **tutti** i membri (vuoto = nessun filtro).
4. **Persistenza**: solo di sessione (stato sul root `Model`), nessuna scrittura
   su config.

## Architettura e flusso

```
Home (scope=team)
  │  premo `f`
  ▼
screenMembers  ──(loadMembersCmd, se membri non in cache)──▶ membersMsg
  │  Space toggle · a: all/none · Enter conferma · Esc annulla
  ▼
Home (Scope: team · Members: 2/3)
  │  premo Enter
  ▼
screenLoading ──(loadEntriesCmd con assignees selezionati)──▶ entriesMsg ─▶ Report
```

Con `scope=me` il tasto `f` è inattivo (la selezione membri non ha senso).

### Stato sul root `Model`

Nuovi campi:

- `teamMembers []clickup.Member` — lista membri del workspace, cache di sessione
  (popolata al primo ingresso nella schermata Members).
- `selectedMembers map[int]bool` — set degli id selezionati. Semantica **vuoto =
  tutti**: una mappa vuota (o tutti `false`) significa "nessun filtro".

`selectedMembers` sopravvive ai reload (`r` nel report) perché vive sul root.

### Comando di caricamento membri

`loadMembersCmd(c *clickup.Client, teamID string) tea.Cmd` → esegue
`c.TeamMembers(ctx, teamID)` in goroutine e ritorna `membersMsg{members}` o
`errMsg`. In demo mode si usa `demoMembersCmd()` (nessuna I/O), analogamente a
`reloadEntriesCmd`.

## Componenti e modifiche per package

### `internal/clickup`

**`teams.go`** — nuova funzione:

```go
// TeamMembers returns the members of the given workspace (team) id.
// Errors if the workspace is not accessible with the token.
func (c *Client) TeamMembers(ctx context.Context, teamID string) ([]Member, error)
```

Estrae la logica "chiama `Teams`, trova il team con quell'id, ritorna i membri"
oggi inline in `loadEntriesCmd`. Se il team non è trovato → errore
`"workspace %s not found or not accessible with this token"` (stesso messaggio
odierno).

**`timeentries.go`** — invariato (il filtro `assignee` esiste già).

### `internal/report` (puro)

**`aggregate.go`**:

- Nuova costante `GroupByMember = "member"`.
- `keyFor`: `case GroupByMember: return e.UserName`.
- `nextGroupBy` diventa **scope-aware** — nuova firma
  `nextGroupBy(g, scope string) string`:
  - scope `team`: `total → task → list → day → member → total`.
  - altri scope (`me`): `total → task → list → day → total` (salta `member`).

`Build` non cambia firma: raggruppa su qualunque `groupBy` gli venga passato,
incluso `member`.

### `internal/tui`

**Nuovo file `members.go`** — schermata di selezione:

- `type membersModel struct { members []clickup.Member; selected map[int]bool; idx int; loading bool }`
- `newMembers(members []clickup.Member, selected map[int]bool) membersModel` —
  costruisce la vista dallo stato del root (copia difensiva della selezione, così
  `Esc` può scartare senza toccare il root).
- `updateMembers(msg tea.KeyMsg) (tea.Model, tea.Cmd)` su `Model`:
  - `up`/`k`, `down`/`j`: muove `idx`.
  - `space`: toggle del membro corrente in `selected`.
  - `a`: se **tutti** selezionati → deseleziona tutti; altrimenti seleziona tutti.
  - `enter`: scrive `m.selectedMembers = sotto.selected`, torna a `screenHome`.
  - `esc`: scarta, torna a `screenHome` senza modificare `m.selectedMembers`.
- `view()`: lista con `[x]`/`[ ]`, riga corrente evidenziata, help in fondo.
  Stato `loading` → "Loading members…".

**`app.go`**:

- Nuovo `screen`: `screenMembers` (aggiunto **in coda** all'iota, per non
  riordinare le costanti esistenti).
- Nuovo msg tipizzato `membersMsg struct{ members []clickup.Member }`.
- `Update`: `case membersMsg` → `m.teamMembers = msg.members`; se
  `m.selectedMembers` è vuota, inizializzala con **tutti** i membri selezionati
  (default all); `m.membersScreen = newMembers(...)`; `m.membersScreen.loading =
  false`; `m.screen = screenMembers`.
- `routeKey`: `case screenMembers: return m.updateMembers(msg)`.
- `View`: `case screenMembers: return m.membersScreen.view()`.
- `q` per uscire resta abilitato su `screenMembers` (come le altre schermate;
  solo setup e rates lo escludono).
- `loadEntriesCmd` — nuova firma
  `loadEntriesCmd(c, teamID, year, month, scope string, assignees []int)`:
  - se `scope=="team"` e `assignees` è vuoto → `c.TeamMembers(...)` e usa **tutti**
    gli id (fallback attuale).
  - se `assignees` non vuoto → usa direttamente quelli (salta la chiamata Teams).
  - risoluzione dei `ListName` invariata.
- `reloadEntriesCmd` calcola gli `assignees` da `m.selectedMembers`
  (`selectedAssignees()` helper: gli id `true`; slice vuota se nessuno) e li passa.

**`home.go`**:

- `updateHome`: nuovo `case "f"` → **solo se `scope=="team"`**: se `teamMembers`
  è già in cache apre subito `screenMembers` (con `newMembers`), altrimenti
  `m.screen = screenMembers`, `m.membersScreen.loading = true` e ritorna
  `loadMembersCmd(...)` (o `demoMembersCmd` in demo).
- `view`: quando `scope=="team"` mostra il conteggio selezione
  `Scope: team · Members: k/n` (dove `n = len(teamMembers)`, `k` = selezionati;
  se `teamMembers` non ancora caricati mostra solo `Scope: team`). Help aggiornato
  con `f: select members`.

**`report.go`**:

- `updateReport` `case "g"`: usa `nextGroupBy(m.report.GroupBy, m.scope)`.
- Titolo: se `scope=="team"` e la selezione è parziale
  (`0 < k < n`), mostrare `scope team (k/n members)`; altrimenti `scope team`.

**`demo.go`**:

- `demoEntries`: distribuire le entry su **3 utenti** (es. `alice`, `bob`,
  `carol`, id 1/2/3) mantenendo liste/task attuali, così group-by-member e
  selezione sono significativi.
- `demoMembers() []clickup.Member` — i 3 membri demo.
- `demoMembersCmd() tea.Cmd` → `membersMsg{members: demoMembers()}`.

## Gestione errori

- `loadMembersCmd` fallisce → `errMsg`; `clickup.ErrUnauthorized` rilancia il
  wizard di setup (comportamento globale esistente), altri errori →
  `screenError`. La schermata Members non ha bisogno di logica d'errore propria.

## Testing (TDD, table-driven)

**`internal/report/aggregate_test.go`**:

- `keyFor` con `GroupByMember` → ritorna `UserName`.
- `Build` con `groupBy="member"`: due utenti → due bucket, ordinati per ore desc.
- `nextGroupBy` scope-aware: in `team` il ciclo include `member` tra `day` e
  `total`; in `me` da `day` torna a `total`.

**`internal/clickup/teams_test.go`** (nuovo o esistente):

- `TeamMembers` con `httptest`: risposta `/team` multi-team → ritorna i membri del
  team richiesto; team id assente → errore.

**`internal/tui`** (`members_test.go` nuovo + estensioni):

- `updateMembers`: `space` toggla, `a` all/none, `enter` scrive la selezione sul
  root e torna a Home, `esc` la scarta.
- `f` da Home: in `scope=team` apre `screenMembers`; in `scope=me` è no-op.
- `membersMsg`: popola `teamMembers`, default all quando selezione vuota.
- `Enter` in Home con selezione parziale: gli `assignees` passati riflettono i
  membri selezionati (verifica via il comando / stato, senza rete).
- Ciclo `g` scope-aware via `Update()`.

## Fuori scope

- Persistenza della selezione su `config.yml`.
- Filtri per lista/progetto, tag, status e range di date custom → issue **#2**,
  ciclo successivo della v1.4.
- Selezione di membri esterni al workspace configurato.
