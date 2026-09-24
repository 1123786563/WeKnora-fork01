# 交互面板盘点矩阵（Phase I）

- 日期：2026-09-24（spec：docs/specs/2026-09-24-interaction-panel-parity.md）
- 基线：main@d35583fa3；Vue :5174（frontend/）/ React :5175（apps/web/）/ 后端 :8084（parity fixture 账号）
- 产出脚本：`scripts/parity/panel-inventory.mjs`（盘点）→ `inventory.json`（原始数据，本目录）→ `scripts/parity/panel-matrix-gen.mjs`（草稿）；本矩阵为人工归并策展版
- 截图：`png/<页面>-<触发器>-<vue|react>.png` 共 118 张（同类面板每页只取代表入口）

## 方法与检测机制

- headless 双端按 auto-scan.mjs 同款登录注入 + fixture 解析（kb/chat 页用 fixture 名），逐页三段滚动枚举可见可点击元素（`button/a/[role]/[class*=btn|button|icon-btn|trigger]/.t-select…` + 纯图标 CSS 兜底）+ 真实鼠标 hover 首个会话行揭示 CSS `:hover` 门控触发器。
- 逐触发器**真实鼠标点击**（`data-px-target` 标记法），点击后用**浮层基线 diff**（R0 快照 + key 计数差量）检测新面板并按类名/role 分类；React 会话行菜单"面板常驻 DOM、打开只切状态"的场景用**触发器 `aria-expanded` 兜底**。
- 只打开不确认：截图 → ESC → 安全空白点击 → 兜底整页重载；破坏性/写操作动词类（删除/保存/发送/测试/上传…约 60 个模式）只记录不实点。
- 页面清单 = auto-scan `ALL_PAGES` 的 46 个静态页（14 个 ix-* 交互态条目的触发器按签名匹配标注"复用"）；`redirect-system`→`settings-system-global`、`redirect-integrations`→`settings-integration-im` 落点去重。

### 已知盘点口径（读表必读）

1. **全局壳触发器**（左侧栏，42 个登录页重复）单列一节，不入分域表。
2. **双端命名/类名异构已人工归并**：`.kb-info-button`(Vue) ↔ `查看知识库信息`(React aria)；`.t-select-input`(Vue) ↔ `.t-select`(React)；`.t-button`(Vue) ↔ `.t-trigger`(React)；`.t-button__text`(Vue 供应商选择) ↔ React 端 lost。表内保留原始双端两行时以"待核实对端"呈现，归并结论见异构点清单。
3. **盘点扰动**：settings-parser 的 9 张引擎卡片 React 端因面板关闭重载后异步渲染未复点击（lost），非 UI 缺失——双端同为 `button.engine-card` 开 drawer（代码级同构）。
4. React 端会话行 `session-action-menu-panel` 常驻 DOM（12 个 laid-out 实例，视觉隐藏），基线吸收后不误报；会话行"更多"(`.menu-more-wrap`) hover 探针在 React 端未稳定揭示，但双端代码同构（SessionSidebarRow.vue / session-sidebar.tsx）。

## 全局壳触发器（42 个登录态页面通用）

| 触发器 | 选择器（双端兜底链） | 面板(Vue/React) | 建议 |
|---|---|---|---|
| 用户菜单 | `.user-button`（双端同名） | dropdown / dropdown | **新增扫描项**（截图 kb-list-user-menu-*.png） |
| 会话行"更多" | `.menu-more-wrap`（hover 门控，双端同名） | popover / popover（React 探针未揭示，代码级同构） | **新增扫描项**（Phase II 用 hover 序列） |
| 搜索 | `.header-icon-btn[aria-label=搜索]` | - | 排除：deny（即时过滤） |
| 新对话 / 知识库 / 智能体 / 共享空间 / logo / 会话行标题 | `.menu_item` 等 | - | 排除：导航（静态项已覆盖） |
| 收起侧边栏 | `.sidebar-toggle` | - | 排除：shell 状态切换 |
| 关闭设置（settings 域 29 页） | `.close-btn` | - | 排除：导航 |

## 分域总表


### 知识库域（kb-list + 6 个 kb fixture 页）

