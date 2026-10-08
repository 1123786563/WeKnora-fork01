# 同步完整性三票 + FAQ 导出（#3778 / #3692 / #3690 / #3774）

日期：2026-10-06 · 分支：fix/sync-integrity-3778-3692-3690-3774（基于 merge/upstream-20261003）· 模式：lead 直改 #3774 + 2 并行子代理 + 验证门

## 背景

数据源增量同步的「失败被游标确认」族 + Notion 截断可见性 + FAQ 导出 id 丢失：

- #3778：Notion 单页 >1000 block 静默截断（client.go:274/:311 cap 提前 break，无告警；飞书同款 cap 有 Warn 先例）。参考 PR #3771：抽 blocksTruncated() + 只在确有下一页时 Warn；cap 数值与递归语义不改。
- #3692：Notion FetchIncremental 先记 last_edited_time 再取 blocks；GetBlockChildrenAll 失败只 Warn 返回空 items → 换游标成功返回，失败页被确认、恢复后永不再取。修法：页级取块失败→返回错误（游标不推进），任务层重试。
- #3690：Yuque 同形——listing 成功但某文档 detail 503 全重试失败 → 游标仍确认该文档新版本。修法同 #3692 语义。
- #3774（lead 已完成）：chunk_repo.go ListAllFAQChunksForExport 投影补 seq_id（一行）+ ingest 包投影回归测试（导出 SeqID 非零且与库一致）。

## 任务 N（#3778 + #3692）：internal/datasource/connector/moauth/connector/notion/

1. #3778：blocksTruncated() helper + Warn（措辞对齐飞书 blocks.go:149-151，仅 hasMore 时告警防恰好等于上限误报）；cap 与递归语义不动。
2. #3692：FetchIncremental/fetchPage 路径——页 blocks 获取失败（含分页读与 child_page 遍历）不得静默吞掉：失败即返回错误使任务失败重试，游标不确认未读内容；一页成功一页失败时整轮报错（票面验证语义：失败轮保留旧内容旧游标，恢复后任务重试两页都更新）。数据库记录聚合/附件路径票外不动。
3. 测试：a) >1000 block 页 → 截断 Warn 且行为同现状；恰好 1000 无下一页 → 不告警；b) 两页一败 → 返回错误、游标未推进（或按实现形态等价断言）；恢复后重试两页均更新。

## 任务 Y（#3690）：internal/datasource/connector/moauth/connector/yuque/

1. 文档 detail 获取失败（重试耗尽）→ 同语义：返回错误，游标不确认失败文档的新版本；成功文档内容照常落（以实现形态决定整轮失败 or 部分成功+整体报错，对齐 #3692 选择并报告）。
2. 测试：两文档一败（503 全重试）→ 错误且游标不推进；恢复后增量重取失败文档；全成功不回归。

## 验收

1. 两 connector 包定向 go test 绿（有预存失败先记基线比对）；ingest/faq 包绿（#3774）。
2. `go build ./...`；diff 域不越界。
3. 语义不变式：任何源端读取失败不得被游标确认；截断必须可见。

## 交付

按域 3 commit（#3774 一个、notion 一个、yuque 一个；本地不 push）；回帖草稿下轮统一补。
