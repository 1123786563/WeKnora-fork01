# Issue #140 终局交付报告

- 报告日期：2026-09-28（Asia/Shanghai）
- 报告人：#140 动态工作流·终局报告员
- 交付范围：集成工作区 `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`，分支 `codex/issue-140-integration`（**全部为本地提交：未推送、未开 PR、未合并、未上线**）
- 事实源（优先级从高到低）：批准 Spec `docs/specs/2026-09-23-weknora-job-search-design.md` > 主计划 `docs/plans/2026-09-24-issue-140-implementation.md` > ADR 0015–0018 > `CONTEXT.md` > GitHub Issue 文本
- 证据核查方式：本报告员于 2026-09-28 在集成工作区实跑 `git log`/`git status`/`grep`/`sed` 等只读命令核对；凡引用主控终验门结果处均注明"主控 ask 材料载明"，本报告员未重跑全量验证门（如实声明，不冒充）

---

## ① 33 个子 Issue 与 #140 总体验收状态

状态依据：DAG 状态表（`docs/plans/issue-140/2026-09-24-issue-140-dag.md` 节点表，最后一次更新于 Wave 16 集成提交 dc2558355）+ SDD 台账终态记录（progress.md Wave 15–17 与 OCR 轮集成段）。台账 Wave 16 明确：running/pending 的终态翻牌属主控 Step 4 终裁，调度员/集成员不代裁；下表"建议终态"为台账随 T33 终裁材料提交主控的建议口径，尚未经主控落定。

| 任务 | 子 Issue | DAG 状态 | 验收要点与证据（来源） | 建议终态（台账，待主控终裁） |
| --- | --- | --- | --- | --- |
| T01 | #143 | **blocked** | 鸿蒙原生闸口：本机无 SDK/DevEco/hdc/hvigor/设备/.hap（台账 97 行）；blocked 证据 commit 080cf7d47 经独立评审后集成为 5d6904c24；`docs/plans/issue-140/verification/harmony-native-gate.md` 在档 | blocked（维持） |
| T02 | #145 | running | iOS 模拟器受认证 Task 读取证据链完整评审通过（task-2-live-ios-validation.md、task-2-evidence-ios/、R2 评审）；Android 无 adb 为永久环境缺项（台账 105/188 行） | verified-with-environment-gate |
| T03 | #141 | verified | Career 合同测试+SQLite 本地启动+HTTP 授权检查+浏览器 E2E；集成 `pnpm test:web` 2309/2309（DAG 表） | verified（已落） |
| T04 | #142 | verified | 隔离 Lite 服务真实下载 SHA-256 比对；越权/过期/篡改/撤销拒绝（task-4-live-http-validation.md） | verified（已落） |
| T05 | #144 | pending | Expo 移动 Task Office：未启动，被 T01/#143 blocked 传导（台账 312 行） | blocked |
| T06 | #148 | verified | 真实微信开发者工具全链路复验、产物断言 62/62、typecheck 仅 13 个基线预存错误（task-6-live-validation.md、wave-report-T06修复.md） | verified（已落） |
| T07 | #147 | verified | 上传/抽取/确认链：最终验证 b09919c3f，浏览器全流程证据+真实 DocReader 容器验收（task-7-final-validation.md） | verified（已落） |
| T08 | #146 | verified | JD 快照/重放/恶意 JD 合同测试+Web E2E（DAG 表） | verified（已落） |
| T09 | #149 | verified | 来源完整性五分类固定响应契约；集成门 go 6 包全绿、test:web 2363/2363（wave-report-T09后端.md/-Web.md） | verified（已落） |
| T10 | #150 | verified | 三值资格：临时 SQLite+真实浏览器验证 2026 不符合/2027 符合/旧评估不变/390px 无溢出（task-10-live-browser-validation.md） | verified（已落） |
| T11 | #152 | verified | 一次性找岗：后端 Wave3+Web Wave4 集成，test:web 2389/2389（wave-report-T11*.md） | verified（已落） |
| T12 | #151 | verified | 去重/更新差异/历史快照：后端第 4 轮+Web Wave14 集成；聚焦门 80/80+50/50（全量因 CPU 争用欠账，后由 OCR r2 web 组串行全量 2503/2503 exit 0 偿还，台账 326 段） | verified（已落） |
| T13 | #154 | verified | 持续规则：后端 Wave7+Web Wave8；E2E 8 截图+server.log 交叉证实；test:web 2430/2430 | verified（已落） |
| T14 | #155 | verified | 申请关联 Task：后端子任务 1+2+Web（浏览器 E2E 12 相位评审核验真实渲染）；test:web 2378/2378 | verified（已落） |
| T15 | #153 | verified | 结构化材料：后端 Wave4+Web Wave5 第 2 轮复审；test:web 2401/2401 | verified（已落） |
| T16 | #158 | verified | 同版 PDF/DOCX：真实渲染器验收；E2E 16 相位（双格式 digest、撤销 404、幂等重放）；test:web 2420/2420 | verified（已落） |
| T17 | #157 | verified | 追加事件/阶段投影：后端 Wave5+Web Wave6；test:web 2412/2412 | verified（已落） |
| T18 | #159 | verified | 本人投递确认：后端 Wave8+Web Wave9；归档证据+HTTP 幂等探针；test:web 2442/2442 | verified（已落） |
| T19 | #156 | verified | 求职信/面试准备：15 相位浏览器 E2E+HTTP 探针，评审独立复跑 2459 全绿；集成偿还 F1（careerPurgeTables 补 preparations，56c164d4a） | verified（已落） |
| T20 | #160 | verified | 站内待办/隐私通知：隐私字面量与 reminder.go 逐项一致；test:web 2466/2466 | verified（已落） |
| T21 | #161 | verified | 额度准入：聚焦门 career 四页 82/82+改动文件 27/27+api-client 43/43（全量因 CPU 争用欠账，偿还口径同 T12） | verified（已落） |
| T22 | #162 | verified | 导出/删除：联合合同测试+Web E2E；test:web 2449/2449 | verified（已落） |
| T23 | #163 | pending | Expo 移动端 C 找岗/建档/分享：未启动，T01 传导（台账 312 行） | blocked |
| T24 | #164 | verified | 小程序建档/分享导入：F1/F2/F3 修复经独立复核（单测 90/90、DevTools 复验 14/14、6 截图 6 哈希）（wave-report-T24修复.md） | verified（已落） |
| T25 | #165 | pending | Expo 移动端申请/材料/投递：未启动，T01 传导 | blocked |
| T26 | #166 | verified | 小程序申请到投递：真实 DevTools（端口 57817）全链 18/18、fixture 23/23、7 截图 sha256 与 manifest 一致；miniprogram 118/118 | verified（已落） |
| T27 | #167 | pending | Expo 移动端时间线：未启动，T01 传导 | blocked |
| T28 | #169 | running | 小程序时间线/按需准备：功能与可达环境（DevTools）证据链评审通过；F3 真机/人工录屏验收面为环境门槛保留（台账 278 行） | verified-with-environment-gate |
| T29 | #168 | pending | Expo 移动端规则/额度/提醒：未启动，T01 传导 | blocked |
| T30 | #170 | running | 小程序规则/用量/订阅：四条验证命令评审实跑全过（test 172/172、build:weapp 编译成功）；真机与订阅弹层真实环境如实 blocked（简报允许）；3 medium 主控知悉项（台账 d6b3e1bb5 终态段） | verified-with-environment-gate |
| T31 | #171 | pending | Expo 移动端导出/删除：未启动，T01 传导 | blocked |
| T32 | #173 | verified | 小程序导出/删除：M1 主按钮门控修复+M2/M3 证据补档；修复轮第 2 轮复审 138/138；miniprogram 157/157（wave-report-T32修复.md） | verified（已落） |
| T33 | #172 | running | 五环境终局验收：launch-matrix.md + source-coverage.md + 证据包已集成（8997b212e）；Wave16 修复轮闭环 2 medium（17×HTTP 200 回执原文在档+PDF SHA-256 自洽）；评审认定不阻塞终裁；**verified 待主控 Step 4 终裁（43 条故事覆盖核对）**；余 4 low+3 low 欠账披露不阻塞（台账 dc2558355 终态段） | verified（材料齐备，待终裁） |