| 页面 | 触发器 | 选择器 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |
|---|---|---|---|---|---|---|---|
| kb-demo | .doc-tag-filter-trigger__prefi | .doc-tag-filter-trigger__prefix | popover | popover | 一致 | 新增 |  |
| kb-demo | .doc-tag-filter-trigger__suffi | .doc-tag-filter-trigger__suffix | popover | popover | 一致 | 新增 |  |
| kb-demo | .doc-view-toggle-btn | .doc-view-toggle-btn.active | popover | - | 类型异 | 复用 ix-kb-listview |  |
| kb-demo | .kb-info-button | .kb-info-button | popover+drawer | - | 单端面板 | 待核实对端 |  |
| kb-demo | .kb-settings-button | .kb-settings-button | dialog+drawer | - | 单端枚举 | 复用 ix-kb-settings | kb-demo-kb-settings-button-vue.png |
| kb-demo | .t-button | [data-guide=kb-detail-add-doc] | popover+dropdown | - | 单端面板 | 待核实对端 |  |
| kb-demo | .t-select | .t-select.t-select-input | - | select-popup+popover | 单端面板 | 待核实对端 | kb-demo-t-select-t-select-input-react.png |
| kb-demo | .t-select-input | .t-select-input.t-select | select-popup+popover | - | 单端面板 | 待核实对端 | kb-demo-t-select-input-t-select-vue.png |
| kb-demo | .t-trigger | .t-trigger | - | popover+dropdown | 单端面板 | 待核实对端 |  |
| kb-demo | Parity KB Demo | .breadcrumb-link.dropdown | popover | dropdown+popover | 类型异 | 新增(先核实DOM异构) |  |
| kb-demo | 全部标签 | .doc-tag-filter-trigger__label | popover | popover | 一致 | 复用 ix-faq-tagfilter |  |
| kb-demo | 批量管理 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-s | popup-other | popup-other | 一致 | 复用 ix-kb-batch | kb-demo-批量管理-vue.png |
| kb-demo | 按标签筛选 | aria:按标签筛选 | popover | popover | 一致 | 复用 ix-faq-tagfilter |  |
| kb-demo | 查看知识库信息 | aria:查看知识库信息 | - | popover | 单端面板 | 待核实对端 | kb-demo-查看知识库信息-react.png |
| kb-demo | 设置 | aria:设置 | - | dialog | 单端枚举 | 复用 ix-kb-settings | kb-demo-设置-react.png |
| kb-demo-creatchat | .control-btn | [data-guide=chat-kb-mention] | mention-menu+popover | - | 单端面板 | 待核实对端 | kb-demo-creatchat-chat-kb-mention-vue.png |
| kb-demo-creatchat | mock-stream-model 200K | .model-selector-trigger | select-popup+dropdown | - | 单端面板 | 待核实对端 |  |
| kb-demo-creatchat | 上传附件 | aria:上传附件 | - | - | 单端枚举 | 排除:写操作 |  |
| kb-demo-creatchat | 发送 | [data-guide=chat-send] | - | - | 一致无面板 | 排除:写操作 |  |
| kb-demo-creatchat | 快速问答 | .control-btn.agent-mode-btn.is-normal | select-popup+dropdown | - | 单端面板 | 待核实对端 | kb-demo-creatchat-快速问答-vue.png |
| kb-demo-creatchat | 选择智能体 | aria:选择智能体 | - | select-popup+dropdown | 单端面板 | 待核实对端 | kb-demo-creatchat-选择智能体-react.png |
| kb-faq | .card-more-btn | .card-more-btn | popover | popover | 一致 | 新增 |  |
| kb-faq | .content-bar-icon-btn | .content-bar-icon-btn.t-button.t-button--theme-default.t-button--variant-text.t-size-s | - | popover+dropdown | 单端枚举 | 复用 ix-faq-retrieval |  |
| kb-faq | .doc-tag-filter-trigger__prefi | .doc-tag-filter-trigger__prefix | popover | popover | 一致 | 新增 |  |
| kb-faq | .doc-tag-filter-trigger__suffi | .doc-tag-filter-trigger__suffix | popover | popover | 一致 | 新增 |  |
| kb-faq | .kb-info-button | .kb-info-button | popover+drawer | - | 单端面板 | 待核实对端 |  |
| kb-faq | .kb-settings-button | .kb-settings-button | dialog | - | 单端枚举 | 复用 ix-kb-settings | kb-faq-kb-settings-button-vue.png |
| kb-faq | .t-button | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | popover+dropdown | - | 单端面板 | 待核实对端 |  |
| kb-faq | .t-switch | .t-switch.t-is-checked.t-size-s | - | - | 单端面板 | 排除:开关 |  |
| kb-faq | .t-trigger | .t-trigger | - | popover+dropdown | 单端面板 | 待核实对端 |  |
| kb-faq | Parity FAQ Fixture | .breadcrumb-link.dropdown | popover | dropdown+popover | 类型异 | 新增(先核实DOM异构) |  |
| kb-faq | 全部标签 | .doc-tag-filter-trigger__label | popover | popover | 一致 | 复用 ix-faq-tagfilter |  |
| kb-faq | 按标签筛选 | aria:按标签筛选 | popover | popover | 一致 | 复用 ix-faq-tagfilter |  |
| kb-faq | 查看知识库信息 | aria:查看知识库信息 | - | popover | 单端面板 | 待核实对端 | kb-faq-查看知识库信息-react.png |
| kb-faq | 检索测试 | aria:检索测试 | - | - | 单端枚举 | 复用 ix-faq-retrieval |  |
| kb-faq | 设置 | aria:设置 | - | - | 单端枚举 | 复用 ix-kb-settings |  |
| kb-list | .header-action-btn | [data-guide=kb-list-create] | - | dialog | 单端枚举 | 复用 ix-kb-list-create | kb-list-kb-list-create-react.png |
| kb-list | .t-button | [data-guide=kb-list-create] | dialog+drawer | - | 单端枚举 | 复用 ix-kb-list-create | kb-list-kb-list-create-vue.png |
| kb-wiki | .doc-tag-filter-trigger__prefi | .doc-tag-filter-trigger__prefix | popover | popover | 一致 | 新增 |  |
| kb-wiki | .doc-tag-filter-trigger__suffi | .doc-tag-filter-trigger__suffix | popover | popover | 一致 | 新增 |  |
| kb-wiki | .doc-view-toggle-btn | .doc-view-toggle-btn | popover | popover | 一致 | 复用 ix-kb-listview |  |
| kb-wiki | .kb-info-button | .kb-info-button | popover+drawer | - | 单端面板 | 待核实对端 |  |
| kb-wiki | .kb-settings-button | .kb-settings-button | dialog+drawer | - | 单端枚举 | 复用 ix-kb-settings | kb-wiki-kb-settings-button-vue.png |
| kb-wiki | .t-button | [data-guide=kb-detail-add-doc] | popover+dropdown | - | 单端面板 | 待核实对端 |  |
| kb-wiki | .t-select | .t-select.t-select-input | - | select-popup+popover | 单端面板 | 待核实对端 | kb-wiki-t-select-t-select-input-react.png |
| kb-wiki | .t-select-input | .t-select-input.t-select | select-popup+popover | - | 单端面板 | 待核实对端 | kb-wiki-t-select-input-t-select-vue.png |
| kb-wiki | .t-trigger | .t-trigger | - | popover+dropdown | 单端面板 | 待核实对端 |  |
| kb-wiki | Wiki Parity Fixture | .breadcrumb-link.dropdown | popover | dropdown+popover | 类型异 | 新增(先核实DOM异构) |  |
| kb-wiki | 全部标签 | .doc-tag-filter-trigger__label | popover | popover | 一致 | 复用 ix-faq-tagfilter |  |
| kb-wiki | 按标签筛选 | aria:按标签筛选 | popover | popover | 一致 | 复用 ix-faq-tagfilter |  |
| kb-wiki | 查看知识库信息 | aria:查看知识库信息 | - | popover | 单端面板 | 待核实对端 | kb-wiki-查看知识库信息-react.png |
| kb-wiki | 设置 | aria:设置 | - | dialog | 单端枚举 | 复用 ix-kb-settings | kb-wiki-设置-react.png |
| kb-wiki-tab-graph | .kb-info-button | .kb-info-button | popover+drawer | - | 单端面板 | 待核实对端 |  |
| kb-wiki-tab-graph | .kb-settings-button | .kb-settings-button | dialog+drawer | - | 单端枚举 | 复用 ix-kb-settings | kb-wiki-tab-graph-kb-settings-button-vue.png |
| kb-wiki-tab-graph | Wiki Parity Fixture | .breadcrumb-link.dropdown | popover | dropdown+popover | 类型异 | 新增(先核实DOM异构) |  |
| kb-wiki-tab-graph | 查看知识库信息 | aria:查看知识库信息 | - | popover | 单端面板 | 待核实对端 | kb-wiki-tab-graph-查看知识库信息-react.png |
| kb-wiki-tab-graph | 设置 | aria:设置 | - | dialog | 单端枚举 | 复用 ix-kb-settings | kb-wiki-tab-graph-设置-react.png |
| kb-wiki-tab-wiki | .kb-info-button | .kb-info-button | popover+drawer | - | 单端面板 | 待核实对端 |  |
| kb-wiki-tab-wiki | .kb-settings-button | .kb-settings-button | dialog+drawer | - | 单端枚举 | 复用 ix-kb-settings | kb-wiki-tab-wiki-kb-settings-button-vue.png |
| kb-wiki-tab-wiki | Parity 目录改 1 | .wk-wiki-22 | - | - | 单端面板 | 排除:无面板 |  |
| kb-wiki-tab-wiki | Wiki Parity Fixture | .breadcrumb-link.dropdown | popover | dropdown+popover | 类型异 | 新增(先核实DOM异构) |  |
| kb-wiki-tab-wiki | 全库概览 | .wiki-content-link | dialog | - | 类型异 | 新增(先核实DOM异构) |  |
| kb-wiki-tab-wiki | 列表视图 | aria:列表视图 | popover | - | 类型异 | 复用 ix-kb-listview | kb-wiki-tab-wiki-列表视图-vue.png |
| kb-wiki-tab-wiki | 实体：WeKnora | .wiki-content-link | - | - | 一致无面板 | 排除:无面板 |  |
| kb-wiki-tab-wiki | 摘要 1 | .wiki-tab.wk-wiki-69.wk-wiki-71 | - | - | 单端面板 | 排除:tab |  |
| kb-wiki-tab-wiki | 新建目录 | aria:新建目录 | popover | - | 类型异 | 新增(先核实DOM异构) |  |
| kb-wiki-tab-wiki | 新建页面 | aria:新建页面 | dialog | dialog | 一致 | 新增 |  |
| kb-wiki-tab-wiki | 查看知识库信息 | aria:查看知识库信息 | - | popover | 单端面板 | 待核实对端 | kb-wiki-tab-wiki-查看知识库信息-react.png |
| kb-wiki-tab-wiki | 树形视图 | aria:树形视图 | popover | - | 类型异 | 新增(先核实DOM异构) |  |
| kb-wiki-tab-wiki | 概念：检索增强生成 | .wiki-content-link | - | - | 一致无面板 | 排除:写操作 |  |
| kb-wiki-tab-wiki | 知识 1 | .wiki-tab.wk-wiki-69.is-active.wk-wiki-70 | - | - | 单端面板 | 排除:tab |  |
| kb-wiki-tab-wiki | 索引 | .wiki-nav-item.wk-wiki-82.wk-wiki-83 | - | - | 单端面板 | 排除:无面板 |  |
| kb-wiki-tab-wiki | 设置 | aria:设置 | - | dialog | 单端枚举 | 复用 ix-kb-settings | kb-wiki-tab-wiki-设置-react.png |

