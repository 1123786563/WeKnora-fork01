# T08 Architectureguard Route Baseline Report

- Base: `fbad0772c09eed3d66e3e4ce492217de80515962`
- Scope: `tools/architectureguard/discovery_test.go` and `docs/architecture/moves/README.md`
- Integrated route registrations checked in `internal/router/routes_career.go`: POST opportunity import, GET opportunity receipt, GET opportunity evidence.
- Before the update, discovery reported `literal=578 apiKeyRoute=69 handle=0 total=647`, `modules=17`, with 23 Redis/Lite workers and 58 hooks. The runtime guard reported zero ownership violations. The only focused failures were stale expected literal/total counts (575/644).
- Updated current expectations to 578/647 and dated documentation to account for the three T08 routes. Historical 633 and prior 644 counts remain recorded as previous checkpoints.

## Verification

- `go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals' -v` — PASS after update.
- `go test -count=1 ./tools/architectureguard/...` — PASS.
- `go test -count=1 ./...` — PASS across all Go packages. The linker emitted duplicate `-lc++` warnings; test packages completed successfully.
- `go run ./tools/architectureguard --root .` — `literal=578 apiKeyRoute=69 handle=0 total=647 | redis=23 lite=23 | hooks=58 | modules=17`, zero violations.
- `git diff --check` — PASS.

No Career code, router registrations, migration, or manifest ownership files were changed.
