# T14 preview security seam transfer — independent review

Date: 2026-09-24. Scope: the four files transferred by `2026-09-24-craft-107-t14-preview-seam-transfer-plan.md` into the integration worktree. Reviewed against the approved Craft web artifact spec, T14 DAG acceptance, `CONTEXT.md`, relevant ADR constraints, transfer plan, Task 1 report/patch, source commit `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`, and current integration assembly. This review did not run OCR, a live browser/network probe, or B3 route wiring.

## Verdict

- **Scoped Spec compliance: PASS for the four-file security-seam transfer.** All four current file SHA-256 values exactly match the source commit and the Task 1 report. The task-local patch SHA-256 is `e8a710a192738deb6328ca7188234d609fb335f9aa04904fca430a563344b9c0`, matching the report. The transferred service calls current `craft.TaskPreview` access at issue, ticket redemption, and each capability lookup/read; requires a no-egress checker; and gates issue/open on browser-navigation protection. The handler rejects file requests whose Host is not the configured preview host.
- **Scoped code quality: PASS, with no actionable finding in this transfer.** The effective Docker checker derives the sandbox from the tenant/session binding, rejects stale/non-Docker/missing policy, requires configured and live `network_mode=none`, and rejects unsupported remote-daemon inspection. The integration `newCraftPreviewService` constructor at `internal/container/container.go:2547` still supplies only origins. Its absent access/network/browser dependencies fail closed. No static production enablement was added.
- **Full T14/B3 acceptance: NOT VERIFIED.** The production constructor has not injected the current T08 `TaskAccessChecker` or bound sandbox network checker. No live browser navigation/no-egress attestation was run here. The isolated-origin behavior was exercised by Gin/HTTP test fixtures, not a deployed browser/proxy.

## Evidence

| Path | Current and source-commit SHA-256 |
| --- | --- |
| `internal/application/service/craft_preview.go` | `1d877c1193718f022c9f620c2d6bab607a4ac6dec4b55797c7ae41334b0ae4d7` |
| `internal/application/service/craft_preview_network_test.go` | `0c974f83900804a1f303dc7f74d7a5b8b9eb82733e8e8a588332c6f64036ea38` |
| `internal/handler/session/craft_preview.go` | `4f4a5f61e9f9275c77cb936a51a3e7efde7a99bc179502725e18bfd7acc77c3b` |
| `internal/handler/session/craft_preview_test.go` | `1b0ce3b338c1fa5c26d73d49bede1f2027ba9a65ad94c594b6c6a5a7b52ec46f` |

- `git diff --check` on the four paths: pass.
- `go test ./internal/application/service -run 'CraftPreview' -count=1`: pass.
- `go test ./internal/handler/session -run 'CraftPreview|CraftT14Journey' -count=1`: pass.
- `go test ./internal/container ./internal/router -run '^$' -count=1`: both compile/pass; container link emitted a duplicate `-lc++` warning.
- `internal/container/container.go`, `internal/router/router.go`, `internal/router/routes_chat.go`, and `internal/router/routes_craft_features_test.go` contain concurrent integration changes relative to HEAD. None is in the four-file transfer patch or source parity set. Their current route/container assembly compiled; this review does not attribute those other changes to T14 or approve their behavior.

## Findings and limits

No critical, high, or medium finding within the assigned transfer scope. B3 must wire and test current T08 access revocation through the real production route/assembly, while keeping preview disabled until browser/network attestation. The existing focused test uses a permissive test-only network checker and sets `BrowserNavigationProtected=true` in its fixture; it does not establish production no-egress. This is a stated remaining verification dependency, not a transfer defect.
