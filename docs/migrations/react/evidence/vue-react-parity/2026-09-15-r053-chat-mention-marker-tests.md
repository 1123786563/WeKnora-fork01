# R053 Chat mention marker regression coverage

- Added component coverage for all non-KB resource markers and stable `data-mention-type` attributes in the listbox.
- The test asserts file/tag/MCP/Skill markers (`▧`, `#`, `⚒`, `✦`) and ordering, while the existing KB keyboard/removal coverage remains unchanged.
- Verification: Web tests 893/893, `pnpm run typecheck:web`, and `pnpm run build:web` passed.
