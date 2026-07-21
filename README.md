# clickup — ClickUp Hours CLI

TUI da terminale per il report ore mensile di ClickUp (self + team), con
calcolo dell'importo da fatturare ed export CSV/JSON/Markdown.

## Installazione

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clickup@latest
```

## Uso

Lancia `clickup`. Al primo avvio un wizard ti chiede il token API personale
(lo trovi in ClickUp → Settings → Apps → API Token).

Nella home, `t` alterna lo scope tra `me` (solo le tue ore) e `team` (l'intero
team del workspace configurato). Per lo scope `team` il token deve avere
permessi Owner/Admin sul workspace: senza questi permessi la chiamata API
fallisce e l'errore viene mostrato nella schermata d'errore. La selezione
puntuale di singoli membri del team è prevista per una versione futura (v1.3).

## Licenza

MIT
