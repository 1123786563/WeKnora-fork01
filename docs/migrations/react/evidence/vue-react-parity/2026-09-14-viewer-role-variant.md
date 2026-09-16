# 2026-09-14 viewer 角色变体 live 双端对照（S00：R007 N001 N002 N015 R008 R032 角色维度）

## 受控测试账号（可复原）

- 新建 parity-viewer@local.dev（POST /api/v1/auth/register，username parityviewer）。
- owner（parity-test@local.dev）通过 POST /api/v1/tenants/10000/members {email, role:"viewer"} 将其加入租户 10000（与既有 parity 账号同做法，保留作角色变体回归账号）。
- 注意：该账号同时拥有自己的默认工作区（10008 owner），live sweep 必须把活动租户切到 10000（React 会话 tenantId 必须为字符串，否则 readReactPlatformState 校验失败退回登录页）。

## 发现并修复：设置分区角色门禁与 Vue 不一致

- **Vue 基线**（frontend/src/config/settingsAccess.ts SETTINGS_SECTION_MIN_ROLE）：models=viewer、members=viewer（只读页，入口保留、写操作由面板内部 hasRole(admin) 自 gate；TenantMembersPanel.vue 只读呈现 + 「只有 Owner 可以新增或移除成员」提示）。
- **React 修复前**：registry.ts 中 models/members minRole=admin → viewer 看不到入口、直连 URL 渲染「权限不足」。
- **修复**：packages/views/src/settings/registry.ts 两处 minRole → viewer（面板内部已自带只读门控：ModelSettingsPanel canCreate、TenantMembersPanel canManage）；registry.test.ts 同步更新期望（含 viewer 导航应含 models/members 的断言）。
- 全量 diff 其余分区 minRole 与 Vue 一致（ollama/weknoracloud/websearch/chathistory/vectorstore/parser/storage/sandbox/skills/mcp=admin，general/tenant/userprofile/mymemory/envvars/system=viewer，platform 系=system-admin）。

## live 双端对照（viewer@tenant 10000，zh-CN 1440x900，只读导航）

| 格 | Vue | React | 结论 |
|---|---|---|---|
| V1 KB 列表 | 共享空间入口隐藏、无新建知识库入口、看到本空间 3 个知识库 | 同（共享空间 false、新建入口 false、本空间 KB + 初始化横幅） | 一致 |
| V2 设置导航 | 账户组 4 项 + 空间信息/成员管理/模型管理（只读）；无消息管理/长期记忆/MCP/沙箱/技能/系统管理 | 同（members/models 修复后可见） | 一致 |
| V3 直连 members | 只读成员面板 + 「只有 Owner 可以新增或移除成员」提示；无添加/移除控件泄漏 | 同（deniedShown=false, membersPanelLeaked=false） | 一致 |
| V4 直连组织页 | 组织面渲染（viewer 视角我创建的/我加入的） | 同 | 一致 |

截图 screenshots/viewer-variant-20260914/v1..v4-{vue,react}.png。

## 本轮修复：viewer 的「本空间 · 仅查看」分组标签

- **Vue 基线**：KnowledgeBaseList.vue:1083-1086 tenantSectionLabelKey —— hasRole('admin') 为假（contributor/viewer）时分组标题读「本空间 · 仅查看」（knowledgeList.sections.tenantReadonly），图标 browse；admin/owner 读「本空间 · 其他成员」，图标 usergroup。
- **React 修复前**：domain groupKnowledgeBaseSections 恒用 tenantOthers 标签 → viewer 看到「本空间」（实为其他成员所有权口径）。
- **修复位置**：domain groupKnowledgeBaseSections 增加第三参 options.tenantReadonly（为真时 tenant 组 labelKey 切换为 tenantReadonly，组 key 保持稳定以不破坏折叠态/图标映射）；App.tsx 传入 { tenantReadonly: !viewer.isAdmin }，viewer 时组图标切 browse（对齐 Vue tenantSectionIconName）。
- **测试**：packages/domain list.test.ts 新增 1 例（label 切换 + 组 key 稳定）→ 15/15。
- **Live 复核**：viewer 账号 KB 列表页全文含「本空间 · 仅查看」，与 Vue 逐字一致。

## 门禁

- registry.test 6/6（含更新后的期望）；domain list.test 15/15；test:shared 444/444；test:web 856/856；typecheck:web 0 错误。