**DAG 终态统计（台账 8997b212e/dc2558355 段 grep 实核，34 节点含 T00 前置）**：verified 23 节点（T00 + 22 个子 Issue）、running 4（T02/T28/T30/T33）、pending 6（T05/T23/T25/T27/T29/T31）、blocked 1（T01），23+4+6+1=34 无遗漏。

**#140 总体验收状态**：本地实现、集成与验证门全部完成——22 个子 Issue 已 verified、4 个 running 的可推进工作全部完成并集成评审（终态翻牌待主控终裁）、7 个受环境传导的票如实 blocked/pending；主控终验门 go exit=0、web exit=0（双门并行，主控 ask 材料载明）；OCR 三轮范围审查的修复已全部集成（dc2558355→77917de10，19 提交），但 **OCR 仍有未消除 findings（见⑥）**；**未推送、未合并、未上线**；鸿蒙（无工具链）与 Android（无设备/adb）环境门槛如实 blocked。本报告不声称 #140 已在 GitHub 关闭。

## ② DAG 与计划/台账路径

| 材料 | 路径（均在集成工作区内） |
| --- | --- |
| 递归摄取与执行 DAG（33 子 Issue 状态表、Mermaid、10 拓扑波次、43 条用户故事映射） | `docs/plans/issue-140/2026-09-24-issue-140-dag.md` |
| 主实施计划（plan commit 7041e38ab） | `docs/plans/2026-09-24-issue-140-implementation.md` |
| SDD 台账（330 行，75 条 Ruling） | `.superpowers/sdd/2026-09-24-issue-140-implementation/progress.md` |
| 批准 Spec | `docs/specs/2026-09-23-weknora-job-search-design.md` |
| 各票任务报告/评审/验证证据 | `docs/plans/issue-140/`（task-*-validation.md、wave-report-*.md、reviews/ 等） |
| T33 终局验收材料 | `docs/plans/issue-140/verification/`（launch-matrix.md、source-coverage.md、harmony-native-gate.md） |
| OCR 三轮范围审查报告 | `docs/plans/issue-140/ocr/`（round 1–3 各含 resume 版 + ocr-workspace.md） |

DAG 完整性自检（DAG 文件自检节）：33 个唯一子 Issue、33 条 contains 边、Kahn 拓扑覆盖 33/33 节点、0 循环、0 遗漏、10 个波次。

## ③ 并发波次与集成记录（本工作流 + 之前 codex 会话）

### 阶段一：codex 会话串行/修复轮阶段（f7753fa → 21df162a2，148 提交，2026-09-24）

计划入库（b6162c117 设计源导入 → 7041e38ab 主计划）后：T00 Web TDesign 前置（集成提交 197d93eeb）→ T01 blocked 证据（5d6904c24）→ T02 实现与两轮修复（13a8e1dc9/75eb5dcfe/f8f15fedc）→ T06 三轮修复后集成（4b9b550c2 等 4 提交）→ T03 后端 5 轮修复（round-4 起升级 default/gpt-6-sol high）+ SQLite 启动修复 + Web 子任务 + 浏览器验收（集成 2309/2309，DAG checkpoint 69d5c5fe8）→ T04（639b06a2f/8b2bcd2f2/2e50f6b1e + 真实 Lite 下载 SHA-256 门）→ T07 后端 5 轮修复 + Web（b09919c3f verified）→ …… → 集成 HEAD 21df162a2。每步均有独立评审与验证报告在档（台账 96–146 行）。

