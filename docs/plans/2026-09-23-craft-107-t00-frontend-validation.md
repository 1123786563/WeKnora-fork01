# T00 / #119 independent frontend validation

Status: **DONE_WITH_CONCERNS**. Validation is read-only and scoped to T00 frontend acceptance. HEAD/revision: `4bcad69baf033a1310b4dce1372c8153e66adc81`. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.

## Checkpoint identity

The assigned checkpoint is `.superpowers/sdd/2026-09-23-craft-107-implementation/t00-checkpoint-01`, based on the same HEAD. I recomputed SHA-256 for all 14 entries in its `manifest.json`: **14 files, 0 mismatches** (exit 0). The web contract and workbench sources/tests therefore match the implementation checkpoint used by the reported verification.

## Commands and results

| Exact command | Exit | Result |
| --- | ---: | --- |
| `pnpm exec tsx --test packages/contracts/src/craft/web-artifact.test.ts` | 0 | 5 passed, 0 failed |
| `pnpm exec tsx --test packages/views/src/craft/workbench.stop.test.tsx` | 0 | 4 passed, 0 failed |

Checkpoint hash verification command (exit 0, 14 files, 0 mismatches):

```sh
python3 - <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path('.')
manifest=json.loads(pathlib.Path('.superpowers/sdd/2026-09-23-craft-107-implementation/t00-checkpoint-01/manifest.json').read_text())
entries=manifest if isinstance(manifest,dict) else manifest.get('files',{})
errors=[]
for path, meta in entries.items():
    expected=meta['sha256'] if isinstance(meta,dict) else meta
    actual=hashlib.sha256((root/path).read_bytes()).hexdigest()
    if actual != expected: errors.append((path,expected,actual))
print(f'files={len(entries)} mismatches={len(errors)}')
for row in errors: print(*row)
sys.exit(bool(errors))
PY
```

The implementer’s same-checkpoint `pnpm test:shared` and `pnpm typecheck:web` evidence is recorded in `docs/plans/2026-09-23-craft-107-t00-report.md` as exit 0 (987 tests: 986 passed, 1 skipped; web typecheck no diagnostics). I reused it because the checkpoint hashes match.

## Acceptance findings

- **Legacy null defaults:** passed. Legacy inputs without `recognition`, versions without `web_evidence`, and runs without `budget_pause` parse to `null`.
- **Invalid enum rejection:** passed. Contract tests reject an unknown version evidence outcome and an unsupported stop status; supported `unknown` values remain explicit.
- **Workbench feature registration and rendering:** passed. Named features sort deterministically and render in header/aside slots. Duplicate names throw. An entry whose `render` is `null` throws before rendering.
- **Existing stop behavior:** passed. Active Run renders the existing stop control and invokes `onStopRun` once; idle and terminal-success states hide it; `stopping` disables the control and invokes no callback.
- **Loading/error/success/empty states:** no new state flow was introduced by T00. The focused test covers idle/empty state and stop interaction; implementation report records the prior Craft shared suite and typecheck passing. I did not find T00-owned loading/error/success behavior to validate independently.
- **Accessibility/responsive/browser:** the feature slot change adds wrapper `<div>` elements and does not add new controls or text. No accessibility regression is apparent from the inspected diff. Rendering evidence is DOM-based (`tsx`/React test harness), not a real browser run; no viewport matrix or screen-reader audit was performed. The aside slot is nested within the existing side panel, whose responsive visibility remains controlled by existing workbench layout.

## Gaps and risks

No frontend acceptance gap was found in the requested focused checks. Real-browser responsive behavior and assistive-technology output were not exercised; those are residual validation limits rather than explicit T00 acceptance failures. Backend/domain acceptance and Go verification were not independently rerun in this frontend assignment; refer to the same-checkpoint implementation report for their evidence.

## Fix round 1 — focused frontend revalidation

Checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t00-checkpoint-02`; HEAD remains `4bcad69baf033a1310b4dce1372c8153e66adc81`. SHA-256 verification against its `manifest.json`: **15 files, 0 mismatches**, exit 0. The exact verification command was:

```sh
python3 - <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path('.')
manifest=json.loads(pathlib.Path('.superpowers/sdd/2026-09-23-craft-107-implementation/t00-checkpoint-02/manifest.json').read_text())
entries=manifest if isinstance(manifest,dict) else manifest.get('files',{})
errors=[]
for path, meta in entries.items():
    expected=meta['sha256'] if isinstance(meta,dict) else meta
    actual=hashlib.sha256((root/path).read_bytes()).hexdigest()
    if actual != expected: errors.append((path,expected,actual))
print(f'files={len(entries)} mismatches={len(errors)}')
for row in errors: print(*row)
sys.exit(bool(errors))
PY
```

| Exact command | Exit | Result |
| --- | ---: | --- |
| `pnpm exec tsx --test --test-name-pattern='input acceptance|export consent view' packages/contracts/src/craft/web-artifact.test.ts` | 0 | 2 passed, 0 failed; other 4 tests excluded by the name filter |

The selected tests verify rejection of malformed `recognition.reason`, legacy omitted recognition still parsing as `null`, export origin preservation through JSON, invalid origin kind/ref rejection, and export consent decision digest binding. The implementer’s fix-round `pnpm test:shared` (988 total, 987 passed, 1 skipped) and `pnpm typecheck:web` (no diagnostics) evidence applies to this hash-matching checkpoint; I reused it.

Remaining gap: no real-browser viewport or assistive-technology check was run. No additional frontend acceptance gap was found in this focused fix-round validation.
