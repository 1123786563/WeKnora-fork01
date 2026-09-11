# T24 Go 回归结果 — 2026-09-11

Command:

```text
GOWORK=off go test ./...
exit 1
```

The backend packages relevant to the migration passed, including
`internal/router`, `internal/handler`, `internal/handler/dto`, session
handlers, middleware, runtime, and the other packages reported green by the
run. The complete command did not finish green for three environment/test-fixture
boundaries:

- `internal/application/service`: `TestSkillPythonVerifier` expected the
  fixture packages `pandas` and `totally_absent_package` to be unavailable,
  but the current host environment made the verifier return nil for two cases.
- `internal/datasource/connector/notion`:
  `api.notion.com` resolved to `198.18.0.112`, which the repository SSRF guard
  correctly rejected as the restricted `198.18.0.0/15` range.
- `internal/models/chat` and `internal/models/embedding`: Azure/OpenAI fixture
  hosts resolved to restricted `198.18.0.x` addresses and were rejected by the
  same SSRF guard before request-shaping assertions ran.

These failures are recorded rather than bypassed. They are outside the React
worktree changes and do not replace the focused `go test ./internal/router
-count=1` result, which passed. A clean backend release gate still requires a
controlled test Python environment and deterministic non-routable DNS fixtures
that do not collide with the SSRF policy.
