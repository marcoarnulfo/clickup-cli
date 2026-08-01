# Keybinding configurabili (design)

> v1.9. Chiude la **#82** — la sua terza e ultima casella, gli override delle
> keybinding dal config. Le altre due (tema da YAML, due temi built-in) sono
> state chiuse dalla tranche precedente. Con questa la milestone si chiude.

## 1. Obiettivo

Far rimappare all'utente i tasti della TUI dal proprio `config.yml`, senza che
una configurazione sbagliata produca comportamenti silenziosamente ambigui.

Nessuna chiamata API nuova, nessuna schermata nuova, nessun tasto nuovo nei
default.

## 2. Evidenza misurata

Ogni numero qui sotto viene dall'aver **eseguito** il codice. Nella tranche
precedente tre affermazioni scritte a mano si sono rivelate false — un commento
di package, sei rapporti di contrasto e un conteggio di call site — e tutte e
tre sono state prese solo perché qualcuno ha eseguito qualcosa.

### 2.1 Lo spazio dei tasti è già sovraccarico, di proposito

Misurato iterando `keyDefaults` per reflection:

- **51 binding**
- **39 tasti fisici distinti**
- **20 tasti sono rivendicati da più di un binding**

Le collisioni non sono un difetto: sono il modo in cui la TUI sta in una
tastiera. Alcune:

| tasto | binding che lo rivendicano |
|---|---|
| `n` | LogHours, NewOverride, NewTag, No |
| `enter` | Confirm, Generate, Yes |
| `h` | History, PrevMonth, PrevSection |
| `s` | ChangeRange, Save, StopTimer |
| `y` / `Y` | ConfirmDelete, Yes |

Convivono perché `screenKeys` (`keys.go:304`) costruisce per ogni schermata un
`keyMap` assegnando **solo** i campi che quella schermata accetta; i campi non
assegnati restano il `key.Binding` zero, che — misurato — ha `Enabled() == false`
e non corrisponde a nulla.

**La conseguenza governa tutto il design: una regola «due binding non possono
condividere un tasto» rifiuterebbe i default che spediamo.**

### 2.2 La regola sui conflitti che regge senza enumerare gli stati

Il rilevamento corretto sarebbe per-schermata, ma richiederebbe di enumerare
ogni stato raggiungibile — misurato: le 14 schermate, più le 5 modalità di
`entries`, gli 8 passi di `log`, le **4** sezioni di `rates` e i passi del setup
— e di costruire un `Model` rappresentativo per ognuno. È una superficie che va tenuta in sincronia con ogni
schermata futura: esattamente il moltiplicatore di manutenzione che la #82 si
autodenuncia.

La regola scelta si calcola invece dalla **sola tabella dei tasti**:

> Per ogni tasto rivendicato da **due o più** binding dopo gli override,
> l'insieme dei rivendicanti deve essere un **sottoinsieme** di quello che lo
> rivendicava prima.

> **La clausola «due o più» non è un dettaglio, ed era assente dalla prima
> stesura.** Senza di essa la regola, applicata alla lettera, rifiuterebbe anche
> lo spostamento su un tasto libero: misurato, per `log_hours: "L"` il tasto `L`
> passa da `{}` a `{log_hours}`, che non è un sottoinsieme di `{}`. La review ha
> eseguito la formulazione contro il codice e ha trovato che dicevano cose
> diverse.

Cosa permette e cosa no:

- spostare `LogHours` da `n` a `L` → `L` ha un solo rivendicante, e `n` resta a
  `{NewOverride, NewTag, No}` → **ammesso**;
- dare a `Export` il tasto `n` → `n` diventa `{LogHours, NewOverride, NewTag,
  No, Export}`, non è un sottoinsieme → **rifiutato**, nominando `Export`, `n` e
  chi già lo rivendica.

**Il prezzo, misurato e non immaginato.** La prima stesura attribuiva
erroneamente un costo agli scambi innocui. **È falso**: uno scambio pulito passa.
Misurato, `{"quit": "r", "reload": "q"}` viene **accettato**: dopo gli
override `q` ha il solo rivendicante `Reload` e `r` il solo `Quit`, quindi la
clausola «due o più» non si applica. I rivendicanti cambiano, ma ogni destinazione
resta a rivendicante singolo. Per lo stesso motivo passa anche un ciclo a tre.

Il prezzo vero è un altro: **prendere un tasto che un altro binding rivendica
ancora viene rifiutato, anche se i due non convivono su nessuna schermata.** Se
l'utente vuole `Export` su `g`, la regola lo rifiuta perché `ListBudget`
rivendica ancora `g`, e non sa che `Export` vive sul report e `ListBudget` sulle
tariffe. L'utente sceglie un altro tasto, con un messaggio che nomina la
collisione. Il prezzo dell'alternativa è una tabella di ogni stato di ogni
schermata, da mantenere per sempre.

