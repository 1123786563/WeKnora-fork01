# T23 mobile organizations evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`

## Implemented boundary

- `apps/mobile/src/features/management/OrganizationsScreen.tsx` adds `/management/organizations` to the capability-gated management hub and rechecks the server capability on direct route entry.
- The screen uses `client.identity.organizations` for organization listing/creation, member listing, organization member role updates/removal, and pending join-request approval/decline.
- Organization member writes are exposed only when the server-projected organization `my_role` is `admin`; viewers can read the organization and pending-request state but do not receive write controls. A server capability of `supported: false` fails closed in both the hub and the direct route.
- Organization creation validates the name locally and waits for the server-created record before selecting it. Mutation failures preserve the current local rows and display the server error. There is no offline queue.
- The shared API index now exports `OrganizationRole` so the native route uses the same role contract as Web and desktop.

## Automated evidence

```text
pnpm --filter @weknora/mobile exec tsx --test src/features/management/organizations.test.ts src/features/management/administration.test.ts src/features/management/capabilities.test.ts # 7/7, exit 0
pnpm test:mobile                                                                                 # 30/30, exit 0
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

## Isolated Lite HTTP evidence

Using the already running isolated SQLite Lite process at `127.0.0.1:18082` (not production data), a new `orgprobe@example.test` owner and `orgviewer@example.test` viewer were registered in tenants 7 and 8. The owner created organization `6f37070c-0472-4376-9ce7-ec0821bfafa0` (`React Mobile Org`) and received `my_role: admin`; the owner list returned one organization and the organization member list returned both tenant members.

- The viewer joined through the server-issued invite code and received `my_role: viewer`.
- The owner updated tenant 8 from `viewer` to `editor`: HTTP 200, `Member role updated successfully`; a subsequent owner member list returned tenant 8 as `editor`.
- The viewer attempted the same member-role mutation and received HTTP 403 with `Permission denied or invalid operation`; no local/client-side success was inferred.

Credentials and access tokens were held only in shell variables and are not recorded here. This is real backend role evidence for the organization slice, not a mock. It does not cover every organization share/API-key path or native device interaction.

## Evidence boundary

This proves the native route, typed organization API wiring, role-gated mutation composition, join-request decision payload, type safety, and regression compatibility. It does not claim live organization create/member/share/API-key results, real 403/409 permutations, native device interaction, or completion of the remaining T23 configuration/integration/sandbox management matrix.

## Isolated Lite share-management follow-up — 2026-09-12

Using a fresh isolated FTS5 Lite process on `127.0.0.1:18085`, an owner
created a temporary knowledge base and organization, generated an invite code,
and created an editor share. A second registered tenant joined the
organization through that invite code.

- The owner organization share list returned `1` record after share creation.
- The joined tenant could read the organization share list and received the
  same `1` record.
- The joined tenant's attempt to delete the source knowledge-base share
  returned HTTP `403`; no client-side success was inferred.
- The owner removed the share successfully, and a subsequent owner list
  returned `0`. The temporary KB and organization were then deleted with
  successful responses; the Lite process was stopped and no production data
  was used.

This closes the isolated real-backend owner share create/list/remove path and
the cross-tenant member read-versus-remove negative. Agent-share runtime,
native organization interaction, full 403/409 permutations, and the remaining
T23 configuration/integration/sandbox matrix remain open.
