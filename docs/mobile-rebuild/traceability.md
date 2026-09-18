# 追溯：设计包任务（MX-001~036）→ RW 任务 → 模块 → 证据

原任务编号仅用于需求追溯，不继承原代码前置与完成状态（原 36 项在设计中均为 pending 提案）。
"去向"说明每项原设计要求如何被覆盖，或为何被显式移出范围。

| 原任务 | 需求要点 | 去向（RW / 裁决） | 证据位置 |
|---|---|---|---|
| MX-001 代码/接口/台账对账 | 契约冻结前对账 | RW-005 + api-facts.md（后端只读核验已完成） | docs/mobile-rebuild/api-facts.md |
| MX-002 Expo 依赖基线 | 工具链选择 | RW-001（Expo 55/RN 0.83/React 19.3，独立 npm 工程） | decisions.md D-02 |
| MX-003 跨语言契约冻结 | DTO 冻结 | RW-005（从真实 Go DTO 重写 wire 类型，非旧 TS 包） | tests/contracts |
| MX-004 SSE 完整事件与控制帧 | Go bytes→TS parser | RW-010 | tests/sse |
| MX-005 类型化交互与并发命令 | decisions/CAS | RW-016/RW-017 | tests/features |
| MX-006 持久提交身份与对账 | request_id 落盘先行 | RW-013 | tests/submit |
| MX-007 语义令牌与原生主题 | tokens→theme | RW-002 | tests/theme |
| MX-008 原生基础组件 | 10+ 组件 | RW-003 | tests/components |
| MX-009 四 Tab 与路由容器 | 导航结构 | RW-004 | app/ |
| MX-010 登录/SSO/冷启动 | M01 | RW-008 | tests/auth |
| MX-011 空间切换与迟到隔离 | M02 | RW-009 | tests/spaces |
| MX-012 持久事件/快照/恢复 | SQLite | RW-007 + RW-010 | tests/platform |
| MX-013 工作台聚合 | M03 | RW-011（overview 后端缺 → D-04 客户端聚合） | tests/workbench |
| MX-014 会话列表 | M04 | RW-012 | tests/conversations |
| MX-015 新建任务提交 | M05 | RW-013 | tests/submit |
| MX-016 Agent 目录 | M06 | RW-014 | tests/agents |
| MX-017 对话渲染 | M07 | RW-015 | tests/chat |
| MX-018 执行详情 | M08 | RW-016 | tests/executions |
| MX-019 工具审批 | M09 | RW-017 | tests/interactions |
| MX-020 预算/问题/连接交互 | M09 分支 | RW-017（budget 真实 API；question=craft 限定、connection=OAuth 流，D-05 记录差异） | decisions.md D-05 |
| MX-021 通知/收件箱/深链 | M10 | RW-018 + RW-028（后端无通用 inbox → D-06） | decisions.md D-06 |
| MX-022 资源与知识 | M11/M12 | RW-019/RW-020 | tests/resources |
| MX-023 连接生命周期 | M13 | RW-021 | tests/connections |
| MX-024 成果预览/下载/分享 | M14 | RW-022 | tests/artifacts |
| MX-025 原生附件上传 | 附件 | RW-027 | tests/attachments |
| MX-026 执行目标 | M15 | RW-023 | tests/targets |
| MX-027 Paseo 远程治理 | 远程执行 | RW-023 覆盖目标选择；Paseo 服务端治理不在移动端新建（后端边界） | — |
| MX-028 确认式听写 | M16 | RW-024 | tests/voice |
| MX-029 实时语音 | 语音扩展 | RW-024 如实展示"能力不可用"（后端无实时语音端点，不假装） | decisions.md D-07 |
| MX-030 账户偏好与退出 | M17 | RW-025 | tests/profile |
| MX-031 空间用量 | M18 | RW-026 | tests/usage |
| MX-032 能力清单与部署开关 | capabilities | RW-005（能力字段来自后端 capabilities/limits，不默认 true） | tests/contracts |
| MX-033 恢复/并发/故障注入 | 测试 | RW-032 | tests/integration |
| MX-034 双平台 E2E | E2E | RW-032（模拟器环境可用时执行，否则 blocked-env 如实记录） | progress.md |
| MX-035 视觉/无障碍/性能 | 验收 | RW-031 | screenshots/ |
| MX-036 发布门禁与交接 | 交付 | RW-033 | README + progress |

设计包 plans/01~05（S00-S04 阶段计划）与 07-code-agent-handoff 中的"复用 apps/mobile / Happy"路径全部按用户 2026-09-18 指令覆盖为从零实现；产品需求、验收条件、交互语义保留并映射到上表 RW 任务。
