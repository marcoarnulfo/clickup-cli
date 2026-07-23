# Security Policy

## Supported Versions

Only the latest minor release receives security fixes (currently `1.6.x`).
clup is maintained by one person in their spare time, so backporting fixes to
older minors isn't a promise that can be kept — upgrading to the latest
release is the only supported way to get a fix.

## Reporting a Vulnerability

Please report security issues privately using GitHub's **"Report a
vulnerability"** button, in this repository's **Security** tab. That opens a
private advisory visible only to the maintainer, and is the only reporting
channel for this project — there is no security email address.

This is a solo-maintained open-source project, so responses are best-effort.
There's no SLA, but security reports get priority over other work.

## Where Your ClickUp Token Lives

This is the part worth reading carefully before you trust clup with your
token.

- Your personal API token is stored **in plain text** in the config file, on
  disk, created with `0600` permissions (readable/writable by your user only,
  no group/other access).
- The path depends on your OS (both under `os.UserConfigDir()`):
  - **Linux:** `~/.config/clup/config.yml`
  - **macOS:** `~/Library/Application Support/clup/config.yml`
  - **Legacy path (pre-`clup` rebrand):** `os.UserConfigDir()/clickup-cli/config.yml`
    (e.g. `~/.config/clickup-cli/config.yml` on Linux). If you upgraded from
    a version before the v1.6 rebrand and haven't run a `clup` command that
    saves the config since, this file may still hold your token; the first
    save under the new binary migrates it to the path above and rewrites the
    legacy file to a pointer stub.
- **Consequence, stated plainly:** anyone with read access to your user
  account's filesystem — a shared machine, a compromised account, a backup
  someone else can read — has access to your ClickUp token. Treat it like a
  password.
- If you'd rather not have the token touch disk at all, set the
  `CLICKUP_TOKEN` environment variable instead. clup reads it and uses it for
  the session; if a config file already exists and something later triggers a
  save (e.g. changing a billing setting), clup writes back whatever token was
  already in that file — it never persists the environment value to disk.
- System keychain support (so the token never has to sit in a plaintext file)
  is planned for the **v1.10 — Task context (read) & accounts** milestone.
  It isn't implemented yet.

## What clup Talks To

- **The ClickUp API (v2)**, using your personal token, for everything the
  tool does: reading time entries, tasks, and members, and — if you use the
  log-hours feature — writing time entries. This is the only place your
  ClickUp token is ever sent.
- **The GitHub releases API** (`api.github.com`), to check whether a newer
  clup release exists. This request is anonymous — no token, no
  Authorization header of any kind, is attached or could be — and it only
  ever reads release metadata: it does not download or execute any code, and
  clup has no self-update mechanism. It's cached locally for 24 hours and can
  be turned off with `CLUP_NO_UPDATE_CHECK=1`, with `update_check: false` in
  the config, or by using demo mode.

Nothing else. clup makes no other outbound network calls.
