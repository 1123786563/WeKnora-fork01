# T23 mobile organizations evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`

## Implemented boundary

- `apps/mobile/src/features/management/OrganizationsScreen.tsx` adds `/management/organizations` to the capability-gated management hub.
- The screen uses `client.identity.organizations` for organization listing/creation, member listing, organization member role updates/removal, and pending join-request approval/decline.
- Organization member writes are exposed only when the server-projected organization `my_role` is `admin`; viewers can read the organization and pending-request state but do not receive write controls.
- Organization creation validates the name locally and waits for the server-created record before selecting it. Mutation failures preserve the current local rows and display the server error. There is no offline queue.
- The shared API index now exports `OrganizationRole` so the native route uses the same role contract as Web and desktop.

## Automated evidence

```text
pnpm --filter @weknora/mobile exec tsx --test src/features/management/organizations.test.ts src/features/management/administration.test.ts src/features/management/capabilities.test.ts # 6/6, exit 0
pnpm test:mobile                                                                                 # 29/29, exit 0
pnpm typecheck:mobile                                                                             # exit 0
pnpm typecheck:shared                                                                             # exit 0
pnpm test:shared                                                                                  # 157/157, exit 0
pnpm build:web                                                                                    # exit 0
node scripts/check-react-boundaries.mjs                                                            # exit 0
git diff --check                                                                                  # exit 0
```

Expo exports after this increment also passed:

- iOS: exit 0, Hermes bundle about 3 MB, output `/tmp/weknora-react-mobile-org-ios.KyDKoG`.
- Android: exit 0, Hermes bundle about 3.1 MB, output `/tmp/weknora-react-mobile-org-android.qCRNXX`.

A fresh device or native-install run is not claimed here.

## Evidence boundary

This proves the native route, typed organization API wiring, role-gated mutation composition, join-request decision payload, type safety, and regression compatibility. It does not claim live organization create/member/share/API-key results, real 403/409 permutations, native device interaction, or completion of the remaining T23 configuration/integration/sandbox management matrix.
