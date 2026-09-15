# R312 Settings load fallback localization (2026-09-15)

## Change

`SettingsPage` no longer constructs an English `Unable to load …` fallback from the section title. Unknown settings-section load failures now use the current locale's shared `common.error` message, while an explicit server `Error.message` remains authoritative.

## Verification

- `node --import tsx --test apps/web/src/settings/SettingsPage.test.tsx`: 16/16 passed.
- `pnpm run typecheck:web`: passed.
- `pnpm test:web`: 911/911 passed; failed/cancelled/skipped: 0.

## Remaining evidence gap

This is static/unit and full Web-suite evidence. Protected settings backend failures, five-locale browser screenshots, responsive comparison against Vue, and Wails/native runtime evidence remain open.
