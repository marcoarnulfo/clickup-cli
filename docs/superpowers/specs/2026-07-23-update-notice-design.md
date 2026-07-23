# Avviso di aggiornamento (#104) — design

> Spec per l'issue [#104](https://github.com/marcoarnulfo/clickup-cli/issues/104)
> (milestone *Distribution & packaging*). I file di community (#108) hanno una spec
> separata: condividono solo il tema "release", non il codice.

## 1. Obiettivo

Dire all'utente che esiste una release più recente, senza farsi sentire: nessun
rallentamento percepibile, nessun rumore negli script, nessun download.

**Non** è un self-update: il tool non scarica né sostituisce mai il proprio binario.
Suggerisce il comando di aggiornamento e basta.

## 2. Decisioni prese (owner)

- **Superfici:** TUI *e* CLI. L'avviso della CLI va su **stderr**, mai su stdout.
- **Cadenza:** al massimo un controllo ogni **24 ore**, con cache su disco.
- **Opt-out:** variabile d'ambiente **e** chiave di config; attivo di default.
- **Rimandati:** goreleaser/binari (#97), tap Homebrew (#99), Scoop/AUR/Nix (#103).

Lo stderr non è un dettaglio estetico: `clup report --json` è il gancio headless
documentato nella spec v1.7, e un avviso su stdout romperebbe ogni `jq` a valle.

## 3. Architettura

Si rispetta la separazione del repo — logica pura nei suoi package, I/O in
`internal/service`:

| Package | Natura | Responsabilità |
|---|---|---|
| `internal/version` (nuovo) | **puro** | forma di una versione e confronto |
| `internal/service/update.go` (nuovo) | impuro | versione corrente, cache su disco, chiamata HTTP |
| `internal/cli/report.go` | impuro | stampa l'avviso su stderr |
| `internal/tui` | impuro | `tea.Cmd` asincrono + riga sulla home |

`internal/version` non importa nulla oltre alla stdlib e non fa I/O.

## 4. Che cos'è "la versione corrente"

`internal/cli/cli.go:18` ha oggi `const version = "dev"`, mai valorizzata: `clup --version`
mente a tutti. Diventa un resolver a tre livelli, in ordine:

1. una variabile di package valorizzabile via `-ldflags -X` (oggi nessuno la valorizza:
   esiste come cardine di iniezione e per i build futuri);
2. `debug.ReadBuildInfo().Main.Version`;
3. `"dev"`.

**Testabilità (vincolante).** `debug.ReadBuildInfo()` dentro un test restituisce le
informazioni del binario di test, quindi un resolver che lo chiama internamente non è
testabile. La funzione con la logica prende gli input come parametri —
`Resolve(ldflagsVersion, mainVersion string) string` — e un wrapper sottile
non testato chiama `ReadBuildInfo`. Senza questo i test del §9 sarebbero vacui.

### 4.1 Quando il controllo NON parte — regola positiva

Il controllo parte **solo** se la versione corrente ha la forma di una release
pubblicata: `vMAJOR.MINOR.PATCH`, tutti e tre interi, **senza** segmento di prerelease
e **senza** build metadata.

Questa regola è positiva per un motivo preciso. Da **Go 1.24** `go build` stampa la
versione dal VCS: una build locale a valle di un tag produce una **pseudo-version**
(`v1.6.1-0.20260723143812-50d39f89c2fe`), non `(devel)`; con albero sporco compare
`+dirty`; `(devel)` appare solo con `-buildvcs=false` o fuori da un checkout git.
Elencare i casi da escludere sarebbe quindi incompleto per costruzione. La regola
positiva copre in una condizione sola `dev`, `(devel)`, le pseudo-version e `+dirty`.

Precisazione: una build pulita fatta esattamente su un commit taggato riporta il tag
semplice (`v1.6.0`), supera quindi il controllo di forma e il controllo parte. È corretto
così — quell'utente *è* su una release. La regola esclude chi sta a valle di un tag, non
chiunque abbia compilato in proprio.

Conseguenza gradita sullo scope: siccome `/releases/latest` non restituisce mai
prerelease, entrambi i termini del confronto sono sempre `vX.Y.Z` semplici.
**`internal/version` non implementa l'ordinamento dei prerelease** (SemVer §11) e non
introduce dipendenze: tre interi, confronto, tutto il resto rifiutato.

## 5. Il flusso del controllo

### 5.1 Cache

File `update.json` in `os.UserCacheDir()/clup/`. Si usa `os.UserCacheDir()` e non
`~/.cache` hardcoded: rispetta `XDG_CACHE_HOME` su Linux e dà il percorso giusto su
macOS, come già fa `internal/config` con `os.UserConfigDir()`. Il percorso è
iniettabile nei test, sullo stesso schema di `internal/config`.

```json
{ "checked_at": "2026-07-23T10:00:00Z", "latest": "v1.8.0" }
```

### 5.2 Freschezza

La cache è **scaduta** se manca, se non si legge, se non si parsifica, se
`now - checked_at >= 24h`, **oppure se `checked_at` è nel futuro** (orologio spostato
indietro: senza questa condizione la cache resterebbe "fresca" per sempre).

Una cache corrotta o illeggibile è sempre trattata come scaduta — si rifà la chiamata
e si sovrascrive — **mai** come un errore mostrato all'utente.

### 5.3 Chiamata

`GET https://api.github.com/repos/marcoarnulfo/clickup-cli/releases/latest`

- timeout **2 secondi**, `context` con deadline;
- header `Accept: application/vnd.github+json` e **`User-Agent: clup/<versione>`**
  (GitHub rifiuta le richieste senza User-Agent);
- **nessun header `Authorization`**: la chiamata è anonima e non deve mai portare con sé
  il token ClickUp dell'utente. (Il client ClickUp usa `Authorization: <token>` senza
  `Bearer`; questo codice non passa da lì e non deve toccare quell'header.)
- si legge `tag_name`.

`/releases/latest` esclude draft e prerelease. È la scelta che rende silenziosa la
finestra "tag pubblicato, note ancora in draft" del flusso di release: in quella
finestra l'endpoint restituisce ancora la release precedente. **Non sostituire questo
endpoint con l'API dei tag**, o quella proprietà si perde.

### 5.4 Scrittura

Scrittura **atomica**: file temporaneo nella stessa directory più `os.Rename`. Due
invocazioni concorrenti di `clup` possono scrivere insieme, e un file troncato sarebbe
proprio la cache corrotta del §5.2.

`checked_at` viene scritto **anche quando la chiamata fallisce**. Senza, un utente
offline ritenterebbe a ogni singola invocazione, pagando il timeout ogni volta:
esattamente lo scenario in cui la funzione deve farsi sentire di meno.

In caso di fallimento il campo `latest` **conserva il valore precedente** se c'era. Un
utente offline che aveva già visto "v1.8.0 disponibile" continua a vederlo: resta vero.
Al primo fallimento assoluto `latest` è vuoto e non si mostra nulla.

### 5.5 Errori

Rete, timeout, DNS, 404, rate limit, JSON malformato, `tag_name` di forma inattesa:
**tutti silenziosi**. Nessun avviso, nessun messaggio d'errore, nessun exit code diverso.

## 6. Quando si mostra l'avviso

Solo se `latest` è **strettamente maggiore** della versione corrente — non "diversa".
La differenza conta: l'owner che compila e tagga `v1.8.0` in locale prima di pubblicarla
non deve ricevere un avviso che lo invita a "aggiornare" a `v1.7.0`.

## 7. Opt-out

| Meccanismo | Effetto |
|---|---|
| `CLUP_NO_UPDATE_CHECK` valorizzata | disattiva, **vince su tutto** |
| `update_check: false` nel config | disattiva |
| `CLICKUP_DEMO=1` | disattiva **nella TUI** (vincolo "zero I/O" della demo) |
| chiave assente | **attivo** |

Il campo di config è **`*bool` con tag `yaml:"update_check,omitempty"`**. Il puntatore
serve perché con un `bool` semplice l'assenza della chiave varrebbe `false` e la
funzione nascerebbe spenta in tutti i config esistenti; `omitempty` serve perché senza
di esso `Save` scriverebbe `update_check: null` in ogni file salvato. È lo stesso
*meccanismo* del `Billable *bool` del client, ma è un pattern nuovo per
`internal/config`: verificato che con `gopkg.in/yaml.v3` chiave assente → `nil`,
`false` esplicito → `&false`, ed entrambi sopravvivono a un round-trip Save/Load.

L'aggiunta è additiva e non tocca `migrate`.

**La demo riguarda solo la TUI.** Il comando headless ignora deliberatamente
`CLICKUP_DEMO` — `internal/cli/report.go` lo dichiara in un commento e
`TestReportIgnoresDemoEnv` lo blocca: il percorso headless passa sempre dalla config e
dall'API vere. Quindi `clup report` controlla gli aggiornamenti anche con
`CLICKUP_DEMO=1` impostata, coerentemente con il resto di ciò che fa in quella
condizione. Chi vuole silenzio usa `CLUP_NO_UPDATE_CHECK`.

## 8. Testo e collocazione

**CLI**, su stderr, dopo che il report è stato scritto:

```
clup v1.8.0 is available (you have v1.7.0)
  go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest
  disable: CLUP_NO_UPDATE_CHECK=1
```

L'avviso **porta con sé il modo di spegnerlo**. È il punto in cui la funzione si rivela,
e chiedere all'utente di cercare la risposta nel README sarebbe scortese: la prima
richiesta in uscita può avvenire alla primissima esecuzione.

**Comandi che controllano:** l'avvio della TUI e `clup report`. **Non** `clup --version`,
che resta senza rete.

**Latenza (CLI):** il controllo parte in una goroutine all'ingresso del comando e viene
raccolto prima dell'uscita, in parallelo alla chiamata API del report. Serialmente
aggiungerebbe fino a 2 secondi a un `clup report` ogni 24 ore.

**TUI:** `Model.Init()` (oggi ritorna `nil`) ritorna il comando di controllo, guardato su
`m.demo`. Il comando ritorna un msg tipizzato gestito nel type switch di `app.go`; la
versione trovata vive come campo del Model radice e la home la rende in una riga
discreta, con gli stili di `styles.go`.

**Semantica di errore nella TUI (eccezione dichiarata).** Il comando non emette **mai**
`errMsg` né `retryableErrMsg` e non porta mai a `screenError`. È l'unica eccezione alla
convenzione "gli errori si vedono" del repo, ed è deliberata: un controllo di
aggiornamento fallito non è un problema dell'utente. In caso di errore il msg
semplicemente non arriva.

**Wizard di setup:** se la config non è valida `New` instrada su `screenSetup`. Il
comando parte comunque; l'eventuale avviso comparirà sulla home dopo il setup.

## 9. Test

**`internal/version`** — table-driven: `v1.7.0` vs `v1.8.0`, uguali, maggiore/minore su
tutte e tre le componenti, `v1.10.0 > v1.9.0` (confronto numerico, non lessicografico),
e i rifiuti: `dev`, `(devel)`, `v1.6.1-0.20260723143812-50d39f8`, `v1.7.0+dirty`,
`v1.7.0-rc1`, stringa vuota, `1.7.0` senza `v`.

**Resolver** — i tre livelli con input iniettati.

**`internal/service`** — `httptest` per l'API e `t.TempDir()` per la cache:

- cache fresca: **il server non viene mai chiamato**, asserito su un contatore di
  richieste (senza quel contatore il test non prova nulla);
- cache scaduta → rifà la chiamata;
- **cache corrotta** (JSON spazzatura, file troncato) → trattata come scaduta, nessun errore;
- **`checked_at` nel futuro** → trattata come scaduta;
- errore di rete / timeout / 500 / JSON malformato → nessun avviso, nessun errore;
- `checked_at` scritto anche in caso di errore, con `latest` precedente conservato;
- `latest` più vecchio della corrente → nessun avviso;
- opt-out via env, via config, e demo che salta;
- header: `User-Agent` presente, `Authorization` **assente** (asserito sul server di test).

**`internal/cli`** — l'avviso finisce su stderr e **stdout resta JSON valido**: è
l'asserzione che protegge gli script. `report_test.go` già sostituisce `os.Stdout` con
una `os.Pipe`, si estende lo stesso schema a stderr.

**`internal/tui`** — il msg alimenta la riga sulla home; in demo mode il comando non
parte; un errore non porta mai a `screenError`.

## 10. Documentazione

Nota nei due README (`README.md` inglese, `README.it.md` italiano): che cosa controlla,
ogni quanto, che è anonimo e senza token, e i due modi di spegnerlo.

## 11. Fuori scope

- Self-update (download/sostituzione del binario): mai.
- goreleaser, binari di release, tap, packaging: #97, #99, #103.
- Telemetria di qualunque genere.
- Ordinamento dei prerelease in `internal/version` (§4.1).
