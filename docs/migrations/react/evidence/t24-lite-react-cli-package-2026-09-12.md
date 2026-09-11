# T24 React Lite CLI package — 2026-09-12

## Scope

This is a local macOS arm64 `package-lite.sh` run using an isolated React
candidate. It verifies the CLI package assembly path and does not represent a
production release or a cross-platform build.

## Command and result

```text
REACT_FRONTEND=1 REACT_BUILD_VERSION=verify VITE_FRONTEND_COMMIT=f231d1c \
  ./scripts/package-lite.sh verify-lite
```

The command exited 0. It built both React Web and Embed entries, built the
Lite binary, and produced:

```text
dist/WeKnora-lite_verify-lite_darwin_arm64.tar.gz       81M
dist/WeKnora-lite_verify-lite_darwin_arm64.tar.gz.sha256
```

The tarball contains `web/index.html`, `web/embed.html`,
`web/BUILD_INFO.json`, `web/assets/`, and `web/embed/assets/`. The extracted
`BUILD_INFO.json` reports `renderer: react`, `version: verify`, commit
`f231d1c`, and Web/Embed entry names. Running
`cd dist && shasum -a 256 -c
WeKnora-lite_verify-lite_darwin_arm64.tar.gz.sha256` returned `OK`.

## Boundary

This closes the previously missing React Web/Embed copy step in the local
Lite CLI packaging path. It does not prove Windows/Linux packaging, deployed
registry behavior, provider-backed chat, or T24 acceptance.