### 对话域（chat）

| 页面 | 触发器 | 选择器 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |
|---|---|---|---|---|---|---|---|
| chat | .control-btn | [data-guide=chat-kb-mention] | mention-menu+popover | - | 单端枚举 | 复用 ix-chat-mention | chat-chat-kb-mention-vue.png |
| chat | .t-button__text | .t-button__text | - | - | 一致无面板 | 排除:无面板 |  |
| chat | mock-stream-model 200K | .model-selector-trigger | select-popup+dropdown | - | 单端面板 | 待核实对端 |  |
| chat | 上传附件 | aria:上传附件 | - | - | 单端枚举 | 排除:写操作 |  |
| chat | 发送 | [data-guide=chat-send] | - | - | 一致无面板 | 排除:写操作 |  |
| chat | 快速问答 | .control-btn.agent-mode-btn.is-normal | select-popup+dropdown | - | 单端面板 | 待核实对端 | chat-快速问答-vue.png |
| chat | 更多对话操作 | aria:更多对话操作 | popover | popover | 一致 | 复用 ix-chat-header-menu | chat-更多对话操作-react.png |
| chat | 检索完成 | aria:检索完成 | - | - | 一致无面板 | 排除:写操作 |  |
| chat | 沙箱终端 | aria:沙箱终端 | drawer | popover | 类型异 | 新增(先核实DOM异构) | chat-沙箱终端-vue.png |
| chat | 添加到知识库 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-round.t-size-s | drawer | dialog | 类型异 | 新增(先核实DOM异构) | chat-添加到知识库-react.png |
| chat | 请求信息 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-round.t-size-s | popover | dialog | 类型异 | 新增(先核实DOM异构) |  |
| chat | 选择智能体 | aria:选择智能体 | - | select-popup+dropdown | 单端面板 | 待核实对端 | chat-选择智能体-react.png |

