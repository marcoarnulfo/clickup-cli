# Design — Browser liste workspace (Space→Folder→List) (#13)

> Spec di progetto. Documento di design in italiano (convenzione `docs/superpowers/`).
> Tutto ciò che finisce nel codice (identificatori, commenti, stringhe UI, test) va in **inglese**.

## Obiettivo

Poter scegliere **qualunque** lista del workspace — non solo quelle note
(report ∪ config) — sia nel **log ore guidato** sia nella **schermata tariffe**,
navigando l'albero ClickUp **Space → Folder → List** in drill-down lazy.

Issue: [#13](https://github.com/marcoarnulfo/clickup-cli/issues/13).

## Contesto (stato attuale)

- Due picker mostrano solo le liste note, come lista piatta `{id, name}`:
  - **Log ore guidato** (`internal/tui/log.go`): step `logListPick`, `lists
    []taskListChoice`; alla selezione parte `listTasksCmd(listID)` →
    `taskListMsg` → step `logTaskPick`.
  - **Tariffe** (`internal/tui/rates.go`): `newRates(entries, cfg)` costruisce
    `rows []rateRow{listID, name}` da entries ∪ config.
- Il client (`internal/clickup`) ha `ListName`, `ListTasks`, ma **nessun**
  listing di Space/Folder/List.
- Pattern client: `c.get(ctx, path, query, &out)`; test con `httptest`
  (`c := New("tok"); c.BaseURL = srv.URL`). Cache nomi lista protetta da mutex.
- Pattern TUI: `Model` value-receiver, write-back esplicito; schermate
  `updateX`/`view`; lavoro async come `tea.Cmd` che ritorna msg tipizzati.

## Decisioni di design (dal brainstorming)

1. **Ambito**: sia log ore sia tariffe, tramite un **browser condiviso**.
2. **Navigazione**: **drill-down lazy** (Space → contenuti Space → liste folder),
   non lista piatta.
3. **Integrazione**: si mantiene il picker "liste note" veloce, con una voce
   **"Browse all workspace lists…"** (log) / un tasto **`b`** (tariffe) che apre
   il browser.

## Gerarchia e API ClickUp

- Workspace(team) → Spaces: `GET /team/{team_id}/space` → `{spaces:[{id,name}]}`.
- Space → Folders (con liste **incluse**): `GET /space/{space_id}/folder` →
  `{folders:[{id,name,lists:[{id,name}]}]}`.
- Space → liste **senza folder**: `GET /space/{space_id}/list` →
  `{lists:[{id,name}]}`.
- Poiché le liste dei folder arrivano **incluse** nella risposta folder,
  scegliendo un folder le sue liste sono già in memoria: **nessuna chiamata al
  terzo livello**. Per uno Space servono quindi **2 chiamate** (folder + liste
  folderless).

## Componenti e modifiche per package

### `internal/clickup` — nuovi endpoint

- Tipi: `Space{ID, Name string}`, `List{ID, Name string}`,
  `Folder{ID, Name string, Lists []List}`.
- `func (c *Client) Spaces(ctx, teamID string) ([]Space, error)` — `GET
  /team/{id}/space`.
- `func (c *Client) SpaceContents(ctx, spaceID string) (folders []Folder,
  folderless []List, err error)` — esegue `GET /space/{id}/folder` e `GET
  /space/{id}/list`; ritorna i folder (con le loro liste) e le liste senza
  folder. Errore su una delle due → errore complessivo.

### `internal/tui` — browser condiviso

**Nuovo file `internal/tui/listbrowser.go`** (`screenListBrowser`):

- `listBrowserModel`:
  - `origin screen` — chi ha aperto il browser (`screenLog` | `screenRates`);
    determina il write-back al ritorno.
  - `level` — `browseSpaces` | `browseSpaceContents` | `browseFolderLists`.
  - Items del livello corrente + indice; breadcrumb (nome Space / nome Folder).
  - `loading bool`.
  - Riferimenti ai dati del livello: `spaces []clickup.Space`; per lo Space
    corrente `folders []clickup.Folder` + `folderless []clickup.List`; per il
    Folder corrente le sue liste.
- Livello `browseSpaceContents`: mostra prima i **folder** (📁) poi le **liste
  folderless** (🗒); scegliendo un folder → `browseFolderLists`; scegliendo una
  lista folderless → **selezione**.
- Livello `browseFolderLists`: le liste del folder; scegliendo una → **selezione**.
- Navigazione: `↑/↓`, `Enter` entra/seleziona, `Esc` **risale** di un livello
  (dal livello Spaces torna al picker chiamante = `origin`).

**Cache di sessione** sul root `Model` (come `teamMembers`/`taskStatus`):
- `browserSpaces []clickup.Space` (fetch una volta).
- `browserContents map[string]browserSpaceContents` (per `spaceID`).
Il browser usa la cache se presente, altrimenti fa fetch.

**Fetch lazy** (`tea.Cmd` + msg tipizzati):
- `spacesMsg struct{ spaces []clickup.Space }`,
  `spaceContentsMsg struct{ spaceID string; folders []clickup.Folder; folderless []clickup.List }`.
- `loadSpacesCmd(c, teamID)`, `loadSpaceContentsCmd(c, spaceID)`; in demo
  `demoSpacesCmd()`, `demoSpaceContentsCmd(spaceID)`.
- Handler in `Update` popolano cache + livello e tolgono il loading.

**Selezione** (`selectBrowsedList(id, name string)` sul root, pointer receiver
per coerenza con `applyReport`/`assignStatuses`): instrada per `origin`:
- `screenLog`: imposta `m.logScreen.loading = true`, `m.screen = screenLog`,
  ritorna `listTasksCmd(id)` → entra nel normale flusso `logTaskPick`.
- `screenRates`: aggiunge una `rateRow{listID:id, name:name}` a `m.ratesScreen`
  (se non presente) e la seleziona (`idx`); `m.screen = screenRates`.

### Integrazione nei due picker

- **Log** (`log.go`, step `logListPick`): dopo le liste note si renderizza una
  riga extra **"🔍 Browse all workspace lists…"** (indice `len(lists)`). La
  navigazione arriva fino a quell'indice; `Enter` su di essa apre il browser
  con `origin = screenLog`; altrimenti comportamento attuale (`listTasksCmd`).
- **Tariffe** (`rates.go`): nuovo tasto **`b`** (quando non si sta editando una
  tariffa) apre il browser con `origin = screenRates`. Help aggiornato.
- **Apertura browser** (helper `m.openListBrowser(origin)`): se `browserSpaces`
  è in cache → `newListBrowser` popolato; altrimenti `loading` +
  `loadSpacesCmd`/`demoSpacesCmd`. `screen = screenListBrowser`.

### `app.go`

- `screenListBrowser` in coda all'iota; `browserScreen listBrowserModel` +
  cache fields sul root; `routeKey`/`View` wiring; handler `spacesMsg`/
  `spaceContentsMsg`.

## Gestione errori

- `loadSpacesCmd`/`loadSpaceContentsCmd` falliscono → `errMsg` (auth → wizard;
  altro → screenError), come le altre schermate di sola navigazione. Il browser
  non ha form da preservare.

## Demo mode

- `demoSpaces()` (es. 2 Space) e `demoSpaceContents(spaceID)` (folder con liste +
  liste folderless), senza I/O, così il browser è provabile in demo.

## Testing (TDD)

- `clickup`: `Spaces` (httptest, parsing `spaces[]`); `SpaceContents` (folder
  con liste incluse + liste folderless da due endpoint; errore se uno fallisce).
- `tui`: transizioni `listBrowserModel` (Spaces→contenuti→folder-lists; `Esc`
  risale; selezione lista chiama `selectBrowsedList` con id/name giusti);
  la voce "Browse all…" nel log apre il browser con `origin=screenLog`; il tasto
  `b` in tariffe apre con `origin=screenRates`; `spacesMsg`/`spaceContentsMsg`
  popolano livello e cache; la cache evita il refetch; `selectBrowsedList`
  instrada log vs rates (log → `listTasksCmd`/step task; rates → riga aggiunta e
  selezionata); demo.

## Composizione e invarianti

- Il browser è **un solo componente**; log e tariffe cambiano solo `origin` e il
  write-back. La cache di sessione è condivisa.
- Nessun impatto su range/filtri/membri: il browser produce solo una scelta di
  `(listID, listName)` per il chiamante.
- Purezza di `internal/report` invariata (il browser vive in `tui`/`clickup`).

## Fuori scope

- Ricerca/filtro testuale globale nel browser (resta drill-down puro).
- Liste archiviate; sotto-liste/viste; multi-select di liste.
- Persistenza della selezione o dell'albero su config.