### 阶段二：本工作流并发波次 Wave 1–16（21df162a2 → dc2558355，94 提交）

主控提供的波次集成记录（16 个集成点，全部 gate=true、blockers=[]）：

| # | 集成 HEAD | 落地任务 |
| --- | --- | --- |
| Wave 1 | 7cbad8941 | T14任务1fix续接、T09后端、T06实测（+集成修复：迁移终版本断言 119） |
| Wave 2 | d88cd513a | T14子任务2、T09-Web、T06修复、T02实测iOS（11 提交） |
| Wave 3 | 6a21e0df5 | T11后端、T14-Web、T02补缺 |
| Wave 4 | 05122c146 | T15后端、T11-Web、T24小程序 |
| Wave 5 | 4b09af298 | T17后端、T15-Web、T24修复 |
| Wave 6 | 067aa1d0e | T16后端、T17-Web |
| Wave 7 | 24708145c | T13后端、T16-Web |
| Wave 8 | 10e2f30da | T18后端、T13-Web |
| Wave 9 | f6b14cf95 | T22后端、T18-Web（+复审 critical 迁移 force-add） |
| Wave 10 | ff5b2e843 | T19后端、T26小程序、T22-Web |
| Wave 11 | 14b81d24c | T20后端、T32小程序、T19-Web |
| Wave 12 | 38c22abe9 | T21后端、T28小程序、T20-Web（10 提交） |
| Wave 13 | 5f32c77b8 | T12后端、T21-Web、T32修复 |
| Wave 14 | d6b3e1bb5 | T12-Web、T30小程序 |
| Wave 15 | 8997b212e | T33终局（五环境矩阵+来源覆盖+证据包，零生产代码） |
| Wave 16 | dc2558355 | T33修复（2 medium 证据缺口闭环，零生产代码） |

Wave 17（调度员收口）：全 DAG 可派发工作耗尽，无可派任务；待主控 Step 4 终裁。

### 阶段三：主控侧 OCR 三轮修复集成（dc2558355 → 77917de10，19 提交）

| 轮次 | BASE → 新 HEAD | 落地内容（台账 322/326/329 段） |
| --- | --- | --- |
| OCR 第 1 轮 | dc2558355 → cbd4db5f8 | 7 任务提交+1 集成修复：web 组 1 critical+20 high+10 红线 medium（ac25a84a5/e338250f8）、miniprogram 组六 high+四 medium（1cada9857）、backend 组 15 项（a06189a24/eeb129a39）、misc 组（b9c397f27/8c7977862）；集成修复 6b6581457（reminder 双 ID 测试桩对齐冻结契约） |
| OCR 第 2 轮 | cbd4db5f8 → fcc149133 | 4 提交：miniprogram 86615d2d5（恢复链三件套等）、web 70310fb6a（19 high+8 红线 medium）、backend a4e8020be（3 high+7 medium，11 新回归测试含 -race）、misc c77da2710 |
| OCR 第 3 轮 | fcc149133 → 77917de10 | 4 提交：web 24821e5b1（15 critical/high+6 红线 medium）、miniprogram 4f1c765ea（3 high+5 幂等红线 medium）、backend 0d50acb4a（2 high+2 红线 medium，6 新测试含并发）、misc 9818d7cef（T33 证据脚本三处可靠性修复+原型 syncUrl 降级）+ 台账提交 77917de10 |

## ④ 原始 BASE 与最终 HEAD

- **原始 BASE**：`f7753fa160927195e388c65e1dbbdff7e288506c`（.worktrees/issue30-sweep 的 codex/issue30-mobile-office 已提交 HEAD；DAG 文件声明）
- **最终 HEAD（本报告核查时点）**：`77917de1014831e5691a8f7280d3af5c3a10cbeb`（"docs(plan): record OCR round 3 four-domain integration ledger"，2026-09-28 02:04:49 +0800，`git log -1 --format="%H %ci" HEAD` 实核）
- **提交统计**（`git log --oneline f7753fa..HEAD | wc -l` 实核）：**261 个提交**；分段：codex 会话段 148（f7753fa..21df162a2）+ 波次段 94（21df162a2..dc2558355）+ OCR 段 19（dc2558355..77917de10）
- **未推送状态**：`git status` 分支 `codex/issue-140-integration`，无远端跟踪；台账历波均载明"未 push；任务 worktree HEAD 实核未触碰"

## ⑤ 实际测试与设备验证证据

### 终验门（主控 ask 材料载明，本报告员未重跑）

- **go 门 exit=0；web 门 exit=0**（双门并行执行）——主控终验材料原文。命令细节未随 ask 提供，本报告按原文引用，不扩大口径。

### 集成门（集成员在各波/各 OCR 轮于集成工作区实跑，台账载明）

最近一轮（OCR 第 3 轮集成，2026-09-28）：
- `go test ./internal/modules/career/... ./internal/handler/... ./internal/database/... ./internal/router/... -count=1` 全 ok 一次过（career 28.8s / handler 1.7s / dto 0.6s / session 32.3s / database 30.1s / router 3.7s）
- `pnpm --filter @weknora/miniprogram test` 188/188 pass 0 fail 0 skipped（build:weapp 重建后仍全绿）；typecheck 13 错均为基线预存在
- web 聚焦 career 五文件 97/97 + api-client 全量 157/157 + career-core 27/27（node26.4.0）；`pnpm typecheck:web` 退出 0

