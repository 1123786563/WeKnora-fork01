# Task 1 Report — Settle A03 Action Before Delivery

- BASE: `8940df1612d7272e641c8156da128827c5a8c58a`
- Implementation: added `ReadOnlyUnknownResolver`; ActionService queries provider facts without Delivery mutations, then persists A03 terminal state. CodeDelivery reconciles Delivery only after Action settlement succeeds. If the Delivery reconciliation fails, a later resolve sees ActionSucceeded and retries only read/reconciliation work. Dispatch remains blocked while Delivery is ambiguous.
- Provider errors: BranchHead errors now return directly rather than being discarded as absence.
- Regression evidence:
  - Injected Action settlement failure leaves Action unknown and Delivery unknown; dispatch stays blocked with no POST; retry succeeds and resolve issues zero POST.
  - Injected Delivery settlement failure leaves Action succeeded and Delivery unknown; dispatch remains blocked; later resolve settles Delivery with zero POST.
  - Branch lookup failure propagates `GitHubAPIError` status 503, leaves both records unchanged, and causes zero PR create calls.
- Verification:
  - `go test ./internal/modules/codedelivery/ -run 'TestUnknownResolutionSettlementFailureLeavesDeliveryAmbiguousAndRetriesReadOnly|TestUnknownResolutionPropagatesBranchLookupErrorWithoutWrites|TestSucceededActionReconcilesAmbiguousDeliveryOnRetry' -count=1` — PASS
  - `go test ./internal/modules/appconnector/service/appconnector/ -count=1` — PASS
  - `go test ./internal/modules/codedelivery/ -count=1` — PASS
  - `go test ./internal/application/repository/ -run TestT25UnknownResolvesFromRemoteFactsOverHTTP -count=1` — PASS
  - `git diff --check` — PASS
- Residual risk: owner confirmation cannot prove a delayed provider create will never complete after inspection. Resolve remains no-POST; a later dispatch is separate.
- Commit: `0b294b3ffc9da657c10d4b31476faec565179371`.
- Payload SHA (SHA-256 over sorted committed `internal/` paths, filename-NUL-content): `e521743596c0cfe0e6656e7ecb401e270d410ffc0bc4edb5326afd6b36d25d7c`.
