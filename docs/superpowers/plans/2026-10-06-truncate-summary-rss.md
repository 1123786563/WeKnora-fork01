# 截断基线 + 坏摘要 + rss 指纹（#3836 / #3776 / #3823）

日期：2026-10-06 · 分支：fix/truncate-3836-summary-3776-rss-3823（基于 merge/upstream-20261003）· 模式：3 并行实现者 + 验证门

## 背景

- #3836：Notion `QueryDatabaseAll` 撞厂商 10,000 行上限时 `has_more=false`+`request_status.type=incomplete`，但 types.go:267 未解析 request_status、client.go:476 只看 has_more——「截断」与「读完」同形；增量同步把没返回的行当「源端已删」下发 IsDeleted，真删数据且日志成功。
- #3776：非流式摘要 `ParseLLMJsonResponse` 失败走分支①原文当摘要——分支①纯文本兜底是给自定义模板的刻意设计，但默认模板 default_summary 要求严格 JSON；validateSummaryOutput 只拒空白不看 FinishReason，坏 JSON 落库+索引 RAG+标 Completed 永不重试。
- #3823：rss `resolveItem` 只指纹最终 Markdown 正文——feedSignalFingerprint（含 title/link）正确触发解析，但正文 hash 未变即丢弃更新，title/source link 改动永远丢失且新信号已存（后续同步继续跳过）。

三域互不重叠（notion 连接器 / application/service 摘要 / rss 连接器），可并行。

## 任务 N（#3836）：internal/modules/datasource/connector/notion/

1. types.go paginatedResponse 增 request_status 解析（type/incomplete_reason）。
2. client.go 分页循环**每页**反序列化后判 `type=="incomplete"`（厂商要求逐页检查）→ 返回可识别错误（哨兵，如含 query_result_limit_reached 与行数）。
3. connector.go queryDatabaseRecords/FetchIncremental：收到截断哨兵 → 不得基于该轮读取下发任何 IsDeleted（删除判定基线被污染）；整轮报错（含指引：缩小数据源/过滤），对齐既有「失败不确认」语义。
4. 测试：假 query 响应 has_more=false+request_status incomplete → QueryDatabaseAll 返回错误（非静默 100 条）；端到端 cursor 有 r1..r4、本轮只回 r1/r2+incomplete → 报错且产物无 IsDeleted；正常完整读取不回归。

## 任务 S（#3776）：internal/application/service/knowledge_process.go 摘要路径

1. 先摸清模板选择机制（如何区分默认 default_summary 与自定义模板）。
2. 默认模板（严格 JSON 契约）下 ParseLLMJsonResponse 失败 → 不再走原文兜底：返回空/错误走调用方既有空输出处理（可重试、不标 Completed）；自定义模板纯文本兜底行为保留。
3. validateSummaryOutput 增 FinishReason 检查：length（预算截断）→ 按无效输出处理。
4. 测试：坏 JSON（截断半截/裸换行）+默认模板 → 不落库原文、状态可重试；自定义模板纯文本 → 保留；FinishReason=length → 无效；正常 JSON → 不回归。service 包基线 338/90。

## 任务 R（#3823）：internal/modules/datasource/connector/rss/

1. resolveItem 的去重指纹纳入 title 与 link（或对文档 title/link 字段单独比对）——正文 hash 相同但 title/link 变更 → 产出一次更新（1 update），文档标题与 metadata.link 更新，正文/分块/文件 hash 不动。
2. 不变式：完全不变的条目仍 0 update；正文变更行为不回归（既有）。
3. 测试：title-only / link-only 变更 → 各 1 update 且新值落库；unchanged → 0 update；body 变更既有用例不回归。

## 验收

1. 三域定向 go test 绿（notion/rss 前轮已绿须保持；service 基线 338/90）。
2. `go build ./...`；diff 三域互不越界。
3. 不变式：截断不得触发删除；坏 JSON 不得落库标 Completed；指纹变更必须产出更新。

## 交付

按域 3 commit（本地不 push）；回帖草稿欠账（9+3=12 票）下轮统一补。
