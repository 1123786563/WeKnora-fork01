# H03 evidence — product login and secure credentials

## Static implementation evidence

- `packages/api-client/src/auth/login.ts` normalizes the product login envelope (`token`, `refresh_token`, `user.id`, and `active_tenant`/legacy `tenant`) and rejects failed, empty, or malformed responses before a credential can be written.
- `createProductAuth` sends password login to `POST /api/v1/auth/login` and refresh requests to `POST /api/v1/auth/refresh` using the product wire field `refreshToken`.
- `createProductAuthSession` composes the existing `createRefreshCoordinator`; its authenticated request path retries a confirmed 401 exactly once with the rotated access token, while network/unknown failures are not replayed.
- `apps/mobile/sources/weknora/auth/credentials.ts` implements the unchanged `CredentialAdapter` over an injected async store. Product origin keys are namespaced with `weknora:credentials:${encodeURIComponent(origin)}`, so they do not overlap with Happy secrets or another product origin.
- The selected product origin is persisted in SecureStore and restored by `ProductHostBootstrap` before origin-scoped credentials are read. `LoginScreen` writes only parsed bearer credentials, clears the password state on validation failures and request failures, and never logs or persists the password. `ProductAuthProvider` reloads credentials on restart and the root navigation gate sends host-authenticated users without credentials to login.

## Focused test evidence

Command:

```text
pnpm exec tsx --test packages/api-client/src/auth/login.test.ts apps/mobile/sources/weknora/auth/credentials.test.ts packages/api-client/src/auth.test.ts
```

Result: **14 tests passed, 0 failed**. This includes login field normalization and rejection, malformed tenant containers, product login/refresh wire paths, concurrent 401 retry with one refresh, selected-origin restart/clear, credential restart reads, clear, origin isolation, invalid stored data, and concurrent refresh single-flight/clear behavior.

## Native/build evidence

Native export and device launch were not run in this task environment. The mobile typecheck was run after refreshing workspace links; it remains blocked by existing H02/upstream issues in `AnimatedOverlay.tsx`, `MobileGlass.tsx`, and unresolved `@slopus/happy-wire` types. The new H03 files also cannot be typechecked by the current stale workspace link for `@weknora/api-client`; a clean install from the committed workspace lockfile resolves that link.
