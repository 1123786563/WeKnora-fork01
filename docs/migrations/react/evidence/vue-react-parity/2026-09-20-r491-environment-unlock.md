# 2026-09-20 R491 环境解锁轮：4 项阻断面全部解锁+复扫，前端实质差异全域清零

- 主仓 main。用户关账口径选择「解锁环境项后复扫」后的执行轮。
- 提交链：ef2ccb24（并行收编：三端点运行时拉取+system-global Vue 形态+FAQ 对齐主体）→ 412c2192 → af1af690（bookmark 深化+测试拆分）。代码修复多经并行 lane 卷入（第 15/16 次），内容抽查完整。

## 四项解锁与复扫结论

### ① FAQ 条目创建 500（code 1007）— 已解锁
- **根因**：R487 fixture KB 直插 DB 时 embedding_model_id 为空 + 租户无任何 Embedding 模型 → CreateFAQEntry 链路 GetEmbeddingModel("") 失败。非代码 bug（双端同阻断）。
- **解锁**：mock 服务 /v1/embeddings 可用（任意 model 名返回 8 维）→ 创建 builtin-embedding-mock（id 904547b2-…，baseURL http://192.168.3.30:18090，SSRF 白名单已含）→ 删除空 fixture 重建（新 KB 0e4de98e-1400-413e-b627-eb3c65181f2f，创建时带 embedding_model_id——Update 链路不接受该字段，仅创建时可设）→ 条目创建 200。
- **复扫（代理 H）**：条目卡答案预览形态修复（hidden 被 flex 覆盖致答案泄漏，`[&[hidden]]:hidden` 保护类）；批量选择栏重构（已选 N 项/取消选择/批量设置标签/批量禁用/批量删除，删 Vue 无的「推荐」，删除改确认弹窗）；批量标签栏=单选标签弹窗（R488「待 500 解锁」清账）；行菜单/编辑抽屉五字段对称 PASS（R488 基础保持）。FAQPage 60/60。
- 顺带修复：React FAQ 双标题/编辑器等在 R488 已对齐面全部保持。

### ② 模板/预设/占位符三端点 — 后端本来就有（台账勘误），前端已改运行时拉取
- **勘误**：R487「三端点未实现」不实——8084 后端 GET /api/v1/agents/placeholders、/agents/type-presets、/tenants/kv/prompt-templates 全部 200 有数据；缺口在 React api-client 未封装+静态移植。
- **改造（代理 G）**：api-client 补 agents.typePresets()/agents.placeholders()/settings.promptTemplates.get()；AgentEditorModal 三处改运行时拉取（60s TTL+inflight 去重复刻 Vue editorResources store；静态表降级为回退——Vue 无回退为已记录决策）；发现并纠正静态表 2 处漂移（rag-qa 无 retain_retrieval_history、data-analysis 无 web_search_enabled）。api-client 292→293 绿、apps/web 2232/2232。
- **复扫（代理 H）**：三面 PASS——类型下拉 5 选项（后端 zh-CN i18n）、使用模板弹层 7 条（后端 KV 逐字）、可用变量 hint 4 变量；网络监听证实编辑器打开时三请求齐发。

### ③ system-admin 管理态 — 已提权+分区对齐
- **解锁**：docker exec postgres UPDATE users SET is_system_admin=true WHERE parity-test；重登后双端 /platform/settings?section=system-global 可达。
- **数据加载 bug（本轮新修）**：api-client parseSetting 对 last_modified_by 强制非空串，后端未修改过的设置返回 "" → 整分区渲染失败。改 typeof string 宽容（同 description 处理）。administration 4/4、api-client 293/293。
- **system-global 形态**：ef2ccb24（并行 lane）已完成 Vue 形态落地（标题「系统设置」+描述+4 分组带计数 7/3/7/2+分组描述+高风险面板全套：系统管理员管理/重置用户密码/创建用户/自助注册模式/注册默认空间策略/允许自助创建空间/每用户最大空间数）。
- **最终对称差（浏览器实测）**：**onlyVue=0**；React 仅多 8 行噪音（radio option label 泄漏 4、×/▲▼ icon 字符 3、「配置来源与优先级」=Vue 亦有该键 zh-CN.ts:3437 的折叠态时机差）。
- 任务队列/平台 API Key/审计日志分区本轮未逐项展开对比（可达性已验证），列入 R492 快扫。

### ④ 工具调用会话 — fixture 已构造+双端渲染验证
- **构造（代理 F）**：消息 API 无写入端点 → DB 直插（SQL 留存 /tmp/r491-fixture.sql）。会话 a4910001-0000-4000-8000-000000000491「工具调用 Parity Fixture」：agent_steps 2 步（knowledge_search 检索 2 结果+get_document_info 2 文档）+knowledge_references 指向 Parity KB Demo 真实文档。
- **双端实测**：Vue 工具时间线+引用抽屉（文档卡+查看详情链接+chunk 内容）、React「思考与工具」region+引用来源面板均正常渲染。
- **新发现差异已修**：React 引用面板 description 剥前 3 字符（cleanPreview `^(.{3}|…+)` 正则为 addbc480 原创启发式非 Vue 移植）→ 改仅剥省略号；references 3/3、domain 179/179；复验两文档 description 以「WeKnora」完整开头。
- 完成态消息按 R464 契约折叠为「检索完成」行；get_document_info 展开卡片仅流式态可见（未插 is_completed=false 变体，R492 可选）。

## 门禁

全量 apps/web：**2256/2256 fail 0**（node v26.4.0）。期间 9 个 bookmark 失败为并行编辑瞬态，稳定后全绿。

## 收敛状态

R482→R491 十轮后：94 个常规面 + 4 个新解锁面（FAQ 编辑态/三端点/system-admin/工具会话）实质差异全部关闭。剩余均为噪音类（select/radio option 文本泄漏、icon 字符 vs SVG、TDesign input.value 渲染差——视觉一致）与已记录的架构性豁免（React 后加菜单、quick-answer 多 type 模板选择器 R486 既有 gap、共享智能体路径待域落地）。

## R492 候选（低优先）

- system-admin 其余分区快扫（任务队列/平台 API Key/审计日志逐项）
- 工具会话 is_completed=false 变体（测 get_document_info 展开态）
- 批量删除确认气泡形态（t-popconfirm vs Dialog，文案已一致）
- noise 类消除（option 泄漏/自绘下拉）——用户已确认为可接受噪音基线