### 2.3 Far passare la tabella dal Model costa due righe

`defaultKeys()` ha **tre** chiamate in produzione, misurate:

| dove | cosa |
|---|---|
| `keys.go:293` | `keysFor` con la palette aperta |
| `keys.go:305` | `screenKeys` |
| `app.go:663` | `key.Matches(msg, defaultKeys().ForceQuit)` |

Le prime due leggeranno la tabella dal `Model`. **La terza resta com'è**, ed è
una proprietà voluta: `ForceQuit` non è rimappabile, quindi il controllo di
`ctrl+c` legge letteralmente dai default non modificabili e nessuna
configurazione può togliere la via di fuga.

### 2.4 I nomi si derivano, e l'override arriva fino in fondo

Misurato: iterando `keyDefaults` per reflection si ottengono **51** nomi, e la
conversione in snake_case dà `quit`, `back`, `force_quit`, `help`, `palette`,
`palette_up`, `palette_down`, `prev_month`, …

Misurato anche il percorso completo: sostituendo il campo `LogHours` con
`key.NewBinding(key.WithKeys("L"), …)` tramite `reflect.Value.Set`, la tabella
modificata produce `homeKeys(...).LogHours.Keys() == ["L"]`. L'override
sopravvive fino al `keyMap` di schermata senza che nessun costruttore per
schermata debba saperne nulla.

**Questo elimina il moltiplicatore di manutenzione invece di accettarlo**: un
binding aggiunto in futuro a `keyDefaults` diventa rimappabile da solo;
`force_quit` è l'unica eccezione esplicita e resta fisso. Il rischio si sposta
altrove — rinominare un campo Go rinominerebbe in silenzio una chiave del config
di qualcuno — e va inchiodato con un test che fissa l'elenco completo dei 51
nomi.

## 3. Gli interventi

### 3.1 Nessun package nuovo, e questa volta la ragione è che non serve

La tranche precedente ha creato `internal/themes` perché `internal/config` e
`internal/tui` avevano bisogno dello **stesso tipo** — e la prima versione di
quella spec sbagliava perfino la motivazione, dichiarando forzato ciò che era
una scelta.

Qui non c'è nulla da condividere: il config porta stringhe, e il tipo risolto
serve solo alla TUI. La tabella e la sua validazione restano in `internal/tui`,
accanto a ciò che validano.

### 3.2 Il config

```yaml
keys:
  log_hours: "L"
  up: ["up", "ctrl+u"]
```

```go
// KeySpec is one binding's keys as written in YAML: a bare key, or a list.
type KeySpec []string

Keys map[string]KeySpec `yaml:"keys,omitempty"`
```

`KeySpec` ha un `UnmarshalYAML` tollerante che accetta scalare e sequenza, e un
`MarshalYAML` che ricollassa a scalare quando il tasto è uno solo — per la stessa
ragione misurata su `themes.Value`: `config.Save` serializza l'intera `Config`, e
senza il collasso un `log_hours: "L"` tornerebbe su disco come `log_hours: ["L"]`
alla prima cosa che salva.

Il tipo vive in `internal/config` perché nessun altro package ne ha bisogno.

### 3.3 `ResolveKeys` e la validazione

```go
// KeyTable is the resolved binding table the TUI routes and renders with.
type KeyTable struct{ d keyDefaults }

func DefaultKeyTable() KeyTable
func ResolveKeys(overrides map[string]config.KeySpec) (KeyTable, error)
func BindingNames() []string   // the 51 snake_case names, sorted
```

Errori **bloccanti**, con il precedente di `billing.rounding.increment`
(`internal/service/pricing.go:71`) e degli errori dei temi:

| caso | messaggio |
|---|---|
| nome sconosciuto | nomina il nome e elenca quelli validi, **`force_quit` escluso** |
| `force_quit` | dice che è la via di fuga e non è rimappabile |
| lista vuota, o un tasto vuoto nella lista | nomina il binding |
| **lo stesso tasto due volte nella stessa lista** | nomina il binding e il tasto |
| un tasto con due o più rivendicanti dopo gli override ne guadagna uno nuovo (§2.2) | nomina il tasto, il binding che lo chiede e chi già lo rivendica |

Due dettagli che la review ha trovato eseguendo, e che sarebbero passati:

- **L'elenco dei nomi validi non deve contenere `force_quit`**, o il messaggio
  suggerisce all'utente l'unico nome che poi rifiuta.
