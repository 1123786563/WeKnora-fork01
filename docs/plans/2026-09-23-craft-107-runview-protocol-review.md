# RunView OpenCode protocol adapter independent review

Date: 2026-09-23. Reviewed the two-file uncommitted checkpoint in the integration worktree at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Checkpoint manifest SHA-256 `56d92f141fc51a65dcaa620e3757040f7b2f217a5a2ae5f37f3e37c17c516935`; the owned `git diff --binary` SHA-256 independently reproduced as `f5a826d1a021088bc9be745aa6f1c989e60cb0d438321e19d7d280ed45c4107d`. Full file hashes match the manifest: `client.go` `828eb192fab0612fb0eb904dec45acc3c6cd9108da9b65a378b1781f5aa7b05f`; `client_test.go` `babef85a94b21167950200b309fc61f02e0d1fa85a4dad0d5a9be04fd29df61c`. The worktree has other concurrent edits, which this review did not modify. `git diff --check` and independently run `go test ./internal/modules/agentruntime/agent/opencode -count=1` passed.

Sources: approved Craft web-artifact Spec, ADR-0004 and `CONTEXT.md`, assigned RunView protocol plan/report, T01 per-Run isolation design, and the pinned OpenCode 1.18.4 lock (`source_commit` `49c69c5ed3ccf706b61b3febb43c8aaff7f8325e`). This is a scoped protocol review, not T01 or T05 acceptance and not an OCR run.

## Verdict

- **Scoped Spec compliance: FAIL pending redirect correction.** The immutable clone and common header seam satisfy the ordinary-request portion of the plan, but a same-origin redirect can change the pinned server's effective directory while preserving the checked header.
- **Code quality: FAIL pending the same Medium finding.** Validation, clone behavior, legacy default behavior and focused package tests are otherwise sound. The report correctly avoids claiming that a header supplies filesystem or event-stream isolation.
- **T01/T05 acceptance: not evaluated.** Durable RunView binding, per-Run OS/filesystem isolation, session directory validation, and event routing are outside this two-file checkpoint.

## Finding

### Medium — same-origin redirect can override the bound directory through query precedence

**Evidence / affected symbol:** `directoryRedirectPolicy` in `internal/modules/agentruntime/agent/opencode/client.go:89-110` verifies only the redirect URL origin and retained `x-opencode-directory` header. The pinned OpenCode 1.18.4 `workspace-routing.ts` selects `?directory=` before that header, as recorded in the T01 isolation design's pinned protocol audit. Thus a redirect from `/session` to `/session?directory=/other/run` passes the policy when the header remains `/intended/run`, yet the server routes the redirected request to `/other/run`. The tests in `client_test.go:364-467` cover same-origin retention, stripped header and cross-origin redirects, but not query precedence.

**Impact:** A bound client's follow-up request can act on a different OpenCode instance directory, violating the plan's immutable RunView binding. This is a concrete protocol mismatch even though current fixtures do not show a production endpoint issuing such a redirect.

**Smallest defensible correction:** Reject a redirect whose URL contains a `directory` query parameter (including encoded forms parsed by `URL.Query()`), or prove it equals the bound canonical directory with unambiguous parsing. Add a same-origin fixture redirect containing `?directory=/other/run` while retaining the header; assert the target handler is not called. Check the query after any custom `CheckRedirect` callback, since that callback may mutate `req.URL`.

## Confirmed behavior and limits

- `WithDirectory` returns a copy with its own `http.Client` value and rejects rebinding (`client.go:53-68`). The original legacy client remains unbound; two clones keep independent headers. Validation rejects empty/root/relative paths, noncanonical `.`/`..`/duplicate separators, backslash, colon, semicolon and control characters (`:71-86`). It validates path shape only, not existence, symlink resolution or caller authority.
- All nine exposed request methods route through `newRequest` (`:118-296`), including create-session, prompt, messages, status, abort, question replies, permission reply and `/event`; the bound header is set centrally (`:288-296`). The unbound client's request shape is unchanged. Same-origin redirect header retention and cross-origin rejection are tested.
- The pinned `/event` stream is global. Setting `x-opencode-directory` on that request does not filter events by session or Run; `Executor.Execute` still consumes that stream. `GET /session/status` likewise returns a global status map and the client selects its own session entry. These are separate routing/isolation gates for the central adapter, not evidence that this two-file change achieves per-Run isolation.
- The protocol report states no actual pinned-server test was run for this checkpoint. The local package test uses an `httptest` server, so the upstream header precedence and global stream behavior rely on the pinned source audit until a live acceptance test is added.
