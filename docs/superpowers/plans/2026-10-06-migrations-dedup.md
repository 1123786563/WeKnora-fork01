# 迁移去重 + 预存失败归类（round22）

日期：2026-10-06 · 分支：fix/migrations-dedup-preexisting（基于 merge/upstream-20261003）· 模式：2 并行代理 + 验证门

## 背景

- 迁移撞号（合并伤亡·结构形态）：migrations/sqlite/ 内部 10 对重复序号（000014-21、000030-33），migrations/versioned/ 12 对（000093-99、000110-114）——fork 自有迁移与上游迁移同号并存，全新部署的 golang-migrate 会因 duplicate version 报错；多处测试基线失败（service 338/repository 483/handler:session 52）疑似同根因（缺表：browser_authorization、resources、wiki_page_issues）。
- round21 新认知：其余「预存基线」里可能藏「测试进、实现没进」的可治愈项（tools 21/agentcatalog 14/wiki 4/execution 8）。

## 任务 MIG：迁移去重（敏感，规则先行）

1. 对每对重复：git log 判定哪侧来自上游合并、哪侧是 fork 自有（fork 已部署库按 fork 号 applied——**fork 侧编号不动，上游侧重编号到空闲号段**；空闲号段以各方言目录现有最大号+1 起连续分配，保持同组 up/down 成对、相对顺序不变）。
2. 内容完全相同的对：只删一份（记录证据）。
3. 验证因果：去重后跑 service/repository/handler:session 全量，记录基线收缩量（预期缺表类失败大幅减少）；不承诺清零，残余另归因。
4. 报告：逐对决策表（号/fork 侧/上游侧/动作/证据 commit）；**live-DB 影响评估**写进报告（已部署库不受影响的理由：其 applied 集合不含被重编号的上游新号；全新部署由坏变好）。

## 任务 TRI：小包预存失败归类清偿

1. go test -json 重跑 tools(21)/agentcatalog(14)/wiki(4)/execution(8)，按根因归类：A=合并伤亡可治愈（测试进实现没进/路径搬迁遗留，参照 round21 形态）；B=环境依赖（需 Playwright/外部服务）；C=真产品缺陷（另立票）。
2. A 类就地修复（各包域内）；B/C 只报告。
3. 每治愈一簇附 before/after 计数证据。

## 验收

1. 迁移：各方言目录零重复序号（脚本断言）；去重后基线收缩有数据；`go build ./...`。
2. TRI：A 类治愈、包级计数下降；B/C 归类清单。
3. 严守域边界：MIG 只动 migrations/；TRI 只动各自包。

## 交付

2+ commits（本地不 push）；live-DB 影响评估进报告与 commit message；裁决项更新。
