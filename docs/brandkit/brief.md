# envault brand brief

## Category

Developer tool. A local, encrypted vault for environment variables, with a terminal UI (Bubble Tea), a CLI (Cobra) and a Claude Code Skill that assembles a project's `.env` without ever seeing the values.

## Audience

Developers who juggle many `.env` files across projects, and who increasingly work next to AI coding agents. They live in the terminal, distrust cloud secret managers for local work, and want something fast, quiet and predictable.

## Personality

- Precise: one job, done exactly, with stable exit codes and JSON.
- Discreet: values stay masked until you explicitly reveal them.
- Calm: no alarms, no noise, a vault that simply holds.
- Composable: small named envs that combine in a clear order.
- Native: looks like it belongs in a terminal, because it does.

## Core metaphor

The sealed value. An env file is a list of `KEY=VALUE` lines; envault keeps the keys in sight and the values sealed. People, scripts and agents can see which keys exist and where they come from, but the value itself stays behind the vault wall until the moment it is written.

## Logo idea

The mark is a "sealed equals" inside a vault frame. The rounded square frame reads as both a vault door and a terminal window. Inside it, an equals sign is split in two: the top bar is the key, drawn solid and neutral because it is visible; the bottom bar is replaced by three lilac dots, the masked value, the only part of the mark that carries the accent color. The mark is built on a 12 unit grid with a 1 unit stroke, a 3 unit corner radius, and dots whose diameter equals the key bar height, so it reduces cleanly down to 16 px.

The wordmark is `envault` in lowercase, bold system monospace, set tight.

## Tagline

Keys in sight. Values sealed.

Support line: local / encrypted / composable / agent-safe.

## Visual system

- Mode: dark developer, near-black panels, monospace accents, terminal frames.
- Palette: Vault Ink `#0E0C13`, Chamber `#1D1927`, Fog `#ECE8F5`, Seal Lilac `#B69CFF`, Deep Seal `#5A3FC0`, Marked Mint `#7FD99A`. Lilac and mint come from the real TUI theme, where lilac is the accent and mint marks selected envs.
- Type: system monospace (ui-monospace, SF Mono, Menlo, Consolas) for everything the machine reads; system sans (SF Pro, Segoe UI, Helvetica) for headlines only.
- Imagery: sealed chambers seen head on, halftone dot rings, scanlines, low light with a single lilac glow.
- Detail: masked values (`••••••••`), selection chips `[1] [2]`, `SELECTED_ENVS=` lines, key hints, status chips.

## Avoid

Padlock clipart, shields, keys on rings as a logo, cloud imagery, neon hacker green, glitch effects, generic purple gradients, and any mockup that shows a real secret value.

## Files

- `brandkit.svg`: 3x3 overview board.
- `logo.svg`, `logo-mono.svg`, `mark.svg`: logo lockup, single color lockup, symbol alone.
- `../../assets/icon.svg`: app icon tile for the README.
- `../../assets/demo-tui.svg`, `../../assets/demo-cli.svg`: placeholders for the usage GIFs.
- `prompt.md`: prompt for a raster version in an image model.
