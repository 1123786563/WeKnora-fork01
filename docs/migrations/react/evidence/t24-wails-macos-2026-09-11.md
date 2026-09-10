# T24 macOS Wails package evidence — 2026-09-11

## Build and package

Executed from the isolated `codex/react-multiclient` worktree:

```text
PATH="/Users/wuyongjun/go/bin:$PATH" REACT_FRONTEND=1 ./scripts/package-mac-app.sh
```

The run completed with exit code 0. It used Wails CLI `v2.12.0`, matching
`go.mod`, and produced `dist/WeKnora Lite.app` for `darwin/arm64`.

The React packaging path now calls `scripts/build_react_web_bundle.sh`, so the
app Resources contain both entries and their build identity:

- `Contents/Resources/web/index.html`
- `Contents/Resources/web/embed.html`
- `Contents/Resources/web/BUILD_INFO.json`
- `Contents/Resources/web/assets/`
- `Contents/Resources/web/embed/assets/`

The bundle identity assertion
`renderer == "react"`, `entries.web == "index.html"`, and
`entries.embed == "embed.html"` passed.

## Bundle integrity and runtime

```text
codesign --verify --deep --strict --verbose=2 "dist/WeKnora Lite.app"  # 0
file "dist/WeKnora Lite.app/Contents/MacOS/WeKnora Lite"             # Mach-O arm64
```

The script re-signs after copying runtime resources. This is required because
copying `config/`, SQLite migrations, and `web/` after Wails' initial signing
otherwise invalidates the bundle seal.

The final app was launched with `DB_DRIVER=sqlite`, `RETRIEVE_DRIVER=sqlite`,
`STORAGE_TYPE=local`, `STREAM_MANAGER_TYPE=memory`, and an isolated temporary
database/files directory. The Wails process initialized the backend, bound a
loopback ephemeral port, and served the React API request boundary; an
unauthenticated knowledge-base request returned 401. A preceding candidate
run with the same Wails renderer and the packaged React resources exposed the
native `WeKnora Lite` window whose accessibility tree contained the rendered
React `Knowledge bases` screen, form controls, and the live API 401 state.

## Scope limits

This is macOS arm64 package evidence only. It does not prove Windows/Linux
packaging, multi-arch Docker, device-native mobile behavior, the full role and
tenant matrix, performance thresholds, or rollback rehearsal.
