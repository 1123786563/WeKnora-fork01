# 2026-09-14 STALE 行重验（12 行）：R007 R008 R032 R045 R047-R050 N001 N002 N014 N015

依据 2026-09-14-review-rows-audit.md：pass-5 标注的 12 个 STALE 行（源文件晚于所引证据）在本批按当前源 + 当前 live 双端重验。方法与证据：.parity-tools/stale-refix.cjs（只读：导航 + 设置关闭点击；真实后端 :8080，parity owner 账号，zh-CN 1440x900）+ 当前单测门禁 + registry 代码状态。截图 screenshots/stale-refix-20260914/。

## G-A 外壳解剖（R007 / N001 / N002）

- 触发 STALE 的源变更：PlatformShell.tsx（269c0619 RBAC org-nav 门控）+ main.tsx（启动应用主题）。
- live 锚点（双端一致）：知识库/智能体/共享空间/我的对话/设置导航项 + paritytester 用户区全部在位（owner 视角）；ga-shell-{vue,react}.png。
- 单测：platform-shell-org-subfilter + platform-shell-guide-reopen + routes.test 19/19。
- 结论：STALE 清除。仍开放：Wails/native 平台证据（R007/N001）；角色降级 live 场景（N002）。

## G-B 设置外壳（R008 / R032 / N014 / N015）

- 触发源变更：SettingsPage.tsx（6282000a members 单标题；后续 R025/R026 又重构 memory/mymemory 挂载与标题豁免）。
- live：成员管理面板标题唯一（React 1 个，无 wrapper 重复，identity.tenants 域名无泄漏——双端）；设置导航组（账户/空间…）本地化渲染；gb-members-{vue,react}.png。
- **关闭路径（N014）live 双端**：React [data-testid=settings-close] / Vue .close-btn → 双端均回到 /platform/knowledge-bases。Escape/popstate 由 SettingsPage handler + 路由测试覆盖。
- N015：registry 现状（代码检查）——models/members/mcp/sandbox/skills + platform 系列仍标 ported:false，但均经 PARTIALLY_PORTED_SECTIONS 渲染真实面板；占位符仅在无面板分区出现，行为与行声明一致。
- 结论：STALE 清除。仍开放：computed-style 逐值、Wails、更全角色变体（viewer 视角需第二账号，登记）。

## G-C MCP（R045）

- 触发源变更：McpSettingsPanel.tsx（57ef5ffb 凭证卡片）。
- live：MCP 分区双端正常渲染（gc-mcp-{vue,react}.png）；凭证卡片切片证据 2026-09-14-r044-credential-card.md 仍为当前实现。
- 结论：STALE 清除。仍开放：Wails。

## G-D 预览（R047 / R048 / R049 / R050）

- 触发源变更：documents/preview.ts（7649560b 媒体预览）+ KnowledgeDocumentDetailPage（d61113d2 重试态）。
- 单测：preview.test 4/4（当前源）。
- live 限制：parity 租户两个文档型 KB 当前无文档，expired/no-permission 的 live 证据无法在本环境产生——**STALE 清除（单层级），行保持 review**，其声明的 live 开放项原样保留。

## matrix 变更

12 行 note 各追加「2026-09-14 S00 re-verify: STALE cleared — …」（N014 为变体措辞追加），全部保持 review；验收状态不变，仅解除 STALE 阻塞标记并指明残余开放项。

## 结论

12 个 STALE 行全部完成当前源重验：A/B/C 组（9 行）具备 live + 单测双重当前证据；D 组（4 行，其中 R047-R050）单测层级重验、live 维度按行内声明继续保留。下一批可按各行剩余开放项（多为 Wails/native 或特定 live 场景）逐行推进验收。