- **Un tasto ripetuto nella stessa lista** (`quit: [Q, Q]`) sfugge alla
  validazione e finisce nella regola sulle collisioni, che produce un messaggio
  senza senso: misurato, «binding "quit" cannot take key "Q": it is already
  claimed by » — con l'elenco vuoto, perché l'unico altro rivendicante è il
  binding stesso. Va rifiutato prima, con un messaggio suo.

Nessun fallback silenzioso: un tasto che non si riesce a interpretare fermerebbe
l'utente davanti a una TUI in cui un comando semplicemente non risponde, senza
niente a cui aggrapparsi.

### 3.4 Il testo d'aiuto va rigenerato — in due posti, non uno

`key.WithHelp("↑/k", "move up")` porta il tasto **dentro** la stringa d'aiuto,
che è ciò che il footer e il `?` mostrano. Un binding rimappato e non rigenerato
mentirebbe.

**Rigenerarlo dentro `ResolveKeys` non basta, ed è il buco che la review ha
trovato eseguendo.** I costruttori per schermata riscrivono l'etichetta con
stringhe letterali in **24 punti** — 16 chiamate a `pairHelp` e 8 a `SetHelp`
(`keys.go:380,406,448,552,573,611,636-645,666,674,688,714,725,743,744,758,777-779`)
— perché una riga di footer copre spesso una *coppia* di binding:

```go
pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
k.ClearValue.SetHelp("d", "use the default rate")
```

Misurato con una tabella che rimappa `confirm→ctrl+j` e `back→ctrl+q`, il footer
della palette restava:

```
↑/↓ move · enter run · esc close · ctrl+c force quit
```

Il primo test che avevo prescritto guardava solo la `KeyTable`, mai un footer
reso: sarebbe passato mentendo.

**La regola.** L'etichetta letterale si usa **finché nessuno dei binding che
copre è stato rimappato**; se uno lo è, si deriva dai tasti veri. Quindi:

- il footer di default resta quello di oggi, con le frecce tipografiche, e
  **nessun golden si muove**;
- chi rimappa vede i propri tasti.

Perché serve sapere *quali* nomi sono stati sovrascritti, la `KeyTable` se li
ricorda, e i costruttori per schermata la ricevono al posto della sola
`keyDefaults`.

Per il binding sovrascritto la ricostruzione unisce i tasti con `/` e mantiene
la descrizione originale. Le frecce tipografiche si perdono per i soli binding
rimappati — è l'utente ad aver scelto quei tasti, e inventargli una tipografia
sarebbe indovinare.

### 3.4.1 Due stringhe di prosa nominano tasti rimappabili

Fuori dal sistema di help, due messaggi scritti a mano citano un tasto:

- `rates.go:618` — «press 'g' and submit an empty value to remove the budget»,
  dove `g` è `list_budget`;
- `rates_view.go:55` — «press 'b' to browse the workspace», dove `b` è
  `browse_list`.

Diventano false appena quei due binding vengono rimappati. Vanno costruite dal
binding vivo invece che dalla lettera.

### 3.5 La firma, e un seme per la prossima volta

`New(cfg, pal)` diventa `New(cfg, pal, kt)`. Sono di nuovo **44 call site**, i
secondi in due tranche.

Insieme al cambio si introduce un helper di test — `testModel(cfg config.Config)
Model`, in un `helpers_test.go` nuovo — che i 43 call site dei test usano al
posto di `New`. Stessa quantità di modifiche adesso, ma la **prossima** crescita
della firma ne tocca una sola.

I call site vanno trovati **facendo elencare al compilatore**, non con un grep:
nella tranche precedente `grep 'New(cfg)'` ne trovò nove su 44, e la correzione
successiva sbagliò comunque la ripartizione per file.

### 3.6 La `KeyTable` zero deve significare «i default»

Non è una comodità: è un vincolo misurato. I test costruiscono **108 letterali
`Model{…}`** senza passare da `New` — 28 nel solo `footer_golden_test.go`, 15 in
`app_test.go`, 13 in `range_test.go`. Se `screenKeys` leggesse una tabella zero
da quei Model, ogni binding risulterebbe disabilitato e i 108 andrebbero
modificati insieme ai 44 call site: oltre 150 punti per una feature che ne
richiede tre.

> **Il grep originale sovrastimava di due.** La prima stesura contava con
> `grep -oE '(^|[^A-Za-z0-9_.])Model\{'`, che include due occorrenze dentro
> **commenti** (`app_test.go:337`, `footer_golden_test.go:71`). Contati sull'AST
> sono 108. È la quarta volta in tre tranche che un mio numero preso col grep si
> rivela sbagliato, e stavolta in una sezione che apre dichiarando che ogni
> numero viene dall'esecuzione. La conclusione regge — misurato, forzando
> `bindings()` a ritornare `kt.d` senza la guardia, **falliscono 134 test** — ma
> il numero no.

