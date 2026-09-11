# T23 mobile administration evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`

## Implemented boundary

- `apps/mobile/src/features/management/AdministrationScreen.tsx` adds `/management/administration` from the capability-gated management hub.
- The screen reads tenant members, pending invitations, and audit rows through `client.identity.tenants`.
- Owner/admin workspaces can invite a member, change a non-owner member role, remove a non-owner member after native confirmation, and revoke a pending invitation. Every mutation waits for the server response and reloads; rejected writes preserve the current rows and display the error.
- Viewer/contributor/unknown roles receive read-only member/invitation/audit views. The native UI never treats a local role label as authorization; server 403/409 responses remain observable.
- The capability matrix now records identity and Wiki/FAQ as `management`, with server-owned role/version boundaries. Configuration remains read-only; sandbox, offline writes, and Embed/IM administration remain explicitly unsupported.

## Automated evidence

```text
pnpm --filter @weknora/mobile exec tsx --test src/features/management/administration.test.ts src/features/management/capabilities.test.ts # 4/4, exit 0
pnpm test:mobile                                                                                 # 27/27, exit 0
pnpm typecheck:mobile                                                                             # exit 0
pnpm test:shared                                                                                  # 157/157, exit 0
pnpm typecheck:shared                                                                             # exit 0
pnpm test:web                                                                                     # 61/61, exit 0
pnpm typecheck:web                                                                                # exit 0
pnpm build:web                                                                                    # exit 0
node scripts/check-react-boundaries.mjs                                                            # exit 0
git diff --check                                                                                  # exit 0
```

Expo exports passed:

- iOS: exit 0, Hermes bundle about 3 MB, output `/tmp/weknora-react-mobile-admin-ios.S3cL82`.
- Android: exit 0, Hermes bundle about 3.1 MB, output `/tmp/weknora-react-mobile-admin-android.0xJCND`.

## Evidence boundary

This proves the native route, shared identity API wiring, role-gated mutation composition, owner safety rule, error-preserving state handling, type safety, and iOS/Android JS bundleability. It does not claim live backend member writes, real 403/409 role permutations, native device interaction, organization/API-key/system-admin writes, or completion of the remaining T23 management matrix.

## Native iOS live follow-up — 2026-09-12

- Rebuilt and installed the current iOS Release host with
  `EXPO_NO_DOCTOR=1 pnpm --filter @weknora/mobile exec expo run:ios --device
  'iPhone 17 Pro' --no-bundler --configuration Release`. The build reported
  `Build Succeeded`, 0 errors, and 6 warnings (the existing duplicate C++
  library, missing Metal toolchain search path, and SDWebImage deployment
  mismatch warnings).
- On iPhone 17 Pro iOS 26.5, the authenticated owner opened Manage → Members
  and audit. The page rendered one member, the pending invitation, and the
  real audit row `rbac.invitation_sent · success`; the previous parser error
  was gone.
- The isolated Lite response for that audit row contains empty
  `scope_type`, `scope_id`, `request_path`, and `request_method` values. These
  fields are optional metadata in the current Go response and are now checked
  as strings without requiring them to be non-empty. The new regression test
  covers this exact response shape.
- Screenshot after the fixed Release bundle loaded the live data:
  `/tmp/weknora-ios-admin-latest.png` (SHA-256
  `9da69973e3c35d9ae11311bd6236b6125447306749ffe11b50a8a942aabb6961`).

The live result is bounded to an isolated Lite owner/invitation/audit read and
the native Release UI. It does not close non-owner 403/409 permutations,
physical-device behavior, or the remaining T23 management matrix.
