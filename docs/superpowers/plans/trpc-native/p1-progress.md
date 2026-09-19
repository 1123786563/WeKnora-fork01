# P1 parallel execution ledger

Plan: ../2026-09-19-trpc-native-agent-p1-storage.md

Baseline: `c4882bca3972f130b72e4fc22d2d4e15483ef7da`. Integration branch: `codex/trpc-native-p1-integration`.

## Rulings

- Ruling: execute isolated prerequisite Tracks before product storage implementation. P0 has not selected a durable backend/transaction boundary; raw native SQL cannot be assumed to satisfy stable-ID/hash and memory generation. Cost if wrong: prerequisite probes require rework; product wiring remains closed.
- Ruling: independent nested test modules own their dependency pins, avoiding concurrent root go.mod edits. Cost: full validation must explicitly run all three modules in addition to root tests.
- Ruling: start from reviewed repair code plus final repair commit, with P0 NO-GO retained; do not merge into advancing main automatically. Cost: integration onto main is a separate future operation.

## Dispatch register

| Task | Role | Requested model | Effort | Agent | Actual model |
| --- | --- | --- | --- | --- | --- |
| Read-only Session API analysis | sdd_implementer | gpt-5.6-terra | medium | /root/p1_session_analysis | 未验证 |
| Read-only Memory API analysis | sdd_implementer | gpt-5.6-terra | medium | /root/p1_memory_analysis | 未验证 |
| A Session SQL implementation | sdd_implementer | gpt-5.6-terra | medium | /root/p1_session_impl | 未验证 |
| B Memory SQL implementation | sdd_implementer | gpt-5.6-terra | medium | /root/p1_memory_impl | 未验证 |
| C Identity implementation | sdd_implementer | gpt-5.6-terra | medium | /root/p1_identity_impl | 未验证 |

Analysis correction: Memory SQL backends exist as separate modules `memory/sqlite@v1.11.0` and `memory/postgres@v1.11.0`, confirmed from their downloaded go.mod/source. The analysis that inferred nonexistence from root-module directories is rejected. Exact backend compilation/runtime evidence belongs to Track B.

## Track status

| Track | Implementation | Tests | Review | Integration |
| --- | --- | --- | --- | --- |
| A Session SQL probes | pending | pending | pending | pending |
| B Memory SQL probes | pending | pending | pending | pending |
| C Identity/cursor probes | pending | pending | pending | pending |
| Wave 2 product storage | blocked on reviewed backend/transaction boundary | pending | pending | pending |

Root baseline command: `GOWORK=off go test ./... -count=1 -timeout=180s`; output `/tmp/trpc-p1-baseline.log`; exit 1. Failure packages: repository (migration-head expectation), service (budget notification), database (migration-head/rollback fixtures), handler (registration fixtures), workbench (missing `tests/mobile-v2/fixtures/mx-003-crosslang.json`). These failures occurred before Track implementation and do not authorize declaring full tests green. The independent probe modules do not change these packages.

Track worktrees: `/Users/wuyongjun/.codex/worktrees/trpc-native-p1-{session,memory,identity}/WeKnora-fork01`; each corresponding `codex/trpc-native-p1-{session,memory,identity}` branch starts at `474ed75b`. Exactly one implementer per worktree. PostgreSQL probe uses controller-created disposable `weknora-trpc-p1-probe` (postgres:15.2-alpine), each test owns an isolated schema; never use user data.
