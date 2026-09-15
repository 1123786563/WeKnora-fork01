# W17 Paseo compatibility and capability probe

Date: 2026-09-15
Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
Base: `b815f158`

## Fixed source

The adapter pins `@getpaseo/client` to `0.7.2`. The upstream source is `getpaseo/paseo` tag `v0.7.2`, commit `c4e97d5cfbef62934e0915d58a72ddeae5c4caea`. The lockfile records the package integrity and transitive `@getpaseo/*` packages. Node used for the check was `v26.7.0`.

The public SDK surface was checked from `@getpaseo/client@0.7.2/dist/index.d.ts`:

- create: `PaseoApi.agents.create`
- observe: `PaseoAgentHandle.waitForFinish`
- events: `PaseoAgentHandle.subscribe`

`cancel`, approval, steer, artifact export, and request-id lookup are not claimed as public capabilities. In particular, `DaemonClient.cancelAgent` exists on an internal declaration, but the adapter does not import that internal surface or present it as a public SDK feature.

## RED

Command:

```text
pnpm exec tsx --test services/paseo-adapter/src/capabilities.test.ts
```

Result before implementation: exit `1`; `ERR_MODULE_NOT_FOUND` for `services/paseo-adapter/src/capabilities.ts`. This was a behavior RED caused by the missing W17 module, not by a missing daemon or provider.

## GREEN and static contract evidence

Commands:

```text
pnpm exec tsx --test services/paseo-adapter/src/capabilities.test.ts
```

Result: exit `0`, 3 tests passed. The tests cover rejection of each missing core capability, stable `PASEO_CORE_UNAVAILABLE`, and preservation of supported/unavailable/forbidden states.

```text
pnpm --filter @weknora/paseo-adapter typecheck
```

Result: exit `0`. This checks the adapter against the pinned TypeScript dependency graph and does not imply a live daemon or provider is available.

## Live probe status

The controlled daemon probe was not run to a false PASS. No `PASEO_URL` or provider credential was supplied in the execution environment, so the live result is `blocked-env`:

Command:

```text
PASEO_URL=ws://127.0.0.1:6767/ws pnpm exec tsx -e "import { createPaseoClient } from '@getpaseo/client'; void (async () => { const client=createPaseoClient({url:process.env.PASEO_URL!,connectTimeoutMs:1000,reconnect:{enabled:false}}); try { await client.connect(); console.log('LIVE_CONNECTED'); await client.close(); } catch (error) { console.error('LIVE_PROBE_BLOCKED_ENV', error instanceof Error ? error.message : String(error)); process.exitCode=2; try { await client.close(); } catch {} } })();"
```

Result: exit `2`, `LIVE_PROBE_BLOCKED_ENV Transport closed (code 1006)`. The daemon endpoint was not accepting a controlled connection; no provider operation was attempted.

- create → observe → events → finish: not run;
- cancel: not run, and the public SDK surface is unavailable for cancel;
- dropped-receipt request lookup/replay: not run;
- provider authentication and permission/approval behavior: not run.

The next step is to provision one isolated Paseo v0.7.2 daemon and exactly one provider credential, then collect the controlled probe evidence before W20 enables dispatch. W20 must keep `lookupByRequest=false` and stop in `unknown` when a receipt is lost.

## Scope review

Changed files are limited to the W17 adapter package, fixed workspace lock entries, source compatibility manifest, and this evidence report. No W18 target/repository/route files were changed.
