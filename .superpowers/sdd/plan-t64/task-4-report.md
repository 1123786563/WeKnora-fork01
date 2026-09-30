# T64 Task 4 Report — Agent security adjudication surface

## Scope delivered

- Added `internal/types/interfaces/agent_security.go` with the security verdict, revocation input/view/scope contracts and service interface from `plan-t64.md` Task 4.
- Added `internal/application/service/agent_security.go` with the service constructor, sentinel errors, `ReleaseSecurityGate`, operation placeholders for Task 5, and verdict/admission adjudication.
- Added `internal/application/service/agent_security_test.go` using the Task 4 real-service fixture and release/adoption seeders.

The adjudicator returns `ok` for an empty agent ID, an agent with no published adoption-derived variant, or an unaffected release. It prioritizes the latest matching release revocation, then checks the immutable lock against dependency revocations using the exact case-sensitive `(type, id, version, digest)` tuple. It handles tenant-introduced release locks through `ReleaseFacts`; malformed lock JSON returns an error. Admission wraps blocked verdicts with `ErrAgentSecurityReleaseBlocked`.

## TDD and verification evidence

- RED: `go test ./internal/application/service/ -run 'TestAgentSecurityVerdict' -count=1` failed because `AgentSecurityService`, constructor, and verdict API were undefined (plus plan-snippet issues corrected in the owned test file).
- GREEN: `go test ./internal/application/service/ -run 'TestAgentSecurityVerdict' -count=1` — PASS.
- Relevant service regression filter: `go test ./internal/application/service/ -run 'TestAgent(Security|Upgrade|Adoption)' -count=1` — PASS.
- Build: `go build ./...` — PASS; linker emitted existing duplicate `-lc++` library warnings for `cmd/server` and `cmd/desktop`.
- Whitespace: `git diff --check` — PASS.

The introduced-release test fixture includes parent public listing, submission, and public release rows because the real migration stream enforces those foreign keys.

## Assumptions / review notes

- `ReleaseAdmission` returns `ErrAgentSecurityReleaseUnresolvable` when a release is neither revoked nor resolvable in the tenant. This is fail-closed for nonexistent/foreign IDs and aligns with the declared sentinel; Task 4's terse admission algorithm did not separately specify the unknown-ID result.
- Revocation operation methods intentionally return `ErrAgentSecurityInvalidInput` placeholders; Task 5 owns their implementation.
- No authorization behavior is introduced here; caller-specific auth remains at the handler boundary.

## Handoff

Implementation is ready for the independent task review and validator. Commit: pending at report authoring time; recorded in the task response after commit.
