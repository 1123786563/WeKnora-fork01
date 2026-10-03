# 测试卫生债清偿轮实施计划（/loop 第 4 循环）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清偿 `internal/application/service` 包全量跑的 32 个顺序依赖失败（隔离 `-run` 即过；失败模式如 `agent runtime conflict`——包内全局注册表/单例跨测试串扰），以及 handler/session 同型（compliance/marketplace 族），使全包单命令全绿。

**Architecture:** 定责先例：WB-GRAPH 轮 worktree 基线对照确认 32=32 确定性前在、隔离即过（.superpowers/sdd/2026-10-03-wb-graph/task-4-report.md 残留节）。修法取向=测试卫生（注册表/单例的 per-test 重置或惰性隔离、t.Cleanup），**不改生产语义**；若发现真产品级全局状态缺陷则单独报裁。

**Tech Stack:** Go testing（t.Cleanup/并行安全）；`go test ./pkg/ -count=1` 全包门。

## Global Constraints

- 只改测试侧卫生（注册/重置/隔离）；生产码改动需证明为真缺陷并单独提交说明。
- 修后门：`go test ./internal/application/service/ -count=1` 全绿；handler/session 同门；`go build ./...`。
- 不许删测试/跳过测试来「修」；每处修法登记根因。
- 提交带 `(TESTHYG)`；不 push（轮末统一）；工作区 `.superpowers/sdd/2026-10-03-testhyg/`。

---

### Task 1: 根因定位（subagent）
- [ ] 最小复现对（单跑过/同包序跑挂）锁定泄漏全局（嫌疑：agent runtime 注册表、模型/能力注册单例、env 缓存）；产出串扰清单+每处最小修法 → task-1-report.md。

### Task 2: 卫生修复（条件于 Task 1）
- [ ] 逐处修（per-test reset / 隔离构造 / t.Cleanup），全包门到绿；提交。

### Task 3: 判词
- [ ] 复跑三包全量 + 抽查 diff；终局段写入本计划。

## Self-Review
覆盖：定位→Task 1；修复→Task 2（条件任务契约明确）；判词→Task 3。

---

## 终判（2026-10-03）

- **定性修正**：32 例非顺序串扰，而是 09-25~10-01 生产收紧后的**确定性 fixture 漂移**（9 簇：actor 门播种/Craft 准入/直插 pin/租户 guard/control 授权/快照围栏/假库接口/budget 库/compliance UNIQUE）+ handler/session 1 簇。
- **治愈**：service 32/32（全包 `-count=1` ok 288s）+ handler/session 2/2（ok 35s）；8×`test(hygiene)` 提交。
- **安全级真缺陷（本轮最重要发现）**：坏合并 215eba788 丢失 adoption 安全门 3 处调用（6f8f091d4 布置、09-29 侧分支删除、10-01 合并只复活 setter 未复活调用）——**撤销的 release 此前仍可建 Variant/本地发布**。证据链（容器仍装门/setter 文档/契约测试三重）证明非刻意；修复 `eec97936f` 按 ed6968dd8 原样恢复，controller 亲核 diff + 焦点测试复绿后推送。Adopt 路径恰被 09-29 新增 repo 层门兜底，实际暴露面=CreateVariant/PublishVariant。
- main 推至 eec97936f。
