# envault raster prompt

Ready to paste into an image model. It follows the prompt template of the brandkit skill and describes the same system as `brandkit.svg`.

```
Create a premium brand-kit overview image for "envault".

Brand strategy:
- category: developer tool, a local encrypted vault for environment variables with a terminal UI, a CLI and an AI agent skill
- audience: developers who live in the terminal and work alongside AI coding agents
- personality: precise, discreet, calm, composable, terminal-native
- core metaphor: the sealed value; keys stay in sight, values stay sealed behind the vault wall
- logo idea: a rounded square frame that reads as both a vault door and a terminal window, holding an equals sign split in two; the top bar is a solid neutral bar (the visible key), the bottom bar is three lilac dots (the masked value), the only accent in the mark; wordmark "envault" in lowercase bold monospace

Layout:
3×3 grid on a near-black presentation canvas (#09080C) with strong equal gutters, rounded panels, clean alignment, refined negative space, tiny page numbers "01 / 09" to "09 / 09" and small uppercase section labels.

Panels:
- logo cover: large mark centered, wordmark "envault" below, small line "local encrypted env vault"
- logo concept / construction: the mark on a 12 unit grid with dashed lilac guides for the key line, the value line and the corner radius; captions "frame vault + window", "bar key, visible", "dots value, sealed"
- digital application: a terminal window running the envault TUI; header "envault", a boxed line "SELECTED_ENVS=stripe-test,postgres-local", two columns; left column lists envs with "[1] stripe-test", "[2] postgres-local", a focused row "> [ ] redis-local", "[ ] aws-dev"; right column shows "redis-local", a short description, tags "cache, local" and "REDIS_URL: ••••••••"; a quiet key-hint footer
- tagline / brand essence: a solid lilac panel with large dark sans text "Keys in sight. Values sealed." and a row of eight dark dots
- color system: six tall swatch cards, Vault Ink #0E0C13, Chamber #1D1927, Fog #ECE8F5, Seal Lilac #B69CFF, Deep Seal #5A3FC0, Marked Mint #7FD99A, with a thin proportion strip below
- typography: a huge monospace "Aa", primary system monospace and secondary system sans, a lowercase and uppercase alphabet row, and "Keys in sight." as a sans headline
- physical application: a matte black key tag with the mark, the wordmark and "vault.enc 0600", a metal ring, a round lilac sticker with the mark and "AGENT-SAFE", and a small ivory card showing "KEY ••••••••", on a dark desk with soft shadows
- image direction: a vault door seen head on, made of concentric rings of halftone lilac dots fading into darkness, faint scanlines, the mark glowing softly in the center
- system detail: a command bar "$ envault load $(envault selection) --out .env" with a lilac block cursor, mint outlined chips "[1] stripe-test" and "[2] postgres-local", a masked chip "REDIS_URL: ••••••••" next to "v reveal", status chips "local", "age encrypted", "gitignored", and the app icon at five sizes

Visual mode:
Dark Developer / Builder

Palette:
near-black #09080C and #0E0C13, chamber #15121C and #1D1927, fog #ECE8F5, seal lilac #B69CFF as the single dominant accent, deep seal #5A3FC0 for depth, marked mint #7FD99A only for selected state

Style:
premium, sparse, cinematic, intentional, polished, brand-guidelines deck, no clutter, no copied real-world logos, no padlocks, no shields, no neon green, no glitch effects, never show a real secret value.

Typography:
readable, minimal, high hierarchy, no tiny fake text; system monospace for UI and labels, bold system sans only for the tagline.

Logo:
professional, symbolic, simple, ownable, based on the brand's purpose, repeated consistently across panels: cover, construction, key tag, sticker, image glow and icon row.
```

Suggested output: square 1:1 at 2400×2400, or 16:10 if the model prefers wide boards.
