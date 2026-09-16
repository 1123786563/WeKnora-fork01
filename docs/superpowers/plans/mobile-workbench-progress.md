# 移动工作台增量任务台账

日期：2026-09-12。状态：计划已编写，尚未执行。关联[总计划](2026-09-12-mobile-ai-saas-workbench.md)与[任务索引](2026-09-12-mobile-workbench-task-index.json)。

初始全部pending不是对既有代码的否定；执行时逐项复验。profile区分core/remote/resources/voice/full_happy，不允许用core通过覆盖任务其他部分。原H台账保持原状态，仅在实际验收后同步对应汇总。

| ID | 任务 | 状态 | profile | 实现提交 | RED/GREEN证据 | 数据库/真实后端/原生证据 | 规格/质量审查 | 阻塞与下一步 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| W01 | [统一 DTO、事件和能力合同](2026-09-12-mobile-workbench-01-core.md) | accepted-with-blockers | core | `7e34e3b0`, `d0267ce8`, `685280dc` | focused contract tests and diff evidence | container/native runtime pending | Spec PASS / Quality APPROVED | accepted evidence recovered; runtime pending |
| W02 | [持久 Run 驱动分流与所有权查询](2026-09-12-mobile-workbench-01-core.md) | accepted-with-blockers | core | `dbce59b5`, `ca1fa8ae`, `e662b660`, `5fc6d652`, `18329c9d` | repository/database/service tests GREEN | PostgreSQL/container blocked | Spec PASS / Quality APPROVED | runtime pending |
| W03 | [所有权 facade、快照和标准 SSE](2026-09-12-mobile-workbench-01-core.md) | accepted-with-blockers | core | `72e93829`, `2be2ff4c` | focused/race/full/vet GREEN | PostgreSQL/container blocked | Spec PASS / Quality APPROVED | concurrency follow-up deferred |
| W04 | [请求幂等、预算绑定和平台执行入口](2026-09-12-mobile-workbench-01-core.md) | accepted-with-blockers | core | `bae97008`, `9aa98bfc`, `0911fcc0` | Gin/idempotency/race/database GREEN | PostgreSQL/container blocked | Spec PASS / Quality APPROVED | runtime pending |
| W05 | [有类型的命令与跨权限域交互](2026-09-12-mobile-workbench-01-core.md) | accepted-with-blockers | core | `420cf056`, `bdeb71fe`, `f335c2c8`, `1d1ca230`, `2a881191`, `ed808ebb`, `b2d616c7`, `375b1039`, `f6104706`, `a085c912`, `2c8ba09f` | focused/race/interaction/vet GREEN | container blocked-dependency | Spec PASS / Quality APPROVED | runtime pending |
| W06 | [共享 SDK 与请求对账接口](2026-09-12-mobile-workbench-01-core.md) | accepted-with-blockers | core | `8cbdba010`, `1edb056f` | focused 7/7, strict check, shared evidence | Go HTTP blocked-env | Spec PASS / Quality APPROVED | W07/W09 unlocked |
| W07 | [产品会话作用域与刷新失效](2026-09-12-mobile-workbench-02-client.md) | accepted-with-blockers | client | `4a9716306`, `0c3f848b`, `32832b62`, `4dd16297` | auth 9/9, scope/teardown 5/5 | Expo/RN blocked-env | Spec PASS / Quality APPROVED | W08/W09/W10 unlocked |
| W08 | [原生 OIDC 一次性交换](2026-09-12-mobile-workbench-02-client.md) | accepted-with-blockers | client | `588584033`, `538b9437`, `b9068cef`, `9fdf45f4`, `c765f700`, `1196b918`, `ff5cb8f2`, `bd624219`, `661a7b7c` | router/diff/gofmt GREEN；focused Go blocked-dependency on W20 remote_dispatch | Seam review Spec PASS / Quality APPROVED; POST `/auth/mobile/exchange` now verified in test router | accepted-with-blockers | W10 may proceed; full Go/native/IdP evidence pending |
| W09 | [原生流解码、持久投影与 cursor 提交](2026-09-12-mobile-workbench-02-client.md) | accepted-with-blockers | client | `c1d12610`, `9f3e976d`, `51963b18` | Vitest 10/10, domain 3/3, TS/diff GREEN | Expo/iOS/Android blocked-env | Spec PASS / Quality APPROVED | full suite baseline failures |
| W10 | [Happy 会话视图模型和产品导航](2026-09-12-mobile-workbench-02-client.md) | blocked-review | client | `528ba94b`, `edc853b2`, `fde9271f`, `bf2d0f73`, `f5996f4f`, `326184a5`, `ff648ef0`, `cbf32d06` | projection/resource/view-model 16/16, product auth 6/6, mobile/shared typecheck, diff GREEN | Final round-5 review still finds no trusted metadata producer, no W09 durable cursor injection, block delta duplication, incomplete terminal/pending projection, and only mocked RN evidence | Spec FAIL / Quality CHANGES_REQUIRED; report `task-W10-rereview-round5.md` | W11/W12/W25 remain locked; continue independent W13/W22 branches |
| W11 | [工作台列表、Agent 和空间入口](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W12 | [前后台恢复控制器与两条真实链路](2026-09-12-mobile-workbench-02-client.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W13 | [设备注册与账号切换撤销](2026-09-12-mobile-workbench-03-notifications.md) | fixing | notifications | `a71f1128a0db57ce7fac153ae00810c5363f0b2b`, `ffd1b2bc`, `d06ffe0d`, `44107dc3`, `31016abe`, `e73f6edd` | focused Go/race/vet/diff single-run GREEN；repeated SQLite race naming collision and mobile deps blocked-env | final targeted race/lifecycle fix in progress | invalidate lifecycle before refresh await、deterministic bind/revoke orders、full lifecycle assertions、real queue tenant/owner tests、unique race DBs | W14 remains locked |
| W14 | [事务事件到通知 Outbox](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W15 | [供应商投递、回执与重试](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W16 | [安全深链与待处理卡](2026-09-12-mobile-workbench-03-notifications.md) | pending | 尚未执行 | 无实现提交 | 未执行 | 未执行 | 未审查 | 核对依赖后开始本任务 |
| W17 | [固定上游与能力探针](2026-09-12-mobile-workbench-04-paseo.md) | accepted-with-blockers | remote | `153299f5`, `5898e49f` | TS/Go tests/typecheck GREEN | live Paseo blocked-env | Spec PASS / Quality APPROVED | lookupByRequest=false |
| W18 | [执行目标、工作目录与当前授权](2026-09-12-mobile-workbench-04-paseo.md) | accepted-with-blockers | remote | `39482851`, `a0d91049`, `4702441d`, `7e04ef5f` | provider/rotation/fixture GREEN | PG/container blocked | Spec PASS / Quality APPROVED | runtime pending |
| W19 | [薄 SDK Bridge 与固定命令协议](2026-09-12-mobile-workbench-04-paseo.md) | accepted-with-blockers | remote | `da9e4236`, `73075ba8`, `f08fe7fd`, `b80882db` | TS/Go/canonical/HMAC tests GREEN | live Paseo blocked-env | Spec PASS / Quality APPROVED | runtime pending |
| W20 | [分发命令日志与不确定启动恢复](2026-09-12-mobile-workbench-04-paseo.md) | accepted-with-blockers | remote | `2beeff6a`, `11d4efe9`, `b7a32db6`, `207cb66d`, `7e08aa38`, `691ee5d0`, `f0d6c120`, `e9b024d3`, `975bf1dc`, `24fa5cb9` | TS 16/16, Go execution, typecheck GREEN | Paseo/PG/container blocked | Spec PASS / Quality APPROVED | W21 unlocked |
| W21 | [远程事件去重和产品快照](2026-09-12-mobile-workbench-04-paseo.md) | accepted-with-blockers | remote | `ccae3d39`, `8cdc744c`, `db01e8dd`, `cc9d0ec5`, `ca9a11b5`, `f9cf8ff1` | real handler/SSE/gap/race/migration GREEN | Paseo TS/old fixture blocked | Spec PASS / Quality APPROVED | W22 may proceed after current revalidation |
| W22 | [远程取消、审批和工作目录锁](2026-09-12-mobile-workbench-04-paseo.md) | accepted-with-blockers | remote | `f2b91e4e`, `08c135bf`, `d0356398`, `261a4143`, `d2e18040`, `3b108979` | worker/workbench race、vet、Paseo 26/26、typecheck、diff GREEN；migration 000055/live Paseo blocked-env | targeted review Spec PASS / Quality APPROVED | canonical workspace_ref recovery and exact durable retry args verified | W23/W24/W32 unlocked subject to remaining DAG dependencies |
| W23 | [个人节点出站注册与撤销](2026-09-12-mobile-workbench-04-paseo.md) | reviewing | remote | `fa2d160c` | Go registration、Paseo 3/3、typecheck、diff GREEN；W20 HTTP/SQLite/PostgreSQL/live node blocked-env | independent review pending | Ed25519 PoP、challenge replay、tenant/owner isolation、revocation/epoch、DI/route/migrations implemented; migration 000129/000049 occupied, implementation used 000136/000058 pending integration decision | W24 may dispatch when reviewer slot frees |
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
