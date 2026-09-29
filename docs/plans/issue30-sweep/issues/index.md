# Issue #30 层级树与下级 Issue 清单

- 仓库: 1123786563/WeKnora-fork01
- 父 Issue: #30 「Spec: WeKnora 移动 AI Office」（open，标签 ready-for-agent，无评论）
- 下级 Issue: #31–#71（T01–T41，共 41 个），全部在正文中声明 `## Parent` → #30
- 原始树快照: 2026-09-23（各 Issue 文件保存原始 API 数据）
- 实时复核: 2026-09-29（GitHub API，repo `1123786563/WeKnora-fork01`）
- 根 Issue 新增 cross-reference: PR #174（open，dirty mergeable state；详见 [issue-30.md](issue-30.md)）

## 层级树

#30 Spec: WeKnora 移动 AI Office（open）— 原生 sub-issues 为空，下级关系为正文声明式 `## Parent`

├─ T01 #31：原生客户端登录并进入受支持的 Deployment（open）
├─ T02 #32：Active Tenant 切换与 Scoped Vault 隔离（open）
├─ T03 #33：Resource Shelf 展示当前空间可用资源（open）
├─ T04 #34：首页 Attention 与统一 Task 列表（open）
├─ T05 #35：Task 详情 Snapshot、Timeline 与 SSE 恢复（open）
├─ T06 #36：通用目标输入与耐久 Task 创建（open）
├─ T07 #37：运行中调整、排队下一 Run 与停止重启（open）
├─ T08 #38：Attention Inbox 与类型化审批闭环（open）
├─ T09 #39：Task Budget 达限、扩额与恢复（open）
├─ T10 #40：加密离线缓存、草稿与联网确认（open）
├─ T11 #41：注册设备、行动通知与安全深链（open）
├─ T12 #42：Task Owner、Collaborator、Viewer 协作（open）
├─ T13 #43：合规访问、Retention 与安全删除（open）
├─ T14 #44：旧 Session 投影为 Legacy Task（open）
├─ T15 #45：有证据的知识问答闭环（open）
├─ T16 #46：Task Material：Artifact、引用、预览与分享（open）
├─ T17 #47：多来源并行研究与版本化报告（open）
├─ T18 #48：Notion 文档发布端到端闭环（open）
├─ T19 #49：飞书文档发布端到端闭环（open）
├─ T20 #50：Confluence 页面发布端到端闭环（open）
├─ T21 #51：多操作 Action Plan 与部分成功恢复（open）
├─ T22 #52：GitHub 个人连接到草稿 PR（open）
├─ T23 #53：GitHub 空间连接与团队归因（open）
├─ T24 #54：GitLab 草稿 MR 交付闭环（open）
├─ T25 #55：代码交付部分成功、未知结果与凭据隔离（open）
├─ T26 #56：可编辑语音转写草稿（open）
├─ T27 #57：Task 内实时语音会话（open）
├─ T28 #58：Tenant Catalog 不可变 Release 发布闭环（[CLOSED]）
├─ T29 #59：Tenant Adoption、Agent Variant 与移动 Available Agent（open）
├─ T30 #60：Public Marketplace 审核与跨 Tenant Adoption（open）
├─ T31 #61：Agent Upgrade Proposal 与渐进升级（open）
├─ T32 #62：Agent Fork lineage、许可证与再发布（open）
├─ T33 #63：Variant 退役、Adoption 终止与 Listing 生命周期（open）
├─ T34 #64：Release 与依赖安全撤回传播（open）
├─ T35 #65：Marketplace Evaluation、隐私指标与 Publisher Custody（open）
├─ T36 #66：多 Deployment 切换与兼容性降级（open）
├─ T37 #67：自托管盲推送与企业自签名模式（open）
├─ T38 #68：Taro Adapter 复用深 Module 与维护模式门槛（open）
├─ T39 #69：iOS 安装包核心工作流验收（open）
├─ T40 #70：Android 安装包核心工作流验收（open）
└─ T41 #71：跨平台发布证据矩阵与首版验收（open）

## Issue 清单

