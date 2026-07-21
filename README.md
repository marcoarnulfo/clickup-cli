# clickup — ClickUp Hours CLI

TUI da terminale per il report ore mensile di ClickUp (self + team), con
calcolo dell'importo da fatturare ed export CSV/JSON/Markdown.

## Installazione

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clickup@latest
```

## Uso

Lancia `clickup`. Al primo avvio parte un wizard di setup che chiede, in
sequenza: il token API personale (lo trovi in ClickUp → Settings → Apps →
API Token), il workspace da usare (scelto tra quelli visibili al token),
una tariffa oraria opzionale e la valuta (default `EUR`). Il risultato viene
salvato in `~/.config/clickup-cli/config.yml` e riusato ai lanci successivi.

Dalla home scegli mese e scope, poi `Enter` genera il report. Nel report puoi
cambiare raggruppamento, riesportare o tornare alla home. Se il token risulta
invalido o revocato durante l'uso, la TUI ripropone automaticamente il wizard
di setup.

### Comandi nella TUI

| Tasto | Schermata | Azione |
|---|---|---|
| `◂` / `▸` (frecce sin/dx, anche `h`/`l`) | Home | Cambia mese |
| `t` | Home | Alterna scope `me` / `team` |
| `Enter` | Home | Genera il report per mese/scope selezionati |
| `g` | Report | Cicla il raggruppamento: totale → task → lista → giorno → totale |
| `e` | Report | Apre il menu di export (CSV/JSON/Markdown) |
| `m` / `s` | Report | Torna alla home per cambiare mese/scope |
| `r` | Report | Ricarica le voci ore dall'API per lo stesso mese/scope |
| `p` | Report | Apre la schermata **Tariffe per lista** |
| `↑`/`↓` (anche `k`/`j`) | Export | Seleziona il formato |
| `Enter` | Export | Salva `clickup-report-YYYY-MM.<ext>` nella cwd |
| `Esc` | Export | Torna al report senza esportare |
| `q` | Ovunque tranne il setup | Esce dall'applicazione |
| `Ctrl+C` | Sempre | Esce dall'applicazione |

Nella schermata di setup non è previsto `q` per uscire, per evitare di
premerlo per errore durante l'inserimento del token: usa `Ctrl+C`.

#### Schermata Tariffe per lista

Dalla schermata del report, premendo `p` si apre la schermata **Tariffe per lista**,
dove è possibile configurare una tariffa specifica per ogni lista (diverse dal default).
I comandi disponibili sono:

- `↑` / `↓` (anche `k` / `j`): naviga tra le liste
- `Enter`: modifica la tariffa della lista selezionata (edit mode)
- `d`: ripristina la lista alla tariffa di default
- `s` / `Esc`: salva le modifiche e torna al report

Dalla v1.1, ogni importo è calcolato dalle ore reali della lista moltiplicato per la sua
tariffa specifica (non dalle ore arrotondate), quindi il singolo importo può differire di
qualche centesimo dal prodotto `ore_mostrate × tariffa_lista`; tuttavia, il totale della
fatturazione resta sempre la somma esatta degli importi mostrati.

### Scope team

Per lo scope `team` il token deve avere permessi Owner/Admin sul workspace:
senza questi permessi la chiamata API fallisce e l'errore viene mostrato
nella schermata d'errore. In v1.0 lo scope `team` aggrega le ore di **tutti**
i membri del workspace configurato (nessuna selezione puntuale dei singoli
membri, prevista per una versione futura, v1.3).

## Configurazione

La configurazione persiste in `~/.config/clickup-cli/config.yml` (segue
`os.UserConfigDir()`, quindi rispetta `XDG_CONFIG_HOME` su Linux):

```yaml
token: pk_xxx...
workspace_id: "123456"
currency: EUR
rate: 45
rates:
  "111": 60
  "222": 30
```

- `token`: token API personale ClickUp.
- `workspace_id`: id del workspace (team ClickUp) scelto in fase di setup.
- `currency`: valuta usata nel report e negli export.
- `rate`: tariffa oraria di default usata per calcolare l'importo da fatturare.
- `rates` (opzionale): mappa `list_id: tariffa` con tariffe orarie specifiche per
  singola lista. Le liste non elencate usano la tariffa di default `rate`. La mappa
  si compila comodamente dalla TUI premendo `p` nella schermata del report.

La variabile d'ambiente `CLICKUP_TOKEN`, se impostata, sovrascrive sempre il
`token` letto dal file di config (comodo per CI o per non salvare il token su
disco):

```bash
CLICKUP_TOKEN=pk_xxx clickup
```

## Licenza

MIT