### 平台路由域（agents/orgs/apps/creatchat/apps-connections）

| 页面 | 触发器 | 选择器 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |
|---|---|---|---|---|---|---|---|
| agents | .agent-favorite-star | .agent-favorite-star.is-favorited | - | - | 单端面板 | 排除:无面板 |  |
| agents | .header-action-btn | [data-guide=agent-list-create] | - | dialog | 单端枚举 | 复用 ix-agents-create | agents-agent-list-create-react.png |
| agents | .t-button | [data-guide=agent-list-create] | dialog | - | 单端枚举 | 复用 ix-agents-create | agents-agent-list-create-vue.png |
| apps | 刷新 | aria:刷新 | - | - | 一致无面板 | 排除:写操作 |  |
| apps-connections | 刷新 | aria:刷新 | - | - | 一致无面板 | 排除:写操作 |  |
| apps-connections | 授权 | aria:授权 | - | - | 一致无面板 | 排除:写操作 |  |
| apps-connections | 断开 | aria:断开 | - | - | 一致无面板 | 排除:写操作 |  |
| creatchat | .control-btn | [data-guide=chat-kb-mention] | mention-menu+popover | - | 单端面板 | 待核实对端 | creatchat-chat-kb-mention-vue.png |
| creatchat | mock-stream-model 200K | .model-selector-trigger | select-popup+dropdown | - | 单端面板 | 待核实对端 |  |
| creatchat | 上传附件 | aria:上传附件 | - | - | 单端枚举 | 排除:写操作 |  |
| creatchat | 发送 | [data-guide=chat-send] | - | - | 一致无面板 | 排除:写操作 |  |
| creatchat | 快速问答 | .control-btn.agent-mode-btn.is-normal | select-popup+dropdown | - | 单端面板 | 待核实对端 | creatchat-快速问答-vue.png |
| creatchat | 选择智能体 | aria:选择智能体 | - | select-popup+dropdown | 单端面板 | 待核实对端 | creatchat-选择智能体-react.png |
| orgs | .header-action-btn | .header-action-btn.t-button.t-button--theme-default.t-button--variant-text.t-size-s | - | dialog | 单端枚举 | 复用 ix-orgs-create | orgs-header-action-btn-t-button-t-butto-react.png |
| orgs | .t-button | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | dialog | - | 单端面板 | 待核实对端 | orgs-t-button-t-button-variant-text-t-b-vue.png |

### 设置域（settings 22 section + redirect 落点）

