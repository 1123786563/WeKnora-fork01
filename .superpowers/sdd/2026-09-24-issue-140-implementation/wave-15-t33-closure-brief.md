# Wave 15 — T33：五环境真实闭环与发布门槛（validation/closure，终局任务）

你是 frontend_validator + backend_validator 合一角色（终局验收员）。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 你不实现任何生产代码——发现缺陷只记录（缺陷由主控另行派发）。你的产出是两份文档 + 证据 + 终态裁决材料。**诚实优先于完整性：环境不可达的格如实 blocked，不宣称公开门槛完成。**

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01 --detach d6b3e1bb5
cd /Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01
```
BASE = `d6b3e1bb5`（当前集成 HEAD，功能票全部集成）。本地 commit 允许（仅文档/证据）；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。本轮服务器端口 **57828**（隔离临时 DB）。

## 2. 主计划 Task 33 文本（verbatim）

### Task 33: T33/#172 五环境真实闭环与发布门槛
**Depends:** #151、#165、#166、#167、#169、#168、#170、#171、#173。**Files:** `docs/plans/issue-140/verification/launch-matrix.md`、`docs/plans/issue-140/verification/source-coverage.md`。
**验收：** 五个实际环境均跑完整链路和失败恢复；分享导入、PDF/DOCX 下载、通知权限、跨端同步与越权检查有可复现记录；公布真实岗位来源、覆盖城市、使用条件与数据处理说明；来源、模型、微信能力、个人信息和收费流程的运营核验完成
- Step 1：建立 Web、Expo iOS、Expo Android、HarmonyOS 原生、微信小程序五环境的同一档案→岗位→评估→申请→材料→本人投递→进展验收矩阵，每格包含版本、设备、命令与证据。
- Step 2：逐环境运行真实服务端闭环与文件、分享、通知、scope 切换、未知结果恢复；岗位来源列原始链接、获取条件、城市覆盖和失败态。
- Step 3：运行完整 Go/TS/Lint/构建及设备检查；预期全通过。**缺鸿蒙原生或微信真机等环境时将对应格记 blocked，不宣称公开门槛完成**。
- Step 4：独立最终 Review 和 OCR 覆盖原始 BASE..HEAD；保存合规/来源公开门槛与全部 blocker，**只有全部通过才标 verified**。
**验证命令：** `go test ./internal/modules/career/... && pnpm typecheck:web && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/miniprogram typecheck`。
**失败处理：** **任何真实阻塞缺口保留未完成，不以 Web 通过代表其他目标通过。**

## 3. 本环境可达性实勘（调度员核实，矩阵预定格）

| 环境 | 可达性 | 依据 |
| --- | --- | --- |
| Web（浏览器） | ✅ 可达 | T03/T07/T12/T14/T15/T16/T17/T18/T19/T20/T21 各票浏览器 E2E 已归档 + 你本轮复跑关键链路 |
| Expo iOS（模拟器） | ✅ 可达 | iPhone 18 Pro UDID 0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC（iOS 27）；T02 实测报告 docs/plans/issue-140/task-2-live-ios-validation.md 已归档 + 你本轮抽查 |
| 微信小程序（DevTools） | ✅ 可达 | CLI /Applications/wechatwebdevtools.app/Contents/MacOS/cli；T06/T24/T26/T28/T30/T32 已验证证据归档 + 你本轮抽查 |
| 鸿蒙原生 | ❌ blocked | T01 blocked 证据（无 DevEco/hdc/hvigor/设备/.hap，commit 080cf7d47→5d6904c24） |
| Expo Android | ❌ blocked | 无 adb/emulator/设备（历次实勘） |

**既有证据复用规则**：各票 archived live 验证（docs/plans/issue-140/task-*-live-*.md、task-*-validation.md 与各 worktree 报告已 git add -f 入库）可引用为矩阵证据，但每环境本轮至少**新跑一条端到端抽查链**（同一档案→岗位→评估→申请→材料→投递→进展），证明集成 HEAD d6b3e1bb5 上的当前整体可用。

## 4. 你的工作清单

1. **launch-matrix.md**：五环境 × 验收链（档案→岗位→评估→申请→材料→本人投递→进展）矩阵；每格：环境版本/设备/命令/证据指针（既有归档 + 本轮新跑）；鸿蒙/Android 格如实 blocked（引用上述依据）；失败恢复列（scope 切换/未知回执/越权）逐环境证据。本轮新跑：
   - Web：Lite 服务器（57828）+ 真实浏览器走完整链 + 越权抽查（另一租户）；
   - iOS：构建运行 T02 模式抽查（登录→受保护 Task 读取→跨租户拒绝；Career 链若 app 内可达则尽量覆盖，不可达如实记录边界）；
   - 微信：build:weapp + DevTools 登录→找岗/申请/材料抽查（复用 T24/T26 模式与端口 57828）。
2. **source-coverage.md**：如实公布——生产来源 allowlist 为空（T09 设计：无授权 source-review 记录不核验任何招聘域名）；覆盖城市=空；使用条件（登录、request-id 幂等、额度 admission）；数据处理说明（确认事实/不可变快照/删除边界——引 T22 boundary）；运营核验清单（来源/模型/微信能力/个人信息/收费）逐项标注**未完成**（需人工运营介入，工作流内不可自动完成——如实记录，不宣称完成）。
3. **完整门**：跑主计划验证命令四项 + `pnpm test:web` 全量（**先 ps 确认无 CPU 争用**；这是 Wave 13/14 两笔全量欠账的闭环——若仍争用，记录 ps 取证与聚焦门数字，如实标注"待空闲补跑"）；Go 6 包门；miniprogram 172 门。
4. **终态裁决材料**（供主控裁，不代裁）：汇总 T02（Android 永久缺项）、T28-F3、T30 残项（真机/订阅弹层/3 medium 主控知悉项）、T01 及其传导链（T05/T23/T25/T27/T29/T31）、各票遗留 low 清单（台账历次"不阻塞"记录）、生产空 allowlist 对发布门槛的影响——每项附证据指针与建议终态（verified-with-gate / blocked / parked）。
5. **Step 4 对接**：OCR 覆盖为主控侧流程（集成分支已有 OCR WIP 在跑）——你在报告中记录其状态即可，不自行宣称。

## 5. 全局约束（verbatim 摘录）

- 不以 Web 通过代表其他目标通过；任何真实阻塞缺口保留未完成。
- 你是 validator：生产代码零改动；唯一可写文件是 verification/ 下两文档与报告/证据。
- 严禁 push、merge、deploy、GitHub 操作；禁止派发子 agent；凭据 disposable 结束清理；端口 57828 隔离。
- 报告只写事实与实测输出；绝不伪造环境/证据/通过。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t33-closure/` 下写 `task-report.md`：本轮各环境新跑链路的命令与输出、完整门结果（含全量 test:web 状态）、终态裁决材料、发布门槛结论（**如实：来源覆盖与运营核验未完成、鸿蒙/Android blocked，公开门槛未达成**）。COMMIT：`docs(verification): record five-environment launch matrix and source coverage`。最终消息报告 commit SHA 与结论摘要。T33 的 verified 与否由主控依 Step 4 全流程裁决。