历史全量 test:web 台账轨迹：2309（T03）→ 2363（T09）→ 2378 → 2389 → 2401 → 2412 → 2420 → 2430 → 2442 → 2449 → 2459 → 2466；Wave 13/14/15 全量因并行会话 CPU 争用与 agent-editor.test.tsx 间歇性死循环（BASE 预存缺陷）三次未完成，按聚焦门口径交付并 park（Wave 16 Ruling）；**OCR 第 2 轮 web 组会话内已跑串行全量 2503/2503 exit 0**（台账 326 段）——欠账以"聚焦门全绿+串行全量一次通过"记录，未在集成工作区以并行口径复跑闭环。

### 设备/环境验证矩阵（T33 launch-matrix.md + 各票验证报告）

| 环境 | 状态 | 证据 |
| --- | --- | --- |
| Web 浏览器（真实渲染 E2E） | 达成 | T03/T10/T14/T15/T16/T17/T18/T19/T20/T22/T33 等多相位浏览器证据（截图+server.log 交叉证实） |
| iOS 模拟器 | 达成 | T02 受认证 Task 读取/越权拒绝（task-2-live-ios-validation.md、task-2-evidence-ios/）；T33 iOS 实测记录 |
| 微信开发者工具（官方环境） | 达成 | T06 真实 DevTools 全链路；T24 14/14+6 截图哈希；T26 全链 18/18（端口 57817）；T32 修复轮 138/138；T33 wx-driver.cjs 自动化链 |
| 微信真机 | 未达成（环境门槛） | 各票以官方 DevTools 环境完成票面"真机或官方开发环境"条款；T28 F3 真机录屏、T32 M3 真机、T30 真机+订阅弹层项保留（台账 278 行与 d6b3e1bb5 终态段） |
| Android 真机/模拟器 | 未达成（无 adb/设备） | 台账 105/162/188 行历次实勘 |
| 鸿蒙原生 | 未达成（无 SDK/DevEco/hdc/hvigor/设备/.hap） | T01 blocked 证据（080cf7d47）+ verification/harmony-native-gate.md |

生产岗位来源 allowlist 为空（如实空覆盖清单，无"全国"表述）与来源/模型/微信能力/个人信息/收费流程运营核验属人工域，T33 如实记录、不宣称完成（Wave 15 Ruling）。

## ⑥ SDD 结论与 OCR 轮次结论

### SDD 结论（台账 Wave 17 收口段）

- 全 DAG 可派发工作已于 Wave 1–16 耗尽（34 节点：23 verified + 4 running + 6 pending + 1 blocked）；集成 HEAD dc2558355 时点全部 worktree 未 push 留档。
- 待主控 Step 4 终裁的 4 项：① T33 verified 终裁（7 low 披露不阻塞）；② T02/T28/T30 终态（建议 verified-with-environment-gate）；③ mobile 链 6 票终态（建议 blocked，T01 传导）；④ parked 欠账（全量 test:web agent-editor 间歇性死循环，BASE 预存）知悉确认。
- 本报告时点补充：主控终验门（go/web exit=0）与 OCR 三轮已执行完毕（ask 材料载明），终裁落定状态以主控记录为准，本报告不代裁。

### OCR 轮次结论（`docs/plans/issue-140/ocr/`）

| 轮次 | 报告首行结论（原文） | findings 块 | 限流失败项 |
| --- | --- | --- | --- |
| Round 1 | "Review partially complete: 290 finding(s); 111 of 315 selected item(s) failed." | 290 | 111/315 |
| Round 1 resume | "Review partially complete: 372 finding(s); 69 of 315 selected item(s) failed." | 372 | 69/315 |
| Round 2 | "Review partially complete: 376 finding(s); 57 of 319 selected item(s) failed." | 376 | 57/319 |
| Round 2 resume | "Review partially complete: 409 finding(s); 13 of 319 selected item(s) failed." | 409 | 13/319 |
| Round 3 | "Review partially complete: 186 finding(s); 154 of 319 selected item(s) failed." | 186 | 154/319 |
| Round 3 resume | "Review partially complete: 369 finding(s); 40 of 319 selected item(s) failed." | 369 | 40/319 |
| workspace（终次） | "Review skipped: no items were selected."（ocr-workspace.md 原文） | — | — |

- 三轮均因 LLM 限流（HTTP 429）部分失败，随后以低并发 resume 补跑（ask 材料载明"部分项限流失败后已低并发 resume 补跑"）；resume 后仍有失败项（69→13→40/319），失败明细在各报告尾部 retry report。
- 每轮 findings 中 critical/high/红线 medium 经四域修复提交集成（见③阶段三）；**修复后未再跑下一轮复查确认消除**。
- **未消除 findings（如实列出）**：以最新一轮 round-3-resume 为准，报告载 369 个 finding 块，按严重度 grep 统计（`grep -o "^\[[a-z]* · …]"`）：critical 1、high 18（bug 17+security 1）、medium 127（bug 80/maintainability 37/test 4/style 2/performance 2/documentation 1/other 1）、low 220（另有 3 块格式未匹配，合计 369）。按文件域分布：#140 实现面（career/workbench/miniprogram/issue-140/mobile 相关文件）142 块（如 internal/modules/career/search_once.go、reconciliation.go、usage.go、submission.go、search_rule.go、rendering.go、material.go、preparation.go、profile_intake.go、internal/modules/workbench/service/workbench/application_task.go、packages/api-client/src/career.ts、packages/career-core/src/desk.ts、contracts.ts 等，每处 1 块）；**非 #140 实现面 227 块**（packages/views/src/chat/*、craft/*、integrations/* 等 Vue→React 迁移与其他 BASE 改动文件，从未列入 #140 修复计划）。这些 findings 在 HEAD 77917de10 未被消除，亦无 round 4 复查证据；是否阻塞合并由主控/人工评审裁决。
- OCR 现场文件入库状态（`git status` 实勘）：`docs/plans/issue-140/ocr/ocr-round-1.md` 为已跟踪修改，round-1/2/3 及 resume、ocr-workspace.md 共 6 个文件未跟踪——集成员历轮均"保持原样未动"，本报告同样不代为提交，是否入库由主控决定。