| 页面 | 触发器 | 选择器 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |
|---|---|---|---|---|---|---|---|
| redirect-integrations | 按智能体筛选 | aria:按智能体筛选 | popover+dropdown | select-popup | 类型异 | 新增(先核实DOM异构) | redirect-integrations-按智能体筛选-vue.png |
| redirect-integrations | 查看接入文档 | .doc-link | - | - | 一致无面板 | 排除:导航 |  |
| redirect-integrations | 添加渠道 | .channel-card.channel-card--add | drawer | - | 类型异 | 新增(先核实DOM异构) | redirect-integrations-添加渠道-vue.png |
| redirect-system | .t-select | .t-select.t-select-input | - | select-popup+popover | 单端面板 | 待核实对端 | redirect-system-t-select-t-select-input-react.png |
| redirect-system | .t-select-input | .t-select-input.t-select | select-popup+popover | - | 单端面板 | 待核实对端 | redirect-system-t-select-input-t-select-vue.png |
| redirect-system | 创建用户 | .t-button.t-button--variant-text.t-button--theme-primary.t-button--shape-rectangle.create-user-trigger | popover | dialog | 类型异 | 新增(先核实DOM异构) | redirect-system-创建用户-react.png |
| redirect-system | 启用复杂密码 | aria:启用复杂密码 | - | - | 单端面板 | 排除:开关 |  |
| redirect-system | 配置来源与优先级 | aria:配置来源与优先级 | popover | popover | 一致 | 新增 | redirect-system-配置来源与优先级-vue.png |
| redirect-system | 重置密码 | .t-button.t-button--variant-text.t-button--theme-danger.t-button--shape-rectangle.password-reset-trigger | - | - | 一致无面板 | 排除:写操作 |  |
| settings-chathistory | .t-switch | .t-switch.t-size-m | - | - | 单端面板 | 排除:开关 |  |
| settings-envvars | 沙箱密钥说明 | aria:沙箱密钥说明 | popover | popover | 一致 | 新增 | settings-envvars-沙箱密钥说明-vue.png |
| settings-general | .t-select | .t-select.t-select-input | - | select-popup+popover | 单端面板 | 待核实对端 | settings-general-t-select-t-select-input-react.png |
| settings-general | .t-select-input | .t-select-input.t-select | select-popup+popover | - | 单端面板 | 待核实对端 | settings-general-t-select-input-t-select-vue.png |
| settings-general | const msg = 'Hello'; // 0O1l | .font-preview.font-preview--mono | - | - | 单端面板 | 排除:无面板 |  |
| settings-general | 升级 / 续费 | .t-button.t-button--theme-default.t-button--variant-base | - | - | 单端面板 | 排除:导航 |  |
| settings-general | 大 | .t-radio-button | - | - | 一致无面板 | 排除:无面板 |  |
| settings-general | 小 | .t-radio-button | - | - | 一致无面板 | 排除:无面板 |  |
| settings-general | 正常 | .t-radio-button.t-is-checked | - | - | 一致无面板 | 排除:无面板 |  |
| settings-general | 示例 Sample 字体 Font — Aa Gg Oo 0 | .font-preview | - | - | 单端面板 | 排除:无面板 |  |
| settings-mcp | 添加服务 | .service-card.service-card--add | drawer | drawer | 一致 | 新增 |  |
| settings-members | .t-button | .t-button.t-button--variant-text.t-button--theme-danger.t-button--shape-square.t-size-s | - | - | 单端面板 | 排除:无面板 |  |
| settings-members | .t-pagination__btn | .t-pagination__btn.t-pagination__btn-next.t-is-disabled | - | - | 一致无面板 | 排除:分页 |  |
| settings-members | 20 条/页 | .t-select-input.t-select | - | - | 一致无面板 | 排除:无面板 |  |
| settings-members | 了解 RBAC | .doc-link | - | - | 一致无面板 | 排除:导航 |  |
| settings-members | 复制邀请链接 | aria:复制邀请链接 | - | - | 单端枚举 | 排除:写操作 |  |
| settings-members | 审计日志 | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:无面板 |  |
| settings-members | 撤销 | aria:撤销 | - | - | 单端枚举 | 排除:写操作 |  |
| settings-members | 生成共享链接 | aria:生成共享链接 | - | - | 一致无面板 | 排除:写操作 |  |
| settings-members | 角色权限说明 | aria:角色权限说明 | popover | popover+dialog | 类型异 | 新增(先核实DOM异构) | settings-members-角色权限说明-vue.png |
| settings-members | 邀请成员 | aria:邀请成员 | - | - | 一致无面板 | 排除:无面板 |  |
| settings-memory | .t-switch | .t-switch.t-size-m | - | - | 单端面板 | 排除:开关 |  |
| settings-models | .t-button__text | .t-button__text | popover+dropdown | - | 类型异 | 新增(先核实DOM异构) | settings-models-t-button-text-vue.png |
| settings-models | mock-embedding-model OpenAI·向量 | .model-card.model-card--embedding.model-card--clickable | drawer | drawer | 一致 | 新增 | settings-models-mock-embedding-model-OpenAI-向量维度-8-vue.png |
| settings-models | mock-stream-model OpenAI·200K | .model-card.model-card--chat.model-card--builtin.model-card--clickable | - | - | 一致无面板 | 排除:无面板 |  |
| settings-models | 查看内置模型管理指南 | .doc-link | - | - | 一致无面板 | 排除:导航 |  |
| settings-models | 模型测试 | .t-button.t-button--variant-text.t-button--theme-primary.t-button--shape-rectangle.model-test-trigger | - | - | 一致无面板 | 排除:写操作 |  |
| settings-models | 添加模型 | [data-guide=settings-add-model] | - | - | 一致无面板 | 排除:无面板 |  |
| settings-mymemory | .t-switch | .t-switch.t-is-checked.t-is-disabled.t-size-m | - | - | 单端面板 | 排除:开关 |  |
| settings-mymemory | 导出 | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:写操作 |  |
| settings-mymemory | 整理 | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:无面板 |  |
| settings-mymemory | 查看哪些记忆会在对话里被使用 | aria:查看哪些记忆会在对话里被使用 | popover | popover | 一致 | 新增 | settings-mymemory-查看哪些记忆会在对话里被使用-vue.png |
| settings-mymemory | 添加 | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:无面板 |  |
| settings-mymemory | 清空 | .t-button.t-button--variant-text.t-button--theme-danger.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:写操作 |  |
| settings-ollama | 重新检测 | .t-button.t-button--variant-text.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | popup-other | 类型异 | 新增(先核实DOM异构) | settings-ollama-重新检测-react.png |
| settings-parser | A anydoc 可用 进程内 Office 文档解析（无需 | .engine-card.engine-card--anydoc | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | M MarkItDown 可用 Microsoft Mark | .engine-card.engine-card--markitdown | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | M MinerU Cloud 不可用 MinerU Clou | .engine-card.engine-card--mineru_cloud | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | M MinerU 不可用 MinerU 自部署服务 | .engine-card.engine-card--mineru | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | O OpenDataLoader 可用 OpenDataLo | .engine-card.engine-card--opendataloader | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | P PaddleOCR-VL Cloud 不可用 Paddl | .engine-card.engine-card--paddleocr_vl_cloud | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | P PaddleOCR-VL 不可用 PaddleOCR-V | .engine-card.engine-card--paddleocr_vl | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | S Simple 可用 简单格式 & 图片解析（无需外部服务 | .engine-card.engine-card--simple | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | W WeKnora Cloud 不可用 使用 WeKnora | .engine-card.engine-card--weknoracloud | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-parser | 内 内置 可用 DocReader 内置解析引擎（docx/ | .engine-card.engine-card--builtin | drawer | drawer | 一致 | 新增 | settings-parser-内-内置-可用-DocReader-内置解析引擎-docx-pdf--vue.png |
| settings-platform-api-keys | 创建平台 API Key | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-s | drawer | drawer | 一致 | 新增 |  |
| settings-runtime-queues | 自动刷新（每 5 秒） | aria:自动刷新（每 5 秒） | - | - | 单端面板 | 排除:开关 |  |
| settings-sandbox | .t-switch | .t-switch.t-is-checked.t-size-m | - | - | 单端面板 | 排除:开关 |  |
| settings-sandbox | 什么是沙箱？ | aria:什么是沙箱？ | popover | popover | 一致 | 新增 | settings-sandbox-什么是沙箱-vue.png |
| settings-sandbox | 添加沙箱 | .sandbox-card.sandbox-card--add | - | - | 一致无面板 | 排除:无面板 |  |
| settings-sandbox | 集群搭建指南 | .header-action-link | - | - | 一致无面板 | 排除:导航 |  |
| settings-skills | 去配置沙箱 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle | - | - | 一致无面板 | 排除:导航 |  |
| settings-skills | 添加技能 | .t-button.t-button--variant-base.t-button--theme-primary.t-button--shape-rectangle | drawer | dialog+drawer | 类型异 | 新增(先核实DOM异构) | settings-skills-添加技能-vue.png |
| settings-storage | .t-button__text | .t-button__text | popover+dropdown | - | 单端面板 | 待核实对端 | settings-storage-t-button-text-vue.png |
| settings-storage | Parity COS COS · https://cos.p | .backend-card.backend-card--cos.backend-card--clickable | drawer | drawer | 一致 | 新增 | settings-storage-Parity-COS-COS-https-cos-parity-lo-vue.png |
| settings-storage | Parity MinIO MINIO · https://m | .backend-card.backend-card--minio.backend-card--clickable | - | - | 一致无面板 | 排除:无面板 |  |
| settings-storage | System LOCAL 默认 LOCAL · 本地存储 | .backend-card.rs-card.is-storage | - | - | 单端面板 | 排除:无面板 |  |
| settings-storage | 添加存储实例 | .backend-card.backend-card--add | - | - | 一致无面板 | 排除:无面板 |  |
| settings-tenant | 修改名称 | aria:修改名称 | - | - | 一致无面板 | 排除:无面板 |  |
| settings-tenant | 修改描述 | aria:修改描述 | - | - | 一致无面板 | 排除:无面板 |  |
| settings-userprofile | 修改密码 | aria:修改密码 | popover+confirm | popover+confirm | 一致 | 新增 | settings-userprofile-修改密码-vue.png |
| settings-vectorstore | PostgreSQL DEFAULT postgres | .backend-card.rs-card.is-env.is-list | - | drawer | 单端面板 | 待核实对端 | settings-vectorstore-PostgreSQL-DEFAULT-postgres-react.png |
| settings-vectorstore | 添加数据库 | .store-card.store-card--add | drawer | - | 类型异 | 新增(先核实DOM异构) | settings-vectorstore-添加数据库-vue.png |
| settings-websearch | .t-button__text | .t-button__text | popover+dropdown | - | 单端面板 | 待核实对端 |  |
| settings-websearch | T Tavily Parity Tavily | .provider-card.provider-card--tavily.provider-card--clickable | drawer | drawer | 一致 | 新增 |  |
| settings-websearch | 添加搜索引擎 | .provider-card.provider-card--add | - | - | 一致无面板 | 排除:写操作 |  |
| settings-weknoracloud | 保存凭证 | .t-button.t-button--variant-base.t-button--theme-primary.t-button--shape-rectangle.t-is-disabled | - | - | 一致无面板 | 排除:写操作 |  |
| settings-weknoracloud | 查看文档 | .doc-link | - | - | 一致无面板 | 排除:导航 |  |
| settings-integration-api | .t-button__text | .t-button__text | - | - | 单端面板 | 排除:无面板 |  |
| settings-integration-api | 仅空间 | .t-radio-button.t-is-checked | - | - | 一致无面板 | 排除:无面板 |  |
| settings-integration-api | 创建 API Key | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-s | drawer | - | 类型异 | 新增(先核实DOM异构) |  |
| settings-integration-api | 打开文档 | .doc-link | - | - | 一致无面板 | 排除:无面板 |  |
| settings-integration-api | 直接传用户 ID | .t-radio-button | - | - | 一致无面板 | 排除:无面板 |  |
| settings-integration-api | 签名 Token | .t-radio-button | - | - | 一致无面板 | 排除:无面板 |  |
| settings-integration-chrome | 打开 API 信息 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:导航 |  |
| settings-integration-claw | 打开 API 信息 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-s | - | - | 一致无面板 | 排除:导航 |  |
| settings-integration-embed | 按智能体筛选 | aria:按智能体筛选 | popover+dropdown | select-popup | 类型异 | 新增(先核实DOM异构) |  |
| settings-integration-embed | 新建嵌入渠道 | .channel-card.channel-card--add | - | - | 一致无面板 | 排除:无面板 |  |

