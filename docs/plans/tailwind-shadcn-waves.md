# Wave-2/3 派发模板（Orchestrator 用）

每个子任务 prompt 骨架 = conventions 手册 + 以下域参数。均要求：只改清单文件；
typecheck:web 全绿；指定测试全绿；回复改动清单/规则数/选择器改动数/门禁摘要。

| 域 | CSS | 消费 TSX | 测试 |
| --- | --- | --- | --- |
| documents | documents/documents.css | documents/KnowledgeDocumentsPage.tsx | documents/*.test.ts(x) 中引用类名者 |
| agents | agents/agents.css + agent-editor.css | AgentsPage.tsx + AgentEditorModal.tsx | AgentsPage.test.tsx |
| organizations | organizations/organizations.css | OrganizationsPage.tsx | organizations 相关测试 |
| faq | faq/faq.css | FAQPage.tsx | FAQPage.test.tsx、faq-search-drawer.test.tsx |
| chat | chat/chat.css | ChatRoutePage.tsx | chat/chat-page.test.ts |
| settings-A | settings-wrapper.css | SettingsPage.tsx + SystemGlobalSettingsPanel/PlatformApiKeysPanel/SystemAuditLogPanel/RuntimeQueuesPanel/ModelSettingsPanel/OllamaSettingsPanel/ConfigSettingsPanel/CloudSettingsPanel/ResourceSettingsPanel/EnvVarSettingsPanel/PortedSectionsPanel/GeneralPreferencesPanel | 对应测试 |
| settings-B | TenantMembersPanel.css + personal-memory.css | TenantMembersPanel.tsx + PersonalMemorySettingsPanel.tsx | 两面板测试 |
| settings-C | skill-settings.css + memory-workspace.css + sandbox-settings.css（若沙箱未完成） | SkillSettingsPanel.tsx + PersonalMemoryPanel.tsx + SandboxSettingsPanel.tsx | 对应测试 |
| guides | packages/views/src/guides/guides.css | views/guides/NewUserGuide.tsx（PlatformShell 若 shell 波未动则一并） | guides 相关测试；补跑 embed 门禁 |

派发节奏：wave-1（auth/knowledge-list/shell/sandbox-canyary）完成后逐个验收并 commit，
再放 2-3 个并发；chat 与 chat.css 最大，单独一轮。共享 styles.css 规则删除永远由 Orchestrator 收尾。

## 保留 CSS 的判定（settings 域补充）

drawer 作用域的「chrome 类」规则（如 .wks-modal .wks-content-wrapper select:not([multiple])、
.wk-settings-section .setting-row 等祖先作用域批量样式）服务于 10+ 面板的同族控件：
这类规则保留在 css 中（迁入 settings-wrapper.css 保留段）并注明原因；只把面板私有布局规则
转成 utilities。判定标准：一条规则服务的元素超过一个文件，或选择器依赖非组件祖先作用域。
