# T22 mobile artifact preview follow-up — 2026-09-12

This increment closes a bounded native chat artifact interaction gap. It does
not claim full T22 or T24 acceptance.

## Implemented boundary

- `apps/mobile/src/features/chat/artifact-preview.ts` classifies text,
  Markdown, and non-SVG images for native preview. HTML, XHTML, SVG, PDF,
  Office, and unknown content stay download/share-only.
- Markdown is projected into native `Text` rows for headings, bullets, and
  fenced code. The source is never passed to a DOM or HTML interpreter, so
  raw tags remain text.
- `ChatScreen` downloads artifacts through the existing authenticated,
  message-scoped route. Text and Markdown are read from the private cache;
  images use the private cache URI; the native Modal exposes Back and Share.
- `apps/mobile/src/platform/files.ts` keeps file reading platform-owned and
  reuses the existing safe filename and authenticated download path.

## Verification

| Command | Result |
|---|---|
| `pnpm exec tsx --test apps/mobile/src/features/chat/artifact-preview.test.ts` | 2/2 passed after the intentional missing-module RED run |
| `pnpm test:mobile` | 79/79 passed |
| `pnpm --filter @weknora/mobile typecheck` | exit 0 |
| `pnpm exec expo export --platform ios --output-dir /tmp/weknora-react-mobile-artifact-ios` | exit 0; 1,167 modules bundled |
| `pnpm exec expo export --platform android --output-dir /tmp/weknora-react-mobile-artifact-android` | exit 0; 1,190 modules bundled |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

## Evidence boundary

The tests and Expo exports prove source-level classification, safe native
projection, type safety, and iOS/Android JS bundleability. No physical-device
artifact fixture, provider-backed protected download, or native click-through
was available in this run. T22 and T24 therefore remain `review`; the
remaining complex preview and device/provider matrix are still open.
