# W23 production-entry fix report

## Changes

- Added `resolveDesktopPersonalNode`, which resolves the delayed Wails API URL,
  Paseo URL/origin policy, and credential before creating the desktop
  personal-node composition.
- Changed `apps/desktop/src/main.tsx` to compose first and install the desktop
  runtime second, so a late `GetAPIBaseURL()` binding cannot silently disable
  the adapter.
- Added concrete Wails-bound `App` methods for credential read/set/delete and
  deployment-owned Paseo URL/origin policy. macOS builds use the login
  keychain through the `security` command; non-macOS/test environments retain
  an in-process fallback. Empty policy or credentials remain fail-closed.
- Added bootstrap regression tests for delayed binding, missing configuration,
  and revoke cleanup, plus Go bridge tests for credential and policy behavior.

## Focused evidence

```text
pnpm exec tsx --test apps/desktop/src/platform/bootstrap.test.ts
3/3 passed
```

The existing desktop test suite could not fully start because this worktree has
no linked `@weknora/paseo-adapter` package. Desktop typecheck could not start
because the worktree has no installed `vite/client` type definitions.

```text
DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./cmd/desktop
blocked-env / pre-existing W20 compile errors in internal/container
```

The Go bridge tests are included in `cmd/desktop/app_test.go`, but the package
cannot be compiled until the unrelated container errors are repaired. Native
Wails generation/launch and a live keychain prompt remain runtime evidence
gaps; this change proves the Go-bound bridge contract and fail-closed
composition statically/focused.
