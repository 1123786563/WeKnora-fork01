# Wails macOS 打包验证（React bundle，2026-09-12）

- 命令：`PATH="$HOME/go/bin:$PATH" REACT_FRONTEND=1 SKIP_FRONTEND=1 ./scripts/package-mac-app.sh`（React web+embed bundle 构建并同步至 web/ 后，wails build + 打包 + 自签名）。
- 修复两个集成缺陷：desktop vite config 缺新 domain 子路径 alias（message-extras/copy-answer/session-grouping/message-timestamps/auth/password-policy——ChatRoutePage/SettingsPage 的新导入在 desktop 构建中无法解析）。
- 产物：dist/WeKnora Lite.app（codesign --verify --deep --strict 通过；二进制与 Resources 完整）。
- 同时落地后端 CORS 修复：AllowHeaders 增补 X-Embed-Visitor（embed 组件访客头），解除 embed ERR_FAILED 阻塞。
- 前置条件：wails CLI 于 ~/go/bin（打包脚本 PATH 需含）。
- 说明：本轮为构建/签名/资源冒烟验证；运行时交互验收（安装后点击、聊天流、上传）需人工 GUI 操作或后续自动化补齐。

## 复跑确认（Round 53，chat UX 收尾并入后）

- 干净树上重跑 REACT_FRONTEND=1 打包：web+embed bundle → wails build（24.9s）→ 打包 + 自签名全部通过。
- 产物：dist/WeKnora Lite.app（含 chat UX 收尾内容：时间戳分隔/复制按钮/打字指示器/回到底部/分组持久化）。
- 前次失败根因（chat WIP 的 copy-answer 模块缺失）已随 62a77a9 集成消除。
