# Career R2 核销 + OCR 门禁重试轮实施计划（/loop 第 5 循环）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 依 2026-10-03 Career 族审计（`.superpowers/sdd/2026-10-03-career-audit/report.md`）核销 R2 计划 33 个漂移复选框、裁定 11 条 low OCR 历史发现、重试全范围 OCR 完成门，使 Career 族（除 T33/#172）到达 closure-ready 判词。

**Architecture:** 事实源=审计报告（五项 Review Focus 逐项代码核实+测试全绿）。核销=文档对账（勾选+证据引用），不改生产码。OCR=对 sweep-integration 合并范围的既定 BASE..HEAD 重试（历史三次因 provider 配额中止；今日配额窗口健康），产物=完整报告或如实再记受阻。裁定=11 条 low 发现逐条处置（修复/豁免+理由）。

**Tech Stack:** docs 对账；`ocr review --from <BASE> --to <HEAD>`（完整跑完或如实记录）。

## Global Constraints

- 只改文档/测试侧；生产码零改动（如 OCR 发现 Critical/High/Medium 真缺陷→单独报裁，不在本轮内联修）。
- OCR 完整性口径：selected 项全覆盖且零 LLM 失败才算过门；中断=如实记受阻不降口径。
- 提交带 `(CAREER-R2)`；不 push（轮末统一）；工作区 `.superpowers/sdd/2026-10-03-career-r2/`。

---

### Task 1: R2 复选框核销 + low 发现裁定（subagent）
- [ ] 33 复选框逐个勾销并引证据（审计报告 §R2 五项 + 各 r2-* 合并提交）；11 条 low 发现逐条处置记录。
- [ ] 追加核销段到 R2 计划文档；提交 `docs(career): R2 复选框核销+low 裁定 (CAREER-R2)`。

### Task 2: 全范围 OCR 门禁重试（subagent）
- [ ] 既定范围（审计报告载明的 BASE..HEAD）跑 `ocr review`；完整→归档报告+处置新发现；中断→如实记录（第 N 次受阻）。
- [ ] 提交 OCR 产物/记录。

### Task 3: 族判词（controller）
- [ ] 终局段：OCR 门状态、32 票 closure-ready 清单（#141-#171+#173 按票面 AC 对照审计证据）、#172 阻塞边界、follow-up（CommercialSummary 缺字段/web 并发挂起）；推送。

## Self-Review
覆盖：核销→Task 1；门禁→Task 2；判词→Task 3；无占位。
