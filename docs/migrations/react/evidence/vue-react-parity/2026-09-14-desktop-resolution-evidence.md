# 2026-09-14 Desktop-resolution (1200x800) dual-end evidence

## Context
The Wails-runtime evidence slice (dispatched for the N033 "Wails
runtime interaction evidence" gap) terminated without delivering. This
recovery capture provides the desktop-typical-resolution evidence its
fallback path described: a 1200x800 viewport (representative desktop
runtime geometry) across 6 core routes, both ends.

## Scope
- Routes: kb-list / agents / settings-general / creatChat /
  integrations / organizations
- Both ends (Vue :5180, React :5181), zh-CN, logged-in tenant
- 12 screenshots: docs/migrations/react/evidence/vue-react-parity/
  screenshots/desktop-1200x800-20260914/

## Command
node .parity-tools/desktop-evidence-sweep.cjs

## Limits
- Browser-rendered at 1200x800; NOT a native Wails (WebKit) runtime
  capture. The true Wails runtime interaction evidence remains
  blocked-env until the wails CLI toolchain is available (see the
  N033 row). Resolution differences vs the 1440x900 batch: sidebar
  collapse behavior and grid wrap points differ; no layout breakage
  observed in spot inspection.

## Recovery for true Wails evidence
1. Install the wails CLI (go install github.com/wailsapp/wails/v2/cmd/wails@latest)
2. Build: wails build -f -platform darwin/arm64 (config: cmd/desktop/wails.json)
3. Run the built binary, capture the 6 routes in the WebKit window.
