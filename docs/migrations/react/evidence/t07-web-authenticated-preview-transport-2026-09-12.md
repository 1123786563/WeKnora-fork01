# T07 Web authenticated binary transport wiring (2026-09-12)

- Found during review of `369ddb8`: `apps/web/src/platform/http.ts` decorated
  JSON and SSE requests but did not expose the shared client's optional
  `sendBinary` method. A document preview or download therefore failed at the
  real Web runtime boundary even though the raw JSON transport was covered.
- Added `sendBinaryWithRefresh` with the same origin-scoped auth, tenant,
  locale, request-id, and bearer 401 refresh/retry behavior as JSON requests.
  Embed credentials remain isolated and the response body stays binary.
- TDD evidence: the new Web test first failed with
  `TypeError: transport.sendBinary is not a function`, then passed after the
  wiring change. Fresh verification passed: focused transport tests 9/9,
  `pnpm test:web` 97/97, `pnpm typecheck:web`, `pnpm typecheck:shared`,
  `pnpm build:web`, boundary checks, and `git diff --check` all exited 0.
- This proves the Web transport seam and local build only. A live protected
  preview/download response, browser byte/filename assertion, and complete
  format-renderer matrix remain unverified; T07 stays `review`.
