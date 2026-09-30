# T19 gateway two-phase correction — independent scoped review

Date: 2026-09-24. Read-only source and test review against the approved #107/T19 issue snapshot, `CONTEXT.md`, ADR-0004, the two-phase fix plan, prior body-fix FAIL review, high-fix design, and exact checkpoint. No OCR or source/test edits.

## Checkpoint and verdict

The live SHA-256 values match the checkpoint manifest for `craft_budget.go` (`194c96a2ab29ae88ec521c4b64db6e56fcb2ace4a6cc9d4eec385c7fcf43fbb4`), `craft_budget_start_test.go` (`e0aca373ac337ef003d53f6817c5726dfc4e37f98a02075066465dee807a7f2d`), `craft_model_gateway.go` (`c7ce32d115d533f2d59df572c9460fe7318e0df5af47ca014db21f9a42617da8`), and `craft_model_gateway_test.go` (`251a7609e027e9e2bedfca8b03baaa7ca89ed176854cb88adfdca74f3b61231b`). The combined patch hash is `0feaa826fc8b6c193c160d97a5072923d0fa329930b5441afdb132c9a1a51784`; it reverses cleanly. The patch includes the documented pre-task dirty baseline, so this verdict concerns the four-file checkpoint, not attribution of every hunk to this correction.

- **Prior High body-state finding: CLOSED in this scope.** The gateway does not resolve `started` until `ReadAll` and `Close` both finish. Missing, empty, oversized, read-failed and close-failed bodies resolve `unknown`; failed CAS leaves `intent`. The repository Run fence observes `intent|unknown`.
- **Prior Medium initiation deadline finding: CLOSED in this scope.** A real HTTP server stalled before headers is canceled at the short initiation deadline. A headers-first delayed body survives that deadline and completes under the longer response context. The mutex-gated `AfterFunc` and `stopDeadline` prevent a late timer callback from canceling a body after the completion state is set; at the exact deadline boundary a conservative unknown outcome is possible, which does not permit replay.
- **Scoped Spec compliance: FAIL on the remaining usage-fact path below.** The durable start and Run-fence requirements pass the reviewed paths, but the fix plan also requires an unknown usage record for ambiguous outcomes, including inbound cancellation.
- **Scoped code quality: FAIL on the same Medium finding.** Other reviewed close, timeout, CAS and retry paths have no additional blocking finding. Full T19 remains unverified; production activity-ID provenance is a separate open High finding and is not credited here.

## Finding

### Medium — inbound cancellation drops the required unknown usage fact

**Evidence / affected symbols:** `CraftModelGateway.Forward` handles a failed body read at `internal/handler/craft_model_gateway.go:548-560` by calling `g.recordCall(..., nil)`. `recordCall` at lines 654-665 passes `c.Request.Context()` to `RecordPhysicalCall`. When the inbound request was canceled, that context is already canceled. The production `CraftUsageService.RecordPhysicalCall` passes it to `CraftUsageStore.Append`, whose SQL transaction uses `s.db.WithContext(ctx)` (`internal/application/service/craft_usage.go:140-147`, `internal/application/repository/craft_usage.go:134-160`). The new inbound-cancellation test at `craft_model_gateway_test.go:535-578` checks only journal `unknown`; its fake recorder at lines 225-230 ignores the context. The gateway writes a response header on recording failure, but a canceled caller cannot be relied on to receive it.

**Impact:** A provider may have accepted a chargeable request before the client disconnects, while the durable journal and G4 hold say `unknown` but the physical usage ledger has no corresponding unknown attempt fact. This breaks the plan's accounting/reconciliation evidence on a normal cancellation path.

**Smallest defensible correction:** Make the post-send usage append use a separately bounded persistence context that retains trace values but is independent of inbound cancellation, and surface or persist a recording failure for reconciliation. Add a service-backed cancellation test that asserts the unknown fact is durably stored after the request context is canceled. Keep the journal at `unknown` if recording fails; never resend the provider request.

## Positive evidence and verification

`BeginBinding` commits journal `intent` and dispatched G4 reservation before external transport. `Resolve` updates only `state='intent'`, so a failed write retains the hold and a later call cannot promote an `unknown` row to `started`. The handler rejects the same activity key before a second `Do`. Non-nil response bodies are closed on transport error, initiation timeout and normal body handling. The real server tests cover blocked headers, delayed body and inbound cancellation; service tests cover an injected resolver write failure and Run cancellation precedence.

I independently ran `go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1 -timeout=45s` (PASS), `go test ./internal/application/service -run '^TestCraftChargeStart' -count=1 -timeout=45s` (PASS), `git diff --check` on the four owned files (PASS), and `git apply --check --reverse` on the checkpoint patch (PASS). No PostgreSQL or pinned OpenCode live route was tested in this scoped review.