| 编号 | T | 标题 | URL | 父节点 | 状态 | Blocked by | 关联 PR | 处理决定（本次收集建议） |
|---|---|---|---|---|---|---|---|---|
| #30 | - | Spec: WeKnora 移动 AI Office | https://github.com/1123786563/WeKnora-fork01/issues/30 | -（根） | open | 无 | 无 | 作为父规格 Issue 保留为事实源；原生 sub-issues 为空，父子关系依赖 #31–#71 的声明式 `## Parent` |
| #31 | T01 | 原生客户端登录并进入受支持的 Deployment | https://github.com/1123786563/WeKnora-fork01/issues/31 | #30 | open | 无 | 无 | 保持 open / blocked-external（部分）：本地 iOS 27 scene/startup 已修复，R4 Release 登录界面显示在状态栏下方；HTTPS staging 密码/OIDC、真实 Deployment capability 和 Android 真机证据仍待验收 → 待外部验收收口 |
| #32 | T02 | Active Tenant 切换与 Scoped Vault 隔离 | https://github.com/1123786563/WeKnora-fork01/issues/32 | #30 | open | #31 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #33 | T03 | Resource Shelf 展示当前空间可用资源 | https://github.com/1123786563/WeKnora-fork01/issues/33 | #30 | open | #32 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #34 | T04 | 首页 Attention 与统一 Task 列表 | https://github.com/1123786563/WeKnora-fork01/issues/34 | #30 | open | #32 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #35 | T05 | Task 详情 Snapshot、Timeline 与 SSE 恢复 | https://github.com/1123786563/WeKnora-fork01/issues/35 | #30 | open | #32 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #36 | T06 | 通用目标输入与耐久 Task 创建 | https://github.com/1123786563/WeKnora-fork01/issues/36 | #30 | open | #33、#35 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #37 | T07 | 运行中调整、排队下一 Run 与停止重启 | https://github.com/1123786563/WeKnora-fork01/issues/37 | #30 | open | #35、#36 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #38 | T08 | Attention Inbox 与类型化审批闭环 | https://github.com/1123786563/WeKnora-fork01/issues/38 | #30 | open | #34、#35 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #39 | T09 | Task Budget 达限、扩额与恢复 | https://github.com/1123786563/WeKnora-fork01/issues/39 | #30 | open | #36、#38 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #40 | T10 | 加密离线缓存、草稿与联网确认 | https://github.com/1123786563/WeKnora-fork01/issues/40 | #30 | open | #32、#35、#36 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #41 | T11 | 注册设备、行动通知与安全深链 | https://github.com/1123786563/WeKnora-fork01/issues/41 | #30 | open | #32、#34、#35 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #42 | T12 | Task Owner、Collaborator、Viewer 协作 | https://github.com/1123786563/WeKnora-fork01/issues/42 | #30 | open | #34、#35 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #43 | T13 | 合规访问、Retention 与安全删除 | https://github.com/1123786563/WeKnora-fork01/issues/43 | #30 | open | #42 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #44 | T14 | 旧 Session 投影为 Legacy Task | https://github.com/1123786563/WeKnora-fork01/issues/44 | #30 | open | #34、#35 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #45 | T15 | 有证据的知识问答闭环 | https://github.com/1123786563/WeKnora-fork01/issues/45 | #30 | open | #33、#35、#36 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #46 | T16 | Task Material：Artifact、引用、预览与分享 | https://github.com/1123786563/WeKnora-fork01/issues/46 | #30 | open | #35 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #47 | T17 | 多来源并行研究与版本化报告 | https://github.com/1123786563/WeKnora-fork01/issues/47 | #30 | open | #45、#46 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #48 | T18 | Notion 文档发布端到端闭环 | https://github.com/1123786563/WeKnora-fork01/issues/48 | #30 | open | #38、#46 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #49 | T19 | 飞书文档发布端到端闭环 | https://github.com/1123786563/WeKnora-fork01/issues/49 | #30 | open | #48 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #50 | T20 | Confluence 页面发布端到端闭环 | https://github.com/1123786563/WeKnora-fork01/issues/50 | #30 | open | #48 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #51 | T21 | 多操作 Action Plan 与部分成功恢复 | https://github.com/1123786563/WeKnora-fork01/issues/51 | #30 | open | #48 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #52 | T22 | GitHub 个人连接到草稿 PR | https://github.com/1123786563/WeKnora-fork01/issues/52 | #30 | open | #36、#38、#46 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #53 | T23 | GitHub 空间连接与团队归因 | https://github.com/1123786563/WeKnora-fork01/issues/53 | #30 | open | #42、#52 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #54 | T24 | GitLab 草稿 MR 交付闭环 | https://github.com/1123786563/WeKnora-fork01/issues/54 | #30 | open | #52 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #55 | T25 | 代码交付部分成功、未知结果与凭据隔离 | https://github.com/1123786563/WeKnora-fork01/issues/55 | #30 | open | #52、#54 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #56 | T26 | 可编辑语音转写草稿 | https://github.com/1123786563/WeKnora-fork01/issues/56 | #30 | open | #36 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #57 | T27 | Task 内实时语音会话 | https://github.com/1123786563/WeKnora-fork01/issues/57 | #30 | open | #35、#56 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #58 | T28 | Tenant Catalog 不可变 Release 发布闭环 | https://github.com/1123786563/WeKnora-fork01/issues/58 | #30 | closed | 无 | 无 | 已关闭（2026-09-20T19:29:01Z 首次关闭 → 19:34:13Z 因 PostgreSQL 验收缺口重开 → 19:37:43Z 补齐证据后关闭）；sweep 仅归档记录，不重开 |
| #59 | T29 | Tenant Adoption、Agent Variant 与移动 Available Agent | https://github.com/1123786563/WeKnora-fork01/issues/59 | #30 | open | #33、#58 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #60 | T30 | Public Marketplace 审核与跨 Tenant Adoption | https://github.com/1123786563/WeKnora-fork01/issues/60 | #30 | open | #58、#59 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #61 | T31 | Agent Upgrade Proposal 与渐进升级 | https://github.com/1123786563/WeKnora-fork01/issues/61 | #30 | open | #59、#60 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #62 | T32 | Agent Fork lineage、许可证与再发布 | https://github.com/1123786563/WeKnora-fork01/issues/62 | #30 | open | #59、#60 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #63 | T33 | Variant 退役、Adoption 终止与 Listing 生命周期 | https://github.com/1123786563/WeKnora-fork01/issues/63 | #30 | open | #59、#61 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #64 | T34 | Release 与依赖安全撤回传播 | https://github.com/1123786563/WeKnora-fork01/issues/64 | #30 | open | #60、#61 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #65 | T35 | Marketplace Evaluation、隐私指标与 Publisher Custody | https://github.com/1123786563/WeKnora-fork01/issues/65 | #30 | open | #60、#63、#64 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #66 | T36 | 多 Deployment 切换与兼容性降级 | https://github.com/1123786563/WeKnora-fork01/issues/66 | #30 | open | #32 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #67 | T37 | 自托管盲推送与企业自签名模式 | https://github.com/1123786563/WeKnora-fork01/issues/67 | #30 | open | #41、#66 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #68 | T38 | Taro Adapter 复用深 Module 与维护模式门槛 | https://github.com/1123786563/WeKnora-fork01/issues/68 | #30 | open | #34、#35、#36、#38、#46 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #69 | T39 | iOS 安装包核心工作流验收 | https://github.com/1123786563/WeKnora-fork01/issues/69 | #30 | open | #40、#41、#56、#66 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #70 | T40 | Android 安装包核心工作流验收 | https://github.com/1123786563/WeKnora-fork01/issues/70 | #30 | open | #40、#41、#56、#66 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |
| #71 | T41 | 跨平台发布证据矩阵与首版验收 | https://github.com/1123786563/WeKnora-fork01/issues/71 | #30 | open | #43、#44、#47、#49、#50、#51、#53、#55、#57、#62、#65、#67、#68、#69、#70 | 无 | open：无评论、无关联 PR、任务清单 0/3 勾选，未见实现痕迹 → 待处理（纳入 sweep 逐项规划） |

