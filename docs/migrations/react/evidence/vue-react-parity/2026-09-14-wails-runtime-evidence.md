# 2026-09-14 Wails 桌面运行时证据（N033 平台验收解锁）

## 构建命令与环境
- wails CLI: /Users/wuyongjun/go/bin/wails v2.12.0（系统已装，未入 PATH——此前误判缺失）
- 构建: cd cmd/desktop && PATH="/Users/wuyongjun/go/bin:$PATH" wails build -clean -tags "sqlite_fts5" -o "WeKnora Lite"
- 耗时 37.7s，产物: cmd/desktop/build/bin/WeKnora Lite.app（自签名完成）
- 前置修复（0f1c6d45）: apps/desktop/vite.config.ts 补 '@weknora/domain/settings/theme' 别名（desktop-renderer 构建断裂根因）

## 运行时验证
- 启动: open "cmd/desktop/build/bin/WeKnora Lite.app" → 进程存活（dist/WeKnora 二进制）
- 窗口渲染: WebKit 窗口正常显示登录页（邮箱/密码表单、创建账户、特性清单、语言切换 zh）——截图 wails-login-window.png
- 进程稳定性: 启动 8s 后进程仍存活

## 证据产物
- screenshots/wails-runtime-20260914/wails-login-window.png（2539x1651 全屏，应用窗口前置）

## 限制
- WebKit 窗口内深层交互（注册/登录/建库）需 macOS 辅助功能授权的 UI 自动化，本轮未执行——登录页渲染 + 进程存活为本轮取证范围
- 应用为 Lite 版（内嵌 sqlite 后端），与浏览器证据 (:5180/:5181 连 8080 后端) 的数据面不同源
- iOS/Android 原生证据仍 blocked-env（Android SDK/emulator 缺失；iOS 模拟器存在但 React Native 原生构建未在本轮范围）

## 追加：内嵌后端完整运行（.env 依赖发现）

首次 wails build（不走打包脚本）缺少两样 Resources 内容，逐层修复验证：
1. config/ 与 migrations/sqlite/ 需拷入 Resources（打包脚本 package-mac-app.sh:84-98 的步骤）
2. **.env** 必须由 .env.lite.example 拷贝为 Resources/.env（含 DB_DRIVER=sqlite、DB_PATH、WEKNORA_SANDBOX_MODE=disabled）——否则 container initDatabase panic "unsupported database driver: "

修复后验证（wails-app3.log）：
- app 进程存活 1，panics: 0
- 内嵌后端完整启动："Server is running at 127.0.0.1:49342 (proxy -> http://127.0.0.1:49342)"
- HTTP 服务应答（未认证请求 401 为预期行为）

**启动配方（复现步骤）**：
```bash
RES="cmd/desktop/build/bin/WeKnora Lite.app/Contents/Resources"
mkdir -p "$RES/config" "$RES/migrations/sqlite"
cp -r config/* "$RES/config/"
cp -r migrations/sqlite/* "$RES/migrations/sqlite/"
cp .env.lite.example "$RES/.env"
open "cmd/desktop/build/bin/WeKnora Lite.app"
```

