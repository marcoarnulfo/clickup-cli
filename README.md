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
| `↑`/`↓` (anche `k`/`j`) | Export | Seleziona il formato |
| `Enter` | Export | Salva `clickup-report-YYYY-MM.<ext>` nella cwd |
| `Esc` | Export | Torna al report senza esportare |
| `q` | Ovunque tranne il setup | Esce dall'applicazione |
| `Ctrl+C` | Sempre | Esce dall'applicazione |

Nella schermata di setup non è previsto `q` per uscire, per evitare di
premerlo per errore durante l'inserimento del token: usa `Ctrl+C`.

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
```

- `token`: token API personale ClickUp.
- `workspace_id`: id del workspace (team ClickUp) scelto in fase di setup.
- `currency`: valuta usata nel report e negli export.
- `rate`: tariffa oraria usata per calcolare l'importo da fatturare.

La variabile d'ambiente `CLICKUP_TOKEN`, se impostata, sovrascrive sempre il
`token` letto dal file di config (comodo per CI o per non salvare il token su
disco):

```bash
CLICKUP_TOKEN=pk_xxx clickup
```

## Licenza

MIT
