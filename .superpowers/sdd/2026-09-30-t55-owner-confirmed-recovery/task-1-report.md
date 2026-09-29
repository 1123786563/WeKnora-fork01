# Task 1 Report — Owner-Confirmed Backend Recovery

- BASE: `19ae81aa7aad83ddab1fe647dc6292309a9bf4d3`
- Parent-authorized ownership expansion (2026-09-30): `internal/modules/appconnector/service/appconnector/action.go` for the minimal attestation-aware unknown resolver seam. The existing `ResolveUnknown` delegates with `false`; only the delivery service uses the explicit confirmation API.
- Implementation: strict optional JSON body (unknown fields/trailing values rejected), owner-only route unchanged, confirmation is propagated through A03 resolver and fenced action settlement. Matching PR still resolves regardless of flag. Empty PR lookup plus branch requires explicit flag for both unknown and dispatched Delivery rows. PR lookup errors now return directly; provider errors never prove absence. Resolve issues no provider POST.
- RED evidence: before updating the legacy implicit-confirmation expectation, `go test ./internal/modules/codedelivery/ -run 'TestUnknownOutcomeResolvesFromRemoteFacts|TestDispatchAndResolveRejectMismatchedPersistedRunAndOwner' -count=1` failed with `code_delivery_owner_confirmation_required` at the prior no-confirmation resolution call.
- Regression coverage: unknown delivery rejects no-confirmation without state mutation; confirmed empty lookup settles Delivery to pushed and A03 Action to succeeded; GitLab same path; HTTP existing PR remains delivered; handler accepts empty/true/false payload and rejects malformed/unknown fields.
- Verification commands and results:
  - `go test ./internal/modules/appconnector/service/appconnector/ -count=1` — PASS
  - `go test ./internal/modules/codedelivery/ -count=1` — PASS
  - `go test ./internal/application/repository/ -run 'TestT25UnknownResolvesFromRemoteFactsOverHTTP' -count=1` — PASS
  - `go test ./internal/handler/session/ -run TestResolveDeliveryRequiresStrictOwnerConfirmationBody -count=1` — PASS
  - `git diff --check` — PASS
- Residual risk: owner confirmation cannot prove a delayed provider request will never complete after the inspection. A late PR may still appear after the owner confirms; the frontend must disclose this explicitly. Subsequent dispatch remains a separate request.
- Commit: `72dc79800b387cb322d44f3c5fe4cd56e2b3d032`.
- Payload SHA (SHA-256 over sorted committed implementation paths, filename-NUL-content): `d1e87432429f97dc9ec46386875cc9dbb55534cab85009fa5129023585193c95`.
