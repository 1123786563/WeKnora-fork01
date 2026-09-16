# /platform 全局导航外壳落地（2026-09-12，Round 12 续）

- PlatformShell：侧栏（logo 行 + 折叠 260↔60px localStorage 持久化、新对话/知识库/智能体/共享空间、路径高亮含 /knowledgeBase/* 与 /platform/configuration）、KB 语境快捷过滤（全部/我创建的 → ?scope=）、底部用户区（me() 头像/名/邮箱 + 个人设置/退出登录）。所有受保护 /platform/* 分支经 renderShell 包裹；auth/join/onboarding/embed 保持裸页。
- 主代理独立 live 复核：真实登录→KB 列表，外壳/高亮/过滤/用户区渲染正确（platform-shell-live-check.png vs 基准 kblist-vue-live.png）。
- 门禁：typecheck:web ✅、test:web 139/139、build:web ✅。
- 与 Vue 的已知偏差（登记，不视为通过）：无 40px 图标轨（收藏/最近 scope React 页尚无）、文字 logo（无图片资源，weknora.png 待移植）、会话列表未并入侧栏（本轮范围）、无租户切换/邀请铃/命令面板（后续项）。