## ⑦ 全部 Ruling 及判断错误的代价（台账 progress.md 含 "Ruling:" 的行共 75 条，`grep -c` 实核；行号为台账行号，逐条列入）

### A. Preflight 接口冻结（5 条）

1. **[48]** T01/T02 共享 Mobile Runtime：T01 只写鸿蒙试验/证据，T02 拥有 runtime。代价：若错，原生探针需返工。
2. **[49]** T03 对 T07–T33：先冻结 T03 合同（wire/迁移/路由）。代价：若错，下游需要大范围重构。
3. **[50]** T04/T16/T22：T04 先于消费者集成（授权与撤销）。代价：若错，下载/删除授权可能失效。
4. **[51]** T06/T24–T32：T06 只拥有 Task 下载路径。代价：若错，客户端集成冲突。
5. **[52]** Career 后端任务共享路由/迁移：串行集成共享文件并重跑合同测试。代价：若错，迁移/路由冲突。

### B. 早期实现与评审升级（5 条）

6. **[97]** T01 鸿蒙闸口 blocked（无 SDK/DevEco/hdc/hvigor/设备/.hap）：不得把 Android 兼容包或 WebView 记为鸿蒙原生；T05 及下游鸿蒙依赖移动票保持 blocked。代价：若错，移动集成延迟或原生适配策略返工。
7. **[98]** T01 blocked 证据评审后集成（5d6904c24），原始日志留存缺口延后（无原生工具链、转录命令可复现）。代价：若错，须重采失败构建证据才能做技术裁定。
8. **[105]** T02 代码修复后先集成但不解锁 T05（iOS/Android 真机受认证读取与跨租户实测仍缺）。代价：若错，下游排期继续延迟且代码可能返工。
9. **[114]** T03 并发合同连续三轮未过：round-4 分析升级 default/gpt-6-sol high（固定角色实现者多次漏掉跨事务持久边界）；设计报告建议先写 profile gate+有界 unknown+确定性慢胜者测试。代价：若裁定错误，序列化错范围会延迟或误分类重试；独立评审与 DB 测试仍强制。
10. **[118]** T03 Web 子任务扩权：允许前端改共享 errors.ts 解析器以保留 error.currentRevision 等类型化字段，不造 Career 专属影子解析器。代价：若错，通用错误消费方可能回归；需定向解析器+全量 Web 测试+独立评审。

### C. 波次调度（19 条）

11. **[147]** Wave 1 = T14 任务 1 fix 续接 + T09 后端 + T06 实测；T02 实测推迟（429 限流缓和，每波只放一个实测 agent）；T06 解锁小程序链优先于仅通向 blocked T05 的 T02。代价：若错，T02 iOS 证据再延一波且 Android 缺项依旧。
12. **[158]** Wave 2 = T14 子任务 2（换基 7cbad8941，Step 0 偿还子任务 1 遗留 medium）+ T09 Web + T06 D1/D2/D3 修复 + T02 iOS 实测。代价：若错，各任务退回重派，无下游连锁。
13. **[172]** Wave 3 = T11 后端（Step 0 偿还 T09 Wave1 medium）+ T14 Web（Step 0 偿还 T09-Web F2）+ T02 补缺。代价：若错，各任务退回重派（T12/T13/T15/T24 下一波按拓扑就绪）。
14. **[185]** Wave 4 = T15 后端 + T11 Web + T24 小程序（T07+T14 verified 开启最长链 T15→T16→T18→T19）。代价：若错，各任务退回重派。
15. **[199]** Wave 5 = T17 后端（fan-out 最大）+ T15 Web + T24 修复轮。代价：若错，各任务退回重派。
16. **[210]** Wave 6 = T16 后端 + T17 Web（T26 依赖 #159 未就绪不派）。代价：若错，各任务退回重派。
17. **[219]** Wave 7 = T13 后端 + T16 Web。代价：若错，各任务退回重派。
18. **[230]** Wave 8 = T18 后端 + T13 Web（择槽优于 T22/T12）。代价：若错，各任务退回重派。
19. **[239]** Wave 9 = T22 后端 + T18 Web（T22 单任务即解锁 T32，优先于 T20/T21/T12）。代价：若错，各任务退回重派。
20. **[248]** Wave 10 = T19 后端 + T26 小程序 + T22 Web。代价：若错，各任务退回重派。
21. **[259]** Wave 11 = T20 后端 + T32 小程序 + T19 Web。代价：若错，各任务退回重派。
22. **[268]** Wave 12 = T21 后端 + T28 小程序 + T20 Web（T28 新实现优先于 T32 修复轮，避免 adapters/career-platform.ts 并发冲突）。代价：若错，各任务退回重派。
23. **[277]** Wave 13 = T12 后端（最后一个 career 后端票）+ T21 Web + T32 修复轮。代价：若错，各任务退回重派。
24. **[284]** Wave 13 前端门限流裁定：全量 test:web 三次因并行会话 CPU 争用未完成，按 ask"或对应 filter 包测试"授权交付聚焦门（career 四页 82/82、改动文件 27/27、api-client 43/43）；全量待空闲补跑。代价：若补跑失败则重开前端缺陷票。
25. **[288]** Wave 14 = T12 Web + T30 小程序（最后一个功能票）。代价：若错，各任务退回重派。
26. **[295]** Wave 14 前端门：全量再次因争用未完成，交付聚焦门 reconciliation+三页 80/80+api-client 50/50；评审披露 3 个全量失败经评审定性 BASE 预存/环境性（集成侧未能复跑复核，如实记录）。代价：与 Wave 13 欠账一并补跑闭环；补跑失败则重开缺陷票。
27. **[299]** Wave 15 = T33 五环境真实闭环与发布门槛；可达环境新跑抽查链+复用归档证据；鸿蒙/Android 如实 blocked；生产空 allowlist 与运营核验属人工域不宣称完成。代价：若错，T33 退回补缺失环境任务。
28. **[310]** Wave 16 = T33 修复轮（补两 medium 证据缺口），全 DAG 仅此一项可派发，本波单任务。代价：若错，评审复审退回重补。
29. **[319]** Wave 17 收口：本波无任务可派——可派发工作耗尽；4 个 running 的终态翻牌与 6 个 pending 的 blocked 落定均属主控 Step 4 终裁。代价：主控终裁通过则全 DAG 到达终态、调度员返回 done=true；若主控另有裁决以主控为准。

