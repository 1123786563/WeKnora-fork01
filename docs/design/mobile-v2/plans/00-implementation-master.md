# Expo高保真产品化 · 增量实施总计划

> **For agentic workers:** 使用subagent-driven-development或executing-plans逐任务执行，保留独立规格与质量审查。

**Goal:** 将既有Expo移动工作台按本轮18页高保真设计接入WeKnora真实产品链。

**Architecture:** MX任务是原W01–W37、UX00–UX10及相关T/H要求的增量展开，不取代旧执行域/商业域，不重置旧任务完成状态。先验证协议与原生边界，再页面闭环，最后按profile发布。

**Tech Stack:** 当前Go/Gin/GORM、Expo/React Native、共享TypeScript/pnpm；UI延续已有Unistyles/Happy可解耦组件。

**Spec:** [详细设计](../docs/01-detailed-design.md)、[18页规格](../docs/02-screen-specifications.md)、[令牌与组件](../docs/03-design-system.md)、[API契约](../docs/04-api-contracts.md)。

## Global Constraints

继续现有apps/mobile与WeKnora产品控制面；不新增权威mobile_tasks、第二套审批或钱包。保留原W/T/H依赖和验收状态。字段/路由/代码路径必须对照当前checkout；本计划新模块为建议增量，不承诺已经存在。一次用户意图一个持久request_id；未知启动先对账。取消、执行停止、费用结算分别表达。工具、预算与连接授权分别处理。静态、单元、数据库、原生和真实服务证据不混记；blocked-env不能记为通过。仅修改已取得锁的文件；不得擅自push、merge、发布或触发Provider真实写入。

## 1. 任务身份与依赖语义

36项MX任务全为工程待执行状态。本次实际完成的是设计、令牌和HTML原型；不将浏览器QA结果写为MX原生任务accepted。

`depends_on`是直接MX依赖；`conditional_dependencies`按本轮激活profile加入。`maps_w/maps_ux`是追溯关系，不表示必须先完成整个W父任务才做其子切片（例如MX-002基线映射W36，不形成W36反向阻塞早期开发的循环）。MX-001须从仓库读取原始W task-index/DAG，展开其直接/条件/继承要求到切片级，保持原要求不被删去。T18/真实Provider、原生设备与账单环境等外部门槛单独记录。

当前阶段编号只是工作分组，不附加“上一阶段全部完成”的隐藏依赖。资源/远程/语音未启用不阻塞核心；但M05核心版不得声称已具备真实附件上传，资源profile要启用MX-025后才开放该能力。

## 2. 分册与范围

| 分册 | 任务 | 交付 |
|---|---|---|
| [S00 基线与契约校准](01-S00-baseline.md) | MX-001—MX-006，6项 | 基线与契约校准 |
| [S01 原生基础与设计系统](02-S01-native-foundation.md) | MX-007—MX-012，6项 | 原生基础与设计系统 |
| [S02 核心页面与任务闭环](03-S02-core-workflows.md) | MX-013—MX-021，9项 | 核心页面与任务闭环 |
| [S03 资源与受控扩展](04-S03-resources-extensions.md) | MX-022—MX-031，10项 | 资源与受控扩展 |
| [S04 质量与发布验收](05-S04-delivery.md) | MX-032—MX-036，5项 | 质量与发布验收 |

## 3. 完整任务索引

