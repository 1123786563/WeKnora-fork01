# 移动工作台增量任务台账

日期：2026-09-12。状态：计划已编写，尚未执行。关联[总计划](2026-09-12-mobile-ai-saas-workbench.md)与[任务索引](2026-09-12-mobile-workbench-task-index.json)。

初始全部pending不是对既有代码的否定；执行时逐项复验。profile区分core/remote/resources/voice/full_happy，不允许用core通过覆盖任务其他部分。原H台账保持原状态，仅在实际验收后同步对应汇总。

| ID | 任务 | 状态 | profile | 实现提交 | RED/GREEN证据 | 数据库/真实后端/原生证据 | 规格/质量审查 | 阻塞与下一步 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| W01 | [统一 DTO、事件和能力合同](2026-09-12-mobile-workbench-01-core.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W02 | [持久 Run 驱动分流与所有权查询](2026-09-12-mobile-workbench-01-core.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W03 | [所有权 facade、快照和标准 SSE](2026-09-12-mobile-workbench-01-core.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W04 | [请求幂等、预算绑定和平台执行入口](2026-09-12-mobile-workbench-01-core.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W05 | [有类型的命令与跨权限域交互](2026-09-12-mobile-workbench-01-core.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W06 | [共享 SDK 与请求对账接口](2026-09-12-mobile-workbench-01-core.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W07 | [产品会话作用域与刷新失效](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W08 | [原生 OIDC 一次性交换](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W09 | [原生流解码、持久投影与 cursor 提交](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W10 | [Happy 会话视图模型和产品导航](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W11 | [工作台列表、Agent 和空间入口](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W12 | [前后台恢复控制器与两条真实链路](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W13 | [设备注册与账号切换撤销](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W14 | [事务事件到通知 Outbox](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W15 | [供应商投递、回执与重试](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W16 | [安全深链与待处理卡](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W17 | [固定上游与能力探针](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W18 | [执行目标、工作目录与当前授权](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W19 | [薄 SDK Bridge 与固定命令协议](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W20 | [分发命令日志与不确定启动恢复](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W21 | [远程事件去重和产品快照](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W22 | [远程取消、审批和工作目录锁](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W23 | [个人节点出站注册与撤销](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W24 | [受控工具、可信用量与预算树接线](2026-09-12-mobile-workbench-04-paseo.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W25 | [会话附件上传、取消和校验](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W26 | [远程文件导入与不可变产物版本](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W27 | [隔离预览与 AWS 组件来源](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W28 | [知识引用与专业结果注册器](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W29 | [按住说话、转写确认与文本提交](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W30 | [语音会话授权、短期令牌和结算](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W31 | [实时语音、打断与后台进度展示](2026-09-12-mobile-workbench-05-resources-voice.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W32 | [高级交互能力端口与保留清单闭合](2026-09-12-mobile-workbench-06-delivery.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W33 | [删除墓碑、远程停止和迟到用量](2026-09-12-mobile-workbench-06-delivery.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W34 | [安全部署、能力开关与可观测性](2026-09-12-mobile-workbench-06-delivery.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W35 | [事件保留、备份恢复与崩溃演练](2026-09-12-mobile-workbench-06-delivery.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W36 | [原生升级、兼容窗口与性能验收](2026-09-12-mobile-workbench-06-delivery.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W37 | [完整验收门禁与分阶段交付报告](2026-09-12-mobile-workbench-06-delivery.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |

## 执行记录要求

每任务追加一段执行记录，字段为：baseline SHA、实际文件清单、scope profile、RED命令/退出码/关键断言、GREEN命令/退出码、数据库方言与skip、真实服务和客户端版本、脱敏证据路径/hash、规格审查结果、质量审查结果、修复提交、剩余阻塞及下一步。没有对应证据不得修改accepted。

只记录授权范围内的测试；不将token、密钥或DSN写入本文件。已有实现无需重复重写，提供重新收集的行为证据后再调整状态。

