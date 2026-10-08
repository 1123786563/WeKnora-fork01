# Lago 切换、回退窗口关闭与 OpenMeter 移除（#105 / Lago 33）

状态：**代码级切换完成；生产默认切换被 AGPL 门显式阻止（未批准）**。本文是
#105 的交付边界记录，也是 #72 家族终判的切换面依据。

对应规范：`docs/specs/2026-09-20-lago-billing-migration-design.md`（行为矩阵
1–24 项 + Completion gate）；家族台账：`docs/plans/issue-72-execution-ledger.md`。

## 1. 已完成（代码级，本仓库可验证）

- **Lago 是唯一的 CommercialPlatform 适配器**。OpenMeter 从未实现 platform
  seam；platform 家族自 #77 起只有 `lago` 与测试 fake。环境选择
  `WEKNORA_COMMERCIAL_PLATFORM_*`：未设置=合法 blocked-env（调用 fail-closed），
  未知 provider 启动即失败（不静默回退）。
- **OpenMeter 全链路删除**（expand-contract 的 contract 阶段，本轮执行）：
  - `internal/commercial/openmeter/`（official_v3 Gateway 适配器）已删；
  - `deploy/openmeter/`（本地 Kafka/ClickHouse 栈 + 镜像锁 + smoke）已删；
  - 环境变量 `WEKNORA_COMMERCIAL_GATEWAY_FAMILY|URL|API_KEY` 无任何代码读取；
  - container 装配改为 `commercialsvc.NewParkedGateway`（`dig.As(new(domain.CommercialGateway))`）。
- **CommercialGateway 轨道 parked fail-closed**（`service/commercial/gateway_parked.go`）：
  五方法（ApplyBenefit/FindBenefit/RevokeBenefit/Settle/ConfirmSettlement）一律返回
  `domain.ErrGatewayUnconfigured`——与删除前 env 未配置时的运行时姿态逐字节一致
  （`ClassifyFulfillment→FulfillmentUnknown`，挂账 attention，绝不猜重放）。
  该轨道的 Lago usage-event 通道是既有残差 **R-101c**（落地票范围，非本轮）。
- **规范删除判据满足**（spec 矩阵第 24 项：「rollback closure 后，无 runtime
  配置、写路径、worker 或 adapter 可达 OpenMeter」）：grep 全仓，OpenMeter 仅
  存于历史文档/注释与 git 历史。

## 2. 回退窗口：已关闭（关闭前演练与依据）

规范 Out of Scope 明确「Lago 收到真实商业数据后，回退 OpenMeter 不作为数据
恢复策略」；本地 pinned 栈（`deploy/lago`）只有实验室 tenant fixture，无真实
商业数据——正属票面「无真实商业数据时完成回退演练并关闭回退窗口」分支。

关闭前重验（2026-10-05，commit 见本文件同批提交）：

- 回退轨道最后一次全量套件实跑：`go test ./internal/commercial/openmeter/
  -count=1` 全绿（含 grant/lookup 恢复、业务拒绝非成功、unconfigured blocked-env
  四支契约测试）。证据：`.superpowers/sdd/2026-10-05-lago-final/rollback-rail-openmeter-suite.txt`
  （gitignored 报告目录）；此后该包删除，历史证据在 git 历史（本提交的父提交）。
- 回退机制本身=环境选择（platform env unset + gateway env 指向 OpenMeter 栈），
  该机制由两侧适配器各自的 env 构造单测覆盖；随本删除一并退役。

**窗口自本轮起关闭：此后仅前向修复**（forward-fix only）。重新引入 OpenMeter
需要新的获批 spec，不复用本窗口。

## 3. 生产切换：被 AGPL 门阻止（诚实边界）

- `docs/upstream-parity/lago-agpl-production-gate.md` 结论：**生产批准=未批准**
  （残差 R-101a，法务拥有者为仓库所有者）。在人类批准落地前：
  - 不执行生产默认切换（不设置生产 `WEKNORA_COMMERCIAL_PLATFORM_*` 指向 Lago）；
  - 不部署生产 Lago（helm/HA PG/WAL 备份/Prometheus 配方在
    `lago-production-observability.md`，均 AGPL 门后——R-103a/b/c）。
- 批准后的生产步骤（顺序）：secret manager 接线（R-101b）→ helm 部署（#103
  配方）→ WAL 备份与 RPO≤5min 验证（#104 方法已实证，pg_dump 节奏换 WAL）→
  灰度租户接入 → 观察 p95/告警面 → 全量。任何阶段不得重新引入 OpenMeter。

## 4. 行为/契约矩阵 1–24 项切换面处置

逐项的完整验收证据在家族台账各票小节；此处只记切换面归属。

| # | 矩阵项 | 主归属票 | 切换面状态 |
|---|---|---|---|
| 1 | Community/license + AGPL 评审 | #101 | **未批准**（R-101a，人类门禁） |
| 2 | 租户隔离 | #77/#78（横切） | closure-ready |
| 3 | 不可变目录 | #79 | closure-ready |
| 4 | Quote 匹配 | #81 | closure-ready（R-3 裁定族） |
| 5 | 外部付款激活 | #74(done)/#82/#83 | Stripe TEST 实证；支付宝 R-4 边界；微信 R-83a 豁免推定 |
| 6 | 充值到账 | #85 | closure-ready |
| 7 | 付款异常不扩权 | #84 | closure-ready |
| 8 | Credits 顺序 | #75/#86 | closure-ready |
| 9 | 用量身份 | #88 | closure-ready |
| 10 | Settlement Batch | #76/#88/#99 | closure-ready |
| 11 | 延迟与规模 | #103 | 方法+lab 实证；生产 p95 未实测（R-103c，AGPL 门后） |
| 12 | 并发准入 | #87/#89 | closure-ready |
| 13 | 预占生命周期 | #87/#90 | closure-ready |
| 14 | 上界违规暂停 | #90 | closure-ready |
| 15 | BYOK | #91 | closure-ready |
| 16 | 修正不改历史 | #92 | closure-ready |
| 17 | 套餐生命周期 | #93/#94 | closure-ready |
| 18 | 退款 | #95/#96/#97 | closure-ready（微信真环境 R-97a） |
| 19 | Webhook 与对账 | #98 | closure-ready（流式对账 R-98a env-gated） |
| 20 | 故障注入 | #99 | closure-ready（活体 kill 矩阵 R-99a） |
| 21 | 权限 | #100（横切写门） | closure-ready |
| 22 | 隐私与凭据 | #101 | closure-ready |
| 23 | 备份恢复 | #104 | closure-ready（t32 演练 RTO 111.6s） |
| 24 | OpenMeter 移除 | #105 | **本轮执行**（本文件 §1） |

Completion gate 组成：矩阵通过（含具名残差）｜AGPL 评审通过（**否**，R-101a）｜
独立 Spec Compliance 与 code review 无未决 blocker（OCR 终审 low 残差在
backlog）｜回退窗口关闭（本轮）｜OpenMeter runtime 路径移除（本轮）。
**结论：gate 未全绿，生产切换不发生；代码级切换（Lago-only 写面 + OpenMeter
删除 + 窗口关闭）已完成。**

## 5. 已删除资产的去向（历史可考）

- OpenMeter 契约研究/探测证据：git 历史（本提交父提交及更早），spec
  「Prior art」小节有形状级描述，不再有活体引用。
- 本文档与 `issue-72-execution-ledger.md` 是 #105 的唯一权威记录；如需考古
  OpenMeter 时代的运行时行为，以 git 历史为准。
