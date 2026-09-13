# 2026-09-13 Wails macOS 桌面运行验证（S13）

## 打包（含本会话全部 11 个切片后的提交状态）
- 命令：CI=true PATH="$HOME/go/bin:$PATH" GOCACHE=/tmp/wails-gocache REACT_FRONTEND=1 ./scripts/package-mac-app.sh
- 产物：dist/WeKnora Lite.app（wails build 36.2s，darwin/arm64，sqlite_fts5 tags，自签名）
- codesign --verify --deep --strict → OK
- 打包链路修复（本轮主代理提交）：
  - 7db7e4d8 声明 @weknora/web 直接依赖 esbuild@0.28.2（CI 模式安装丢失偶然提升导致测试文件解析失败）
  - bbc892ba apps/desktop/vite.config.ts 补 auth/onboarding、settings/local-preferences、sandbox/skill-install 三个子路径别名（desktop-renderer 构建因裸 @weknora/domain 文件别名误解析而失败）
- 运行环境约束与解法：沙箱禁止 ~/Library/Caches/go-build 写入 → GOCACHE=/tmp/wails-gocache；wails CLI 需 PATH+=~/go/bin；pnpm 非交互需 CI=true

## 运行时冒烟
- open dist/WeKnora Lite.app → 进程存活（pgrep ALIVE），Wails 资产服务器监听 127.0.0.1:59876
- 经 CDP 连接 webview 页面：登录页完整渲染（*邮箱/*密码/登录/创建账户/多模态文档解析✓/混合检索+知识图谱✓/ReAct 智能体问答），7 buttons，无渲染错误
- 截图：screenshots/wails-runtime-smoke-20260913.png
- 退出：osascript quit + 进程终止确认

## 限制（如实登记）
- 运行时「登录→KB 列表→聊天」完整交互链路需 GUI 手工操作，本轮验证到进程存活+UI 渲染层；后续轮次可经 CDP 驱动登录后逐页截图补全
- 脚本：.parity-tools/wails-runtime-smoke.cjs（连接 59876 webview）
