# Lago 计费迁移 — Superpowers 编排计划（Tickets #73-#105）

日期：2026-09-20
状态：执行中（主 Agent 编排文档）
事实源：`docs/specs/2026-09-20-lago-billing-migration-design.md`（#72）、`docs/adr/0012-lago-as-commercial-billing-authority.md`、GitHub Issues #73-#105

## 环境事实（2026-09-20 23:45 CST）

- 主仓 main = `2d0e3dcb`，被并行会话共享，所有 Lago 工作一律在 `.worktrees/` 下隔离。
- 并行 Codex 会话已在 `codex/lago-t01-community-env`（2b672409）完成 #73 的 plan 文档与 Task 1（health snapshot）；`.codex/worktrees/lago-billing-integration` 为其对齐 main 的空集成分支。本编排**复用**其 #73 产出（Task 1 不重做），后续 ticket 由本编排统一推进；若并行会话继续提交，主 Agent 在 wave 集成时合并去重。
- 集成分支：`lago-integration`（worktree `.worktrees/lago-integration`，基于 main）。
- Docker 29.4.0 + Compose v5.1.2 可用（OrbStack）。

## A. Ticket Dependency DAG（Blocked by 关系，源自 GitHub）

```
73 (Lago01 环境启动)                     ← 无依赖（根）
├─→ 74 (Lago02 付款激活实验)
│    ├─→ 82 (Lago10 支付宝激活) ─┬─→ 83 (Lago11 微信付款) ─┬─→ 84 (Lago12 异常付款)
│    │                          │                         └─→ 85 (Lago13 购Credits)
│    └─→ 81 (Lago09 Quote→Invoice)
├─→ 75 (Lago03 Wallet 语义)
│    ├─→ 80 (Lago08 Base Plan)      [还需 78,79]
│    ├─→ 85 (Lago13)                [还需 82,83]
│    └─→ 86 (Lago14 到期顺序消费)   [还需 80,85]
├─→ 76 (Lago04 Pricing Group 批次)
│    ├─→ 87 (Lago15 原子预占)       [还需 80,86]
│    ├─→ 88 (Lago16 Settlement 核对) [还需 87]
│    └─→ 103 (Lago31 生产部署)      [还需 98,99,101]
└─→ 77 (Lago05 Commercial seam)
     ├─→ 78 (Lago06 Lago Customer)
     └─→ 79 (Lago07 Plan Version 发布)

80 (Lago08) ─┬─→ 86 ─┬─→ 87 ─→ 88 ─┬─→ 90 (Lago18 暂停) ─┐
             │       ├─→ 93 (Lago21 升级)                ├─→ 99 (Lago27 故障恢复)
             │       │    ├─→ 94 (Lago22 降级/年付)       │        [还需 92]
             │       │    └─→ 96 (Lago24 套餐退款)       │
             │       │         [还需 92]                 │
             │       └─→ 95 (Lago23 充值退款) ─┬─→ 97 (Lago25 微信退款) [还需 83,96]
             │                                 │
             └─→ 94 ─┬─→ 102 (Lago30 注销)    ├─→ 98 (Lago26 Webhook/对账)
                     │       [还需 96,98,101]  │      [还需 84,85,93]
                     └─→ 100 (Lago28 Billing Center)
88 ─┬─→ 90 ──┬─→ 99 ─┬─→ 100 [还需 91,94,97,98]
    ├─→ 91 (Lago19 BYOK) ──┤   ├─→ 101 (Lago29 数据最小化/AGPL) [还需 97]
    └─→ 92 (Lago20 用量修正)┤   ├─→ 102
         ├─→ 96            │   └─→ 103
         └─→ 99            └─→ 105 (Lago33 切换/移除 OpenMeter)

105 (Lago33) blockers: 86, 89, 91, 92, 94, 97, 100, 102, 104
89 (Lago17 并发预算) ← 87
104 (Lago32 备份恢复) ← 102, 103
```

## B. Ready Frontier（当前时刻）

