# W17 execution report

- Fix base: `153299f51a76b44e706345e1e5ab96f0ca3d474f`
- Original dispatch parent/base: `b815f158`
- Upstream: `getpaseo/paseo` `v0.7.2` / `c4e97d5cfbef62934e0915d58a72ddeae5c4caea`
- SDK: `@getpaseo/client@0.7.2`
- Live provider/daemon: `blocked-env`

## Checks

```text
pnpm exec tsx --test services/paseo-adapter/src/capabilities.test.ts
```

Result: exit `0`; 5 tests passed, including rejection of internal `DaemonClient.cancelAgent` and public SDK runtime mapping.

```text
pnpm --filter ./services/paseo-adapter typecheck
```

Result: exit `0`; the adapter imports `createPaseoClient` and `PaseoClientConfig` from the pinned public package, so typecheck covers the SDK dependency.

```text
pnpm install --lockfile-only --frozen-lockfile --filter ./services/paseo-adapter
```

Result: exit `0`; the frozen lockfile contains only the W17 service importer and exact Paseo package integrity/snapshot entries. The command did not rewrite unrelated workspace peer snapshots.

```text
PASEO_URL=ws://127.0.0.1:6767/ws pnpm exec tsx -e "...createPaseoClient..."
```

Result: exit `2`; `LIVE_PROBE_BLOCKED_ENV Transport closed (code 1006)`. No controlled daemon/provider credential was available, so no create, event, finish, cancel, replay, or permission capability is claimed as live PASS.

## Fix evidence

The public method allowlist admits only `PaseoApi.agents.create`, `PaseoAgentHandle.waitForFinish`, and `PaseoAgentHandle.subscribe`. An observation naming internal `DaemonClient.cancelAgent` is forced to `unavailable`; it cannot satisfy `requireCore`. The compatibility manifest includes MIT license and notice/source fields.
