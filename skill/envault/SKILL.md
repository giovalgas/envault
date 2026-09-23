---
name: envault
description: Loads environment variables from the local envault vault to build a project's .env. ALWAYS use it when the user is about to start, set up or run a project that needs a .env, mentions environment variables, secrets, credentials, API keys, .env.example, or asks to "load"/"use" an env by name, even without mentioning envault.
---

# envault

You build project `.env` files from envs stored in the `envault` vault. You **never see values**: you only work with env names, descriptions and key names.

## Hard rules

- The only envault commands you run without asking for permission are `envault --version`, `envault list`, `envault show` and `envault plan`. Any other envault command needs explicit user confirmation in the current conversation.
- NEVER run `envault get`, `envault shell` or use `--show`.
- NEVER read `.env` files (cat, Read, grep). You may read `.env.example`.
- NEVER run `envault new`/`edit` (they are interactive). If an env is missing, ask the user to create it with `envault` (TUI) or `envault new <name>`.
- NEVER run `envault load` without explicit user confirmation **for that specific load**, in the current conversation.
- Do not change `.gitignore` without asking.

## Flow

### 0. Pre-check
Run `envault --version`. If it fails, explain how to install it and stop. If `envault list` exits with code 4, ask the user to run `envault init`.

### 1. Did the user say which envs to use?
- **Exact names:** confirm they exist with `envault list --json` and go to step 3.
- **Vague reference** ("the stripe one"): run `envault list --json --search <term>`. One clear match: confirm it with the user. Several: step 2.
- **Nothing:** step 2.

### 2. Discovery
1. Understand what the project needs: `.env.example`, dependencies (`package.json`, `go.mod`, `requirements.txt`, `pyproject.toml`...), `docker-compose.yml`.
2. Run `envault list --json`.
3. Match name, description, tags and key names against the project needs. To inspect a match, run `envault show <name> --json`, which shows only key names.
4. Present the matches (name, description, which template keys each one covers) and ask the user to pick **one or more**, and in which order. Use the question tool with multiple selection if available; otherwise, a numbered list.

### 3. Plan
Run `envault plan <envs...> --out <target>`. Read from the JSON: keys and their source, `conflicts`, `missing`, `target.exists`, `target.gitignored`.

### 4. Permission (always)
Show a summary and ask whether to proceed:
- envs in precedence order (the last one wins);
- total keys and conflicts (which env wins);
- template keys that are missing;
- if the target exists: ask whether to **overwrite** (`--force`) or **merge** (`--merge`).

### 5. Load
Only after a "yes": `envault load <envs...> --out <target> [--force|--merge]`.
Then:
- if `gitignored` is `false`, ask whether you may add the file to `.gitignore`;
- report the **names** of the missing keys and suggest creating them in envault.

## Exit codes

| code | meaning |
|---|---|
| 3 | env not found |
| 4 | vault not initialized |
| 5 | target exists |
| 6 | decryption failed |
| 7 | validation |

Explain the error to the user; do not try to work around it.
