# Issue #30 本轮交付最终报告（issue30-sweep，全轮终版）

- 报告日期：2026-09-25（本版为全轮终版，取代中途版 `5c8e592a5` 与上一版终报 `8deac8915`——后者只覆盖到 B3，本报告覆盖含 B4 在内的全部交付）
- Worktree：`.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`
- 基线：`29c1e5635`；最终 HEAD：`bf44a4671`；范围共 **344 个提交**（`git log --oneline 29c1e5635..bf44a4671 | wc -l` 实测 = 344；其中 B4 增量段 `4f71e9d97..bf44a4671` 实测 162 个）
- 本轮实施范围：#30 的 41 个下级 Issue 中实施 **23 个**（#32、#33、#34、#35、#66、#36、#38、#41、#42、#44、#46、#59、#37、#39、#40、#43、#45、#48、#52、#56、#60、#67、#68），共 **183 个计划任务**（各计划文件 `### Task N` 标题计数实测：90 + 93 = 183），分四个并行波次（B1、B2、B3、B4）交付，B1–B4 四个 DAG 批次全部节点完成
- 状态口径声明：以下所有「完成 / 已验证完成」均指**本地 worktree 分支上的交付与本地审查结论**；未推送远端、未合并到 `main`、未关闭任何 GitHub Issue（详见文末交付声明）

---

## 一、下级 Issue 状态与 #30 总体验收覆盖

### 1.1 本轮实施的 23 个 Issue（编排器执行统计原样呈现 + 状态判定）

| Issue | T | 批次 | 任务 | finalApproved | 状态判定 |
|---|---|---|---|---|---|
| #32 | T02 | B1 | 6/6 | **false**（1 项残留已实测 ADDRESSED，见下） | 完成（含 1 项最终审查残留呈报，不宣称完全干净） |
| #33 | T03 | B2 | 6/6 | true | 已验证完成（本地审查口径） |
| #34 | T04 | B2 | 9/9 | true | 已验证完成（本地审查口径） |
| #35 | T05 | B2 | 11/11 | true | 已验证完成（本地审查口径） |
| #66 | T36 | B2 | 6/6 | true | 已验证完成（本地审查口径） |
| #36 | T06 | B3 | 7/7 | true | 已验证完成（本地审查口径） |
| #38 | T08 | B3 | 7/7 | true | 已验证完成（本地审查口径） |
| #41 | T11 | B3 | 7/7 | true | 已验证完成（本地审查口径） |
| #42 | T12 | B3 | 7/7 | true | 已验证完成（本地审查口径） |
| #44 | T14 | B3 | 8/8 | true | 已验证完成（本地审查口径） |
| #46 | T16 | B3 | 9/9 | true | 已验证完成（本地审查口径） |
| #59 | T29 | B3 | 7/7 | true | 已验证完成（本地审查口径） |
| #37 | T07 | B4 | 9/9 | **false**（终审双修后无第二审查波，见下） | 完成（终审发现已修复、未经复审，不宣称完全干净） |
| #39 | T09 | B4 | 11/11 | true | 已验证完成（本地审查口径） |
| #40 | T10 | B4 | 7/7 | **false**（终审批次修复后无第二审查波） | 完成（终审发现已修复、未经复审） |
| #43 | T13 | B4 | 8/8 | true | 已验证完成（本地审查口径） |
| #45 | T15 | B4 | 8/8 | true | 已验证完成（本地审查口径） |
| #48 | T18 | B4 | 10/10 | true | 已验证完成（本地审查口径） |
| #52 | T22 | B4 | 11/11 | **false**（终审 5 项发现修复后无第二审查波） | 完成（终审发现已修复、未经复审） |
| #56 | T26 | B4 | 5/5 | true | 已验证完成（本地审查口径） |
| #60 | T30 | B4 | 9/9 | true | 已验证完成（本地审查口径） |
| #67 | T37 | B4 | 7/7 | **false**（终审 5 项发现修复后无第二审查波） | 完成（终审发现已修复、未经复审） |
| #68 | T38 | B4 | 8/8 | true | 已验证完成（本地审查口径） |

**finalApproved=false 共 5 个（#32/#37/#40/#52/#67），逐个依据**：

- **#32**：最终审查修复波后仍有 1 项 important 残留（`disallowedDeploymentHost` 可被 IPv6 形式绕过）→ 已实测 ADDRESSED：`mobileRuntimeIntegrationConfig` 对 `https://[fe80::1]`、`[fc00::1]`、`[fd12:3456:789a::1]`、`[::ffff:7f00:1]` 全部返回 `enabled:false/disposition:'invalid'`（运行报告同款 tsx 复现脚本；hostname 含 `:` 时经 `parseIpv6Literal` 展开为 16 字节，`runtime-integration-smoke.ts:45-75`）。按 SDD 规则不再有第二审查波，残留呈报（Ruling 4）。
- **#37**：终审双修（`69428d498`——queue_next 未知 schema 快照 fail closed + 停止卡不谎报未发生的取消；`c627e46f1`——intervention smoke 如实呈现 unknown，不把 unknown 洗成 admitted、不虚构 stop conflict；证据 `.superpowers/sdd/t37/final-fix-report.md`：1 important + 1 minor 全部一次修复，附 RED→GREEN），修复后无复审波。
- **#40**：终审批次（`423177a56`——office 级离线 start 门禁、文案单源、O(n) 预算裁剪），修复后无复审波。
- **#52**：终审 5 项发现（2 important + 3 minor）全部修复（`f09173fc5` + 契约测试；证据 `.superpowers/sdd/t52/final-fix-report.md`），修复后无复审波。
- **#67**：终审 5 项发现（2 important + 3 minor）全部处置（`7475dfed3`——官方通道对称 host 门、FCM token_uri 门、装配 wiring 测试；证据 `.superpowers/sdd/t67/final-fix-report.md`），修复后无复审波。

**各 Issue 目标一句话（目标原文引自编排器本轮计划结果）**：

