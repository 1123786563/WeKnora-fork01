# r5-verify — Issue #82 第 4 轮真实流程浏览器复验（R-19 清偿）

2026-09-28（本 worktree 执行会话）。全部真实服务：Lago v1.53.0 digest 锁定栈
（compose project `weknora-lago-82r5`，:48889/:48890）、本 worktree 构建 WeKnora
后端（:8093，sqlite `data/issue82-r5verify.db`）、vite 前端（:5194）、支付宝
回环 stub（:8294，本地 RSA 密钥对 `/tmp/issue82-r5-keys`——协议往返+验签证
明，非沙箱证据，AC4 残余披露不变）、Playwright 1.63.0 无头 chromium。

## 种子

`FLOW82_R5_PW`/`FLOW82_EMAIL_PREFIX=settle-r5`/`DB_PATH`/`FLOW82_PAD_COUNT=2`
经 `seed.sh`（Task 16 修复版）：主角 tenant 3（`settle-r5-a@verify.local` 浏览
器）/ tenant 4（发布者），plan `pro` v1（9900 分 CNY）发布并断言落 Lago。
订单 `ord_e6db1733e2b44914`（¥99.00 支付宝，gating invoice
`81a7f48a-1bbe-4c22-8baa-52a7fa16bf7e`）。

## 四腿浏览器断言（AC1/AC2 — R-19 清偿）

| 腿 | 断言 | 结果 |
|---|---|---|
| browser_01_checkout | 支付宝默认选中；wire 断言 purchases 体 `provider=="alipay"`；等待付款+待付款（权益未开通）+报价明细 ¥99.00+订单号锚点；billing 待付款（权益未开放） | 7/7 PASS（rv5-01/02 截图） |
| browser_02_sync_face | `?order=` 回访+显式刷新（waitForResponse 确定性等待）：仍待付款、无已付款/已生效、同一订单身份（A-12） | 5/5 PASS（rv5-03） |
| browser_03_paid_face | 签名回调后：已付款，权益处理中；billing 已付款待激活；无已生效 | 5/5 PASS（rv5-04） |
| browser_04_active_face | settle+webhook 后：billing 已生效（旧文案 detached 确定性等待）；checkout 深链权益已生效；同一订单身份 | 4/4 PASS（rv5-05） |

Lago gated 形状：`lago-gated-before-settle.txt`（incomplete + activation_rules=1）。

## 四对象（AC4 — `lago-four-objects-r5.txt`）

- subscription `weknora-tenant-3-purchase`：**active**
- invoice `81a7f48a`：**finalized + numbered（WEK-48D1-001-001）+ payment_status=succeeded**，
  fee 1320 分（D6' proration 判据 `0 < 1320 ≤ 9900` 成立，差额披露）
- payments：**恰 1 succeeded**（另 1 条 failed 为 3DS 门自动收款失败的历史入账，
  非第二笔成功）
- wallets：`weknora-tenant-3-purchase-2026-09`（R-18 购买批次身份），balance 990
- 本地履约：`commercial_fulfillment_records` 恰 1 条
  `purchase_activation|applied|weknora-tenant-3-purchase-2026-09`

支付宝侧为本地 RSA stub（`ac4-sandbox-credentials-unavailable` 残余披露不变）。

## 重放三面（AC3 — `rv5-06-replay-idempotent.txt`）

1. 重复签名 notify：HTTP 200 `success`，orders/outbox/activation_records 计数不变。
2. 重复 webhook（真实 PI 体+真实 secret 签名）：Lago 接收路由 400（对已结算
   invoice 的正当业务拒绝）；四对象以同投影重读 **STRICT-BYTE-IDENTICAL（4/4）**。
3. 后端 kill+重启：drain 重跑零二次发放（activation_records=1、fulfill 事件=1、
   订单保持 fulfilled）。
4. D11 竞态回归面：paid 窗口内新 quote 提交返回**同一** paid 订单
   （`paid_awaiting_activation`），渠道侧整轮恰 1 次 PRECREATE。

## 判据对照（计划「通过判据」）

- (a) 三态浏览器断言全过 — **PASS**（R-19 清偿）
- (b) 同步面零推进 — **PASS**
- (c) 三个重放面无第二次入账/发放/激活 + D11 竞态面 — **PASS**
- (d) 四对象齐全 + D6' 金额判据 — **PASS**（沙箱残余披露）
- (e) 红线自查 — **PASS**（sk_/whsec_ 字面量、明文口令、force-active 直写、
  manual payments 调用四项全零）
- (f) Go 回归 10 包全 ok + 双架构门 OK — **PASS**
- (g) web 全量 node26：2311 中 2310 PASS + 1 个本票未触碰的 debounce timing
  测试负载 flake（单文件重跑 7/7 PASS）— **PASS（flake 披露）**；typecheck 5 错
  =F23 预存基线不增
- (h) `go vet -tags lago_integration` clean；T9 集成测试实跑尝试：r5 共享栈
  FAIL（webhook 400——测试取 Stripe 列表首个 succeeded PI，在已 finalize 过浏览
  器轮 PI 的共享栈上属重复应用被 Lago 正当拒绝——环境形状缺陷非产品缺陷）；
  专属 t11 栈 lab.env 本地文件缺失无法构造 env — **如实披露，不以 skip 冒充
  pass**；等价覆盖由本轮真实栈四腿+四对象+重放证据承担

## 环境处置记录

- OrbStack 重启自动恢复了历史 `weknora-lago-82flow` 栈（占 48889/48890）：
  `docker compose -p weknora-lago-82flow down`（不带 -v，数据卷保留；该栈证据
  已冻结落盘）。
- 本轮新增栈 `weknora-lago-82r5`；Stripe provider `weknora-stripe` 经 GraphQL
  注册（真实 TEST key 仅 source 注入）。
