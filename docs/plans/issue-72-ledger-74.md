# Ledger — [Lago 02] 外部付款激活 payment-gated Subscription（#74）运行时证据补齐

## 计划身份

- Ticket: #74（https://github.com/1123786563/WeKnora-fork01/issues/74，OPEN → 本计划目标：AC1–AC3 运行时证据 + AC4 运行时 403 补齐）
- Plan: `docs/plans/issue-72-plan-74.md`（2026-09-23 编写，含自检）
- Worktree: `.worktrees/issue72-n74`；Branch: `codex/issue-72-lago-74`
- 集成基线: `dafba8851`（`issue-72: issues inventory and dag`；计划编写时 worktree HEAD 即此提交，工作区干净）
- Spec/ADR: `docs/specs/2026-09-20-lago-billing-migration-design.md`、`docs/adr/0012-lago-as-commercial-billing-authority.md`
- 前置: #73 已 CLOSED（pinned Lago Community v1.53.0 compose 资产，本票只读复用）

## 计划编写时实测快照（2026-09-23，本 worktree）

| 检查 | 结果 |
|---|---|
| 离线回归 `python3 -m pytest test_lab.py test_phases.py -q` | `60 passed in 37.20s`（实跑） |
| `evidence/` 现状 | 7 阶段 `blocked-env`（缺 Stripe TEST key）；setup/cleanup `pass`；`secrets_scan.clean: true` |
| `~/.zcode/issue72-stripe.env` | 存在；仅 `STRIPE_SECRET_KEY`；前缀 `sk_test_`（未读取值） |
| 端口 48891/48892 | 空闲（lsof 实测） |
| `weknora-lago-74` 容器/卷 | 无运行容器；**残留 3 个旧卷**（postgres/redis/storage data，2026-09-20 运行遗留，旧 lab.env 已随旧 worktree 删除）→ 计划 Task 1 强制清卷 |
| #73 主栈 `weknora-lago`（48889/48890） | 全容器 healthy，运行 9h+；只读不动 |
| 旁支文件 | `clients.worktree-variant.py` 为被跟踪历史旁支（5e0c958e6 引入）；runner 实际导入 `clients.py`；本票不动，已列上报事项 |

## 计划自检结论（编写者）

- 占位符：无 TBD/TODO；证据依赖值均以命令从 JSON 注入；唯一显式槽位在 fail 分支模板并附取值命令。
- 接口一致性：`phase_*(ctx)`、`RunContext.__init__`、`run_lab.py` CLI/退出码（0/1/2）、`lab.sh` 子命令、`t02-environment.json` 字段均对照 `phases.py`/`run_lab.py`/`lab.sh` 源码行核实。
- 追踪矩阵：4 条 AC + 环境可信 + 回归共 6 行，每行落到 Task/Step 与具体断言命令（见计划末节）。
- 本票零生产代码改动（Go/TS/Python lab 代码均只读）；产出 = 运行时证据 + docs 晋升 + DECISION 更新。

## 执行清单（执行者填写）

| Task | 内容 | 状态 | Commit |
|---|---|---|---|
| 0 | 计划 + Ledger 提交 | 完成 | （本提交 `issue-72(#74): plan`） |
| 1 | 预检 + 清卷 + init/up + 健康快照 | 待执行 | |
| 2 | Stripe TEST key 真实运行 + DB 观察者 + AC 断言 | 待执行 | |
| 3 | 证据晋升 + README/DECISION 更新 | 待执行 | |
| 4 | 密钥双扫描 + 回归 + down + 兄弟栈复核 | 待执行 | |

## AC → 证明（执行后填写）

| AC | 证据 | 结论 |
|---|---|---|
| AC1 incomplete/Entitlement 不可用 | `t02-gating.json` | 待执行 |
| AC2 active 恰一次 + 重复拒绝 | `t02-activation.json` + `t02-duplicates.json` + `runs/db-watch-*.tsv` | 待执行 |
| AC3 同一商业身份可恢复 | `t02-retries.json` | 待执行 |
| AC4 manual 替代路径/blocker | `t02-manual.json`（运行时 403）+ `t02-decline.json` + `DECISION.md` | 待执行 |

## 上报事项（主 Agent 处理）

1. #74 证据完成后的关票决策；#81/#82 解锁（主链恢复）。
2. DECISION.md §5 选项 (a) Premium manual vs (b) provider 轨道需 spec/ADR owner 确认（本票刷新 (b) 的运行时证据，(a) 仍源码级）。
3. `clients.worktree-variant.py` 建议删除或归档（防误导）。
4. 本机复跑依赖 `~/.zcode/issue72-stripe.env`（本机测试凭据，不入库）；CI/他人复跑自备 TEST key。