| ID | 任务 | 直接依赖 | profile |
|---|---|---|---|
| [MX-001](01-S00-baseline.md) | 当前代码、接口与台账对账 | 无 | core |
| [MX-002](01-S00-baseline.md) | Expo 与 React 原生依赖基线 | MX-001 | core |
| [MX-003](01-S00-baseline.md) | 跨语言契约与能力规则冻结 | MX-001 | core |
| [MX-004](01-S00-baseline.md) | SSE 完整事件与控制帧接通 | MX-003 | core |
| [MX-005](01-S00-baseline.md) | 类型化交互与乐观并发命令 | MX-003 | core |
| [MX-006](01-S00-baseline.md) | 持久提交身份与请求对账 | MX-003 | core |
| [MX-007](02-S01-native-foundation.md) | 语义设计令牌与原生主题 | MX-001 | core |
| [MX-008](02-S01-native-foundation.md) | 原生基础组件与状态组件 | MX-007 | core |
| [MX-009](02-S01-native-foundation.md) | 四 Tab 与产品路由容器 | MX-003, MX-008 | core |
| [MX-010](02-S01-native-foundation.md) | 产品登录、SSO 与冷启动身份 | MX-002, MX-003, MX-009 | core |
| [MX-011](02-S01-native-foundation.md) | 空间切换、缓存与迟到响应隔离 | MX-010 | core |
| [MX-012](02-S01-native-foundation.md) | 原生持久事件、快照与恢复 | MX-002, MX-004, MX-011 | core |
| [MX-013](03-S02-core-workflows.md) | 工作台聚合读模型与首页 | MX-008, MX-011 | core |
| [MX-014](03-S02-core-workflows.md) | 会话列表、搜索与分页 | MX-013 | core |
| [MX-015](03-S02-core-workflows.md) | 新建任务、资源编排与提交 | MX-006, MX-012, MX-016 | core |
| [MX-016](03-S02-core-workflows.md) | Agent 目录与可用能力选择 | MX-008, MX-011 | core |
| [MX-017](03-S02-core-workflows.md) | 产品对话渲染与富消息 | MX-004, MX-009, MX-012 | core |
| [MX-018](03-S02-core-workflows.md) | 执行详情与保守取消展示 | MX-006, MX-017 | core |
| [MX-019](03-S02-core-workflows.md) | 工具审批详情与二次确认 | MX-005, MX-011, MX-017 | core |
| [MX-020](03-S02-core-workflows.md) | 预算、问题与连接授权交互 | MX-019 | core |
| [MX-021](03-S02-core-workflows.md) | 通知注册、收件箱与安全深链 | MX-013, MX-019 | core |
| [MX-022](04-S03-resources-extensions.md) | 资源与知识消费页面 | MX-008, MX-011 | resources |
| [MX-023](04-S03-resources-extensions.md) | 连接详情与授权生命周期 | MX-019, MX-022 | connectors |
| [MX-024](04-S03-resources-extensions.md) | 不可变成果预览、下载与分享 | MX-017, MX-022 | resources |
| [MX-025](04-S03-resources-extensions.md) | 原生附件与上传校验 | MX-012, MX-015, MX-022 | resources |
| [MX-026](04-S03-resources-extensions.md) | 执行目标选择与能力解释 | MX-016, MX-018 | remote |
| [MX-027](04-S03-resources-extensions.md) | Paseo 服务身份与远程治理接线 | MX-003, MX-006, MX-026 | remote |
| [MX-028](04-S03-resources-extensions.md) | 确认式听写与会话草稿 | MX-017, MX-025 | dictation |
| [MX-029](04-S03-resources-extensions.md) | 实时语音与独立中断语义 | MX-006, MX-021, MX-028 | voice |
| [MX-030](04-S03-resources-extensions.md) | 账户偏好、外观与退出 | MX-009, MX-011 | core |
| [MX-031](04-S03-resources-extensions.md) | 空间用量与只读商业视图 | MX-013, MX-030 | core |
| [MX-032](05-S04-delivery.md) | 服务端能力清单与部署开关 | MX-013, MX-019, MX-021 | core |
| [MX-033](05-S04-delivery.md) | 恢复、并发与安全故障注入 | MX-006, MX-012, MX-018, MX-019, MX-032 | core |
| [MX-034](05-S04-delivery.md) | 双平台产品链 E2E | MX-015, MX-017, MX-018, MX-019, MX-021, MX-030, MX-031, MX-033 | core |
| [MX-035](05-S04-delivery.md) | 视觉、无障碍与性能验收 | MX-008, MX-017, MX-034 | core |
| [MX-036](05-S04-delivery.md) | 发布门禁、回退与交接 | MX-032, MX-033, MX-034, MX-035 | core |