## 收集方法与完整性

- 命令: `gh api repos/1123786563/WeKnora-fork01/issues/N`、`.../issues/N/sub_issues`、`gh api --paginate .../issues/N/comments`、`gh api --paginate .../issues/N/timeline`（N ∈ 30–71），以及 `gh pr list -R 1123786563/WeKnora-fork01 --state all`。
- 抓取失败的编号（瞬时网络错误，重试成功）: #35（connection reset，Issue 主体）、#42 与 #62（timeline）。重试后 42 个 Issue 主体、全部评论、全部 timeline 均完整，无读取失败遗留。
- 一致性校验（脚本核对）: 每个 Issue 的评论数在 API `comments` 字段、实际抓取评论条数、timeline `commented` 事件数三处一致；正文声明式 `## Blocked by` 与 timeline 原生 `blocked_by_added` 关系逐项一致（41/41）。
- 任务清单合计 123 条 checkbox（41 × 3），全部未勾选。

## 原生关系 vs 声明式关系（scope 说明）

1. #30 的原生 sub-issues 为空数组；#31–#71 的父子关系仅存在于各 Issue 正文 `## Parent` 段（声明式），与任务给定事实一致。
2. #31–#71 各自的原生 sub-issues 也为空（无嵌套子 Issue），下级结构到 T01–T41 一层为止。
3. `## Blocked by` 声明与 GitHub 原生 blocked-by 关系完全重合，无仅声明未建原生关系（或反之）的项。Blocked by 目标全部落在 #31–#70，即 #30 后代集合内部，无外部依赖 Issue。
4. #30 正文与评论均无 `#NN` Issue 引用、无 T 编号引用，评论数为 0 → 无额外声明的子任务；#31–#71 正文除 Parent/Blocked by 段外亦无其它 `#NN` 引用。评论中仅 #58 第 4 条评论存在一处自引用（"Ticket #58"）。
5. 关联 PR（原始 2026-09-23 快照）: 当时 #30–#71 的 timeline 无来自 PR 的 cross-reference；2026-09-29 复核发现根 Issue #30 新增 PR #174 cross-reference，当前状态见 [issue-30.md](issue-30.md)。子 Issue 的各自关联 PR 字段仍代表 2026-09-23 快照，尚未逐个重新轮询。
6. #58（T28）为唯一 closed Issue，历经 关闭→重开→再关闭（详见 issue-58.md 评论摘要）；#31（T01）open 但评论表明实现已集成分支、验收开放。

## 下游产物

- 依赖 DAG（拓扑批次 / readyOrder / 阻塞节点 / 状态总表）: [../dag.md](../dag.md)