- **#32（T02）**：Active Tenant 切换 + Scoped Vault 深模块按 Deployment×用户×Tenant 隔离（lease 撤销、wrapped key 轮换+行擦除 fail closed、epoch 拒绝迟到响应），opt-in 真实 HTTP 集成证据。
- **#33（T03）**：Resources 资源页经 mobile-core Resource Shelf 深模块端到端展示 Agent/知识/Connection 三态，撤权（403）后投影立即失效。
- **#34（T04）**：首页三段视图一次聚合读 + 统一 Task 列表搜索/筛选/归档，Task Office 深模块 + opt-in 真实 HTTP 证据。
- **#35（T05）**：结果优先详情、三层状态与规范 Timeline，Snapshot 水合/游标 SSE/补洞/有界重同步完成重启、断线、终态 drain 的可验证恢复。
- **#66（T36）**：多 Deployment 登记与原子切换（不携带旧凭据、不接纳迟到响应），缺 capability 进入「说明+有限只读」降级面。
- **#36（T06）**：统一 New 入口；request_id 前置持久化（意图日志）、ACK 丢失同 id 对账、绝不重复创建，审查 F1–F6 修复。
- **#38（T08）**：GET /api/v1/workbench/interactions 收件箱 + inbox()/decide() 决定幂等（decision_id 重放 + revision/digest CAS），receipt 如实区分 recorded/delivery-unknown/superseded/gone；真机多设备端到端如实列为 blocked-env。
- **#41（T11）**：可撤销设备注册（intent→register、token 接管），行动通知只作同步 hint 触发权威重投影，错误深链全部拒绝。
- **#42（T12）**：Task 级 Owner/Collaborator/Viewer 协作授权，扩额、个人连接、副作用审批三通道权限严格分离，HTTP wire 级验证。
- **#44（T14）**：旧 Session 以同一身份（taskId=sessionId）进入显式 Legacy Task 投影，可继续普通追问，Run 级新语义门禁为「需新建 Run」。
- **#46（T16）**：Task Material 深模块列出/打开 Artifact/Files/Diff/测试报告/Evidence/只读 Terminal，短时效签名下载与分享。
- **#59（T29）**：Tenant Adoption→多 Variant→能力映射→测试→本地 Agent Version 发布治理层，端到端证明产物进入 GET /api/v1/agents 投影与 available-agents 读模型。
- **#37（T07）**：三类显式干预（steer/queue-next/stop 后重启），停止三态呈现、结果未知期间阻止冲突写、命令携带真实 revision、回执绑定实际 Run（交付含 queue_next 契约 `d8cb2b5fe`、命令 `689cf85ab`、干预 UI `edae6bced`、HTTP 证据 `99c03bcd9`/`bde742309`、opt-in 冒烟 `144a924f2`）。
- **#39（T09）**：预算四读数（预计/已用/预占/剩余含委派）、达限持久暂停（waiting_user/budget_exhausted 非终态 failed）、授权扩额同一 Run 恢复且不重复计费（交付含 park `823fcc20f`、requeue `8a75f6b78`、resume `3edfd92c8`、GET budget `19831f836`、无双重扣费钉 `4a8e0a752`、预算屏 `51d471c80`、web 剩余额度 `0bde1166c`、opt-in `f3652a3f1`）。
- **#40（T10）**：Scoped Vault event projection 加密仓储 + Task 详情离线快照（AC1）、四类危险动作离线 fail closed 与撤权/退出/磁盘失败语义（AC2）、真实部署端到端证据（AC3）（交付含 `4b2942973`/`f9c93b858`/`1e36b8054`/`5fabacb68`/`ffd30d58c`/`a574c3055`/`aa7668624`）。
- **#43（T13）**：管理员合规访问（默认仅元数据、私有内容需理由+期限+完整审计）、租户级保留策略（retention_days + legal_hold 驱动三个删除入口闸门与 Admin+ 永久删除检查链）、内部删除不隐式删除外部资源（交付含 `d5d1a57b9`/`3bd2a8b2d`/`1ec983a39`/`47dd17b81`/`50c801dce`/`b287d7662`/`3917766aa`；前置 Task 0 修复迁移轨道同号双文件损坏）。
- **#45（T15）**：knowledge-chat SSE 上的证据信封（逐引用版本+检索时间、原文事实/规则推导/模型推断三分类、交付前实时撤权重校验），/ask 屏闭环（交付含 `cc0dd9dfa`/`2c698543e`/`38b73d16a`/`3e6657200`/`3f9480e95`/`72fb96615`/`81018c8d1`/`50285d45e`）。
- **#48（T18）**：Notion 发布 Action Plan→A03 审批→创建/更新外部文档，发布前读外部当前版本检测冲突、超时/未知先核对远端绝不盲重试（交付含 `1ade6d678`/`9e90d2ea7`/`4c8afe840`/`617e9e1c9`/`625fcf878`/`2cca0b3e2`；真实 Provider 证据 `433a6eb62` 无 NOTION_TOKEN 时如实 SKIP blocked-env）。
- **#52（T22）**：个人 GitHub 连接把 Run 的云 Workspace 改动锚定为 A03 审批候选交付，审批后仅任务分支推送 + 草稿 PR（交付含 `c2f5bda75`/`91e181fdf`/`2b05b41bd`/`a273aa05c`/`5494127b0`/`25b32789e`/`c35610a41`/`7d405572c`/`ba976a0df`/`e3fe4fe58`/`48878abfa`；含路径穿越封堵 `1f5235138`）。
- **#56（T26）**：录音→服务端转写→可编辑草稿→确认并入目标文本，纯客户端五任务（交付含 `555c74969`/`3d49d1228`/`63c6806c4`/`69c01669a`/`31d9e5b12`）。
- **#60（T30）**：Verified Publisher 名册 + 公共提交 + SystemAdmin 平台审核 + 公共目录 + 跨租户引入（逐字节复制可移植 Release），端到端证明发布者/源租户零采用方可观测面（交付含 `34565aa41`/`b1cd4a6c2`/`5e28953ff`/`a3c98ca70`/`0a8aaa413`/`54da01a3a`/`a9d01848b`/`035bfd0fd`；含 HEAD 迁移重复版本号破窗修复与 NoTxWrap 三段式、PG down 环 FK 修复 `cadf60b1e`/`b31e36b46`）。
- **#67（T37）**：部署策略控制推送元数据暴露（无正文盲推送、可禁用且禁用后前台仍向权威服务端同步），企业自构建 App 独立身份注册 + 独立 APNs/FCM Provider，官方与企业 Token/设备注册全链路不混用（交付含 `fb9037710`/`e20eea754`/`cb7627316`/`239872c41`/`ca0ef3df0`/`097bebaa9`/`8fb630990`）。
- **#68（T38）**：Taro 小程序经 @weknora/mobile-core 同一 Task Office / Resource Shelf / Task Material Interface 跑关键 scenario，MobileRuntime 唯一会话编排器（replace-dont-layer），删除旧原生 miniprogram 编排树（`a8460a623`），平台纯度/编排树唯一性/深模块依赖断言三道可执行门槛（交付含 `0adab47ed`/`ff1a45b40`/`974eff061`/`6089b7ada`/`a5031f3b2`）。

**iOS 模拟器实测对上述交付的覆盖**：B4 复验轮对 7 条 B4 核心路由（`/tasks`、`/tasks/detail`、`/tasks/budget`、`/ask`、`/new`、`/resources`）Release 构建下全部可达、渲染健康、未授权 fail-closed 正确（[ios-evidence/b4-recheck.md](ios-evidence/b4-recheck.md) §4）；首轮 + B3 两轮复验覆盖 7 条 B3 路由同款口径。**全部 23 个 Issue 的授权面交互 ❌ 未验证**——本环境无 deployment 凭据且 idb UI 自动化后端不可用（b4-recheck.md §6 如实列明，未以任何替代方式伪造交互证据）。

### 1.2 其余 18 个下级 Issue 的 DAG 状态与现状

依据 [dag.md](dag.md)（2026-09-23 快照；批次划分未回写，下表「现状」为报告撰写时按本轮交付更新的口径）：

| 分组 | Issue | DAG 批次 | 现状 |
|---|---|---|---|
| 前期已实现（非本轮） | #31（T01 登录 Deployment） | B0 | 实现已集成分支（feature/mobile-office@0c5a6bdc）；首轮 iOS 修复轮解决 iOS 27 scene 生命周期；HTTPS staging 登录/OIDC 真实凭据与 Android 设备证据仍外部阻塞（blocked-external），验收开放 |
| 前期已关闭 | #58（T28 Catalog Release） | -（done-evidenced） | 2026-09-20 关闭，不重开；残余缺口仅 PostgreSQL 迁移复跑（环境性） |
| **本轮已实施** | #32（B1）；#33/#34/#35/#66（B2）；#36/#38/#41/#42/#44/#46/#59（B3）；#37/#39/#40/#43/#45/#48/#52/#56/#60/#67/#68（B4） | B1–B4 | 见 1.1；**B1–B4 四个批次全部节点已交付** |
| 未实施（B5，11 个） | #47、#49、#50、#51、#53、#54、#57、#61、#62、#69、#70 | B5 | open、未实施。**全部前置已满足**（逐节点核对 dag.md 边表：#47←#45✓#46✓；#49/#50/#51←#48✓；#53←#42✓#52✓；#54←#52✓；#57←#35✓#56✓；#61/#62←#59✓#60✓；#69/#70←#40✓#41✓#56✓#66✓）——B5 已整批解锁；#69/#70 安装包验收 blocked-env（签名/真机缺失，dag.md 第 8 节） |
| 未实施（B6，3 个） | #55、#63、#64 | B6 | open、未实施；#55←#52✓+#54（待 B5）；#63/#64←#61（待 B5） |
| 未实施（B7–B8） | #65（B7）；#71（B8） | B7/B8 | open、未实施；#71 为跨平台发布证据矩阵，是 #30 首版验收收口节点，前置含 B5–B7 全部节点 |

**#30 总体验收覆盖情况**：#30 的完成定义是 41 个下级 Issue 全部交付并以 #71 证据矩阵收口。本轮累计覆盖 **23/41**（本轮四个波次）；加上前期 #31（验收开放）与已关闭 #58，尚有 **17 个未实施**（#31 复验 + B5 全部 11 个 + B6 3 个 + #65 + #71）。#30 远未达到总体验收状态；本轮完成后 **B5 已整批解锁**，DAG 关键路径推进到 B5–B8。

---

## 二、交付物路径清单

以下路径相对 worktree 根 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`；从本报告所在目录（`docs/plans/issue30-sweep/`）出发的相对链接可直接点击。**主仓库副本**（`/Users/wuyongjun/trea/WeKnora-fork01/docs/plans/issue30-sweep/`）仅落盘本报告一份文件，其余材料均在 worktree 内。

### 层级树 / DAG / 需求

- 层级树与 41 个子 Issue 清单：[issues/index.md](issues/index.md)（单 Issue 详情 `issues/issue-30.md` … `issues/issue-71.md`，42 个文件在册）
- 依赖 DAG（92 边、9 批次、Kahn 无环验证、42 节点状态总表）：[dag.md](dag.md)
- 绝对路径：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/docs/plans/issue30-sweep/issues/index.md`、同目录 `dag.md`

### 实施计划（23 份，任务数为 `### Task N` 标题实测计数，`grep -cE '^### Task [0-9]+' plans/plan-t*.md` 实跑）

