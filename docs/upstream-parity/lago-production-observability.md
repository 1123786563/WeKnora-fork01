# Lago 生产部署与可观测门（#103 / Lago 31）——交付记录

> 状态：**生产部署未批准**（沿 #101 AGPL 生产门裁定，
> `lago-agpl-production-gate.md`）。本文档是 #103 的可审计交付物：本地
> 可验证面已实现并实跑，生产侧项以合理默认值记录，待 AGPL 门批准后
> 落地。

## AC② readiness 分类（已交付，本地可验证）

- 栈侧：`deploy/lago/health.py` 的 `classify_service` / `overall_status`
  把依赖故障分为 `ready / degraded / unavailable`（API 探测 + worker /
  db / redis 逐项分类，clock 未知 → degraded，核心依赖未知 →
  unavailable），`deploy/lago/test_health.py` 单测覆盖分类矩阵。
- seam 侧：`internal/commercial/platform.go` 的闭合词表
  `ReadinessState{ready,degraded,unavailable}` + 闭合 reason token
  （`unconfigured|unreachable|invalid_response`），适配器把依赖故障映射
  进 token（不透传 provider 文本）。
- 活体验证：本地固定版本栈 v1.53.0（:48889）`health.py --json` 快照
  `overall=ready`（见轮次报告 evidence）。

## AC③ 指标与告警（本仓库可验证面已实现）

四类信号的算子面（闭合 token、计数+最老年龄）由
`commercialsvc.BillingHealthService.ScanBillingHealth` 提供：

| 信号 | 面 | 来源 |
|---|---|---|
| 队列 lag | `pricing_lag`（settlement 未确认） | #90 结算面，阈值 5/15 分钟随答案下发 |
| Webhook | `webhook_backlog`（收据无 audit = 死信） | #98 inbox/audit 投影 |
| 负余额 | `negative_balance`（可用额度 < 0 的账户） | #86 预算账户投影 |
| 商业超时 | `commercial_timeout`（settlement 停在 unknown） | #99 不确定态 |

Prometheus/Alertmanager 管道与 scrape 挂接属生产部署面：**blocked-env**
（无生产部署目标），落地配方：扫描周期 ≤60s，阈值沿用 #90（lag 5min
pause / 15min alert），webhook_backlog>0 且最老>5min 告警，
negative_balance>0 即告警，commercial_timeout 最老>15min 告警。

## AC① 生产依赖 TLS/备份/容量（blocked-env，合理默认值）

helm/ 仅 WeKnora 主 chart，无 Lago chart；`deploy/lago` 为 loopback-only
compose（无 TLS/备份/容量）。生产需求与默认值：

- **部署形态**：固定版本 Helm chart（镜像 digest 锁定沿用
  `deploy/lago/images.lock.json` 的 verified digests），独立高可用
  PostgreSQL（点对点 TLS + 每日全量 + WAL 归档，RPO≤5min 对齐 #104）、
  Sidekiq Redis 与 cache Redis 分实例、对象存储 S3 兼容
  （`LAGO_USE_AWS_S3=true`）。
- **TLS**：API 入口与依赖间链路全 TLS（server-side env 配置，沿用
  #78 env-only 凭据姿势）；secret 走 secret manager（R-101b 轨道）。
- **容量基线**：#76 lab 负载画像（目标负载下 event→batch p95 6.893s）
  外推：api 2 副本 + worker 2 副本起步，db 4C16G，Redis 1G；扩容以
  p95 与 Sidekiq 队列深度的告警面驱动。
- **AGPL 前置**：上线前须过 #101 法务门（记录在案：未批准）。

## AC④ p95≤60s（lab 证据 + 生产验证 blocked-env）

- 方法学：`deploy/lago-lab/pricing-group/measure.py`
  （`P95_TARGET_SECONDS=60`，nearest-rank p95，`p95_verdict` 判定，
  18 项单测）。
- lab 证据：#76 pricing-group 轮在目标负载下 p95=6.893s（远低于 60s
  目标；证据在 #76 轮 evidence）。
- 生产验证：blocked-env（无生产栈）。落地时用同一 measure 方法学对
  生产栈采样，判定口径 `p95_verdict(p95, 60.0)`。

## 残差

- R-103a：helm/lago chart 未创建（等 AGPL 门批准；配方如上）。
- R-103b：Prometheus/Alertmanager 管道未部署（配方如上）。
- R-103c：生产 p95 未实测（lab 证据外推，方法学已固化）。
