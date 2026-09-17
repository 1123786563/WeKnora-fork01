# Code Agent 开发交接提示词

以下用于未来获准实施，不表示本轮已执行。

你负责将WeKnora现有Expo移动端按本交付详细设计产品化。先读取仓库AGENTS、当前HEAD与未提交改动，再阅读本交付README、docs/01–04、plans/00总计划、task-index.json及当前任务分册。源代码事实优先当前checkout；原架构要求与本交付增量提案冲突时记录裁决，不隐式变更产品权威。

以MX-001建立当前实现/接口/证据矩阵，再计算MX直接依赖与激活profile条件闭包；原W/T/H要求须读取全文并展开到切片级。maps_w只用于追溯，不制造W36阻塞早期依赖校准的循环。已有正确实现保留并复验；不要把全部pending理解为全部需要重写。

工作区必须隔离，保护用户和并行任务改动。每项任务拥有精确Files、module_key、file_lock_keys、测试设备/schema/端口租约；写集合重叠则排队。锁文件、根路由、DI、迁移序号、令牌源与台账单写。默认最多2个实现者及1个独立审查者；实际工具不支持则串行，不声称做过独立agent审查。

每任务先建立真实测试接缝，确认行为RED，分小步实现、实际接到repository/service/handler/SDK/Host/Screen，GREEN后独立规格与质量审查。分册probe是观测适配器，不得直接返回expected常量；缺环境记blocked-env。原型状态机、HTML截图和node测试不能替代原生App或Go后端证据。

原生UI消费tokens/native-tokens.ts映射到现有主题系统；不要复制HTML全局状态机、DOM、sessionStorage或模拟计费。保留双主题、Safe Area、动态文字、键盘与返回语义。按钮必须根据真实能力和revision决定，未知请求先lookup，批准不代表外部成功，取消不代表停止或退款。

只在任务本地授权范围内修改代码和测试。Provider写入、真实支付、发消息、公开部署、push/merge需要当轮明确授权。结束时给出BASE/HEAD/integrated SHA、逐任务状态、命令/环境/退出码、真实证据路径、未验收profile与准确阻塞。不要将设计完成写成工程accepted。
