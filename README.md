[![Continuous Integration][ci-shield]][ci-url]
[![Release Workflow][release-workflow-shield]][release-workflow-url]
[![Release][release-shield]][release-url]

<br />
<p align="center">
  <a href="https://github.com/giovalgas/envault">
    <img src="./assets/icon.svg" alt="Logo" width="128" height="128">
  </a>

<h3 align="center">envault</h3>

  <p align="center">
    Keys in sight. Values sealed.
    <br />
    <br />
    <a href="https://github.com/giovalgas/envault/issues">Report Bugs</a>
    ·
    <a href="https://github.com/giovalgas/envault/issues">Request Features</a>
  </p>
</p>

## Table of Contents

* [About the Project](#about-the-project)
  * [Built with](#built-with)
  * [Support the dev](#support-the-dev)
* [Getting Started](#getting-started)
  * [Installation](#installation)
    * [Linux](#linux)
    * [Manually](#manually)
    * [macOS](#macos)
    * [Windows](#windows)
  * [Homebrew](#homebrew)
* [Shell wrapper](#shell-wrapper)
* [Quick usage](#quick-usage)
* [TUI](#tui)
  * [Screens](#screens)
  * [Shortcuts](#shortcuts)
  * [Selection](#selection)
* [CLI reference](#cli-reference)
  * [Combining envs and templates](#combining-envs-and-templates)
  * [JSON output of list](#json-output-of-list)
  * [JSON output of plan](#json-output-of-plan)
  * [JSON output of selection](#json-output-of-selection)
  * [Selection from another terminal or agent](#selection-from-another-terminal-or-agent)
  * [Exit codes](#exit-codes)
* [Editor file format](#editor-file-format)
* [Threat model](#threat-model)
* [Claude Code Skill](#claude-code-skill)
  * [Recommended permission rules](#recommended-permission-rules)
* [Moving the key to the keychain](#moving-the-key-to-the-keychain)
* [Environment variables](#environment-variables)
* [Architecture](#architecture)
* [Contributing](#contributing)
* [Contact](#contact)

## About the Project

<table>
  <tr>
    <td><img src="./assets/demo-tui.svg"></td>
    <td><img src="./assets/demo-cli.svg"></td>
  </tr>
</table>

envault is a local, encrypted vault for environment variables. It stores credentials in named envs, for example `postgres-local`, `stripe-test` or `aws-dev`, combines one or more envs and generates a project's `.env`, without the values ever passing through Claude's chat.

Three interfaces sit on the same core:

- **TUI** (Bubble Tea). An interactive interface to view, create, edit and load several envs at once.
- **CLI** (Cobra). For scripts and for Claude, with predictable output, `--json` and stable exit codes; never interactive when `--json` is on.
- **Claude Code Skill**. Teaches Claude to discover, suggest and load envs, always asking permission and never seeing the values.

The vault is global: envs are reused across projects. Syncing between machines and sharing with a team are out of scope for now.

### Built with

This project was built with:

- [Go](https://go.dev/)
- [Cobra](https://github.com/spf13/cobra)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Bubbles](https://github.com/charmbracelet/bubbles)
- [Lipgloss](https://github.com/charmbracelet/lipgloss)

### Support the dev

Enjoyed the app and want to support me monetarily? [Buy me a coffee!](https://www.buymeacoffee.com/giovalgasdev)
Any donations are **greatly appreciated!**

## Getting Started

For copying values or the composed `.env` with `y` (and `load --clipboard`) on Linux you'll need [xclip](https://github.com/astrand/xclip), [xsel](https://github.com/kfish/xsel) or [wl-clipboard](https://github.com/bugaevc/wl-clipboard).

### Installation

#### Linux

```bash
PROJ_VERSION=$(curl -s "https://api.github.com/repos/giovalgas/envault/releases/latest" | grep -Po '"tag_name": "v\K[^"]*')
curl -Lo envault.tar.gz "https://github.com/giovalgas/envault/releases/download/v${PROJ_VERSION}/envault_${PROJ_VERSION}_linux_amd64.tar.gz"
tar xf envault.tar.gz envault
sudo install envault /usr/local/bin
```

Swap `amd64` for `arm64` on an ARM machine.

#### Manually

You will need to [install Go](https://go.dev/doc/install), version 1.23 or newer.

```bash
go install github.com/giovalgas/envault/cmd/envault@latest
```

#### macOS

[Install manually](#manually), or download the `darwin` archive matching your architecture from the [releases page](https://github.com/giovalgas/envault/releases).

#### Windows

Download the `windows` zip matching your architecture from the [releases page](https://github.com/giovalgas/envault/releases) and place `envault.exe` somewhere on your `PATH`, or [install manually](#manually).

### Homebrew

Not published yet. When it exists, it will be an optional tap, without replacing the two methods above.

## Shell wrapper

`envault load` without `--out` exports the variables straight into the current terminal, and that needs a shell function installed in your rc file. Add this to `~/.zshrc`:

```
eval "$(envault shell-init zsh)"
```

For bash, the same line in `~/.bashrc`, swapping `zsh` for `bash`. For fish (3.1 or newer), in `config.fish`:

```
envault shell-init fish | source
```

The function intercepts only `load` and `envault` without arguments (the TUI); every other command passes straight through to the binary. Without the wrapper installed, `load` without `--out` exits with code 2 and points you to install the wrapper or use `--out`.

## Quick usage

```
envault init
envault new postgres-local --description "Postgres local via docker-compose" --tags db,local
envault list
envault plan postgres-local
envault load postgres-local
```

With the shell wrapper installed (see the section above), `load` exports `DATABASE_URL` and the other keys straight into the current terminal. Without the wrapper, use `envault load postgres-local --out .env` to write a file instead.

Running `envault` without arguments, in an interactive terminal, opens the TUI. Without an interactive terminal, it shows the help.

## TUI

### Screens

1. **List** (starting screen). The header shows only `envault`. Two columns: on the left, the envs with name, key count, last edit and selection marker; on the right, description, tags and keys of the focused env, values always masked, each value right after its key name, on the same line, with a short separator. A fixed panel below the header shows the global selection as `SELECTED_ENVS=a,b,c`, names in marking order separated by commas, no spaces, and `SELECTED_ENVS=` when nothing is selected; each marked env carries its matching number (`[1]`, `[2]`) in the list, the rest show `[ ]`. If the vault doesn't exist at the resolved location, the TUI creates it before listing and shows in the status bar where `vault.enc` ended up; the CLI keeps exiting with code 4 in that same situation.
2. **Detail**. A full screen table of keys and masked values.
3. **Compose** (marked envs). Sortable list, a preview of the result with the origin of each key and a conflict indicator, a checklist of `.env.example` keys when one exists, and a choice of destination. With the shell wrapper installed, the default destination is the current terminal and `t` switches to a file; without the wrapper, only the file destination is available and the screen explains how to install `shell-init`. On the file destination, if it already exists, the screen offers to overwrite, merge or cancel. On any destination, `y` copies the composed `.env` to the clipboard instead, with the same content and precedence the file would get, and writes nothing to disk.
4. **Confirmations**. A reusable modal for deleting, overwriting and reviewing the post-edit diff.

### Shortcuts

| Key | Action |
|---|---|
| `↑/↓`, `k/j` | Navigate |
| `space` | Mark or unmark env |
| `l` | Load the marked envs (compose) |
| `enter` | Open detail |
| `v` | Reveal or hide the value of the focused row |
| `y` | Copy the value to the clipboard; on compose, copy the whole composed `.env` |
| `n` | Create a new env (opens the editor) |
| `e` | Edit env (opens the editor) |
| `c` | Duplicate env |
| `r` | Rename env |
| `i` | Import a `.env` file |
| `d` | Delete, with confirmation by typing the name |
| `K`/`J` | Reorder on the compose screen |
| `t` | On compose, switch destination between terminal and file |
| `/` | Filter by name, description, tag or key name |
| `?` | Help with every shortcut |
| `q`, `ctrl+c` | Quit or go back |

The footer of each screen shows at most six shortcuts; `?` opens the help with the full list above.

No TUI screen shows a value without explicit confirmation from `v`, and copying with `y`, of a single value or of the composed `.env`, never prints the value to the screen.

### Selection

The selection is global: the same marked envs show up in any TUI session, in the order they were marked, and the last marking wins in case of a key conflict during compose. Every mark, unmark, reorder (`K`/`J`), rename or delete of an env makes the TUI write the selection to `selection.json`, in the vault directory, with only the names and `0600` permissions. On open, the TUI restores that selection; names that no longer exist in the vault drop out of it and the screen shows a warning.

## CLI reference

General rules: messages for humans go to stderr, data goes to stdout. With `--json`, errors also come out on stdout as `{"schema_version":1,"error":{"code":...,"message":"..."}}`. No read command (`list`, `show`, `plan`) ever prints variable values.

| Command | Description |
|---|---|
| `envault init` | Creates the directory, the key and the empty vault. Idempotent. |
| `envault list [--search term] [--tag t] [--json]` | Lists envs: name, description, tags, key names and `updated_at`. No values. `--search` matches name, description, tags and key names, case insensitive. |
| `envault show <env> [--json]` | Metadata and key names of an env. No values. |
| `envault get <env> <KEY>` | Prints a value. For humans and scripts. |
| `envault set <env> KEY=VALUE...` | Sets one or more keys. `envault set <env> KEY`, without `=`, reads the value from stdin. |
| `envault unset <env> KEY...` | Removes one or more keys. |
| `envault new <env> [--description d] [--tags a,b] [--from-file f]` | Creates an env. Without `--from-file`, opens the editor in the format described below. |
| `envault edit <env>` | Opens the existing env in the editor. |
| `envault import <env> <file> [--description d]` | Creates or replaces an env from a `.env` file. |
| `envault rename <current> <new>` | Renames an env. |
| `envault copy <source> <destination>` | Duplicates an env. |
| `envault delete <env> [--yes]` | Removes an env. Without `--yes`, requires an interactive terminal and confirmation by typing the name. |
| `envault plan <env>... [--out .env] [--template f] [--no-template] [--only-template]` | Shows in JSON what `load` would do. Always JSON, never writes anything. Without `--out`, `target` is `{"mode":"shell"}`; with `--out`, `target` carries `path`, `exists` and `gitignored`. |
| `envault load <env>... [--out .env] [--force \| --merge] [--clipboard] [--template f] [--no-template] [--only-template] [--json]` | Without `--out`, exports the variables into the current terminal through the `shell-init` wrapper, without writing a file and without printing values; without the wrapper installed, exits with code 2. With `--out`, writes the destination file and refuses when it already exists, unless `--force` or `--merge` is passed; both flags only apply with `--out`. With `--clipboard`, copies to the clipboard the same content `--out` would write, without touching any file; it can't be combined with `--out`, `--force` or `--merge`. |
| `envault shell-init bash\|zsh\|fish` | Prints the shell function that makes `load` and the TUI export into the current terminal. Install with `eval "$(envault shell-init zsh)"` (or `bash`) in your shell rc, or `envault shell-init fish \| source` in `config.fish`. |
| `envault exec -e a,b [--template f] [--no-template] [--only-template] [--] <command...>` | Runs a command with the combined variables injected into the environment, without creating a file. The `--` before the command is optional. |
| `envault shell <env>... [--shell bash\|zsh\|fish]` | Prints one `export` line per variable, for use with `eval`. Without `--shell`, detects the dialect from the `$SHELL` variable; falls back to `bash` when it doesn't recognize it. |
| `envault selection [--json]` | Shows the selection written by the TUI, in marking order. Without `--json`, prints one name per line to stdout; warnings about envs missing from the vault and "no env selected" go to stderr. Never prints values. |
| `envault skill install [--dir d]` | Installs the Skill at `~/.claude/skills/envault/SKILL.md`, or in the directory passed to `--dir`. |
| `envault key migrate` | Moves the encryption key from the file into the operating system keychain. |
| `envault completion`, `envault --version` | Cobra defaults; the version is set at build time. |

### Combining envs and templates

`plan`, `load`, `exec` and `shell` combine envs in the order given on the command line: the last env wins in case of a repeated key. The template flags are shared by these commands:

- `--template <file>`: uses the given file as the template of expected keys, instead of auto-detecting one.
- Without `--template` and without `--no-template`, the command looks for `.env.example` in the current directory.
- `--no-template`: ignores any template, including an auto-detected `.env.example`.
- `--only-template`: writes only the keys listed in the template; requires a template to exist, whether explicit or auto-detected.

With a template, the output follows the template's order. Keys with a default value in the template and no value in any env keep the default. Keys with no value anywhere go into `missing`. Keys from the envs that aren't in the template go at the end of the list, unless `--only-template` is on.

`--merge` in `load` preserves the order of the existing destination file: it keeps keys that only exist there, updates the ones coming from the envs, and appends the new ones at the end. `--force` and `--merge` together are a usage error. Without either flag and with the destination already existing, `load` exits with code 5.

After writing, `load` checks whether the destination is covered by the current repository's `.gitignore` (via `git check-ignore`) and warns on stderr, and in the JSON when `--json` is on, without ever touching `.gitignore` itself.

### JSON output of list

```json
{
  "schema_version": 1,
  "envs": [
    {
      "name": "postgres-local",
      "description": "Postgres local via docker-compose, port 5432",
      "tags": ["db", "local"],
      "keys": ["DATABASE_URL", "DATABASE_POOL"],
      "updated_at": "2026-09-20T18:30:00Z"
    }
  ]
}
```

### JSON output of plan

```json
{
  "schema_version": 1,
  "envs": ["postgres-local", "stripe-test"],
  "target": { "path": ".env", "exists": true, "gitignored": true },
  "template": { "path": ".env.example", "found": true },
  "keys": [
    { "key": "DATABASE_URL", "from": "postgres-local", "shadows": [] },
    { "key": "APP_URL", "from": "stripe-test", "shadows": ["postgres-local"] },
    { "key": "PORT", "from": null, "default": true }
  ],
  "conflicts": ["APP_URL"],
  "missing": ["SENTRY_DSN"],
  "extra": []
}
```

Without `--out`, `target` becomes `{ "mode": "shell" }`, the destination `load` uses in the terminal. With `load --clipboard`, `target` becomes `{ "mode": "clipboard" }`.

`load --json` returns the same shape as `plan`, with an extra `"mode"` field: `"created"`, `"overwritten"` or `"merged"` with `--out`, `"exported"` when it exports into the terminal through the shell wrapper, or `"copied"` with `--clipboard`.

### JSON output of selection

```json
{
  "schema_version": 1,
  "envs": ["b", "a"],
  "missing": ["gone"],
  "updated_at": "2026-09-23T12:00:00Z"
}
```

With no selection saved, `envs` and `missing` come back empty and `updated_at` comes back `null`; the command still exits with code 0. Without an initialized vault, `selection` exits with code 4.

### Selection from another terminal or agent

With the TUI open in one terminal, marking envs with `space`, another terminal or an agent reads the same selection through the CLI, without needing to know the names beforehand:

```
envault selection --json
envault load $(envault selection) --out .env
```

`envault load` accepts several envs in the precedence order of the command line, and the human mode of `selection` prints one name per line, so the `$(envault selection)` expansion passes exactly that list as arguments.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Generic error |
| 2 | Incorrect flag or argument usage |
| 3 | Env not found |
| 4 | Vault not initialized |
| 5 | Destination file already exists, use `--force` or `--merge` |
| 6 | Failed to decrypt, wrong key or corrupted vault |
| 7 | Validation error, `.env` parse error or invalid name |
| 130 | Cancelled by the user |

`envault exec` propagates the exit code of the child process instead of using this table; when the child is killed by a signal, the code is 128 plus the signal number.

## Editor file format

`new`, `edit` and the TUI open the user's editor (`$VISUAL`, then `$EDITOR`, then the operating system default) with a file in this format:

```
# @description: Postgres local via docker-compose, port 5432
# @tags: db, local
#
# Format: KEY=VALUE, one per line. Lines starting with # are comments.
# Save and close the editor to apply. Leave the file empty to cancel.

DATABASE_URL=postgres://user:pass@localhost:5432/app
DATABASE_POOL=10
```

Parser rules:

- The `# @description:` and `# @tags:` lines become the env's metadata; other comments are ignored.
- Accepts the `export ` prefix, single quotes (literal value), double quotes (with `\n`, `\t`, `\"` and `\\`) and an inline ` #` comment on unquoted values.
- Parse errors always cite the line number.

On save and close:

- An empty file, ignoring comments, on a new env: the creation is cancelled.
- Content with the same hash as the original: nothing is written.
- Parse error: the editor reopens with the typed content and the error as a comment at the top, for example `# ERRO linha 4: chave inválida "1KEY"`. Saving empty at that point abandons the edit.
- Valid content: envault computes a per-key diff (added, removed, changed, never the values), asks for confirmation and only then writes.

## Threat model

envault protects against:

- Leaking the isolated vault file. Without the key, `vault.enc` is useless.
- Accidental commits of secrets. `load` warns when the destination isn't in `.gitignore`.
- Secrets showing up in Claude's conversation. The Skill and the permission rules block reading values.

envault does not protect against:

- Someone with access to the machine's user account. While the key lives in a file, before migrating to the keychain, anyone who can read the home directory can read the key.
- Local malware running with the user's privileges.
- Undo files or swap files created by some editors during editing.
- Memory zeroing. Go doesn't guarantee it; decrypted values may stay in the process memory for a while.

## Claude Code Skill

The Skill teaches Claude to assemble a project's `.env` without ever seeing variable values. It only works with env names, descriptions and key names, and always asks for explicit confirmation before writing any file.

Install it with:

```
envault skill install
```

Installs at `~/.claude/skills/envault/SKILL.md` by default. Use `--dir <directory>` to install somewhere else.

The Skill only works where Claude runs commands on the user's machine (Claude Code or Cowork), not on claude.ai web.

### Recommended permission rules

Add this to your Claude Code permissions file (check the current path and syntax in the Claude Code documentation):

```json
{
  "permissions": {
    "allow": [
      "Bash(envault --version)",
      "Bash(envault list:*)",
      "Bash(envault show:*)",
      "Bash(envault plan:*)"
    ],
    "ask": [
      "Bash(envault load:*)",
      "Bash(envault exec:*)"
    ],
    "deny": [
      "Bash(envault get:*)",
      "Bash(envault shell:*)",
      "Read(./.env)",
      "Read(./.env.local)"
    ]
  }
}
```

The block above lists two specific files instead of a wildcard pattern, because a wildcard over `.env` would also block reading `.env.example`, and Claude needs to read that file to find out which keys a project expects. If your project has other secret file name variants, such as a production or staging environment, add a specific deny entry for each one.

## Moving the key to the keychain

By default, the vault's encryption key lives in a `0600` file in the user's home directory. To move it to the operating system keychain:

```
envault key migrate
```

The command only removes the key file after confirming it can read the key back from the keychain.

## Environment variables

| Variable | Effect |
|---|---|
| `ENVAULT_HOME` | Overrides the vault directory (`vault.enc`, `key` and `vault.lock`). Without it, uses `os.UserConfigDir()/envault`. |
| `ENVAULT_KEY` | Encryption key in base64, overriding any key storage implementation. |
| `ENVAULT_DEBUG` | With value `1`, `true`, `yes` or `on`, turns on the TUI's debug log to a file. |

## Architecture

Clean Architecture with three Bounded Contexts under `internal/`: `vault` (the vault, the core), `compose` (assembling the `.env`, selection, `exec` and templates) and `skill` (Claude Code integration). Each one has `domain`, `usecase` and `infra`. The shared kernel lives in `internal/shared` (`dotenv` and `config`), and `cmd/envault/main.go` is the composition root.

```
internal/
  vault/    domain usecase infra
  compose/  domain usecase infra
  skill/    domain usecase infra
  shared/   dotenv config
  delivery/
    cli/    app command/{vault,compose,skill} presenter
    tui/    app screen/{list,detail,compose,confirm} viewmodel theme
```

Delivery lives in two packages:

- `internal/delivery/cli`: `app` wires up the commands and the global flags; `command/vault`, `command/compose` and `command/skill` each have one file per command, which reads the flags, calls the use case and hands the result to the presenter; `presenter` handles human and JSON output, the `schema_version`, the exit code and the error envelope; `root.go` registers everything.
- `internal/delivery/tui`: the `tui.go` facade opens `app`, the root model that routes between screens; `screen/list`, `screen/detail`, `screen/compose` and `screen/confirm` each have one model per screen; `viewmodel` translates use case output into screen lines (value masking, selection, conflicts, template checklist); `theme` holds styles, keys and help text.

`internal/architecture_test.go` locks in the dependency rules: `domain` only imports `internal/shared`; `usecase` only imports its own `domain` and `internal/shared`; `internal/shared` doesn't import any Bounded Context or the delivery layer; no Bounded Context imports `domain`, `usecase` or `infra` from another Bounded Context, with the single exception of `compose/infra/vaultsource` over `vault/usecase`; only `cmd/envault` imports `internal/delivery`, and inside delivery only the same interface (`cli` or `tui`); production delivery doesn't import `domain`, `infra`, `shared/dotenv` or `os/exec`; `command/...` and `screen/...` don't import `os`, `io/fs` or `path/filepath`; `command/...` doesn't import `fmt`, because it writes only through the presenter; and a TUI screen only imports `theme`, `viewmodel` and `usecase` from the Bounded Contexts. Run it with `go test ./internal -run Architecture`.

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

For the brand assets used across the project, see [docs/brandkit](./docs/brandkit).

## Contact

Giovani Valgas - [/in/giovalgas](https://www.linkedin.com/in/giovalgas/) - giovalgascom@gmail.com

[ci-shield]: https://github.com/giovalgas/envault/actions/workflows/ci.yaml/badge.svg
[ci-url]: https://github.com/giovalgas/envault/actions/workflows/ci.yaml
[release-workflow-shield]: https://github.com/giovalgas/envault/actions/workflows/release.yaml/badge.svg
[release-workflow-url]: https://github.com/giovalgas/envault/actions/workflows/release.yaml
[release-shield]: https://img.shields.io/github/v/release/giovalgas/envault
[release-url]: https://github.com/giovalgas/envault/releases/
