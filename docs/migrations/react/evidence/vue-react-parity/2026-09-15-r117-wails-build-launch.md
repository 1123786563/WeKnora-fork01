# 2026-09-15 Wails build and launch evidence

- Command: `PATH="/Users/wuyongjun/go/bin:$PATH" wails build -clean -tags sqlite_fts5 -o 'WeKnora Lite'` from `cmd/desktop`.
- Result: Wails v2.12.0 produced and self-signed `cmd/desktop/build/bin/WeKnora Lite.app` in 28.2s.
- Launch: after provisioning the Lite app's `Resources/config`, `Resources/migrations/sqlite`, and `Resources/.env` from repository sources, `open` started the packaged app and the process remained alive after 8 seconds.
- Boundary: build/packaging/process evidence is complete for this run; deep Wails UI interaction and per-feature backend acceptance remain open.