## 4. 可以滚动并行的工作

| 候选并行 | 先决条件 | 不可共享的资源 |
|---|---|---|
| MX-002依赖校准 ∥ MX-003契约 ∥ MX-007令牌 | MX-001完成；锁文件仅MX-002拥有 | package/lock单写，不让UI样式升级依赖 |
| MX-004事件 ∥ MX-005审批 ∥ MX-006请求 | MX-003已冻结；完整Files无冲突 | 路由/DI和共同SDK出口由集成负责人单写 |
| MX-013首页 ∥ MX-016目录 ∥ MX-022资源 | MX-008/MX-011前置具备 | 共享路由入口申请合入队列 |
| MX-019审批页面 ∥ MX-024成果 | 各自直接前置通过 | ConversationScreen集成接缝仍需单写 |
| MX-027远程 ∥ MX-028听写 | 各自前置通过；受控外部环境独立 | 执行节点、provider额度与原生设备不同租约 |

上述只是候选，不代表全写集合天然无交集。协调者每次从JSON计算ready队列，再按file_lock_keys和环境租约过滤。默认最多2个实现者、1个审查者；平台不支持独立agent时串行，不虚构并行审查。

## 5. 热点文件与集成顺序

`pnpm-lock.yaml`、根/移动package、`internal/router/routes_workbench.go`、路由聚合/DI、`agent_runtime.go`、auth根layout、ConversationScreen、contract出口、设计令牌源、PG/SQLite迁移序列、台账都保持单写。新路由的具体router.go/DI文件必须在MX-001补入拥有该功能任务的写集合；仅创建handler不等于接通。

每分支基于已接受集成SHA。review期间不释放冲突锁。合入后跑跨语言契约、受影响共享包和相关原生挂载测试；不能以各自分支曾通过代替集成结果。

## 6. 验收层级

| 层级 | 证据 | 不能替代它的证据 |
|---|---|---|
| static | 类型/格式/结构/来源 | UI截图 |
| unit | 真实行为断言/发现测试数 | 空probe、返回expected的假适配 |
| backend | 当前路由/DI/service/repository、PG+SQLite | handler单文件存在 |
| native-component | 实际RN页面挂载、按钮接VM | HTML按钮或纯函数 |
| native-e2e | iOS/Android产品链、权限/杀进程/恢复 | Expo Web、打包成功 |
| live-service | 受控Provider/节点/推送/语音和授权资源 | mock或skip |
| release | 启用profile全部子门禁、兼容/回滚 | 任务顶层一行passed |

阶段分册的probe仅组织验收，不实现假业务。每任务有具体输入/观测输出示例；建立接缝后测试真实行为RED，而不是拿缺模块错误当RED。缺环境记录blocked-env，不能常量构造成功。

## 7. 发布profile与原任务追溯

core先完成登录/空间/发任务/事件/审批/恢复/通知/清理/发布。resources加入MX-022/024/025；connectors加入MX-023与原T13/T18真实门槛；remote加入MX-026/027及W17–W24、W26等激活依赖；dictation是本轮W29切片，voice还要W30/W31及原发布闭包；personal_node另需W23；full_happy另需W32与H24–H33等原矩阵完整实现。OIDC能力另有W08真实IdP与双平台回跳证据。

MX-036如果core通过而remote受阻，只发布core并关闭remote，不把总计划改为全量通过。写入商业真实验收须具有当前用户明确授权，不从本设计推定。

## 8. 恢复、报告与交接

状态：pending→ready→implementing→review→integrating→accepted；修复回到fixing；阻塞使用blocked-env/blocked-dependency/blocked-review。每项记录BASE/HEAD/integrated SHA、证据hash、profile、依赖accepted SHA、写锁、命令/目录/退出码、测试数、DB/设备/服务环境、双审结论、剩余风险。

源任务状态与本轮设计状态分开。执行前保存本交付版本与原W/T/H规格hash；中断恢复核对真实commit和证据而非重派全部任务。没有原生/外部环境时完成可验证分支并保留准确阻塞，不把本地原型当产品完成。