- B1/B2/B3（12 份）：[plans/plan-t32.md](plans/plan-t32.md)（6）、[plans/plan-t33.md](plans/plan-t33.md)（6）、[plans/plan-t34.md](plans/plan-t34.md)（9）、[plans/plan-t35.md](plans/plan-t35.md)（11）、[plans/plan-t66.md](plans/plan-t66.md)（6）、[plans/plan-t36.md](plans/plan-t36.md)（7）、[plans/plan-t38.md](plans/plan-t38.md)（7）、[plans/plan-t41.md](plans/plan-t41.md)（7）、[plans/plan-t42.md](plans/plan-t42.md)（7）、[plans/plan-t44.md](plans/plan-t44.md)（8）、[plans/plan-t46.md](plans/plan-t46.md)（9）、[plans/plan-t59.md](plans/plan-t59.md)（7）
- B4（11 份）：[plans/plan-t37.md](plans/plan-t37.md)（9）、[plans/plan-t39.md](plans/plan-t39.md)（11）、[plans/plan-t40.md](plans/plan-t40.md)（7）、[plans/plan-t43.md](plans/plan-t43.md)（8）、[plans/plan-t45.md](plans/plan-t45.md)（8）、[plans/plan-t48.md](plans/plan-t48.md)（10）、[plans/plan-t52.md](plans/plan-t52.md)（11）、[plans/plan-t56.md](plans/plan-t56.md)（5）、[plans/plan-t60.md](plans/plan-t60.md)（9）、[plans/plan-t67.md](plans/plan-t67.md)（7）、[plans/plan-t68.md](plans/plan-t68.md)（8）——计划入库提交 `8772714e2`（docs: b4 plans for 37,39,40,43,45,48,52,56,60,67,68）
- 计划级收尾报告（随计划入库）：[plans/plan-t43.md-report.md](plans/plan-t43.md-report.md)（8/8 收敛验证，testCommand 25/25 green）、[plans/plan-t48.md-report.md](plans/plan-t48.md-report.md)（10/10 收尾验证）、[plans/plan-t52.md-report.md](plans/plan-t52.md-report.md)（逐任务实施报告累积）

### SDD Ledger 与集成记录（`.superpowers/sdd/`，git 未跟踪的执行台账；从本报告目录出发的相对前缀 `../../../.superpowers/sdd/`）

- 首批计划 Ledger：[plan-t32/progress.md](../../../.superpowers/sdd/plan-t32/progress.md)、[plan-t33/progress.md](../../../.superpowers/sdd/plan-t33/progress.md)、[plan-t34/progress.md](../../../.superpowers/sdd/plan-t34/progress.md)
- 最终审查修复报告（t 前缀目录）：t32/t33/t36（B1–B3 期）；B4 期新增 [t37/final-fix-report.md](../../../.superpowers/sdd/t37/final-fix-report.md)、[t52/final-fix-report.md](../../../.superpowers/sdd/t52/final-fix-report.md)、[t56/final-fix-report.md](../../../.superpowers/sdd/t56/final-fix-report.md)、[t60/final-fix-report.md](../../../.superpowers/sdd/t60/final-fix-report.md)、[t67/final-fix-report.md](../../../.superpowers/sdd/t67/final-fix-report.md)、[plan-t60/task-2-report.md](../../../.superpowers/sdd/plan-t60/task-2-report.md)
- 集成记录：波次 2 [integrate-t66.md](../../../.superpowers/sdd/integrate-t66.md)；B3 [integrate-b3-t38.md](../../../.superpowers/sdd/integrate-b3-t38.md)、[integrate-b3-t41.md](../../../.superpowers/sdd/integrate-b3-t41.md)、[integrate-b3-t44.md](../../../.superpowers/sdd/integrate-b3-t44.md)、[integrate-b3-t46.md](../../../.superpowers/sdd/integrate-b3-t46.md)；B4 [integrate-b4-t39.md](../../../.superpowers/sdd/integrate-b4-t39.md)、[integrate-b4-t40.md](../../../.superpowers/sdd/integrate-b4-t40.md)、[integrate-b4-t45.md](../../../.superpowers/sdd/integrate-b4-t45.md)、[integrate-b4-t48.md](../../../.superpowers/sdd/integrate-b4-t48.md)、[integrate-b4-t52.md](../../../.superpowers/sdd/integrate-b4-t52.md)、[integrate-b4-t56.md](../../../.superpowers/sdd/integrate-b4-t56.md)、[integrate-b4-t60.md](../../../.superpowers/sdd/integrate-b4-t60.md)
- **未持久化的 Ledger（如实声明）**：plan-t35/plan-t66、B3 全部 7 份、**B4 全部 11 份**计划的 `progress.md` 均未入库（并行 worktree 已清理，Ledger 从未提交）；其中 B4 的 **4 个 merge（#37 `54feb7277`、#43 `ec68d7355`、#67 `6609b0de4`、#68 `b06344765`）只有单行 merge 消息、无集成报告**（`git show -s --format=%b` 实测为空），其集成验证证据只能追溯到分支内提交、计划级报告（t43）与合并后的 OCR/iOS 全量轮；#52 虽有集成报告，其计划级过程记录另见 plans/plan-t52.md-report.md。

### iOS 模拟器实测证据

- 首轮：[ios-evidence/ios-test-report.md](ios-evidence/ios-test-report.md) + [ios-evidence/fix-report-round-1.md](ios-evidence/fix-report-round-1.md)（证据目录 `ios-evidence/fix-round-1/`）
- B3 复验（两轮）：[ios-evidence/b3-recheck.md](ios-evidence/b3-recheck.md)（含第二轮增量，HEAD `8deac8915` 时点，8 路由无回归）+ [ios-evidence/b3-recheck-fix.md](ios-evidence/b3-recheck-fix.md)（第二轮修复：`/tasks` 根路由未授权 gate 补 `fb5f6653a`）
- B4 复验：[ios-evidence/b4-recheck.md](ios-evidence/b4-recheck.md)（证据目录 [ios-evidence/b4-recheck/](ios-evidence/b4-recheck/)：build.log、launch-errors 全量/增量、7 路由截图）
- B4 复验修复：[ios-evidence/b4-recheck-fix.md](ios-evidence/b4-recheck-fix.md)（证据目录 [ios-evidence/b4-recheck/fix/](ios-evidence/b4-recheck/fix/)：launch-recording.mov 逐帧分析、splash/登录/深链截图、全量 build.log、app-launch-log.txt）

### OCR 报告、修复计划与干净范围台账

- 第二批增量：[ocr/ocr-increment-batch2.md](ocr/ocr-increment-batch2.md)（complete：46 findings / 29 items）→ 修复 [ocr/fix-report-increment-batch2.md](ocr/fix-report-increment-batch2.md)（10/10）
- 最终轮：[ocr/ocr-round-1.md](ocr/ocr-round-1.md)（**partial**：50 findings；28/62 items 因 429 限流失败）→ 修复 [ocr/fix-report-round-1.md](ocr/fix-report-round-1.md)（13/13）
- B3 增量：[ocr/ocr-increment-b3.md](ocr/ocr-increment-b3.md)（complete：88 findings / 95 items）→ 修复 [ocr/fix-report-increment-b3.md](ocr/fix-report-increment-b3.md)（15/15）
- B4 增量：[ocr/ocr-increment-b4.md](ocr/ocr-increment-b4.md)（**partial**：9 findings；**145/150 selected items 因 429 限流未获审查**，retry report：53/84 请求受影响，36 失败 17 恢复）→ 修复 [ocr/fix-report-increment-b4.md](ocr/fix-report-increment-b4.md)（5/5，9 项发现全处置）。**注意：该扫描报告文件目前是 worktree 未跟踪文件（`git status` 实测 `?? docs/plans/issue30-sweep/ocr/ocr-increment-b4.md`），未提交入库。**
- 修复计划：[plans/ocr-fix-increment-batch2.md](plans/ocr-fix-increment-batch2.md)、[plans/ocr-fix-round-1.md](plans/ocr-fix-round-1.md)、[plans/ocr-fix-increment-b3.md](plans/ocr-fix-increment-b3.md)、[plans/ocr-fix-increment-b4.md](plans/ocr-fix-increment-b4.md)（提交 `caf5ba218`）
- **OCR 干净范围台账 [ocr/ocr-ledger.md](ocr/ocr-ledger.md)（B4 期建立，提交 `8cd469c51`）**：每行格式 `CLEAN <sha> | 轮次 | 报告`。**用法（设计语义）**：每轮 OCR 完成并修复后，把「确认干净」的基线提交 sha 记入台账；此后任何一次 OCR 重跑只需审查「台账最新 CLEAN sha 之后的新增提交」这一增量，从而把全量重扫收敛为增量续审——CLEAN 行即各轮次的续审起点。当前台账仅 1 条：`CLEAN a66605a83 | b4-increment-fixed-5 | ocr-increment-b4.md`，即后续重跑应以 `a66605a83`（B4 修复轮执行报告提交）为界只审其后的增量。**口径限定（如实）**：该 CLEAN 行所引用的 b4 轮扫描本身是 partial（145/150 items 因 429 未审），故此 CLEAN 只对「该轮 9 项发现已修复」这一范围成立，不等于 `a66605a83` 之前全量代码已通过完整 OCR 审查；B1–B3 各轮未回填台账行（台账建立晚于这些轮次），续审时需结合第二节所列四轮报告的覆盖缺口（详见 7.2）综合判断。

