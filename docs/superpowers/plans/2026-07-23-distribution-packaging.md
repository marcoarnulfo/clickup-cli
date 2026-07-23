# Piano — Distribution & packaging (#97/#99/#103)

> Implementa la spec `docs/superpowers/specs/2026-07-23-distribution-packaging-design.md`.
> Branch `distribution-packaging`, worktree `../clickup-cli-distribution`.
> **Zero file in `internal/`**: parallelizza con v1.9 e #125 senza conflitti.

## Task 1 — `.goreleaser.yaml`

- [x] Build `cmd/clup`: `CGO_ENABLED=0`, goos darwin/linux/windows, goarch
      amd64/arm64, `-trimpath`, ldflags `-s -w -X …/service.ldflagsVersion={{ .Tag }}`.
- [x] Archivi con LICENSE/README/CHANGELOG; `format_overrides` windows →
      `formats: [zip]` (la chiave singola `format` è deprecata in v2.10+).
- [x] `checksums.txt` + `signs:` cosign v3 (`sign-blob --bundle`, `artifacts: checksum`).
- [x] `changelog: disable` + `release.prerelease: auto` + footer con comandi install.
- [x] `homebrew_casks` → `marcoarnulfo/homebrew-tap` (`Casks/`), hook
      `postflight` xattr, `skip_upload: auto`.
- [x] `scoops` → `marcoarnulfo/scoop-bucket` (`bucket/`), `skip_upload: auto`.
- [x] `aurs` `clup-bin`: `disable: '{{ if index .Env "AUR_KEY" }}false{{ else }}true{{ end }}'`
      (**`index`**, non `.Env.AUR_KEY`: la chiave assente farebbe fallire il
      template), `skip_upload: auto`, `provides`/`conflicts: clup`.

**Verify:** `goreleaser check` pulito;
`goreleaser release --snapshot --clean --skip=sign` → 6 archivi + checksums,
cask e manifest in `dist/`, AUR skipped; `./dist/clup_darwin_arm64_v8.0/clup
--version` stampa il tag iniettato.

## Task 2 — Workflow GitHub

- [x] `.github/workflows/release.yml`: trigger `push.tags: v*`,
      `permissions: contents: write + id-token: write`, vet+test, step awk che
      estrae `## [X.Y.Z]` da `CHANGELOG.md` (exit 1 se assente),
      `sigstore/cosign-installer@v4`, `goreleaser/goreleaser-action@v7` con
      `args: release --clean --release-notes=notes.md`. Env: `GITHUB_TOKEN`,
      `TAP_GITHUB_TOKEN`, `AUR_KEY`.
- [x] `ci.yml`: job `goreleaser` con `release --snapshot --clean --skip=sign`
      (anti-rottura config su ogni push/PR).

**Verify:** YAML valido; il job gira sulla PR.

## Task 3 — Repo e secret esterni

- [x] `gh repo create marcoarnulfo/homebrew-tap --public --add-readme`.
- [x] `gh repo create marcoarnulfo/scoop-bucket --public --add-readme`.
- [x] `TAP_GITHUB_TOKEN` su clickup-cli (token gh corrente, scope `repo` —
      copre contents:write su entrambi i repo; sostituibile con un fine-grained
      PAT più restrittivo in futuro).
- [ ] `AUR_KEY` — **azione utente**, quando vorrà: account AUR + chiave SSH
      senza passphrase → `gh secret set AUR_KEY`. Da allora AUR pubblica da solo.

## Task 4 — Docs repo

- [x] `.gitignore` += `dist/`.
- [x] `README.md` / `README.it.md`: sezione installazione riscritta (binari
      precompilati + verifica cosign, Homebrew, Scoop, AUR con nota "pending
      key", go install).
- [x] `CHANGELOG.md` `[Unreleased]`: voce pipeline + nuovi canali.
- [x] `CLAUDE.md` (file locale nel worktree principale): sezione "Release
      (da v1.9, goreleaser)" con procedura tag-driven e gotcha `{{ .Tag }}`.

## Task 5 — Issue tracker (bilingue)

- [ ] #97: commento con il design (si chiude con la PR).
- [ ] #99: commento sulla deviazione "nessun cask clickup" (tap mai esistito;
      shim `cmd/clickup` resta per `go install`).
- [ ] #103: stato Scoop/AUR + Nix deferito con spiegazione completa (NUR:
      repo template + PR a nix-community/NUR + nix-hash in CI); label
      `good first issue` + `help wanted` (crearle se mancanti). Issue resta
      aperta solo per Nix.
- [ ] Follow-up (commento, non codice): il testo dell'update notice dice
      ancora "go install…" — aggiornarlo richiede `internal/tui`, fuori scope.

## Task 6 — Chiusura

- [ ] `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./... -race`.
- [ ] Commit convenzionali, push branch, PR verso `main`.
- [ ] Post-merge: smoke test con tag `v1.8.1-rc1` (prerelease → niente
      package manager), poi prima release piena.

## Ledger

`.superpowers/sdd/progress.md` (gitignored, nel worktree).