### 免登录域（login/register）

| 页面 | 触发器 | 选择器 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |
|---|---|---|---|---|---|---|---|
| login | GitHub | .header-logo | - | - | 一致无面板 | 排除:导航 |  |
| login | 信息 | .header-link | - | - | 一致无面板 | 排除:导航 |  |
| login | 创建账户 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-l | confirm | confirm | 一致 | 新增 | login-创建账户-vue.png |
| login | 官方网站 | .header-link | - | - | 一致无面板 | 排除:导航 |  |
| login | 登录 | .t-button.t-button--variant-base.t-button--theme-primary.t-button--shape-rectangle.t-size-l | - | - | 一致无面板 | 排除:写操作 |  |
| login | 简体中文 | .header-link | dropdown | dropdown | 一致 | 新增 | login-简体中文-vue.png |
| register | GitHub | .header-logo | - | - | 一致无面板 | 排除:导航 |  |
| register | 信息 | .header-link | - | - | 一致无面板 | 排除:导航 |  |
| register | 创建账户 | .t-button.t-button--variant-outline.t-button--theme-default.t-button--shape-rectangle.t-size-l | confirm | confirm | 一致 | 新增 | register-创建账户-vue.png |
| register | 官方网站 | .header-link | - | - | 一致无面板 | 排除:导航 |  |
| register | 登录 | .t-button.t-button--variant-base.t-button--theme-primary.t-button--shape-rectangle.t-size-l | - | - | 一致无面板 | 排除:写操作 |  |
| register | 简体中文 | .header-link | dropdown | dropdown | 一致 | 新增 | register-简体中文-vue.png |

