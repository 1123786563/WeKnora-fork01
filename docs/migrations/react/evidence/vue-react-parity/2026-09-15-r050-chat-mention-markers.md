# R050 Chat mention type markers

- Scope: selected and available chat mention items now expose `data-mention-type` and a compact type marker: `@` KB, `▧` file, `#` tag, `⚒` MCP, `✦` Skill.
- Purpose: make the five Vue resource categories visually distinguishable without changing names, selection behavior, ARIA listbox semantics, or request payloads.
- Verification: shared tests passed, Web tests 892/892, `pnpm run typecheck:web`, and `pnpm run build:web` passed.