### D. 现场/证据/集成修复与设计边界（46 条）

30. **[148]** T14 worktree 未提交修改确认为中断 fix-r1 现场（三个测试俱在、ensureOnce 已提取、聚焦测试含未提交状态 ok 19.271s）；fix-resume 语义=保留续接补齐，禁止 checkout -- 丢弃。代价：若现场有缺陷，续接者以最小 diff 修正并说明理由。
31. **[149]** 迁移编号以目录实核为准：集成 HEAD 21df162a2 最大 versioned 000195/sqlite 000116（台账"197/118"实为 T14 预留口径）；T09 分配 000198/000119。代价：若集成顺序变化撞号，由主控重排。
32. **[150]** T06 实测基线定为集成 HEAD 21df162a2（验证对象是已集成代码）。代价：若错误，证据基线不符需重跑。
33. **[153]** T06 简报 headSha 与 worktree 实际 HEAD 前 9 位一致后 31 位不符：reflog 实核单次 commit 无 amend、内容与评审描述吻合，判定为简报侧 SHA 誊写笔误，按实际提交集成。代价：若判定错误则集成内容与评审对象不符需重审。
34. **[154]** 集成门首跑迁移断言 FAIL（116 vs 117）：根因跨任务基线交互（T14 在 117 基线一步下移断言 116，T09 的 119 落地后跳过缺号 118）；按集成员 <20 行授权改显式版本目标 m.Migrate(116)，提交 7cbad8941（+7/−1）。代价：若错误，回退该提交并退回 T14 重写断言。
35. **[159]** T14-T2 弃旧基线换基 7cbad8941（旧基线必冲突，内容已 cherry-pick 无损失）。代价：若错误，本地提交可经 reflog 找回。
36. **[160]** T14 子任务 1 遗留 medium 捆入 T14-T2 Step 0；T09 遗留 medium park 至 final validation 轮。代价：若错误，各自补独立修复轮。
37. **[161]** T06 修复必须保持 TDesign Miniprogram 路线，禁止以原生 button 静默替代；确不可行时上报主控裁决。代价：若实际可行却替换，Spec 不符需返工。
38. **[162]** T02 派发仅 iOS 部分（模拟器实勘可用）；Android 无 adb 维持环境门槛裁定如实记录；T05 仍被 T01 blocked 传导不解锁。代价：T02 verified 与否待证据由主控裁决。
39. **[167]** miniprogram 门首跑 4 fail 根因是集成工作区 dist/ 旧构建产物（属构建前置条件缺失而非代码缺陷）；pnpm build:weapp 重建后 62/62 全绿，未改代码。代价：若错误则退回 T06 检查产物断言设计。
40. **[168]** T09-Web F1（node>=26 引擎要求）由集成阶段偿还：node22 下全量 2363/2363 + node26.4.0 复跑 2363/2363——F1 清偿，node26 为终验口径；miniprogram typecheck 13 错全为基线预存。代价：如实记录，无。
41. **[173]** T09 Wave1 medium 捆入 T11 Step 0（同邻包、无并发所有者）；T09-Web F2（空 rawText 证据页）裁定为 api-client 客户端真实缺陷，修解码器捆入 T14 Web Step 0。代价：若裁定错误（后端不应发空 rawText），需回退解码放宽并重开后端缺陷。
42. **[174]** T11 设计边界：不实现持续规则结构（T13 专属）；额度只做注入式 seam+typed 可恢复拒绝，不虚构额度状态（真实额度属 T21）；只经 SourcePolicy/SourceTransport seam，生产空 allowlist=如实空覆盖、禁"全国"表述；search_once 不经 linker 不写 Workbench 表。代价：若错，T13/T21 集成时返工。
43. **[175]** T02 补缺范围收窄为 F3 迟到响应运行时证据（或如实"不可构造"）+ 截图 19 定性更正；不伪造时序。代价：若错误，T02 永久维持 running 口径由主控最终裁决。
44. **[180]** Go 门首跑 career 包并发 flake 定性为负载敏感非代码缺陷（共享内存 SQLite 多 worker 并发写；单测 -count=3 ok、整包 ok、全量门重跑 6 包全 ok 三次复跑证据）。代价：若错误则升格为真实竞态退回 T09。
45. **[181]** 全量 test:web 两次卡死根因为环境陈旧孤儿进程（ps 实勘 4h/3h 遗留死循环 runner）；kill 清理后第三次全量 2378/2378 pass（node26、292s）；卡死定性环境级预存在（改动面实核零交集）。代价：如实记录，无。
46. **[186]** 本波 career 后端任务唯一性维持（T15 独占共享文件；T12/T13/T17 排队）——共享文件并行必致集成冲突，Wave 1-3 每波恰一个 career 后端零冲突先例。代价：若错误（实际可并行），仅损失吞吐无正确性风险。
47. **[187]** T24 按"前置后端合同 reviewed+integrated 即可派发"先例放行（跨票延伸）；若 T11-Web 评审迫使后端合同变更，集成员负责调序/回退 T24。代价：若错误，T24 需对合同返工。
48. **[188]** T02 无进一步可派发工作（iOS 证据链完整评审通过、Android 无 adb 永久缺项）——维持 running，终态留待 T33 主控裁决。代价：若错误，仅剩 iOS 补充项可再派。
49. **[189]** T11 评审 4 low 与 T14 评审 2 低危瑕疵不捆绑（避免债务捆绑滥用；low 留 T33 收口统一清点）。代价：若 T33 前升级为阻塞再补修复轮。
50. **[194]** Go 门间歇 FAIL 抓到失败现场：requireSpace 的 First 查询遇 table is locked 直接透传（claim 事务之前，绕过 isSQLiteBusy→typed unknown 转换）；归属 T09 Wave1 预存在缺陷非本波引入；按 <20 行授权修复（ImportURLReceipt 入口对 requireSpace 错误做 isSQLiteBusy 分类，+6 行，提交 2edced621）；修复后 12/12 连跑通过、Go 门 6 包全 ok。代价：若错误则回退 2edced621 并退回 T09 重新设计并发错误分类。
51. **[195]** T24 评审结论为"核心验收真实达成……但三项 medium 建议修复后再集成"——非"通过"口径，但主控指令明列本波集成；集成员按指令集成全部 4 提交、DAG 标 running，F1/F2/F3 修复待下波，集成门不推翻评审修复要求。代价：如实记录，无。
52. **[200]** career 后端槽位本波归 T17（T12/T13 纯槽位排队）。代价：仅损失吞吐无正确性风险。
53. **[201]** T15 后端评审遗留 medium（重复 heading compare 误报）不模糊捆绑——改为 T15-Web V1/V2 比较 E2E 定点观测：可复现则精确记录供后续修复轮，不可复现如实记录。代价：若 Web E2E 确认可复现且影响验收，Wave 6 派 targeted 修复。
54. **[202]** T24 修复轮沿用原 worktree 续接不换基（集成侧已有 cherry-pick 副本）；F3 取证修复允许改 tests/ 与自动化辅助但不动业务逻辑。代价：若错误，冲突时由集成员调序。
55. **[211]** T16 渲染器策略受约束：优先纯 Go stdlib（DOCX=zip+xml；PDF=最小自写 writer）；go.mod 无现成库；如需新依赖须实测可得并披露；禁止运行时外部渲染 API；chromedp 不得作生产路径。代价：若依赖不可得且纯 Go 不可行，T16 如实 blocked 上报。
56. **[220]** T16"导出下载为认证兑付（非 T04 式 credential-free）"裁定成立不改：材料属 Career 域更高敏感级；T06 小程序下载本就带 Authorization，全端兼容；T04 credential-free 仅是其票面验收不外溢为全局规范。代价：若某端确需免认证兑付，届时按该端票据单独扩。
57. **[221]** T13 触发机制受约束：注入时钟+显式触发 seam（TriggerDueRules），不引入常驻后台调度 goroutine 作生产路径；真实常驻调度如 T33 验收需要，届时主控另裁。代价：若错误，T33 前补调度集成轮。
58. **[226]** T16-Web 评审自述未能复跑全量（遗留进程抢占 CPU）：集成阶段偿还——pkill 清理后在集成工作区 node26 实跑全量 2420/2420 pass（254s），与实现者归档数字一致。代价：如实记录，无。
59. **[231]** T16 verified 附带 1 medium 截图证据缺陷维持遗留记录（同 T24-F3 模式，T33 收口统一清点）。代价：若 T33 前升级为阻塞再补取证轮。
60. **[240]** T22 删除跨域经窄接口（internal/types/interfaces 新删除端口+container 接线，T14 linker 同型先例），不直接写 Workbench 表；删除测试强制隔离数据库。代价：若错误，架构边界破环需返工。
61. **[249]** T19 版本锚定语义冻结：优先固定实际投递版本，unconfirmed/缺失时返回显式提示态而非猜最新（"V2 已投递时引用 V2、未知时不猜 V3"直译为测试）。代价：若错误，T28 小程序端准备页需返工。
62. **[250]** T26 复用 T24 已验证的小程序基础设施并书面继承三条 T24 修复教训为验收前置。代价：若错误，T26 评审将重蹈 T24 三 medium。
63. **[255]** T19 评审遗留 F1（career_preparations 未纳入 DeleteCareer purge）由集成员偿还：careerPurgeTables 加一行+回归测试，提交 56c164d4a（+13/−1），6 个 DeleteCareer 测试全 PASS；boundary 视图未同步加 preparations 段记为遗留观察项。代价：若错误回退 56c164d4a 并退回 T22/T19 重审删除边界。
64. **[260]** T20 新持久表强制纳入删除 purge 清单与 boundary 视图（T19 F1 先例）；boundary 遗留观察项由 T20 实现者顺带核对并在报告载明现状。代价：若遗漏，重复 F1 类债务。
65. **[269]** T21 将 T11 的 SearchQuotaGate 空实现升级为真实 admission（预占/消耗/余额幂等）；只读零额度；付费状态不得影响排序/资格（显式测试）。代价：若错误，T30 小程序端额度页需返工。
66. **[278]** T28 F3（真机/人工录屏）与 T32 M3（真机）同属设备缺失环境门槛——不在工作流内强补，维持 running 待 T33 统一裁决（同 T02 Android 口径）。代价：若 T33 主控要求人工录屏，届时由人工补充。
67. **[279]** T12 为最后一个 career 后端票，其后槽位腾空。代价：无。
68. **[289]** 集成门遗留（Wave 13 延续）：全量 test:web 欠账按聚焦门口径交付，Wave 14 集成门应在环境空闲时补跑闭环。代价：若补跑失败则重开前端缺陷票。
69. **[290]** T30 订阅消息用 wx.requestSubscribeMessage 平台原生 API，记录为明确原生能力例外（验收条款允许）；拒绝路径与"不伪造已送达"为强制测试点。代价：若错误，隐私语义违规需返工。
70. **[300]** T30 残项（真机/订阅弹层/3 medium 主控知悉项）并入 T33 终态裁决材料，不单独派修复轮。代价：若 T33 抽查发现功能性缺陷则另开修复票。
71. **[301]** 全量 test:web 两波欠账列入 T33 完整门清单：空闲时补跑闭环；仍争用则取证+聚焦门数字+如实标注待补。代价：若补跑出真实失败则重开前端缺陷票。
72. **[306]** 全量 test:web 再次尝试补跑仍挂起（空闲环境下 agent-editor 间歇性死循环仍复现）；该欠账 T33 评审已如实列入 3 low 且主控知悉；Wave 15 提交为零生产代码 docs，集成门以 Go 门全绿+评审侧四门独立复跑+Wave 14 聚焦门为据。代价：欠账留待 agent-editor 缺陷修复或环境窗口由后续轮次闭环。
73. **[311]** 全量 test:web 欠账 park（不派修复）：agent-editor.test.tsx 间歇性死循环为 BASE 预存缺陷，修复需动 agents 域无关文件超出 #140 范围；聚焦门全绿为交付口径，欠账 parked 移交后续轮次。代价：若后续判真实回归，另开独立缺陷票。
74. **[312]** mobile 链 T05/T23/T25/T27/T29/T31 终态建议 blocked（T01 传导不可达：无鸿蒙工具链+无 Android 设备）；T02/T28/T30 建议 verified-with-environment-gate（功能与可达环境证据链完整评审通过，永久缺项均为设备类门槛）——建议随 T33 终裁材料交主控 Step 4 终裁，调度员不代裁。代价：若主控另有裁决以主控为准。
75. **[323]** OCR 第 1 轮集成门 miniprogram 首跑 4 fail：跨组语义交互——web 组收紧 decodeReminderIds 为"progress_event 恒双 ID"，miniprogram 组测试桩按旧宽松契约构造；后端冻结契约实核支持 web 组（reminder.go:348/:362 恒写两列）；按"后组让先组"授权修测试桩两处（+2/−2，提交 6b6581457），修后 176/176 全绿。代价：若错误则 web 组解码器收紧过度需回退重审。

