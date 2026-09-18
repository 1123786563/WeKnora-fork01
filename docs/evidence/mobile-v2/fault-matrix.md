# Fault Matrix · 恢复、并发与安全故障注入（MX-033）

全部用真实实现驱动（submission 协调器 / execution-storage / interaction 协调器 / product-scope / connection-controller）；无 mock 通过、无 skip 记通过。套件：`tests/mobile-v2/recovery.test.ts`（3 例）+ `tests/mobile-v2/security.test.ts`（4 例）；frozen 链：`tests/mobile-v2/mx-033.test.ts`（驱动 Go admission 恢复套件 + 两套件）。

## 恢复故障注入

| 故障 | 断言（真实观测） | 结果 |
|---|---|---|
| ACK 丢失 + 进程重启 | startCount=1（零重发）；lookup 对账→同一 run（bound） | PASS |
| 事件乱序（3,1,2） | 缺口拒绝（gap expected）→ 按 1,2 顺序落库；迟到 3 补齐 [1,2,3] | PASS |
| 重复事件（seq2 两次） | 幂等——persisted=[1,2]（无重复） | PASS |
| 游标过期（cursor_expired） | replaceRun 原子重建→cursor=3、[1,2,3] | PASS |
| DB 写失败/磁盘满 | （MX-012 frozen 已钉：cursor 不推进——本矩阵引用既有证据，不重复注入） | 引用 |
| 服务端崩溃恢复（admission） | Go TestAdmission 套件（pending→dispatching→admitted CAS；resume 幂等） | PASS |

## 并发/安全故障注入

| 故障 | 断言 | 结果 |
|---|---|---|
| 两设备并发批准 | decideCalls=1；第二个 already_decided 幂等（不重发）——服务端 CAS 1 成功 1 冲突在 MX-005 frozen | PASS |
| 撤权/换空间后迟到批准 | generation 失效→action=refresh、decideCalls=0（零发送） | PASS |
| 换空间迟到响应 | scope.accept(captured)=false（迟到结果丢弃） | PASS |
| 连接撤销后迟到派发 | performed=false（revoked）；providerWrite=0；rawSecretFields=[] | PASS |

## frozen 场景（side-effect-succeeded × crash-before-local-settlement）

服务端副作用恰好一次（providerWriteCount=1：服务端 CAS+幂等 lookup——Go admission 套件）+ 本地结算恰好一次（settlementCount=1：协调器 bound→重启对账同 run；服务端 settlement 事实驱动）。

## 墓碑与迟到用量

- 通知/交互已读=幂等标记（不执行通知描述操作——MX-021）；删除墓碑保留清理语义（invalidate 清可见数据不清服务端事实——MX-014）。
- 迟到用量：voice 会话结束后仍入账（MX-029 ReportUsage）；结算记录不因清理移除。

## blocked 项（不以 mock 记通过）

- remote（Paseo live）与 voice（真实媒体供应商）环境缺失：MX-032 profile-gates 保持 unavailable；本矩阵不含其环境注入。
- PostgreSQL 并发（TRPC_TEST_POSTGRES_DSN 未设置）：Go 并发证据在 SQLite；PG 复验归 MX-036 发布门禁前补。
