# RunView Protocol Redirect Fix Report

Status: **bound-client redirect override rejected before target dispatch; tests pass.** No commit was created. Integration worktree HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

## Scope and change

This task edited only `internal/modules/agentruntime/agent/opencode/client.go` and `client_test.go`.

The reviewed starting checkpoint was the immutable-directory client at `client.go` SHA-256 `828eb192fab0612fb0eb904dec45acc3c6cd9108da9b65a378b1781f5aa7b05f` and `client_test.go` SHA-256 `babef85a94b21167950200b309fc61f02e0d1fa85a4dad0d5a9be04fd29df61c`, as recorded by the independent protocol review. This task changes the redirect policy after the existing/custom `CheckRedirect` callback: for a bound client it parses the final redirect query and rejects invalid query encoding or any decoded `directory` key. `url.ParseQuery` catches percent-encoded key names and duplicate keys. It does not rewrite or clear the URL. Unbound legacy clients keep their existing behavior.

## RED → GREEN evidence

Before the implementation, ran:

```text
gofmt -w internal/modules/agentruntime/agent/opencode/client_test.go
go test ./internal/modules/agentruntime/agent/opencode -run 'TestWithDirectoryRejectsRedirectQueryDirectoryOverride|TestUnboundClientRetainsLegacyDirectoryQueryRedirectBehavior' -count=1
```

RED: the four bound redirect subtests failed because the redirect was followed. The unbound compatibility test passed.

Final verification ran:

```text
gofmt -w internal/modules/agentruntime/agent/opencode/client.go internal/modules/agentruntime/agent/opencode/client_test.go
go test ./internal/modules/agentruntime/agent/opencode -run 'TestWithDirectoryRedirects|TestWithDirectoryRejectsRedirectQueryDirectoryOverride|TestUnboundClientRetainsLegacyDirectoryQueryRedirectBehavior' -count=1
go test ./internal/modules/agentruntime/agent/opencode -count=1
git diff --check
```

GREEN: focused redirect tests and the full OpenCode package passed; `git diff --check` passed. The fixture proves that plain, same-value, percent-encoded, duplicate and custom-callback-added `directory` query keys cannot reach the target on a bound client. An ordinary same-origin redirect without the key remains covered by the prior test; an unbound client with a `directory` query redirect remains compatible.

## Limits

These are `httptest` client-policy tests; no live OpenCode server was run for this fix. The directory header and redirect protection remain routing context, not filesystem isolation. The pinned global `/event` stream is still unscoped; RunView/container isolation and T01/T05 acceptance remain pending.

## Checkpoint

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged).
- No commit or staging. The integration worktree contains other concurrent edits; they were preserved.
- Final full-content SHA-256:

```text
809d9dab9d3e58f6ccf3cf2d2dce0f8cff9d3d849fd555e02a174b005284c0bd  internal/modules/agentruntime/agent/opencode/client.go
67678202fa5caf2d7e89919d6302a932e22bf333bda894ac185b26a172551934  internal/modules/agentruntime/agent/opencode/client_test.go
2c8099fe748ef9a5ab585c0c956a151bff7c2a50caff9bd0aba115c907a1fe79  git diff --binary -- internal/modules/agentruntime/agent/opencode/client.go internal/modules/agentruntime/agent/opencode/client_test.go
```

The final file hashes and this report hash are returned with the task handoff. The independent reviewer should inspect the two owned files at this full-content checkpoint; the incoming two-file diff was already reviewed separately.
