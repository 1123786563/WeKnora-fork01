# T24 桌面发布 React 资源闭环 — 2026-09-12

## Scope

本次 follow-up 只验证桌面发布工作流会取得同一份 React Web/Embed 候选
产物、在解包前校验 SHA-256，并把 Go/Wails 运行时所需的 `web/` 目录放入
各平台发布布局。它不等同于 GitHub-hosted runner 执行、Windows/Linux
Wails 安装包运行、已部署 registry 或完整桌面升级验收。

## Implementation

- `.github/workflows/release-lite.yml` 的 `build-desktop-app` 依赖
  `build-react-artifact`，下载 `react-web-dist`，用 `shasum -a 256 -c`
  （无该命令时回退 `sha256sum -c`）校验候选 tarball，然后验证并复制
  `web/index.html`、`web/embed.html` 和 `web/BUILD_INFO.json`。
- macOS `.app/Contents/Resources/web`、Linux 桌面 tarball 的 `web/`，以及
  Windows NSIS 安装器的 `$INSTDIR\web` 均明确包含该目录。
- `cmd/desktop/build/windows/installer/project.nsi` 使用现有许可证资源同级
  的仓库根相对路径 `web\*`，不依赖开发机生成的 `apps/desktop/dist`。

## Verification

| Command/assertion | Result |
|---|---|
| `bash -n scripts/test_release_lite_react_desktop.sh` | exit `0` |
| `bash scripts/test_release_lite_react_desktop.sh` | exit `0`; workflow and NSIS resource assertions passed |
| `bash scripts/test_validate_release_lite_artifacts.sh` | exit `0`; valid and negative fixtures passed |
| PyYAML parse of `.github/workflows/release-lite.yml` | exit `0` |
| `git diff --check` | exit `0` |
| local candidate archive + `shasum -a 256 -c` + extraction | exit `0`; Web/Embed/BUILD_INFO entries present |

## Boundary

This is static workflow plus local artifact assembly evidence. No release was
created, no registry or Homebrew publication was attempted, and no production
data was used. T24 remains `review` until the remaining cross-OS, installed
upgrade, deployed proxy, performance, and role/tenant gates are evidenced;
T25 remains gated and Vue has not been removed.
