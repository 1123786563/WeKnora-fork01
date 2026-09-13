# Mobile test runner fix — 2026-09-13

Slice: Vue→React parity / react-multiclient worktree. Defect: `pnpm run test:mobile` failed `src/features/auth/onboarding-component.test.ts` (MODULE_NOT_FOUND) while `npx tsx --test` on the same file passed 4/4.

Environment: node v26.7.0, tsx 4.23.1, pnpm 10.28.2, strict pnpm workspace layout.

## Root cause

The test imports `esbuild` (and `jsdom`) as bare specifiers, but `@weknora/mobile` declares neither as a dependency. Under pnpm's strict layout `esbuild@0.28.2` exists only as:

- a declared dependency of `@weknora/web` → symlinked at `apps/web/node_modules/esbuild`, and
- a (private) dependency of `tsx` itself → symlinked at `node_modules/.pnpm/tsx@4.23.1/node_modules/esbuild`.

Neither location is on Node's walk-up path from `apps/mobile/src/...`, so plain resolution must fail. `jsdom` resolves only because the workspace root `package.json` declares it as a devDependency (root importer deps are always present in `<root>/node_modules`, which the walk-up reaches).

The two runner invocations differ in one environmental detail, captured by dumping the spawned test-runner child's `process.env`/`process.execArgv` from a probe test:

- `npx tsx --test <file>` — the tsx CLI spawns node with `--require .../tsx/dist/preflight.cjs`, `--import file://.../tsx/dist/loader.mjs`, **and sets `NODE_PATH`** to a chain of tsx-internal module dirs:
  `.../.pnpm/tsx@4.23.1/node_modules/tsx/dist/node_modules:...tsx/node_modules:...pnpm/tsx@4.23.1/node_modules:...pnpm/node_modules`
  `NODE_PATH` entries feed `Module.globalPaths` and are consulted as a fallback for bare specifiers, so `import 'esbuild'` accidentally resolves into **tsx's own private esbuild** (the probe showed the resolved path: `node_modules/.pnpm/esbuild@0.28.2/node_modules/esbuild/lib/main.js`). The pass was accidental — a leak of tsx's implementation detail.
- `node --import tsx --test 'src/**/*.test.ts'` — node's test runner propagates only `execArgv` (`--import tsx`, bare specifier, resolved from cwd); no `NODE_PATH` is set (`process.env.NODE_PATH === null` in the child). tsx's own `_resolveFilename` hook (`resolveTsPaths`) is a passthrough without tsconfig path matches, so the bare `esbuild` import throws:

```
Error: Cannot find module 'esbuild'
Require stack:
- apps/mobile/src/features/auth/onboarding-component.test.ts
    at resolveTsPaths (.../tsx/dist/register-BoI6-WNn.cjs:10:770)
    at Module._resolveFilename (...)
  code: 'MODULE_NOT_FOUND'
```

Controlled bisects confirming the mechanism (probe test dumping `createRequire(<test file>).resolve('esbuild')`):

| invocation | esbuild resolves |
|---|---|
| `npx tsx --test` | yes (via CLI-set `NODE_PATH`) |
| `node --import tsx --test` | no — MODULE_NOT_FOUND |
| `node --require preflight.cjs --test` | no |
| `node --import loader.mjs --test` | no |
| fresh combined flags, no CLI (env diff proves `NODE_PATH` only comes from the CLI spawn) | no |

Changing the test script to `npx tsx --test` was rejected: it would pass only through the accidental `NODE_PATH` leak and is therefore not an equivalent, stable fix.

## Fix (apps/mobile only)

1. `apps/mobile/src/features/auth/onboarding-component.test.ts` — removed the bare `import { build } from 'esbuild'`; esbuild is now loaded through the `createRequire(apps/web/package.json)` require the test already uses for `react`/`react-dom`, i.e. from a **declared** dependency of a workspace package, deterministic under pnpm regardless of runner:

   ```ts
   const { build } = require('esbuild') as typeof import('esbuild');
   ```

   (`jsdom` kept as static import: the root workspace devDependency is always present in root `node_modules`, so walk-up resolution is deterministic.)

2. `apps/mobile/tsconfig.json` — added a type-resolution-only mapping so `tsc` finds esbuild's real declarations (runtime is unaffected by `paths`):

   ```json
   "paths": { "@/*": ["./src/*"], "esbuild": ["../web/node_modules/esbuild"] }
   ```

   This also fixed a **pre-existing** `pnpm run typecheck:mobile` failure (verified by stashing the test fix and re-running: `tsc --noEmit` exited 2 before any change from this slice), caused by the same gap: `TS2307 Cannot find module 'esbuild'` in the four component tests (`onboarding-component.test.ts`, `DataSourcesScreen.test.tsx`, `KnowledgeBaseListScreen.pins.test.tsx`, `KnowledgeBaseListScreen.upload.test.tsx`) cascading into `TS7006`/`TS7031` implicit-any errors in the esbuild plugin callbacks once `build` degraded to `any`. No shared files (apps/web, packages/**, root lockfile) were touched.

## Before / after

Red (before), `cd apps/mobile && node --import tsx --test 'src/**/*.test.ts'` (exit 1):

```
Error: Cannot find module 'esbuild'  (requireStack: .../onboarding-component.test.ts)
ℹ tests 115
ℹ pass 114
ℹ fail 1        ← the failing entry is the module-load crash of onboarding-component.test.ts
```

Green (after), same command (exit 0) and `pnpm run test:mobile` from the root:

```
ℹ tests 118
ℹ pass 118
ℹ fail 0
```

(115 → 118: previously the crashed file contributed 1 failed module entry and 0 runnable tests; it now contributes its 4 tests: 114 + 4 = 118.)

`pnpm run typecheck:mobile` after the fix: exit 0, no errors.

## Limitations / follow-ups for the coordinator

- `src/features/knowledge/*.test.tsx` (DataSourcesScreen, KnowledgeBaseListScreen.pins, KnowledgeBaseListScreen.upload) share the bare-`import { build } from 'esbuild'` runtime pattern. They are **not executed** by the test script (glob `'src/**/*.test.ts'` does not match `.test.tsx`), so they do not fail today, but they will break the same way if the glob is ever widened to `.tsx`. Under `tsc` they are already fine now (paths fix). Recommended follow-up: either route their esbuild import through the same web-rooted require, or (cleanest long-term) declare `esbuild` + `jsdom` as devDependencies of `@weknora/mobile` — that requires a `pnpm-lock.yaml` (workspace root, shared) update, so it is left to the coordinator's call.
- The passing behavior of any bare-import test under the `tsx` CLI should not be trusted: it depends on tsx's `NODE_PATH` leak, not on declared dependencies.
- Evidence artifacts referenced: `/tmp/mobile-test-before.log`, `/tmp/mobile-test-after.log` (raw runner output during this session).