## ⑧ 遗留风险与真实阻塞

1. **鸿蒙无工具链（真实阻塞）**：无 SDK/DevEco/hdc/hvigor/设备/.hap，T01/#143 blocked，传导 T05/T23/T25/T27/T29/T31 六票建议终态 blocked（未启动）。证据：harmony-native-gate.md、台账 97 行。
2. **Android 无设备（真实阻塞）**：无 adb/真机，T02 Android 验收面永久缺项；T28/T32/T30 的真机项保留为环境门槛。证据：台账 105/162/188/278 行。
3. **未推送、未合并、未上线**：集成分支 261 个提交全部本地；未 push、未开 PR、未合并 main、未发布。任何"已交付上线"表述均不成立。
4. **OCR 仍有未消除 findings**：round-3-resume 载 369 块（critical 1/high 18/medium 127/low 220，按严重度 grep 统计+3 块未匹配），其中 #140 实现面 142 块、非 #140 面 227 块；三轮修复（critical/high/红线 medium）后无 round 4 复查证据；三轮 resume 后仍各有 69/13/40 个 items 因限流失败未复审。合并前需人工或再轮 OCR 处置。
5. **全量 test:web 并行口径欠账（parked）**：agent-editor.test.tsx 间歇性死循环为 BASE 预存缺陷（Wave 13/14/15 三次取证）；以聚焦门全绿+OCR r2 web 组串行全量 2503/2503 exit 0 记录，未在集成工作区以并行口径闭环。
6. **miniprogram typecheck 13 个基线预存错误**：全部位于 src/features/account/pages.tsx（CommercialSummary），非 #140 引入，历波未清偿。
7. **真机微信验证缺失**：各小程序票以官方 DevTools 完成票面"真机或官方开发环境"条款；真机/人工录屏证据（T28 F3、T32 M3、T30 真机+订阅弹层）不存在。
8. **运营域未核验**：生产岗位来源 allowlist 为空（无真实来源接入）、来源/模型/微信能力/个人信息/收费流程运营核验属人工域，T33 如实记录未完成。
9. **终裁未落定**：T33/#172 verified、T02/T28/T30 终态、mobile 链 6 票 blocked 落定均属主控 Step 4 终裁（含父规格 43 条故事覆盖核对）；本报告按 DAG 现状+台账建议如实呈现，不代裁。
10. **PostgreSQL 运行时迁移未实测**：早期票（T03/T04/T07 等）披露 SQLite 全量验证，live PostgreSQL 运行时迁移未演练（task-4-final-validation.md 披露）。
11. **OCR 现场文件未入库**：docs/plans/issue-140/ocr/ 下 1 个已跟踪修改+6 个未跟踪文件（含本报告引用的全部 OCR 证据），是否入库由主控决定。

---

## 声明

本报告仅陈述有证据支撑的事实：DAG/台账/OCR 报告/git 历史均为集成工作区实读，终验门结果（go exit=0、web exit=0）引自主控 ask 材料原文。本报告员未重跑任何全量验证门、未执行推送/合并/上线操作，也未将任何研究性结论记为设备验收通过。
