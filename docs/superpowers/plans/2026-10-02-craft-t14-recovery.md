# Craft T14（#129 预览隔离 no-egress）恢复与验证轮实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 T14 的 policy-helper 实现从本地 Docker 镜像层恢复回仓库（`deploy/craft/render-boundary/policy-helper/`），经 SHA-256 档案链校验 + Docker 活体验收复跑，给出 T14 verified 判词，解锁 T15（#130）及其下游。

**Architecture:** 事实源=四份权威计划（`docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy{,-fix1,-fix2,-fix3}{,-plan}.md` 族）+ 检查点（`docs/testing/craft/t14/checkpoints/2026-09-29-fix3/`：test 源 after-SHA `9c95492a…`、task.patch）+ 已归档活体证据（`docs/testing/craft/t14/2026-09-29-sockdiag-live-acceptance/`：live-test-run.json **passed**、七判据、helper 镜像 provenance）。恢复源=本地镜像 `craft-t14-policy-helper:2026-09-24`（c0cfa88b4db1）与 `craft-t14-current-reviewed:2026-09-24`。台账判据=craft-107-ledger 对 T14 的收口标准（live acceptance + 独立审查 + OCR 口径）。

**Tech Stack:** Python（helper/tests, unittest）+ Docker（nft/ctnetlink 特权 helper、renderer）+ Go craftegress 面（不动）。

## Global Constraints

- 恢复必须过哈希链：helper.py = `c8d30121927350e3cea5fd3b5823ca230b1f4f550af1d730d29ac7e92fd87c2`（fix1 终态）或以 fix3 链文档核最新；`test_controller_integration.py` = `9c95492af5f2f72c78c1f95f742a82aed5d1e03c3269561be97007586a3c0bf8`。哈希不合=停下报告，不得手改凑数。
- 不改 `internal/modules/craftegress`（Go 面冻结）；owned=新增 `deploy/craft/render-boundary/policy-helper/**` + 证据目录。
- 活体跑须真实 Docker（nft 计数器/回环/预览判据原样），不降断言；清理验证照旧。
- 密钥不入库；不 push、不动 GitHub；提交带 `(T14 #129)`。

---

### Task 1: 镜像提取与源码恢复（subagent）
- [ ] `docker create`/`docker cp` 自 `craft-t14-policy-helper:2026-09-24` 提取 helper 全量（helper.py、Dockerfile/构建上下文若在层内）；自检查点恢复 `test_controller_integration.py`（用 after 文件）；逐文件 sha256 对档案。
- [ ] 落位 `deploy/craft/render-boundary/policy-helper/{helper.py,tests/,Dockerfile(或重建说明)}`；`python3 -m py_compile` + `python3 -m unittest`（离线可跑子集）。
- [ ] 提交 `feat(craft): 恢复 T14 policy-helper 源码（镜像层提取+哈希链校验）(T14 #129)`。

### Task 2: 活体验收复跑（subagent，Docker）
- [ ] 依 sockdiag-live-acceptance 原命令族重建 helper 镜像并跑 `PolicyControllerIntegrationTests.test_target_counters_cover_random_preview_loopback_and_controlled_dns`；判据=档案七项原样。
- [ ] 清理验证（docker ps/label 过滤 + counter canary）照旧；证据新目录 `docs/testing/craft/t14/2026-10-02-live-acceptance-rerun/`（run.json+cleanup.json+SHA256SUMS）。
- [ ] 失败→归因（恢复缺件 vs 环境漂移 vs 产品），如实 BLOCKED 或修复装配后重跑。

### Task 3: 独立审查 + 判词（opus）
- [ ] 审查恢复 diff 与活体证据（哈希链、判据覆盖、无生产面改动）；判词「T14 verified → T15 解锁」或阻塞清单。
- [ ] craft-107-ledger 追加终局段（live acceptance 2026-09-29 passed + 2026-10-02 复跑 + 恢复链）；OCR 按既定口径记录。

## Self-Review
覆盖：ledger 收口三件（live→Task 2、独立审查→Task 3、OCR 口径→Task 3）；恢复哈希链写死；无占位。