```go
type KeyTable struct {
	d   keyDefaults
	set bool // false on the zero value, where d is meaningless
}

// bindings returns the table to route with. The zero KeyTable means the
// built-in defaults, which is what a Model built by hand in a test gets.
func (kt KeyTable) bindings() keyDefaults {
	if !kt.set {
		return defaultKeys()
	}
	return kt.d
}
```

Il rischio di «zero significa default» è noto ed è l'altra faccia della moneta
del campo `*bool` di `UpdateCheck`: se `cli` dimenticasse di passare la tabella,
gli override verrebbero ignorati **in silenzio**. Lo chiude il test di
instradamento end-to-end già richiesto in §4 — una tabella che sposta
`LogHours` su `L` deve far aprire la schermata di log su `L` e non su `n`, e non
passa se il valore non arriva fino a `Update`.

## 4. Test

- **Il test che porta il peso** fissa i **51 nomi** derivati per reflection,
  scritti per esteso. È l'unica cosa che si accorge se qualcuno rinomina un
  campo Go e con esso, in silenzio, una chiave nel config di un utente.
- **La regola sui conflitti** ha una tabella per la clausola post-override: solo
  un tasto con due o più rivendicanti deve conservare un sottoinsieme dei
  rivendicanti di default. Copre uno spostamento che libera un tasto (ammesso),
  un binding che si aggiunge a un tasto già conteso (rifiutato), uno scambio
  pulito fra due binding (ammesso perché ogni destinazione resta a rivendicante
  singolo), e un binding che prende un tasto ancora rivendicato da un altro
  (rifiutato: è il prezzo conservativo dichiarato in §2.2).
- **La rigenerazione dell'aiuto**: un binding rimappato deve mostrare i tasti
  nuovi nel footer, e la descrizione vecchia.
- **`force_quit`** rifiutato per nome, e `ctrl+c` che continua a funzionare con
  una tabella che rimappa tutto il resto.
- **L'instradamento end-to-end**: con una tabella che sposta `LogHours` su `L`,
  `Update` deve aprire la schermata di log su `L` e **non** su `n`.
- **Ogni difetto ha un test visto fallire** contro il difetto, col transcript.
- **Golden**: attesi immutati. Da verificare eseguendo.
- Gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race`.

## 5. Fuori scope (dichiarato)

- **Rilevamento dei conflitti per-schermata** (§2.2): richiede una tabella di
  stati da mantenere per sempre. La regola conservativa costa all'utente un
  tasto diverso; questa costerebbe a ogni schermata futura una riga.
- **Rimappare `ForceQuit`**: `ctrl+c` è la via di fuga e resta fissa.
- **Una schermata per rimappare i tasti dentro la TUI**: si configura dal file.
- **Sequenze multi-tasto** (`g g`, `<leader> x`): bubbles/key non le modella.
- **Frecce tipografiche per i tasti rimappati** (§3.4).
- **Il wizard di setup che cancella `keys:` dal file.** Misurato: `setupModel.tmpCfg`
  parte da un `config.Config` azzerato (`setup.go:34-43`) e `setup.go:124-125` lo
  salva, quindi ri-autenticarsi dopo un token revocato cancella dal disco `keys`
  insieme a `themes`, `timezone`, `rate` e all'intero blocco `billing`. È
  **preesistente** ed è già registrato come **issue #149**; questa tranche
  aggiunge una vittima e non la risolve. Il fix indicato nella issue —
  inizializzare `tmpCfg` dal config caricato — vale per tutte insieme.

## 6. Vincoli globali

- `internal/report` e `internal/duration` restano **puri**: solo stdlib, nessun
  I/O, nessun import di `internal/config`, `internal/clickup`, `internal/tui`,
  `internal/themes`.
- `internal/themes` resta foglia.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- Mai chiamare l'API ClickUp vera: il comportamento di rete si esercita solo con
  `httptest`.
- Tutto ciò che vive nel repo è in **inglese** — codice, commenti, stringhe UI,
  nomi e messaggi dei test, messaggi di commit. Eccezioni: `README.it.md`,
  `CONTRIBUTING.it.md` e i doc di design sotto `docs/superpowers/`.
- **Ogni numero scritto in un commento va misurato eseguendo.**
- I golden si rigenerano solo con `go test ./internal/tui -update`, mai a mano.
- Conventional Commits. **MAI** `Co-Authored-By`.
