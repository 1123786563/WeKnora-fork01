# wiki 写保护 + RRF 排名（#3792 / #3714 / #3796）+ 草稿补账

日期：2026-10-06 · 分支：fix/wiki-guard-3792-3714-rrf-3796（基于 merge/upstream-20261003）· 模式：2 实现代理 + 1 草稿代理 + lead 预演

## 背景

- #3792：UpdatePage 是 wiki 唯一写出口，无内容丢弃保护；六条机器写入路径中 #3617 守卫只覆盖 ingest 一条——agent 整页重写丢行照样落库。期望：出口处对非人类写入做「旧行身份集合丢失」判定并拒绝（user/revert 豁免）。
- #3714：wiki ingest 把源文档查找的任何错误当「已删除」——isKnowledgeGone 丢弃 ErrKnowledgeNotFound 与其他错误的区分，map/reduce 双路径把可重试的 pending 操作无声裁掉。
- #3796：fuseWithRRF 用切片位置当排名，而并发检索的切片顺序=goroutine 完成序——FAQ 块占据 rank 1..N，全局最优文档 chunk 排名数百，MatchCount 截断后永远进不了 reranker。
- 草稿欠账 12 票（round14-16 三分支）；合并预演需扩到 15 分支。

## 任务 W（#3792 + #3714）：wiki 域（一代理，文件域内部自洽）

1. 先 `gh issue view 3792` 读全豁免规则（user/revert 有意缩短豁免；判据=每行第一个非空单元格身份集合，行还在只是格式化/重排照常写）。
2. UpdatePage 增写入方身份（writer kind：user/agent/pipeline/revert 之类，最小枚举），六条调用方接线各自真实身份。
3. 出口守卫：非人类写入 && 旧内容有表格行 && 新内容丢旧行（身份集合比较）→ 拒绝写入保留原页，日志含 slug/行数变化/内容长度。
4. #3714：isKnowledgeGone 区分 ErrKnowledgeNotFound（真删）与其他错误（取消/超时/DB 错→返回失败）：mapOneDocument 进 failedOps 保住 pending；filterLiveUpdates 失败时不 drop additions 走既有重试。
5. 测试：agent 重写丢行被拒；user 删行照常；revert 照常；行格式化/重排照常；查找错误→failedOps 保留；真删→照常跳过。

## 任务 R（#3796）：RRF 排名修正

1. fuseWithRRF 各检索器切片先按 score 降序稳定排序再赋 rank（顺序不再依赖 goroutine 完成序）；RRF 公式与权重不变。
2. 测试：FAQ 切片先完成但含低分、文档切片高分 → 融合后文档 chunk 排名合理不被 MatchCount 截断埋掉；单 KB 既有行为不回归。

## 任务 D（草稿补账）

1. 为 round14-16 三分支 12 票补草帖（规则同前：根因+修法+测试+分支/commit+语言随票），追加进 2026-10-06-issue-reply-drafts.md。

## lead 直做：15 分支合并预演（时间序逐 merge，记冲突）

## 验收

1. wiki 域定向 go test 绿（wiki 包 4 条预存失败基线）；retrieval/fusion 定向绿。
2. `go build ./...`；草稿 12 票齐；预演结论更新。
3. 不变式：非人类写入丢行必拒；查找错误≠已删除；融合排名来自 score。

## 交付

按域 2 commit（本地不 push）；草稿文档更新。
