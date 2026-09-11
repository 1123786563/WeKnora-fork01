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

This increment proves source contracts, role-gated route composition, validation/error handling, type safety, and iOS/Android JS bundleability. It does not claim a live backend FAQ/Wiki write, real 403/409 response, native editor interaction, or completion of the remaining T23 management matrix.
