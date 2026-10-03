# 2026-10-03 全范围 OCR 补全——完整报告 + 增量裁定（CAREER-OCR-GATE）

## 门禁结论：PASS

- 完整报告：`ocr-2026-10-03-fullrange-final.md`（333/333 selected 全覆盖，0 项失败）
- 覆盖：range `db234c5eb..b3d48d5cb`（既定 review target，未换范围）
- 补全过程：`0f0bcb06`（第 6 次，332/333）→ resume 会话 `617ad6a5`（332 复用 + 81/82 重跑，1 项超时失败：wiki-reader.css）→ 定向 resume 会话 `786220f3`（332 复用 + 1 项重跑成功）。
- 总量：**511 findings（0 critical / 18 high / 157 medium / 336 low）**；第 6 次部分报告为 378（13H/120M/245L），已裁定的 133 条 H/M 见 `ocr-2026-10-03-high-medium-digest.md`，口径不变。
- 失败重试全程零 429（少量超时/取消均被内建重试吸收；两次会话合计 1h19m，token ~39.6M input / ~0.95M output）。

## 增量 = 133 条（全部落在 56 个此前未覆盖文件）

分布：5 high / 35 medium / 93 low。Low 93 条维持原口径：待门禁通过后一并终局裁定（现门禁已过，可与旧 Low 245 合并 backlog 处理，非本轮职责）。

## High 增量裁定（5 条，对 main @ dd79be999 现码复核）

| # | 位置 | 内容 | 裁定 |
|---|------|------|------|
| H-A1 | apps/web/src/agents/AgentsPage.tsx:1014 | toggleFavorite 失败回滚按「当前集再取反」而非 wasFavorited 定向撤销，连续两次失败 UI/DB 发散 | **open-high**（现码逐字吻合） |
| H-A2 | apps/web/src/documents/KnowledgeDocumentsPage.tsx:4359 | `${stageNoticeClass(tone)}wk-stage-notice` 拼接缺空格 + `wk-documents-toast` 无 CSS 定义 + tone 裸类不匹配 `.wk-stage-notice.is-*`，全局上传 toast 三重失效 | **open-high**（stageNoticeClass@2786 现码吻合） |
| H-A3 | apps/web/src/settings/SandboxSettingsPanel.tsx:1323-1340 | TInputNumber 清空回调是 null 而非 ''，`value === ''` 永假 → Number(null)=0 污染 docker 数值字段（idle_ttl/cpu/memory/pids 及 cube/e2b 超时） | **open-high**（同仓 fromTInputNumber 已有修复先例） |
| H-A4 | apps/web/src/settings/SettingsPage.tsx:502 | envvars 壳层加载失败静默渲染空列表（SHELL_FEEDBACK_SELF_PANELS 常量已被重构掉，但 self-header 分支 502 行对 envVarPanel 仍无 sectionError/sectionLoading 渲染；EnvVarSettingsPanel 自身 error 态只覆盖变更操作） | **open-high**（成因载体变了，缺口仍在） |
| H-A5 | apps/web/src/settings/TenantMembersPanel.tsx:770-773 | permissions 弹层类名全仓无 CSS 规则 | **already-fixed-on-main**（settings.td.css:8439-8528 已补齐 .permissions-popup-overlay/.perm-* 全组规则） |

## Medium 增量裁定（35 条）

**open-medium（27 条，现码复核成立）**：

- agents：收藏水合无条件覆盖竞态（AgentsPage:926-934）；popup-menu div 无键盘可达（AgentsPage:447 一带）；`resourceOrigin.*` i18n 键缺失（packages/i18n 无此键）；error/notice 半死链（notice 恒 null，AgentsPage:648/1186）；AgentEditorModal 16× `as never`。
- auth：LoginPage 密码框丢失明文/密文切换（453 行裸 type="password"）。
- documents：div[role=menuitem] 无 tabIndex/键盘（KnowledgeDocumentsPage:468）；批量删除 Popconfirm→再确认双重流程矛盾（4771）；documents-u.css `#07c05f]/4` 无效色（2704）；calc 缺空格 ×4（75/79/348/859）；chunks 预取吞错且与详情竞速（KnowledgeDocumentDetailPage:176-184）。
- chat（packages/views）：mention 菜单 fixed 定位开伞只算一次不随滚动重算（composer:403/405 仅两处 set）；推荐问题 loading 指示缺失（message-face 无 suggestions 态，list:487 仅 ready）；回退面 ⋯ 按钮无 onClick（page:722）；回底控件 div 无键盘（page:790）；session-sidebar 丢 parent_session_id 分叉徽标⑂。
- settings：mymemory 与 envvars 同构（502 分支 payload-only 无错误态）；ResourceSettingsPanel TInput type=number 丢 min/max（239-246）；SandboxBackendBadge 在 Skill/Sandbox 两面板重复定义；SystemAuditLogPanel wk-audit-* 规则仅挂 .wk-settings-drawer-root 作用域（跨挂载点缺口待确认）；TenantMembersPanel invitationTableColumns useMemo deps 漏 revoke。
- 其它域：OrganizationsPage org-card div 无键盘（1031）；org-u.css calc(90vh-120px)（928）；KnowledgeSettingsPage.css 隐藏 checkbox 无焦点指示；parserSettings aria-label 挂无角色 span（227）；wiki-reader.css 硬编码浅色（267-273）；market-u.css 主题令牌平铺为字面值（46-47）。

**needs-ruling（3 条，需人工核对 Vue 源后定夺）**：
- SystemGlobalSettingsPanel:395 React 保留段类名与 settings-wrapper.css 1000-1013 规则错位（未复现明显坏样式）
- SystemGlobalSettingsPanel:474 int 项丢 max 上限（Vue 侧未找到 9999 证据）
- settings.td.css:5000 .wk-settings-panel-heading 优先级冲突（行漂移后未能定位冲突对）

**false-positive / already-fixed（5 条）**：
- AgentEditorModal:2011 nav button→div 降级 —— 现码 2114 是 `<button type="button">`，不成立
- SystemGlobalSettingsPanel:465 i18n 双重插值 —— 已修（SETSYS-N4，见 470 行注释）
- TenantMembersPanel:281 TablePager 死代码 —— maxPage/jump 在 jumper 中使用、846 行挂载，不成立
- settings.td.css:3421 hint-popover 重复平移 —— 现仅 761 一处定义，不成立
- LoginPage:435 注册后邮箱预填丢失 —— 现码 207-210 不清 email，预填保留，不成立

## 处置

- 新 open-high 4 条 + open-medium 27 条 + needs-ruling 3 条：并入 high-medium digest 待裁/待修 backlog（与既有 13H/120M 同通道），本轮生产码零改动（核销轮约束）。
- Low 336（旧 245 + 新 93）：门禁已过，待统一终局裁定。
- 门禁判据「selected 全覆盖 + 零 LLM 失败」首次满足，**OCR 全范围门禁关闭**。