### 其他（dev-markdown）

| 页面 | 触发器 | 选择器 | 面板(Vue) | 面板(React) | 双端 | 建议 | 截图 |
|---|---|---|---|---|---|---|---|
| dev-markdown | GitHub | a | - | - | 一致无面板 | 排除:导航 |  |
| dev-markdown | Reset | .btn | - | - | 一致无面板 | 排除:无面板 |  |
| dev-markdown | Start | .btn | - | - | 一致无面板 | 排除:写操作 |  |
| dev-markdown | 复制代码 | aria:复制代码 | - | - | 一致无面板 | 排除:写操作 |  |

## 统计

| 口径 | 数值 |
|---|---|
| 原始可见候选（页面×元素×端） | 1487 |
| 实际点击 | 433（其余为导航/deny/上限排除） |
| 去重触发器（全局壳 22 + 域内 203） | **225** |
| 打开面板的触发器（域内 119 + 全局 2） | **121** |
| 建议新增扫描项（表内 53 行 + 全局 2；代表入口归并后约 40 项分 4 批） | **55** |
| 复用既有 ix-* 项 | 29 行命中（14 个 ix 项全部有对应触发器） |
| 双端异构点（见下节分类） | 23 组 |

### 建议分布

- 纯新增（双端同型面板）：23 行
- 新增但先核实 DOM 异构（双端都有面板、类型不同）：30 行 → 归并 11 组
- 单端待核实：39 行 → 别名归并 20 + composer 组 1 + 真实待核 12
- 排除（导航/tab/开关/分页/deny 写操作/无面板）：82 行

## 双端异构点清单（归并后 23 组）

### A. DOM 结构异构（双端都有面板，DOM 类型不同；视觉是否同构需 Phase II 像素收敛验证）

1. chat/添加到知识库：Vue `t-drawer--right`(drawer) vs React `wk-bookmark-dialog-backdrop`(dialog)
2. chat/请求信息：Vue `t-popup`(popover) vs React `chat-request-card`(dialog)
3. chat/沙箱终端：Vue `chat-sandbox-panel`(drawer) vs React 首检出 `t-tooltip`(popover)——React 沙箱面板形态待核
4. KB 卡片/面包屑菜单（kb-faq/wiki/demo/tab 页同款，5 页归 1 组）：Vue `t-popup`(popover) vs React 自定义 dropdown
5. settings-用户管理(redirect-system)/创建用户：popover vs dialog
6. settings-integration-embed+api/按智能体筛选（2 页归 1 组）：`popover+dropdown` vs `select-popup`
7. settings-members/角色权限说明：popover vs popover+dialog（React 多一层）
8. settings-skills/添加技能：drawer vs dialog+drawer
9. settings-models+storage+websearch/供应商选择器（`.t-button__text`/`.t-select`）：popover+dropdown，React 端 lost 待补测（settings-general 同款已证双端一致）
10. settings-parser/解析引擎卡片（10 张）：drawer 双端同构（React lost 为盘点扰动，归入一致）
11. settings-ollama/重新检测：React 检出 popup-other（疑似 backdrop），Vue 无——低置信待核

