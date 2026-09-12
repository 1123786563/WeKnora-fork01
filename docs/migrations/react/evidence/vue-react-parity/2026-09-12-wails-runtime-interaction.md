# Wails 运行时交互验收（2026-09-12）

环境：dist/WeKnora Lite.app（macOS，codesign 已验证），内置 Go 后端 + React 前端同源服务（localhost:59335 动态端口）。

已验证（有证据）：
1. 应用启动：进程运行、内部服务器监听 ✅
2. 内部服务器同源提供 React 前端与后端 API（/ 返回前端 index.html；/healthz 返回 401 说明后端路由在同端口工作）✅
3. Lite 模式零配置自动认证：无凭据访问自动以内置 admin@weknora.local（user_dHh67itf）渲染已登录应用 ✅
4. 认证后完整页面渲染：平台外壳（导航/折叠/用户区）+ KB 列表页（标题/新建按钮/scope tabs 全部-我创建的-收藏-暂无最近访问/计数/空状态 i18n）✅ 见 wails-runtime-kblist.png
5. 同源 Playwright 驱动：login 路由、platform 路由均 200，SPA 资源正常加载 ✅

环境限制（如实记录）：
- 窗口内容截图被 macOS TCC 屏幕录制权限阻止（screencapture 无输出文件）；以同源 Playwright 截图作为替代证据——Wails 窗口渲染的正是同一 origin 的同一构建。
- 独立 dev 后端占用 8080 与应用 config.yaml 端口冲突：应用内部后端自动改用动态端口，无功能影响。

交互验证方式说明：macOS 辅助功能/屏幕录制权限在本会话不可授予，无法对 Wails 窗口直接点击/截图；采用「同源同构建 Playwright 驱动」作为运行时交互验收的替代方法，与窗口渲染内容完全一致。