- **W1 frontier = { #73 }**（唯一无 blocker 的 ticket）。
- #73 状态：plan + Task 1 已由并行会话完成（2b672409）；剩余 Task 2（pinned compose 环境）、Task 3（真实契约探测）、Task 4（文档+运行证据）。
- #73 完成后 frontier 变为 { #74, #75, #76, #77 }（各只被 #73 阻塞，彼此独立）。

## C. Parallel Waves（完整划分，≤4/wave）

| Wave | Tickets | 说明 |
|---|---|---|
| W1 | #73 | 环境基线；**进行中** |
| W2 | #74, #75, #76, #77 | 3 实验 + 1 seam（满 4） |
| W3 | #78, #79 | Customer / Plan Version（均依赖 seam） |
| W4 | #80, #81 | Base Plan 初始化 / Quote→Invoice |
| W5 | #82 | 支付宝激活（唯一入口，串行） |
| W6 | #83 | 微信付款 |
| W7 | #84, #85 | 异常付款 / 购买 Credits |
| W8 | #86 | 到期顺序消费 |
| W9 | #87, #93, #95 | 预占 / 升级 / 充值退款 |
| W10 | #88, #89, #94 | Settlement / 并发预算 / 降级年付 |
| W11 | #90, #91, #92 | 暂停 / BYOK / 用量修正 |
| W12 | #96, #99 | 套餐退款 / 故障恢复 |
| W13 | #97, #98 | 微信退款 / Webhook 对账 |
| W14 | #100, #101 | Billing Center / 数据最小化+AGPL |
| W15 | #102, #103 | 注销 / 生产部署 |
| W16 | #104 | 备份恢复 |
| W17 | #105 | 切换 Lago、移除 OpenMeter（收官） |

关键路径：73→77→78/79→80/81→82→83→85→86→95→96→97→98→101→102→104→105（17 wave）。

## D. Branch / Worktree 命名约定

- 集成分支：`lago-integration`（`.worktrees/lago-integration`）。
- 每 ticket：branch `lago-<issue>-<slug>`，worktree `.worktrees/lago-<issue>`，基于上一 wave 的集成分支头。
- 当前 Wave：
  - #73 → branch `lago-73-community-env`，worktree `.worktrees/lago-73`（基于 2b672409，含复用的 plan+Task1）
- Wave 2（预定）：
  - #74 → `lago-74-payment-activation-lab`，`.worktrees/lago-74`
  - #75 → `lago-75-wallet-semantics-lab`，`.worktrees/lago-75`
  - #76 → `lago-76-pricing-group-lab`，`.worktrees/lago-76`
  - #77 → `lago-77-commercial-seam`，`.worktrees/lago-77`
- SDD ledger：`docs/plans/ledgers/lago-<issue>.md`（各 branch 内），证据 `docs/migrations/lago/<slug>/`。

## E. 潜在冲突登记（frontier 检查）

W1（#73 单 ticket）：无并行冲突。与并行 Codex 会话同 branch 风险已通过"接管其提交、后续由本编排独占推进"化解；集成时去重。

W2（4 ticket 并行）：
1. **共享 Lago 实验实例**（#74/#75/#76 都要起真实 Lago）：#73 的 plan 已规定 `COMPOSE_PROJECT_NAME`/`LAGO_API_PORT`/`LAGO_FRONT_PORT` 可按 worktree 覆盖 —— 每 ticket 独立 project + 独立端口（73 用 48889/48890；74→48891/48892；75→48893/48894；76→48895/48896；77→48897/48898）。不可复用 OpenMeter 的 db/redis/volumes。
2. **文件重叠**：#74/#75/#76 产出各自隔离目录（`deploy/lago-lab/<slug>/` + `docs/migrations/lago/<slug>/`），禁止在 W2 内抽公共 helper（共享抽象留给 #77 seam / 后续 ticket）。#77 只动产品侧新模块（`internal/infrastructure/commercialplatform/` 新目录 + handler 路由注册 + fake/lago adapter），与实验 ticket 零重叠。
3. **schema 冲突**：W2 内 #77 如需本地投影表 migration，由主 Agent 分配编号段（#77=000060 起），实验 ticket 不写 migration。
4. **API/interface dependency**：#77 的 provider-neutral 接口是 W3+ 全部产品 ticket 的依赖，#77 plan 必须冻结接口文件路径与签名，review 时重点核对。
5. **shared mutable state**：GitHub issue 状态只由主 Agent 更新；`docs/plans/` 文件名带 issue 号避免互撞。

W4+ 渐进风险（提前登记）：#80/#81 都会触及 `internal/commercial/`（lifecycle/fulfillment/quote/order）与 handler 路由 —— W3 集成后主 Agent 在派发 W4 plan 时明确文件边界；migration 编号由主 Agent 逐 wave 分配（历史上发生过三 lane 撞号 000058）。

## F. 建议执行顺序

1. **W1（现在）**：完成 #73 Task 2-4（pinned compose + 契约探测 + 真实运行证据）→ 集成到 `lago-integration` → 全量 `go build ./...` + 相关测试 → 关闭 #73 → frontier 重算。
2. **W2**：4 ticket 并行（3 实验 + 1 seam），每 ticket 独立 branch/worktree/plan/ledger；#77 接口冻结为 W3 前置。
3. 依 wave 表推进；每 wave：集成 → `go build ./... && go test ./...`（Go 侧）+ python unittest（deploy 侧）+ lint → GitHub 状态更新 → frontier 重算。
4. W17 收官后：requesting-code-review → verification-before-completion → whole-branch final review。
5. 并行会话产出处理：每 wave 集成前 `git log` 扫描 `codex/*` 分支新提交，有则 cherry-pick/合并去重，避免重做。

## 主 Agent / Worker 职责（本编排）

- 主 Agent：编排、依赖跟踪、集成 merge、冲突解决、wave 级测试、GitHub 状态、migration 编号分配。
- Worker（子 Agent）：单 ticket 的 TDD 实现、自己 branch 的 commit、自己的 ledger；最小上下文（指向 CONTEXT.md/spec/ADR/ticket/plan，不复制主会话）。
- 禁止：多写者共享 worktree；worker 直接操作 `lago-integration`。

- **2026-09-21 Wave 2 完成**：四票并行（planning→SDD implement→review→集成）。
  - #75 关闭：verdict = a1/b/c/d PASS-with-coordination；**a2 十二个月批次到期 BLOCKED**（Lago 钱包级到期 + 每客户 6 活跃钱包硬上限 422 实测）→ 升级设计决策：(1) 产品上限≤5 并发充值批次 (2) 批次到期权威移入协调层+修订 ADR-0012。
  - #76 关闭：4/4 AC 真实栈证据（64/64 精确核对、422 幂等、acceptance≠rating 负对照、p95 6.9s）；4 处实测契约修正移交 #87/#88。
  - #77 关闭：seam 三方法接口冻结（byte-level 零漂移）、readiness 切片上线、ADR-0014、旧 OpenMeter gateway 零改动全绿；含 #73 遗留 probe 修复。
  - #74 保持 open：AC4 决策文档交付（manual payment Community 403 Premium-gated 源码级证据）；AC1-3 运行时证据 blocked-env 待 Stripe TEST key（解锁命令在 issue 评论）→ 升级决策需求 2。
  - Wave 测试：go test ./... 零 FAIL + 4 个 python 套件全绿。
  - 事件：GitHub push protection 拦截 #74 测试里 secret 形状 canary（sk_test_51Canary.../rk_live_51BadBadBad...）→ 已 defuse（拼接字面量）+ #74 squash 并入（d1eec16b）后推送成功；lago-74 分支保留完整本地历史。
- **W3 启动**：#78（Lago Customer 映射）/ #79（Plan Version 发布）planning 中，基于 6a651c30（#77 seam 冻结接口）。