### B. 单端缺失待核实（12 组）

1. chat/creatchat/kb-demo-creatchat composer：Vue `快速问答`+`mock-stream-model 选择器`（select-popup）vs React `选择智能体`（wk-agent-selector-overlay）——功能布局差异或合并，3 页归 1 组
2. kb-wiki-tab-wiki/树形视图、新建目录、全库概览：Vue 有 popover/dialog，React 未检出
3. redirect-integrations/添加渠道：Vue drawer，React 未检出
4. settings-vectorstore/添加数据库：Vue drawer，React 未检出
5. settings-vectorstore/PostgreSQL 卡片：React drawer，Vue 未枚举到
6. settings-integration-api/创建 API Key：Vue drawer，React 未检出
7. orgs/.t-button：Vue dialog（疑似列表操作菜单），React 未检出
8. chat+creatchat/.control-btn 附件按钮 tooltip（popover）：Vue 检出，React 未检出

### C. 已归并的命名/类名异构（行为同构，Phase II 用选择器兜底链覆盖，不计异构）

`.kb-info-button`↔`查看知识库信息`；`.t-select-input`↔`.t-select`；`.t-button`↔`.t-trigger`；`.t-button__text`↔React lost；`.doc-tag-filter-trigger__prefix/suffix`（同一 select 两半区）。

## Phase II 分批建议（4 批，每批 10-20 项）

### 批 1：全局壳 + 对话域（12 项）

1. 用户菜单 `.user-button`（任一代表页，如 kb-list）→ dropdown
2. 会话行"更多" `.menu-more-wrap`（hover 序列：hover `.submenu_item` → click）→ session-action-menu
3. chat 沙箱终端（异构 A3 核实）
4. chat 添加到知识库（A1）
5. chat 请求信息（A2）
6. composer 选择器组：快速问答/选择智能体/模型选择（B1，先核实双端功能对应）
7. chat 附件按钮 tooltip（B8，tooltip 稳态门确认后纳入）
8. ix-chat-header-menu 复用核对
9. ix-chat-mention 复用核对
10. ix-kb-list-create 复用核对（kb-list 新建）
11. ix-agents-create + ix-orgs-create 复用核对
12. ix-kb-doc-detail 复用核对

### 批 2：知识库域（10 项）

1. KB 卡片/面包屑菜单（A4，代表入口 kb-faq `Parity FAQ Fixture` 行）
2. `card-more-btn` 卡片更多菜单（kb-faq）
3. 查看知识库信息 `.kb-info-button` / aria（popover+drawer）
4. content-bar 新建下拉 `.t-trigger`/`.content-bar-icon-btn >> nth=0`（ix-faq-import 复用）
5. 检索测试（ix-faq-retrieval 复用）
6. 全部标签 + 排序（ix-faq-tagfilter 复用 + `__suffix` 半区补）
7. wiki 工具栏：新建页面 dialog / 新建目录 / 树形视图 / 全库概览（B2，React 端先核实）
8. ix-kb-settings 复用核对
9. ix-kb-batch + ix-kb-listview 复用核对
10. KB 数据源选择器 `.t-select-input`/`.t-select`（kb-wiki/demo）

### 批 3：设置——账户/模型/引擎卡片（14 项）

1. userprofile 修改密码（popover+confirm）
2. mymemory 记忆使用说明 popover
3. models 模型卡片 drawer（代表：mock-embedding-model）
4. models 供应商选择器（A9）
5. parser 引擎卡片 drawer（代表：内置 DocReader；其余 9 张同款归并）
6. storage backend 卡片 drawer（代表：Parity COS）
7. vectorstore 添加数据库 + PostgreSQL 卡片（B4/B5）
8. sandbox/envvars hint 类 popover（`什么是沙箱？`/`沙箱密钥说明`，`hint-trigger` 同款）
9. skills 添加技能（A8）
10. mcp 添加服务 drawer
11. websearch provider 卡片 drawer（代表：Tavily）
12. platform-api-keys 创建平台 API Key drawer
13. members 角色权限说明（A7）
14. ollama 重新检测（A11 低置信）

### 批 4：系统/集成/免登录（9 项）

1. system 配置来源与优先级 hint popover
2. system-global 同款 hint（如有）
3. integration 按智能体筛选（A6，embed+api 两处）
4. integration 添加渠道（B3）
5. integration 创建 API Key（B6）
6. login 语言切换 dropdown（`.header-link`）
7. register 语言切换 dropdown
8. login 创建账户 confirm（deny 记录态，打开即收）
9. register 创建账户 confirm

> 批内验收口径沿用 spec：每项 pixdiff 容差 8、1280×720 稳态截图，逐项收敛 0.00%；异构组（A/B 类）先做代码级归因再决定收敛或豁免（台账判例 #16-#22）。

## 附录：inventory.json 字段速查

`results[].ends.{vue,react}.candidates[]`：`sig`（DOM 签名）/`category`（click|nav-link|deny|toggle|inline-tab|pagination|lost|blocked|cap|not-found）/`panel.types`（分类）/`panel.items[].cls|at`（面板类名与坐标）/`shot`（截图文件名）/`ix`（复用项）/`hover`（hover 门控）；`summary`：cands/clicked/panels/shots/reloads。
