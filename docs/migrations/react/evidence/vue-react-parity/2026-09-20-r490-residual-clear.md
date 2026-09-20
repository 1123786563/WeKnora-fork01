# 2026-09-20 R490 残留清理轮：R489 五实质 + 轻残留全处理（三代理 + 浏览器复验 6/6 PASS）

- 主仓 main；三代理 A（KB 设置抽屉域 9 项）/ B（文档+聊天域 6 项）/ C（组织+引擎域 5 项）。
- 主体被并行 lane 卷入 3176c312 + 161adf69（第 13 次卷走事故，内容抽查完整）；增量 bf736610。
- 前置：R489 台账（2026-09-20-r489-regression-sweep.md）残 5 实质 + 十余轻。

## 浏览器实测勘误（R489 报告修正，主会话双端 IAB 实测）

1. **#9 NEW/52/200 为误报**：双端基本信息分区都有；真实差=Vue 多「知识库已有内容，索引策略暂不支持调整。如需变更，请先清空知识库。」提示行（→A4 修复）。
2. **C2 models 服务商预填（D4-1）为误判**：Vue resetForm 也预填 generic=「自定义 (OpenAI兼容接口)」；TDesign 选中值渲染为 input.value 不进 innerText，M3 采集缺陷。双端一致，未改。
3. **C4 共享空间卡片菜单（N3）为误判**：React 自 R488 已是 ⋯ 菜单（共享空间设置/删除）；探针把 aria-label="编辑" 误读。已顺手修触发器 aria-label 为 common.moreActions。
4. **B5 trace 摘要块（#8）为误报**：Vue 菜单本体无摘要块，内容在卡片 hover popover（300ms 延迟）；React DocumentCardHoverPopover 已完整存在。探针 mouseenter 不冒泡未触发。补端到端测试 1/1 证明。

## 修复清单（全绿数字为代理 scoped 运行）

| 域 | 项 | 处置 | 测试 |
|----|----|------|------|
| A1 | #13 分块设置两套实现 | chunkingSection.tsx 共享组件（策略说明+测试分块效果真 API+档位 100/1000/2000/4000+512 字符/重叠 0/250/500+80 字符/分隔符 7 项/父子分块/高级选项），抽屉+设置页双挂载 | 10/10+126/126 |
| A2 | #18 存储引擎 | 照 Vue KBStorageSettings 重写：删空「本地存储」option、provider 大写 tag+默认 tag、endpoint/bucket hint、迁移 hint 条件化、「管理存储实例」入口（→?section=storage）、租户默认预选 | 同上 |
| A3 | #10 模型配置尾巴 | openEdit 异步拉详情（LLM 以 summary_model_id 为准不再预填，select value=""）；Embedding 有文件锁定提示+disabled | 同上 |
| A4 | 基本信息索引锁定提示 | 有文件时索引卡禁用+60% 透明+lockedTip 行 | 同上 |
| A5 | #16 图谱帮助链接 | 「如何启用知识图谱？」→ VITE_KG_GUIDE_URL/GitHub | 同上 |
| A6 | #17 0/4000 计数器 | tableMetadataInstructions textarea maxlength 4000+实时计数 | 同上 |
| A7 | #20 共享空表 Ⓘ/＋ 字符 | 改 KbIcon SVG（新增 add 图标） | 同上 |
| A8 | #19 数据源 + 字符 | 两处改 SVG | 同上 |
| A9 | #21c 活动时间格式 | Intl 2-digit 日期+时分秒（2026/09/19 00:22:01） | 同上 |
| B1 | D12 @ 弹层过滤 | mention-agent-filter.ts（Vue Input-field + tool-capabilities 逐函数 port：kb_selection_mode 三态/quick-answer 隐含能力位/file_types 门控）+ 空态文案 | 13/13 |
| B2 | #7 标签管理入口 | TagManageDialog（列表/搜索/新建/重命名/删除 force）+ TagFilterPanel footer 入口（canManage）+文档页接线（changed 刷新/清筛选/800ms 二次刷新） | 4/4+12/12 |
| B3 | N2 加入知识库接线 | BookmarkAnswerDialog（Vue botmsg formatManualTitle+buildManualMarkdown 预填语义、faq 过滤、draft 提交）+ ChatRoutePage onBookmark 接线 | 5/5 |
| B4 | N1 {'{{'} 转义 | agent-editor-fallback.ts 5 locale 净化 | 51/51 |
| B6 | D18 检索抽屉 | section-header（搜索设置+副标题）+保存按钮收窄为仅 parser（retrieval 防抖自动保存同 Vue） | 8/8 |
| C1 | owner「我」badge | 根因=role prop 早退吃掉 currentUserId；auth/me 照常填充 | 55/55 |
| C3 | 三引擎提交按钮 | footer 统一「测试连接/取消→保存」，创建+编辑同「保存」 | 15/15 |
| C5 | API 信息 URL | onTabChange→select('integration-'+tab)，URL replace+侧栏高亮 | 68/68 |

## 浏览器复验（主会话 IAB 双端，6 面 PASS）

| 面 | 结论 |
|----|------|
| 分块设置 | 主体形态全对齐（说明/档位/分隔符/父子/高级选项/测试分块效果）；轻残=▶/▸ 展开符与分隔符 × 移除符为文本字符（Vue 是 SVG 不进 innerText，噪音级） |
| 存储引擎 | 实例 select（value=实例 id）+管理存储实例+迁移 hint；innerText 实例行=native select option 泄漏（噪音） |
| 模型配置 | LLM select value=""（不预填 ✓）、Embedding 锁定提示 ✓ |
| 基本信息索引 | 锁定提示行 ✓ |
| @ 弹层（快速问答） | 不再泄漏 3 KB+未分类+2 文件，与 Vue 无弹层行为一致 ✓ |
| 搜索设置分区 | 副标题 ✓、无保存按钮（自动保存）✓、阈值 0.15/0.30/0.00 逐字一致 ✓ |

## 门禁与残留

- 全量 apps/web：fail 0（node v26.4.0，27.8s）。
- 轻残留（噪音/后续轮）：分块 ▶/▸/× 符号、策略占位「不填则按长度切分」（Vue 无）、select option 泄漏类、questionGeneration customInstructions 计数器（开关关闭不显示）、B1 共享智能体路径（待该域落地）、React 独立 integrations 页 URL（超前面豁免）。
- 环境阻断不变：FAQ 条目 500、三端点、system-admin、工具会话。
- 扫描方法反馈（R491+）：t-select 触发器改采 input.value；icon-only 按钮 aria-label 与 innerText 分列；hover popover 探针用 mouseover {bubbles:true}+300ms。