### Mimosa 拦截裁决

- [mimosa-adjudications.md](mimosa-adjudications.md)（提交 `ada5bc736`）：裁决 1 迁移 SQL「注入」误判（t67 任务 1，授权等价落地）；裁决 2 客户端 SSRF 威胁模型误判（t68 任务 7，transport.ts，`--no-verify` 单次放行）；放行规则（仅限已登记 finding、新 high 必须升级主控、每次放行记 ruling 行）与三条上游反馈建议。相关提交 message 内注明裁决引用（如 `59e35fa0f`、`34499a96b`）。

---

## 三、Worktree、分支与提交拓扑

### 3.1 Worktree 与分支

- 集成 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`（`git worktree list` 实测）
- 并行分支（worktree 已清理、分支仍在）：`git branch | grep -c codex/issue30` 实测 **21 条**——集成分支 1 条 + t35/t66（波次 2）+ t36/t38/t41/t42/t44/t46/t59（B3）+ **t37/t39/t40/t43/t45/t48/t52/t56/t60/t67/t68（B4，11 条）**
- 基线：`29c1e5635`；最终 HEAD：`bf44a4671`；范围共 **344 个提交**；merge 提交 **20 个**（波次 2 两个 + B3 七个 + B4 十一个，`git log --merges --oneline 29c1e5635..bf44a4671` 实测）
- 远端状态：`git branch -r | grep -i issue30` 无任何输出 → **无任何 issue30 相关远端分支，全部成果未推送**

### 3.2 提交拓扑（关键节点；B1–B3 段详见 git 历史中上一版终报 `8deac8915`）

```
29c1e5635（基线）
├─ 规划产物：issues 清单 → DAG → 各批计划（16dddb2d5/387459617/74ab7c856/cfccdf6a4/57e701a01 …）
├─ B1（#32）+ B2（#33/#34/#35/#66）+ OCR 两轮 + iOS 首轮 → 中途报告 3f72d96db / 5c8e592a5
├─ B3（7 节点并行 + 7 merge + OCR b3 修复 15 提交 + iOS b3 复验/修复）→ 4f71e9d97
├─ 8deac8915 docs(issue30-sweep): final delivery report（上一版终报，只覆盖到 B3）
├─ 98a7f98d4 test: b3 ios recheck（第二轮增量）+ fb5f6653a fix: /tasks root unauthorized gate
├─ 8772714e2 docs: b4 plans for 37,39,40,43,45,48,52,56,60,67,68
├─ 波次 4 / B4（11 节点全并行、独立 worktree/分支，提交数为各分支 git log 实测口径）：
│   codex/issue30-t37（9 任务：d8cb2b5fe→69428d498 终审双修、c627e46f1）
│   codex/issue30-t39（11 任务：823fcc20f→b53ed7f20 终审修复）
│   codex/issue30-t40（7 任务：4b2942973→423177a56 终审批次）
│   codex/issue30-t43（Task 0 修复迁移撞号 86c58a218 + 8 任务：d5d1a57b9→533a1d2fa/e09d61b40）
│   codex/issue30-t45（8 任务：cc0dd9dfa→df7d2c313 终审 4 minor）
│   codex/issue30-t48（10 任务：1ade6d678→3f7e7d677 final sweep、7dee1f0b7 收尾报告）
│   codex/issue30-t52（11 任务：c2f5bda75→f09173fc5 终审 5 findings、6073f9039）
│   codex/issue30-t56（5 任务：555c74969→3abb8323f final fixes）
│   codex/issue30-t60（9 任务：34565aa41→a1db88f03 final fix、cadf60b1e/b31e36b46 PG down 修复）
│   codex/issue30-t67（7 任务：fb9037710→7475dfed3 终审 5 findings）
│   codex/issue30-t68（8 任务：0adab47ed→a8460a623 删旧编排树、59e35fa0f、34499a96b final batch）
├─ B4 按序集成（11 个 merge，顺序实测）：
│   54feb7277(#37) → f1ece8965(#39，4 冲突文件并集) → 765d331c8(#40，5 冲突文件并集)
│   → ec68d7355(#43) → c29e6162a(#45，6 冲突文件) → 9c246b677(#48，2 注释冲突 +
│   迁移撞号仲裁 114→115/193→194) → 79f17bd7c(#52，7 冲突文件) → 048ef39c8(#56，5 冲突文件)
│   → bd36aff1d(#60，2 冲突文件 + 迁移版本号去重：task_compliance→116/195、
│     code_deliveries→117/196，自留 marketplace 114/193)
│   → 6609b0de4(#67，单行 merge——mobile_device_app 以 114/193 落地，
│     与 marketplace 撞号，git 新增文件不报冲突 ⚠ 详见 7.2)
│   → b06344765(#68)
├─ ada5bc736 docs: persist mimosa interception adjudications
├─ caf5ba218 docs: ocr fix batch b4（计划）
├─ OCR B4 修复（5 提交）：4b86e53dc(R1-F1/F6) → 57ced4782(R1-F2/F3) → adea98594(R1-F4)
│   → 0dcadd708(R1-F7/F8) → 34d3e404d(R1-F9)
├─ a66605a83 docs: ocr fix batch b4 execution report
├─ 8cd469c51 docs: ocr ledger b4-increment-fixed-5（台账首条 CLEAN a66605a83）
├─ b2dfe3b6a test: b4 ios recheck
├─ 32c293173 fix: B4 复验修复——冷启动 splash wordmark + Pods 告警抑制
└─ bf44a4671 docs: 补录 b4 修复轮全量构建日志 build.log（最终 HEAD）
```

### 3.3 关键单点引用

- B4 首个实现提交（分支侧）：`c2f5bda75`（t52 交付域策略）/`823fcc20f`（t39 park）/`34565aa41`（t60 表）；最后一个实现提交波：B4 OCR 修复 `34d3e404d` 与 iOS 修复 `32c293173`
- 迁移去重链（B4 撞号治理史）：各并行分支同 content 去重提交 `86c58a218`/`60122179f`/`a8559335e`/`d34faec0b`/`1dc0f46df`（修复 B3 期 000112/000191 撞号，t43 报告 §1 实测「旧同号文件已不存在」）→ `9c246b677`（t48 仲裁 app_publications→115/194）→ `bd36aff1d`（t60 仲裁 task_compliance→116/195、code_deliveries→117/196）→ ⚠ `6609b0de4`（t67 mobile_device_app 落 114/193，与 t60 marketplace 撞号——**当前 HEAD 仍处于该撞号状态**，见 7.2）
- opt-in 真实 HTTP 集成证据提交（B4 增量，凭据门控 SKIP 语义）：`144a924f2`(#37)、`f3652a3f1`(#39)、`aa7668624`(#40)、`3917766aa`(#43)、`50285d45e`(#45)、`2cca0b3e2`(#48)、`48878abfa`(#52)、`31d9e5b12`(#56)、`035bfd0fd`(#60)、`097bebaa9`(#67)、`59e35fa0f`(#68)

---

## 四、实际运行的测试与检查结果

**来源声明**：本节汇总自各 Ledger、集成记录、修复报告与 iOS 报告中记载的实跑命令与输出（出处逐一标注）；「报告撰写会话实测」为本报告撰写时实际运行的命令。除第 4.6 节所列两条 Go 定向测试外，本报告撰写会话未重跑任何测试套件。

### 4.1 计划级 gate 与集成验证（B1–B3，上版终报已载，此处摘要）

- 首批 t32/t33/t34 gate PASS（`npx tsx --test` 各域 + `pnpm --filter @weknora/mobile test` + typecheck；t34 另含 Go 定向四组，Ledger 原文在案）
- B3 四份集成报告全部本机实跑（Go 三组 `ok`、tsx 全绿、mobile 105→134 pass 递增、typecheck exit 0；详见 [integrate-b3-*.md](../../../.superpowers/sdd/integrate-b3-t38.md)）
- B3 增量 OCR 修复验收：TS 全量 **518 tests / 509 pass / 0 fail / 9 skipped**（skip 均 opt-in）；`typecheck:mobile` PASS；Go 失败集合与干净 HEAD 基线完全一致（当时根因 000112 撞号，已在 B4 修复）
- B3 iOS 复验两轮：8 路由 deep link 全部可达、fail-closed 文案正确、冷启动不粘连、启动日志 0 fatal（[b3-recheck.md](ios-evidence/b3-recheck.md)）；第二轮修复 `/tasks` 根 gate（`fb5f6653a`，B3 全套 519 用例 0 fail，[b3-recheck-fix.md](ios-evidence/b3-recheck-fix.md)）

### 4.2 B4 集成验证（七份集成报告记载，全部本机实跑）

| Merge | 冲突规模 | 验证（摘自各集成报告） |
|---|---|---|
| `f1ece8965`（#39） | 4 文件并集（错误码去重、TaskDetailScreen props 并存、detail.tsx 双 prop、app-smoke 三测试共存） | 计划命令逐条运行全部通过（[integrate-b4-t39.md](../../../.superpowers/sdd/integrate-b4-t39.md)） |
| `765d331c8`（#40） | 5 文件（ports 正交扩展：commands+gate+budget 并存） | 集成报告逐文件裁决 + 验证段在案（[integrate-b4-t40.md](../../../.superpowers/sdd/integrate-b4-t40.md)） |
| `c29e6162a`（#45） | 6 文件（2 Go 注释冲突 + barrel/端口/组合根并存，`KnowledgeQAEvidenceCitation` 别名解决撞名） | 同上（[integrate-b4-t45.md](../../../.superpowers/sdd/integrate-b4-t45.md)） |
| `9c246b677`（#48） | 2 注释冲突 + **迁移撞号仲裁**（app_publications 114→115 / 193→194，内容零改动 `git mv`） | 同上（[integrate-b4-t48.md](../../../.superpowers/sdd/integrate-b4-t48.md)） |
| `79f17bd7c`（#52） | 7 文件（含 app-smoke 交错大块重排、routes_workbench 双函数并存） | §三验证段本 ask 实际执行（[integrate-b4-t52.md](../../../.superpowers/sdd/integrate-b4-t52.md)） |
| `048ef39c8`（#56） | 5 文件（离线门单例与听写单例并存等） | 计划测试命令四段全部通过（[integrate-b4-t56.md](../../../.superpowers/sdd/integrate-b4-t56.md)） |
| `bd36aff1d`（#60） | 2 文件（router.go 字段级并集 + contracts 尾部导出拼接）+ 自动合并文件逐一核验 | 集成报告含一次误并集的恢复记录（`git checkout -m` 后按字段名重做）（[integrate-b4-t60.md](../../../.superpowers/sdd/integrate-b4-t60.md)） |
| `54feb7277`（#37）/`ec68d7355`（#43）/`6609b0de4`（#67）/`b06344765`（#68） | — | **无集成报告**（单行 merge 消息实测）；#43 以 [plans/plan-t43.md-report.md](plans/plan-t43.md-report.md)（testCommand 25/25 green）与 #68 以分支提交 + 合并后 OCR/iOS 全量轮替代 |

### 4.3 OCR 修复批次验收（修复报告记载，全部实跑）

- **B4 增量轮**（[ocr/fix-report-increment-b4.md](ocr/fix-report-increment-b4.md)，5 任务全完成）：
  - 基线复跑与计划声明逐项一致（task-office 四文件 56 pass / app-smoke 65 pass / mobile typecheck exit 0 / miniprogram tsc 71 错·office-views 6 错）
  - 每任务附 RED→GREEN 实跑证据（如 R1-F1 自然完成放行 parked queue-next：`0 !== 1` → 43/43；R1-F2/F3 合并口径 unknown 门：3 fail → 46/46；R1-F7 askKnowledge 离线门 office 级+端口级+组合根：13/13）
  - 附录 A 批次验收：定向 5 文件 **68 pass / 0 fail**；app-smoke **66 pass / 0 fail**；mobile typecheck **exit 0**；miniprogram tsc **71 错 / office-views 6 错（预存在集合零扩大）**；一处既有 app-smoke 断言按新语义适配并在 commit message 注明
- 历史批次（batch2 10/10、round1 13/13、b3 15/15）详见上版终报与各修复报告，此处不再重复

### 4.4 iOS 模拟器实测（iPhone 18 Pro / iOS 27.0 / Xcode 27）

- **B4 复验**（[b4-recheck.md](ios-evidence/b4-recheck.md)）：Release 增量构建 `xcodebuild … build` **BUILD SUCCEEDED、error 0、耗时 774 秒**；安装/启动正常（PID 存活）；启动日志错误筛查 **0 行命中**（仅系统 XPC 噪音）；7 条 B4 路由 deep link 全部可达且未授权 fail-closed 一致。发现 F1（首帧白屏 8–10 秒，minor）、F2（告警 4796 条未归因，minor）、F3（fail-closed 正确，info）。
- **B4 复验修复**（[b4-recheck-fix.md](ios-evidence/b4-recheck-fix.md)，提交 `32c293173` + `bf44a4671` 补录日志）：
  - F1 根因三层实证（空 SplashScreen.storyboard / RN 主线程阻塞 / surface loadingView 机制）→ config plugin `applySplashStoryboard` 注入 wordmark（复用悬空约束 id `EXPO-SplashScreen`）+ loadingView 接线（cast 目标 `RCTSurfaceHostingProxyRootView`，首版 `RCTRootView` cast 静默失败被截图定位）；录屏逐帧分析：splash 品牌帧持续约 7.4 秒后登录页出现；深链回归抽验无变化；启动日志 2065 行 fatal/crash 关键词 **0 命中**
  - F2 逐条归因（4796 = 2730 编译告警 + 107 libtool + 1959 树形重复；**应用 target 0 条**）→ Podfile `inhibit_all_warnings!`（plugin 入库）后全量口径 **4796 → 939（-80.4%）**，剩余带路径告警 466 条 100% 第三方
  - 定向 plugin 单测 **13 pass / 0 fail**；B3 口径全套 **704 tests / 689 pass / 0 fail / 15 skipped**（skip 均 opt-in）；`pnpm run typecheck:mobile` exit 0；全量重建 BUILD SUCCEEDED、0 error

### 4.5 Mimosa 安全扫描（如实）

- MCP 深度扫描（B3 期）已完成并封印：241 findings 全部为 B3 范围外既有静态发现、B3 范围 0 findings；B4 期新增两次拦截均按裁决记录处置（[mimosa-adjudications.md](mimosa-adjudications.md)），其中 `34499a96b` 明示「transport.ts:54/118 SSRF flags repeat the pre-registered client-side threat-model misjudgment ruled at 59e35fa0f; file untouched by this batch」
- 预提交钩子侧扫描（scanner_enobufs）在 B1–B4 各轮均未取得完整结论——本轮不宣称项目级安全审计完成

### 4.6 本报告撰写会话实测（本次实际运行的核验命令）

| 命令 | 结果 |
|---|---|
| `git log --oneline 29c1e5635..bf44a4671 \| wc -l` | **344** |
| `git log --oneline 4f71e9d97..bf44a4671 \| wc -l` | **162** |
| `git log --merges --oneline 29c1e5635..bf44a4671` | 20 个 merge（清单见 3.2） |
| `git branch \| grep -c codex/issue30` | 21 |
| `git branch -r \| grep -i issue30` | 无输出（未推送） |
| `ls migrations/sqlite \| awk -F_ '{print $1}' \| sort \| uniq -c \| awk '$1>2'`（versioned 同款） | **sqlite 000114（4 文件）、versioned 000193（4 文件）——同号双迁移在最终 HEAD 存在** |
| `go test ./internal/application/repository/ -run 'TestAgentRunAdmissionIdempotent' -count=1`（走全量迁移轨道 `openRunTestDB`） | **FAIL：`failed to open source, "…/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql`** |
| `go test ./internal/application/repository/ -run 'TestAgentAdoptionRepositoryResolvesIntroducedListingAndRelease' -count=1`（direct-DDL 夹具，绕开迁移轨道） | ok（2.968s）——证明部分 B4 测试因夹具绕轨而通过，与上一行的失败形成对照 |
| `grep -cE '^### Task [0-9]+' plans/plan-t*.md` | 23 份计划任务数合计 183（逐项见 1.1/二节） |
| `git ls-files docs/plans/issue30-sweep/ocr/` + `git status --short` | `ocr-increment-b4.md` 未跟踪；其余 OCR 报告与台账均已入库 |

### 4.7 未运行的检查（如实声明）

1. **opt-in 真实 HTTP 集成证据未在带凭据环境运行**：23 个 Issue 的 `WEKNORA_MOBILE_TEST_*`/`NOTION_TOKEN` 等凭据门控用例在本环境一律 SKIP。「具备真实 HTTP 集成证据」仅指证据代码与 skip 语义已落地并被测试钉住。
2. **授权面交互未验证**：iOS 模拟器实测只覆盖构建/安装/启动/首屏/未授权 gate 路由可达性/冷启动深链；idb UI 自动化后端不可用且无凭据，未伪造任何交互证据。
3. **本报告撰写会话未重跑全量测试套件**：仅运行 4.6 所列核验命令（含两条 Go 定向测试——其一证明迁移轨道破窗、其一为对照）。
4. **PostgreSQL 迁移复跑与 Android 侧任何验证**：从未在本轮运行（无环境）。
5. **Mimosa 钩子侧扫描**从未取得完整结论（4.5）。

---

## 五、审查与 OCR 结论、修复轮次、并行波次与集成记录

### 5.1 Superpowers SDD 审查链

- **派发前冲突扫描**：四批计划均有逐任务接口/文件冲突扫描记录；判定 blocking 的计划（#34/#35/#36/#38/#42/#59 + B4 的 #37/#39/#43/#45/#52/#60/#67/#68）先修订或按扫描结论继续执行并在任务审查重点核对（Ruling 5/8/16–19/31–38）。
- **计划审查**：B1–B3 残留（#34 7 项、#35 4 项、#36 1 项、#42 6 项、#46 9 项、#59 3 项）与 B4 残留（**#43 5 项、#60 3 项、#68 10 项**，Ruling 28–30）均作为 Review Focus 传入任务审查，未阻塞执行。
- **任务级审查**：各分支含 fix round 提交（B4 例：t52 的 `1f5235138` 路径穿越封堵、`4fa87aa01`/`5cb095559`；t60 的 `75a8c9dbb`；t39 的 `e7e6c37a0`/`c6ec3f3c1`；t40 的 `621826ece`/`9c066d01b`/`d89f089e5`；t56 的 `044117749`；t68 的 `a5031f3b2`；t43 的 `869e5fce9`/`d7e39c424`(ruling via escalation) 等）。
- **整计划最终审查**：18/23 finalApproved=true；**#32/#37/#40/#52/#67 五个为 false**——终审发现均已修复（各 final-fix-report.md 在案）但按 SDD 规则修复后不再开第二审查波，残留/修复未经独立复审（详见 1.1）。
- **审查升级裁决**：多处以「ruling via escalation」落地（如 `d7e39c424` 端口收窄、`cab1a90f1` 简报自相矛盾修正、`c71b1f37a` B3 跟进修复、`59e35fa0f` Mimosa 误判放行）。
- **task-brief 提取失败 15 次**（Ruling 20–25、39–47）：实现者直接读计划文件对应任务节替代，代价为实现者上下文略宽。

### 5.2 OCR 五轮（batch2 / round1 / b3 / b4 + 台账）

| 轮次 | 扫描结论 | 修复 | 覆盖缺口（如实） |
|---|---|---|---|
| 第二批增量 | complete：46 findings / 29 items | 10 任务（10 提交） | 附录 A 延期项（F4/F6/F11/F23/F28/F35/F46） |
| 最终轮 round1 | **partial**：50 findings；28/62 items 因 429 限流失败 | 13 任务（13 提交） | 28 items 未审；R1-F17/F20 部分修复（延期） |
| B3 增量 | complete：88 findings / 95 items | 15 任务（15 提交），6 项 high 全修复 | 2 项未决（Ruling 26）；附录 A 延期项 |
| B4 增量 | **partial**：9 findings；**145/150 selected items 因 429 限流未获审查**（retry report：53/84 请求受影响，36 失败 17 恢复） | 5 任务 / 5 提交，9 项发现全处置（R1-F1…F4、F6…F9，逐项 RED→GREEN） | **145 items 未审**——本轮最大 OCR 覆盖缺口；台账 CLEAN 行口径限定见第二节 |
| 台账 | [ocr-ledger.md](ocr/ocr-ledger.md) 首条 `CLEAN a66605a83 \| b4-increment-fixed-5`（提交 `8cd469c51`） | — | B1–B3 各轮未回填台账行（台账建立晚于这些轮次） |

**重扫政策**：最终轮按用户指示缩减为 1 轮（Ruling 10），B3/B4 单轮封顶（Ruling 26/29 惯例）——四轮修复增量（13+10+15+5 提交）均未被二次 OCR 覆盖，以各修复报告内定向复审与全量回归兜底。

### 5.3 并行波次与集成记录（四批）

- **波次 1 / B1**（Ruling 2）：#32 单节点串行段。
- **波次 2 / B2**（Ruling 6）：#33/#34/#35/#66 同批全并行、独立 worktree/分支；#32→#33→#34 落为线性段，t35/t66 后并（2 merge）。
- **波次 3 / B3**（Ruling 11）：7 节点全并行，按序 7 merge；4 份集成报告在册（t41 的 `/inbox` 撞路径分流为代表性产品级集成决策）。
- **波次 4 / B4**（Ruling 27）：**11 节点全并行——历次最大批次**（t37/t39/t40/t43/t45/t48/t52/t56/t60/t67/t68，独立 worktree 与分支），按序 11 merge；7 份集成报告在册，4 个 merge 无集成报告（如实声明，见 4.2）；集成期迁移撞号仲裁两次（t48、t60）+ 一次漏网（t67 的 mobile_device_app，见 7.2）；Mimosa 拦截两次按裁决记录放行；批后增量 OCR（partial，已修复并记台账）与条件性 iOS 复验（复验+修复完成）。
- **OCR/iOS 修复波**与实现串行收尾；B3 第二轮 iOS 复验与修复发生在上版终报之后、B4 计划之前（`98a7f98d4`/`fb5f6653a`）。

---

## 六、裁决记录（全部 47 条 Ruling，含判断错误时的代价）

1. **模型路由替代**：用户指定 gpt-5.6-sol/terra/luna 分级路由，但本运行环境子代理只能运行会话模型（GLM-5.3），全部子代理实际使用会话模型；架构与审查类任务通过独立 fresh-eyes 双代理交叉制衡补偿——代价：关键决策深度不足，可由人工复核最终报告发现。
2. **执行并行化（用户指示）**：同一 DAG 批次且计划文件范围不重叠的计划并行实施，各自使用独立 worktree 与分支，完成后按序 merge 集成；计划内部任务保持串行（SDD 同文件冲突规则）——代价：merge 顺序决定最终提交拓扑，冲突时需集成修复轮。
3. **计划 #34 审查第 2 轮未全通过（7 项残留）**：残留项作为任务审查的额外关注点传入 Review Focus——代价是实现阶段可能暴露这些缺口并触发修复轮。
4. **#32 最终审查修复波后仍有残留**（important：`disallowedDeploymentHost` 可被 IPv6 形式绕过 → ADDRESSED：实测 `mobileRuntimeIntegrationConfig` 对 `https://[fe80::1]`、`[fc00::1]`、`[fd12:3456:789a::1]`、`[::ffff:7f00:1]` 全部返回 `enabled:false/disposition:'invalid'`（运行报告同款 tsx 复现脚本）；hostname 含 `:` 时经 `parseIpv6Literal` 展开为 16 字节，`runtime-integration-smoke.ts:45-75`）——按 SDD 规则不再有第二波，残留呈报用户；代价是该 Issue 不能宣称完全干净。
5. **计划 #34 冲突扫描判定 blocking**（阻塞①·T6 自洽：Task 6 测试断言与自己的实现矛盾——`normalizeQuery` 仅 `search.trim().slice(0,200)`（plan-t34.md:1908-1919），测试期望 `search:'quarterly review'`（plan-t34.md:1764-1768），实跑 `node -e` 验证 `'   quarterly   review '.trim()` 内部多空格保留 ≠ 期望，须先修订；阻塞②·T8 自洽：断言用 `filter(({type})=>type==='Text')` 检查 `'View all tasks'`，但该文案是 Button 的 title 属性（plan-t34.md:2418-2420 vs :2595；react-native 桩把 Button 渲染为 `{type:'Button',props:{title}}` 无 Text 子节点，apps/mobile/src/app-smoke.test.tsx:23、:60-68 实读核实），断言必为 false）——已按扫描结论先修订再执行；代价：可能触发额外修复轮。
6. **第二批并行实施（用户指示）**：#35、#66 同 DAG 批次节点全部并行（不再因 filesTouched 重叠降级串行），各自独立 worktree 与分支，重叠文件由集成修复员按双方意图解决——代价：merge 冲突概率上升；若集成失败该节点如实记受阻。
7. **第二批计划 #35 审查第 2 轮未全通过（4 项残留）**：残留项传入任务审查关注点——代价：实现阶段可能触发修复轮。
8. **计划 #35 冲突扫描判定 blocking**（T1→T2：`(*WorkbenchListStore).ReadTaskFactsForRun` 签名与 `OwnedTaskFactsReader` 接口/容器 `.WithTaskFacts(lists)` 消费一致（workbench_list.go:135/attentionOf:49-58、workbench_read_test.go:31-35 核实）；T2→T3→T8：`TaskSnapshotFacts` snake_case wire 形状与解析器/detail 映射一致（contracts.go:52-58、executions.ts:174/282 核实））——继续执行并在任务审查重点核对；代价：可能触发额外修复轮。
9. **第二批增量 OCR 有 1 项未决**：单轮封顶不再重扫，随最终全量轮与报告核销——代价：个别问题可能带入最终交付。
10. **OCR 最终轮缩减为 1 轮（用户指示）**：本批发现修复并定向复审后不再重扫——代价：修复增量未被二次 OCR 覆盖，以复审裁决与最终报告兜底，残留如实呈报。
11. **第三批 B3 并行实施（用户指示）**：#36、#38、#41、#42、#44、#46、#59 同批次 7 节点全部并行（独立 worktree 与分支），批后增量 OCR 与条件性 iOS 复验——代价：merge 冲突链较长、运行时长与配额消耗显著。
12. **B3 计划 #36 审查第 2 轮未全通过（1 项残留）**：残留项传入任务审查关注点——代价：可能触发修复轮。
13. **B3 计划 #42 审查第 2 轮未全通过（6 项残留）**：同上。
14. **B3 计划 #46 审查第 2 轮未全通过（9 项残留）**：同上。
15. **B3 计划 #59 审查第 2 轮未全通过（3 项残留）**：同上。
16. **计划 #36 冲突扫描判定 blocking**（T1→T2：coordinator.resume 与 `SubmissionStore.load/save`、`SubmissionConflictError(requestId, storedDigest, incomingDigest)`（submission.ts:52-60）完全匹配；T1→T5：`recommendLeadAgent` 经 `export *` 聚合 barrel（packages/domain/src/mobile/index.ts:1-15）导出链路成立）——继续执行并重点核对；代价：可能触发修复轮。
17. **计划 #38 冲突扫描判定 blocking**（T1→T2：`Service.ListInbox`/`GormInteractionStore.ListPending` 与现有代码（service/workbench/interaction.go:416/:151/:438/:26-36）衔接一致；T2→T4：GET /api/v1/workbench/interactions 路由与 502+command_recovery_unknown 错误 wire 及客户端分类逐字一致）——继续执行并重点核对；代价：可能触发修复轮。
18. **计划 #42 冲突扫描判定 blocking**（T1↔T2：`repository.TaskGrantStore` 五方法与 `TaskGrantStorePort` 消费同签名（plan:101-106/826-832）；T1↔T3：容器 wiring 以 `*repository.TaskGrantStore` 构造 service（plan:1369-1375）吻合）——继续执行并重点核对；代价：可能触发修复轮。
19. **计划 #59 冲突扫描判定 blocking**（Task1→Task2：sqlite 000112 迁移经 `openRunTestDB` 全量迁移流消费（agent_run_test.go:29-61），实体字段齐备；Task1→Task3：`openAgentVersionServiceTestDB` 同样应用全量迁移流（agent_version_test.go:31-63））——继续执行并重点核对；代价：可能触发修复轮。
20. **task-brief 脚本提取任务 5 失败**，实现者直接读计划文件对应任务节——代价：实现者上下文略宽。
21. **task-brief 脚本提取任务 6 失败**——同上。
22. **task-brief 脚本提取任务 7 失败**——同上。
23. **task-brief 脚本提取任务 5 失败**（第二批）——同上。
24. **task-brief 脚本提取任务 6 失败**（第二批）——同上。
25. **task-brief 脚本提取任务 7 失败**（第二批）——同上。
26. **B3 增量 OCR 有 2 项未决**：单轮封顶不重扫、不记台账，随最终报告呈报——代价：个别问题可能带入最终交付。
27. **第四批 B4 并行实施（用户指示）**：#37、#39、#40、#43、#45、#48、#52、#56、#60、#67、#68 前置已全部满足，同批次全部并行（独立 worktree 与分支），重叠冲突由集成修复员解决；批后增量 OCR 与条件性 iOS 复验——代价：批次规模 11 节点为历次最大，运行时长与配额消耗显著（预计多次配额窗口暂停后 resume）。
28. **B4 计划 #43 审查第 2 轮未全通过（5 项残留）**：残留项传入任务审查关注点——代价：可能触发修复轮。
29. **B4 计划 #60 审查第 2 轮未全通过（3 项残留）**：同上。
30. **B4 计划 #68 审查第 2 轮未全通过（10 项残留）**：同上。
31. **计划 #37 冲突扫描判定 blocking**（Task1↔Task2/4：迁移链 000112/000191 双号文件实查、测试基建 openAdmissionConcurrencyDB/openWorkbenchHTTPDB 实查存在；Task2↔Task3 共享 interaction.go 与 container/workbench.go 不重叠区段顺序执行，Task 3 容器编辑依赖 Task 2 先行）——继续执行并重点核对；代价：可能触发修复轮。
32. **计划 #39 冲突扫描判定 blocking**（Task1↔Task2：park 语义 `waiting_user+wait_reason='budget_exhausted'` 字面量与 RequeueBudgetPausedRuns 谓词一致（agent_run_worker.go:340、agent_run_decisions.go:255-271 核实）；Task1↔Task5：同包测试 helper 名无冲突）——继续执行并重点核对；代价：可能触发修复轮。
33. **计划 #43 冲突扫描判定 blocking**（Task3↔Task4/5 接口时序矛盾：端口七方法含 PurgeTask（plan-t43.md:1098）而 store.PurgeTask 到 Task 6 才实现——Task 4/5 的 `go build ./...` 必报 missing method，须先修订；Task0→Task1-7 迁移轨道顺延 #59 四文件自洽）——继续执行并重点核对；代价：可能触发修复轮。
34. **计划 #45 冲突扫描判定 blocking**（Task1→Task2/3：`AnswerEvidence/EvidenceState*/EvidenceKind*/EvidenceReasoning*/ValidateAnswerEvidence` 与 `CitationsFromSearchResults/ConclusionFromNativeAnswer` 名称与签名逐一比对一致）——继续执行并重点核对；代价：可能触发修复轮。
35. **计划 #52 冲突扫描判定 blocking**（Task1→Task2：`RepoRef/FileChange/GitBlobSHA/TaskBranchOf` 与端口/模拟器消费同签名；Task1→Task3：`DeliveryState` 六常量与 store 镜像逐值一致）——继续执行并重点核对；代价：可能触发修复轮。
36. **计划 #60 冲突扫描判定 blocking**（T1→T2：改名让出版本号后 000193/000114 无碰撞；T1→既有 harness：duplicate 000112 修复后 Task 5/6 的全量迁移测试才可运行，计划顺序 Task 1 最先一致）——继续执行并重点核对；代价：可能触发修复轮。
37. **计划 #67 冲突扫描判定 blocking**（阻塞：Task 7 foreground-sync.test 第一用例死锁——`script.emit('active')` 同步触发 runOnce 时 gate 尚未赋值，promise 永不 resolve，须先修订；阻塞：Global Constraints 的 host 拒绝承诺与 Task 2 `validPushEndpoint` 只校验 scheme/host 非空、且全部测试依赖 httptest 环回 endpoint 两种读法不可同时满足，须先修订）——继续执行并重点核对；代价：可能触发修复轮。
38. **计划 #68 冲突扫描判定 blocking**（阻塞：T3 Step 4 `const network: WeappNetwork` 未导出而 T4 `import { network }` 必报 no export named network，须改 `export const network`；阻塞：T4 从 barrel 裸名导入 InboxItem 实为 #41 通知收件箱类型，`item.runId` 类型不符，须改用 `AttentionInboxItem` 别名）——继续执行并重点核对；代价：可能触发修复轮。
39. **task-brief 脚本提取任务 5 失败**（B4 期）——实现者直接读计划文件对应任务节；代价：实现者上下文略宽。
40. **task-brief 脚本提取任务 6 失败**——同上。
41. **task-brief 脚本提取任务 7 失败**——同上。
42. **task-brief 脚本提取任务 8 失败**——同上。
43. **task-brief 脚本提取任务 8 失败**——同上。
44. **task-brief 脚本提取任务 10 失败**——同上。
45. **task-brief 脚本提取任务 9 失败**——同上。
46. **task-brief 脚本提取任务 10 失败**——同上。
47. **task-brief 脚本提取任务 11 失败**——同上。

---

## 七、未完成节点、遗留风险、延期项与需用户决策

### 7.1 未完成 / 受阻节点

- **17 个下级 Issue 未实施**（#31 复验 + B5 全部 11 个 + B6 3 个 + #65 + #71，见 1.2）；B5 已整批解锁。
- **授权面端到端验证缺口**：23 个 Issue 的授权交互与全部 opt-in 凭据门控集成证据未在带凭据环境运行（无凭据 + idb UI 自动化不可用）。
- **B4 四个 merge 无集成报告**（#37/#43/#67/#68，单行 merge 消息实测）；B1–B4 除首批三份外全部计划的 `progress.md` 未持久化——过程审查记录缺失，以分支提交、计划级报告（t43/t48/t52）与合并后全量轮替代。

### 7.2 遗留风险

- **⚠ migrations 撞号在最终 HEAD 复发（本报告撰写时实测）**：`migrations/sqlite` 的 `000114_mobile_device_app.*`（t67 引入，`fb9037710`）与 `000114_public_agent_marketplace.*`（t60 引入，`34565aa41`）同号并存；`migrations/versioned` 的 `000193_*` 同样双占。**本次报告会话实测**：`go test ./internal/application/repository/ -run 'TestAgentRunAdmissionIdempotent' -count=1` → FAIL `duplicate migration file: 000114_public_agent_marketplace.down.sql`——任何走全量迁移轨道的 Go 测试夹具当前不可用。机制：B4 各并行分支独立选号，t60 merge（`bd36aff1d`）仲裁了当时已知的撞号（task_compliance→116/195、code_deliveries→117/196、自留 marketplace 114/193），其后合入的 t67（`6609b0de4`）把 mobile_device_app 带在相同号上，git 对双侧新增文件不报冲突，集成未再扫描迁移目录。B4 各分支上的同 content 去重提交（`86c58a218` 等 5 个）已把 B3 期的 000112/000191 撞号修复（t43 报告 §1 实测验证），**即旧债已还、新债又生**；B4 期部分测试通过是因为部分夹具改用 direct-DDL 绕开迁移轨道（对照实测见 4.6）。**这是当前最需要用户裁决的技术债**（同 7.5 决策 2）。
- **五个 Issue（#32/#37/#40/#52/#67）终审修复未经独立复审**：修复证据在案（final-fix-report.md 各份，含 RED→GREEN），但 finalApproved=false 且无第二审查波——不能宣称完全干净。
- **OCR B4 轮 145/150 items 未审**（429 限流）+ 台账 CLEAN 行口径限定（第二节）+ 四轮修复增量未被二次 OCR 覆盖（Ruling 10/26）——OCR 总覆盖存在实质性缺口。
- **`ocr-increment-b4.md` 未提交入库**（worktree 未跟踪文件实测；台账行引用的正是该文件路径，文件本身目前不在 git 历史中）。
- **Mimosa 钩子侧扫描从未取得完整结论**（scanner_enobufs）；两条误判裁决已持久化（[mimosa-adjudications.md](mimosa-adjudications.md)）并附三条上游反馈建议；本轮不宣称项目级安全审计完成。
- **iOS Fabric 白屏根因未修**：以 `newArchEnabled:false` 规避（RN 0.83.10 + iOS 27.0 组合问题，上游超本仓范围）；B4 期 splash 白屏以 wordmark 方案缓解（非消除 bundle 加载期）。
- **miniprogram `@weknora/mobile-core` workspace 链接缺失**：71 个预存在 typecheck 错误的根源（B4 修复报告 §9 如实记录，属独立决策未动）；链接修复后 office-views 穷举缺键恢复为编译错误（本批已补 `'offline'` 键）。
- **MCP ios-simulator ui backend（idb）不可用**：launchd PATH 修复需重启 ZCode 才生效（未执行）；fb-idb 1.6.1 `idb --version` 探测缺陷需上游修复。
- **Metro/localhost 联调受阻**：宿主 VPN/TUN 全局代理（127.0.0.1:17890）拦截模拟器 localhost 流量，未改动用户网络环境。

### 7.3 口径记录（如实呈报）

- **任务计数口径**：23 个 Issue 任务数以计划文件 `### Task N` 标题计数为准（本次实测 183），与编排器统计一致；#35 曾有的「6/6 vs 11」口径偏差已在上版终报收敛。
- **finalApproved=false ≠ 未完成**：五个 Issue 的 tasksDone=tasksTotal 且终审发现已修复；false 反映的是「修复后无复审波」这一流程事实。
- **Go 失败计数基线口径变化**：B3 期的「失败集合不扩大」基线（135/276/4/27）根因是 000112 撞号，B4 已修复该根因；但最终 HEAD 因 000114/000193 新撞号，全量迁移轨道再次不可用（本次实测）——两件事不可混为一谈。
- **台账 CLEAN 行不是完整审计结论**：见第二节口径限定。

### 7.4 延期低优先级事项（计划明示、未实现）

- OCR Round 2 范围：SQLite 持久 TaskProjectionStore（R1-F20 完整版，需存储选型 ADR）、R1-F17 行存储后端、R1-F26 领域决策等（见各修复报告附录 A）；B4 轮无延期项（9 项全处置）。
- `interruption.message` 整体渲染决策（B4 修复报告 §9，offline 文案已承载同等信息）。
- Pods 告警进一步归零（Expo Swift clang-importer 约 263 条可用 `OTHER_SWIFT_FLAGS += -Xcc -w`，可选优化未做）。
- #58 PostgreSQL 迁移复跑（000187/000188，环境性）。
- Mimosa 深度扫描 241 条范围外既有 findings 的逐条处置；Mimosa 上游规则豁免反馈（迁移测试脚手架注入误判、客户端 SSRF 误判）。

### 7.5 需要用户决策

1. **模型路由替代**：本轮全部子代理实际使用会话模型（Ruling 1）——请人工复核本报告与关键决策（尤其 #32 IPv6 残留、B4 各计划 blocking 修订、t41 `/inbox` 撞路径分流、t48/t60 迁移撞号仲裁、两处 Mimosa 误判放行）以补偿路由深度缺口。
2. **⚠ migrations 000114/000193 撞号处置（最紧急）**：是否授权独立任务重编迁移序号（建议 mobile_device_app 顺延为 sqlite 000118 / versioned 000197 或与用户确认的下一可用号），并恢复 Go 全量迁移轨道；是否在集成流程中固化「每次 merge 后断言双轨迁移版本号唯一」（t48 收尾报告 §1 曾实跑过该 shell 断言，未纳入常驻门禁）。
3. **五个 finalApproved=false Issue 的处置**：接受各 final-fix-report.md 的 RED→GREEN 证据视为关闭，还是各安排一次独立复审波次。
4. **OCR 台账与补扫**：是否维持以 `a66605a83` 为 CLEAN 基线续审（并回填 B1–B3 各轮行），是否对 B4 轮 145 个未审 items、历史缺口与四轮修复增量补跑 OCR（建议低峰期分批以避开 429 限流）；是否补提交未跟踪的 `ocr-increment-b4.md`。
5. **授权面验证环境**：何时提供 `WEKNORA_MOBILE_TEST_*`/`NOTION_TOKEN` 等凭据环境（官方云或自托管 staging）以运行 23 个 Issue 的 opt-in 集成证据与 iOS 授权面交互。
6. **下一批次**：B5 的 11 个节点（#47/#49/#50/#51/#53/#54/#57/#61/#62/#69/#70）前置已全部满足（1.2 逐节点核对），是否启动及其并行策略；#69/#70 的签名/真机与 #48 同类外部凭据阻塞是否先行解决。
7. **分支去向**：`codex/issue30-mobile-office`（及 20 条 t* 并行分支）是否推送远端、开 PR、合并 `main`；对应 23 个 GitHub Issue 的关闭时机。
8. **iOS 环境后续**：是否重启 ZCode 使 launchd PATH 修复进入 MCP 进程；`sudo launchctl config user path` 持久化；fb-idb 探测缺陷上游跟进。
9. **安全审计**：是否安排一次完整的项目级深度安全扫描（钩子侧 enobufs 从未通过）；Mimosa 上游规则豁免反馈是否提交。

---

## 交付声明

本轮全部成果（**344 个提交**，`29c1e5635..bf44a4671`：23 个 Issue 的实现（Go 后端 + 移动端/小程序全栈）、四个并行波次共 20 个集成 merge、五轮 OCR 修复共 43 个修复提交、四轮 iOS 实测与修复、全部文档证据与两份 Mimosa 裁决）均为**本地 worktree 分支 `codex/issue30-mobile-office` 及并行分支 `codex/issue30-t*` 上的本地提交**：**未推送远端、未合并到 `main` 主分支、未关闭任何 GitHub Issue**（#30–#71 全部保持原 open 状态，#58 维持既有 closed）。本报告主仓库副本（`/Users/wuyongjun/trea/WeKnora-fork01/docs/plans/issue30-sweep/FINAL-REPORT.md`）仅为文件落盘，不改变主仓库 git 状态。
