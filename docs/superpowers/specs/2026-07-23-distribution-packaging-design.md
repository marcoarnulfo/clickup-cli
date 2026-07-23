# Distribution & packaging (#97/#99/#103) — design

> Spec per le issue [#97](https://github.com/marcoarnulfo/clickup-cli/issues/97),
> [#99](https://github.com/marcoarnulfo/clickup-cli/issues/99) e
> [#103](https://github.com/marcoarnulfo/clickup-cli/issues/103)
> (milestone *Distribution & packaging*). Spec separata da quella dell'update
> notice (#104): lì il codice, qui la pipeline.

## 1. Obiettivo

Trasformare un tag `vX.Y.Z` in una release completa: binari precompilati per
tutte le piattaforme, checksum firmati, e pubblicazione automatica sui package
manager — senza toccare una riga di `internal/`, così il lavoro parallelizza
pulito con v1.9 (design system TUI) e #125 (edit tag entry).

Completano la storia dell'update notice (#104): oggi dice "aggiorna" ma non
esisteva nulla da scaricare se non via `go install`.

## 2. Decisioni prese (owner)

- **Homebrew: cask, non formula.** GoReleaser ≥2.12 ha deprecato `brews:` in
  favore di `homebrew_casks:` per i binari precompilati. Si parte direttamente
  dalla via supportata. Hook `postflight` con `xattr -dr com.apple.quarantine`
  (binari non notarizzati Apple).
- **Nessun cask `clickup` deprecato.** La lettera di #99 lo chiedeva, ma il tap
  non è mai esistito: nessuno ha mai installato `clickup` via brew, quindi non
  c'è nessuno da migrare. Lo shim `cmd/clickup` resta nel repo per gli utenti
  `go install` (è il suo unico scopo). Deviazione commentata su #99.
- **AUR: scaffold + auto-enable.** Il nome `clup` su AUR è occupato da un
  pacchetto morto del 2014 → `clup-bin` (suffisso che GoReleaser impone
  comunque per i binary package). La sezione `aurs:` è committata ma
  `disable`d finché non esiste il secret `AUR_KEY` (chiave SSH registrata su
  aur.archlinux.org): pipeline verde da subito, pubblicazione automatica appena
  la chiave arriva.
- **Nix: deferito alla community.** La via GoReleaser pubblica su un NUR (Nix
  User Repository): repo da template + PR di registrazione a
  `nix-community/NUR` + `nix-hash` in CI. Superficie di manutenzione non
  banale, e la issue stessa lo marca come delegabile: #103 resta aperta solo
  per Nix, con spiegazione dettagliata e label `good first issue`/`help wanted`.
- **Checksum firmati con cosign keyless** (OIDC GitHub, zero chiavi da gestire):
  un solo bundle `checksums.txt.sigstore.json` verificabile con
  `cosign verify-blob`.
- **Fuori scope:** completions (#101), man page (#102), terminal matrix (#105),
  testo dell'update notice (tocca `internal/tui`, follow-up a parte).

## 3. Architettura

| Pezzo | Dove | Responsabilità |
|---|---|---|
| Build matrix + archivi + checksum | `.goreleaser.yaml` | darwin/linux/windows × amd64/arm64, tar.gz (zip su Windows) |
| Firma | `.goreleaser.yaml` `signs:` | cosign v3 keyless `sign-blob --bundle` su `checksums.txt` |
| Stamp versione | ldflags `-X …/service.ldflagsVersion` | usa **`{{ .Tag }}`**, non `{{ .Version }}`: `version.IsRelease` richiede il prefisso `v` |
| Orchestrazione | `.github/workflows/release.yml` | trigger su tag `v*`; vet+test; note da CHANGELOG; `id-token: write` per OIDC |
| Anti-rottura config | job `goreleaser` in `ci.yml` | `release --snapshot --clean --skip=sign` su ogni push/PR |
| Homebrew | repo `marcoarnulfo/homebrew-tap` | cask `clup`, push automatico via `TAP_GITHUB_TOKEN` |
| Scoop | repo `marcoarnulfo/scoop-bucket` | manifest `bucket/clup.json`, stesso token |
| AUR | `ssh://aur@aur.archlinux.org/clup-bin.git` | attivo quando esiste `AUR_KEY` |

`internal/service.ldflagsVersion` e `internal/version` esistevano già (v1.7):
il seam era pronto, questa milestone è la prima a usarlo davvero.

## 4. Procedura di release (nuova)

1. Su `main`: `[Unreleased]` → `## [X.Y.Z] - data` in `CHANGELOG.md`, commit
   `docs(changelog): release vX.Y.Z`.
2. `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. Il workflow: vet+test → estrae la sezione del tag dal CHANGELOG (**fallisce
   se assente**) → goreleaser con `--release-notes` (`changelog: disable` in
   config) → binari + firme + cask + manifest.

**Smoke test:** un tag prerelease `vX.Y.Z-rc1` esercita tutta la pipeline come
GitHub prerelease; `skip_upload: auto` su ogni publisher lo tiene lontano da
brew/scoop/AUR. `prerelease: auto` inoltre lo esclude da `releases/latest`,
quindi l'update check non lo propone mai.

## 5. Verifica

- `goreleaser check` pulito (deprecation `format`→`formats` risolta).
- `goreleaser release --snapshot --clean --skip=sign` in locale: 6 archivi +
  checksums + cask e manifest generati in `dist/`; AUR correttamente skipped.
- `./dist/clup_darwin_arm64_v8.0/clup --version` → `v1.8.0`: lo stamp ldflags
  funziona (test incrociato con `go build -ldflags -X …=v1.8.1`).
- Job snapshot in CI valida la config sulla PR stessa.
- Primo tag reale dopo il merge: smoke con `-rc1`, poi release piena.

## 6. Rischi / note

- **Quarantine macOS:** mitigato dall'hook `xattr` nel cask; chi scarica il
  tarball a mano deve fare `xattr -dr com.apple.quarantine clup` da sé
  (documentabile nella release page se emerge come problema).
- **`TAP_GITHUB_TOKEN`:** il `GITHUB_TOKEN` del workflow non può pushare su
  altri repo; serve un PAT con `contents:write` su tap+bucket.
- **AUR_KEY** deve essere una chiave SSH **senza passphrase** (requisito
  GoReleaser).
