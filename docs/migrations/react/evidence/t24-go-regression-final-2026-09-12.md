# T24 full Go regression rerun — 2026-09-12

The complete backend regression was rerun from the React migration worktree
with the workspace override disabled so the repository's own module graph was
tested:

```text
GOWORK=off go test ./...   exit 0
```

All packages completed successfully, including the API router, auth/session,
database, document processing, chat pipeline, retrieval adapters, sandbox,
storage, and model/provider packages. The macOS linker emitted only the
existing non-fatal `ignoring duplicate libraries: '-lc++'` warnings for two
test binaries; no test failed or was skipped.

This closes the previously environment-sensitive full-Go-regression rerun for
the current source tree. It does not close T24's remaining deployed registry,
cross-OS installed-app, provider, performance, or full browser/native matrix
gates, and it does not authorize T25 Vue removal.
