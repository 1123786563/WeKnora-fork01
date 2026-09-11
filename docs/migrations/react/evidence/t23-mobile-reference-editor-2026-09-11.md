# T23 mobile Wiki/FAQ editor evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`
Commit: `78928a4`

## Implemented boundary

- `apps/mobile/src/features/knowledge/KnowledgeEditorScreen.tsx` adds a native editor route at `/knowledge/:id/editor`.
- Wiki existing records call the typed `wiki.update` endpoint with the server `slug` and optimistic `version`; new records call `wiki.create`.
- FAQ existing/new records call the typed `knowledge.faq.update`/`create` endpoints and preserve only the existing question, first answer, enabled, and recommended fields.
- The reference list shows New/Edit controls only when the active workspace role is `owner` or `admin`. Other roles remain read-only. The server remains the final authorization boundary.
- Required fields are validated locally. A 409 is shown as a stale-version conflict with a reload-latest action; other API errors preserve the draft and expose an explicit retry path. There is no offline write queue or generic JSON editor.

## Automated evidence

Commands run from the React worktree:

```text
pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/editor.test.ts     # 4/4, exit 0
pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/reference-parity.test.ts # 3/3, exit 0
pnpm test:mobile                                                                        # 24/24, exit 0
pnpm typecheck:mobile                                                                    # exit 0
pnpm test:shared                                                                         # 157/157, exit 0
pnpm typecheck:shared                                                                    # exit 0
pnpm test:web                                                                            # 61/61, exit 0
pnpm typecheck:web                                                                       # exit 0
pnpm build:web                                                                           # exit 0
node scripts/check-react-boundaries.mjs                                                   # exit 0
git diff --check                                                                        # exit 0
```

Expo exports also passed:

- iOS: exit 0, Hermes bundle about 3 MB, output `/tmp/weknora-react-mobile-editor-ios.3zSoPa`.
- Android: exit 0, Hermes bundle about 3.1 MB, output `/tmp/weknora-react-mobile-editor-android.KidKhc`.

## Evidence boundary

This increment proves source contracts, role-gated route composition, validation/error handling, type safety, and iOS/Android JS bundleability. It does not claim a real 403/409 response or completion of the remaining T23 management matrix.

## Native iOS live follow-up — 2026-09-12

- The isolated FTS5 Lite server ran on `127.0.0.1:18084` with a fixed probe
  JWT secret and a local OpenAI-compatible embedding mock on `127.0.0.1:19100`.
  A temporary embedding model (`e47b481e...`), FAQ KB (`07f139c1...`), and
  Wiki KB (`4151be8f...`) were created solely for this probe and removed after
  the run. The server and mock were stopped afterward.
- The iPhone 17 Pro iOS 26.5 Release build installed and authenticated as the
  owner. The FAQ list rendered the real row
  `How does iOS FAQ work? · enabled · recommended`; native Edit changed the
  question to `How does iOS FAQ work after native edit?`, Save returned to the
  list, and Refresh retained the updated server value. A direct authenticated
  GET confirmed the updated question. Screenshot:
  `/tmp/weknora-ios-faq-latest.png` (SHA-256
  `80b2a5ba2202a5eb35c0f5a3cd10318520ed2e5f5634be6c4315057d0c08e2f8`).
- The same Release app rendered the real Wiki row `iOS Wiki Probe · v1`.
  Native Edit loaded the page by its server `slug` (the prior UUID route
  produced a 404), changed title/content, Save returned `Version 2`, and
  Refresh retained `iOS Wiki Probe edited · v2`. A direct authenticated GET
  confirmed title, content, and version 2. Screenshot:
  `/tmp/weknora-ios-wiki-latest.png` (SHA-256
  `92dc9a90f64f7af749041eba44e5c80288691fe0b260f0ec53840c450c994704`).
- The first FAQ list load also exposed that Lite returns nullable
  `similar_questions` and `negative_questions`. The focused parser test was
  observed RED (3/4) before the fix, then GREEN at 4/4 after normalizing only
  those optional arrays to `[]`; `answers` remains strict. Reference routing
  tests similarly went RED (3/5) on the Wiki UUID, then GREEN at 5/5 after
  using Wiki `slug` and FAQ numeric `id` edit keys.

This closes bounded isolated Lite owner read/write/refresh evidence for native
FAQ and Wiki editing. It does not close non-owner 403/409 permutations,
production-provider acceptance, physical-device behavior, or the remaining
T23 management matrix.

## Fresh regression after the live follow-up

```text
pnpm --filter @weknora/api-client exec tsx --test src/knowledge/faq.test.ts # 4/4, exit 0
pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/reference-parity.test.ts # 5/5, exit 0
pnpm test:mobile # 56/56, exit 0
pnpm test:shared # 164/164, exit 0
pnpm typecheck:mobile # exit 0
pnpm typecheck:shared # exit 0
node scripts/check-react-boundaries.mjs # exit 0
git diff --check # exit 0
```
