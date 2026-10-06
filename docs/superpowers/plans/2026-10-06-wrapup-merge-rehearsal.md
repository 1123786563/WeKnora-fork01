# 收尾轮：合并预演 + 回帖草稿（12 分支 / 37 票）

日期：2026-10-06 · 模式：lead 直做合并预演 + 1 子代理编纂草稿 · 无代码改动

## 背景

十二轮修复共产出 12 个 fix 分支（均基于 merge/upstream-20261003）、34 commits、37 票。全部本地未 push、issue 未回帖，等用户裁决。本轮产出两件裁决材料：

## 任务 A（lead 直做）：合并预演

1. 临时 worktree（/tmp/wk-merge-rehearsal）自 merge/upstream-20261003 建 integration 分支。
2. 按时间序逐个 `git merge --no-edit <branch>`，记录每个分支：干净 / 冲突文件 / 已被前序分支覆盖的内容。
3. 完成后删除 worktree（不留任何共享状态）。
4. 产出：合并顺序推荐 + 冲突清单（如有）。

分支清单（12）：
fix/copyindices-3868-3871 · fix/datasource-pagination-3838-3839-3877 · fix/kg-3873-3945-3953 · fix/agent-param-3934-3935-3946 · fix/reparse-3851-qa-usage-3865 · fix/misc-3974-3947-3962 · fix/misc2-3951-3940-3928 · fix/fake-success-3880-3872-3854 · fix/misc3-3911-3918-3922-3926 · fix/anydoc-ext-3932-sandbox-3942 · fix/misc4-3941-3837-3878 · fix/confluence-3856-embed-3898-docx-3849

## 任务 B（子代理）：回帖草稿

1. 读各分支 `git log <base>..<branch>` 的 commit 信息 + issue 正文（gh issue view），为 37 票各写一段回帖草稿：根因一句话 + 修法要点 + 测试 + commit hash + 分支名。
2. 语言跟随 issue 语言（中文票中文回、英文票英文回）；注明「分支尚未合回上游，欢迎验证」。
3. 待裁决项（#3895、anydoc 光栅化、清扫方向 A、#3867 等）单列一节「需产品/所有者决策」。
4. 产出：`docs/superpowers/plans/2026-10-06-issue-reply-drafts.md`（草稿，不发包）。

## 验收

1. 预演无残留（worktree 删除、当前分支不动）。
2. 草稿覆盖全部 37 票 + 待裁决清单。
3. 不发生任何 push / issue 评论 / commit 到 fix 分支。

## 交付

两份材料供用户裁决：合并预演结论（会话内汇报）+ 回帖草稿文档。
