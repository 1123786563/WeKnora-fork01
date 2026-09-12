# T24 React Wails macOS rerun — 2026-09-12

- Command: `PATH="/Users/wuyongjun/go/bin:$PATH" REACT_FRONTEND=1 ./scripts/package-mac-app.sh`
- Result: exit 0; Wails `v2.12.0` built the macOS arm64 `WeKnora Lite.app`.
- The assembled bundle was re-signed and `codesign --verify --deep --strict`
  exited 0.
- `Contents/Resources/web/index.html`, `embed.html`, and `BUILD_INFO.json`
  are present and non-empty.
- `BUILD_INFO.json` identifies renderer `react` and commit `ec7eaae`.

This is a macOS arm64 installed-artifact build/resource/signature smoke only;
Windows/Linux Wails artifacts, runtime behavior, rollback rehearsal, and the
full release matrix remain unverified. T24 remains `review`.
