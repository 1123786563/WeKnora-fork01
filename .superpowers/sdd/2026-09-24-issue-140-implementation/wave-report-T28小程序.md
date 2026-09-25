# Wave 12 — T28小程序（#169 微信小程序申请时间线与按需准备）实施报告

- 状态：**DONE**（实现者自评；T28 verified 由主控裁决）
- BASE `14b81d24c` → HEAD `3d400bff4`，本地 5 commits（未 push）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t28-miniprogram/WeKnora-fork01`
- 完整报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t28-miniprogram/task-report.md`（含 RED/GREEN 原始输出、驱动/探针日志、13 截图 manifest、脱敏 fixture）

## 摘要

T28 小程序申请进展时间线与按需准备，与 Web 同源同版本（同一批 api-client 冻结合同）：T17 进展 append-only 全链（纠错=追加引用事件、expectedRevision 域=每应用事件计数、对账+安全重发）；T19 准备全链（锚定实际投递版、409 preparation_version_unknown typed 提示态绝不静默改用最新版、草稿物化 materials 域修订+回读）；本地草稿 seam（scope 隔离、登出即清、断网可再编辑、重联不自动提交）。

## 验证结果（全为实测）

| 检查 | 命令 | 结果 |
| --- | --- | --- |
| 单测 | `pnpm --filter @weknora/miniprogram test` | 147/147 pass（132 基线 + 15 新增；RED 14/15 失败在档） |
| 类型 | `pnpm --filter @weknora/miniprogram typecheck` | 13 errors 全为 account/pages.tsx 基线豁免，零新增 |
| 构建 | `pnpm --filter @weknora/miniprogram build:weapp` | Compiled successfully；dist/career/progress-preparation.* 产出；t-button=/npm/tdesign/button/button |
| diff | `git diff --check` | 干净 |
| 真实 DevTools（57822/9435，隔离临时 DB，一次性密钥） | app 驱动 4 相位 | online1 19/19 + offline 5/5 + reconnect 3/3 + switch 4/4 = **31/31 PASS**；13 截图 13 个不同 sha256；关键截图经图像模型复核 |
| 服务端探针 | fixture setup/verify | **16/16 PASS** |
| 真机 | — | **blocked（无设备）**，单列于 task-report §6 |

## 验收对照（简报 §5）

- **时间线权威顺序+跨端更正并见**：单测 A1 + 驱动 B/C 组（app 内刷新后原文（corrected 标记）与更正事件并见，事件 3 条）+ 探针（原文 corrected=true）。
- **准备锚定投递版/未知提示态**：单测 B1/B2 + 驱动 E/F/I 组（真实 POST 后「基于实际投递版本 V1」+来源链可见；app-2 无投递→typed 提示态与 Web 同语义）+ 服务端 409 探针。
- **真实环境录入修改查看准备（关键路径）**：生成（恢复入口同一 client.request 链真实 POST）→ 修订（GUI 可编辑模型+本地草稿恢复+同一 editMaterial 链真实 POST）→ 查看（材料域真实 GET 回读+列表）。
- **账号切换缓存隔离**：单测 D1 + 驱动 S 组（B 名下零 wk:career: 键、scope 键读不到 A 草稿）。
- **断网只留草稿不静默改申请状态**：单测 C1 + 驱动 N/R 组（离线失败可见、零编造事件、草稿可再编辑；重联零自动提交、事件数不变）。
- **三 seam 可观察**：冲突回执 A6/B2、未知对账 A4/A5/B5（原 requestId+原 revision 重放）、空间切换 E1；驱动侧恢复入口即三 seam 的真实链路呈现。
- **132 基线/13 typecheck 豁免/三教训（可区分截图、无死代码入口、404 与未知分支恢复态）**：均满足。

## 偏差与声明

- 计划点名的 `progress-preparation.test.tsx` 落位 `tests/progress-preparation.test.mjs`（harness 为 node --test tests/*.test.mjs，strip-only 不支持 JSX；T24/T26/T32 同款偏差）。
- services/career.ts 扩展为调度员补充语义（T26/T32 先例）；另 5 个配套文件（page config/app.config/routes/core.test 守卫/discovery 入口）为页面可达性必要配套，逐条列于 task-report §2。
- live 驱动发现并修复 2 个产品缺口（恢复链路提示态缺失、断网草稿不可再编辑），独立提交（718c904f8、3d400bff4）。
- me 页登出 GUI 不可自动化（native showModal 回调形态 mockWxMethod 不支持，实测在档）；登出清缓存语义由单测 D1 钉死，GUI 侧以切账号后缓存状态作证。

---

## 评审修复轮 R1（第 1 轮评审 F1 high / F2 medium 已修复；F3 medium 环境外如实保留；F4 low 记录不修）

- HEAD `3d400bff4` → `8d6a1abdf`（fix(miniprogram): R1 评审 F1/F2——主张保全与本地优先合并；未 push）。

### F1（high）断网独立入口保存静默清空材料 claims —— 已修复并双重验证

- 修复：本地草稿携带各节 claims 快照；`saveRevision` 在 `editMaterial` 前对任何 claims 为空的小节先 `GET /materials/:id` 取回材料域正文，经 `recoverEmptyClaims`（同名小节取回、已有主张不改写、新增节保持无主张）合并后提交——服务端 DraftBody 整体替换语义下绝不提交 claims 为空的正文。
- 证据：单测 R1-P3/R1-P4（组合链端到端断言 POST 正文主张未丢）；live 驱动 `H f1-claims-kept-visible`（回读卡「引用主张 1 条，未被丢弃」实际可见，图像复核）；服务端探针 `final-material-revised-claims-kept`（claimsSurvived=true：修订落库后引用已确认事实的主张仍在）。

### F2（medium）恢复编辑以服务端为基数截断本地新增节 —— 已修复并 GUI 验证

- 修复：`startEditing` 改本地优先合并 `attachClaimsFromServer`（本地草稿为基数、主张从服务端回填、本地删除节不复活），提示如实呈现「共 N 节，其中 M 节为本地新增」。
- 证据：单测 R1-P1/R1-P2；live 驱动 `G f2-local-first-merge` + `G f2-added-section-editable`（新增节标题在编辑模型 input.value 实际可编辑）。

### 重跑验证（全量，真实输出）

```
pnpm --filter @weknora/miniprogram test        → ℹ tests 151 / pass 151 / fail 0 / skipped 0（147 基线 + 4 新增；RED-R1.txt 在档：R1-P1..P4 先 4 失败）
pnpm --filter @weknora/miniprogram typecheck   → 13 errors（全部 account/pages.tsx CommercialSummary 基线豁免，零新增）
pnpm --filter @weknora/miniprogram build:weapp → Compiled successfully（dist/career/progress-preparation.* 产出）
git diff --check                                → 干净
```

R1 live 复验（全新隔离 DB + 一次性密钥，fixture 材料携带引用已确认事实的主张）：app 驱动 online1 22/22（含 F1/F2 三步新证据）+ offline 5/5 + reconnect 3/3 + switch 4/4 = 34/34；服务端探针 16/16；13 截图 13 个不同 sha256；关键截图图像复核通过。环境已清理（57822/9435 空闲、密钥/DB/含凭据 fixture 删除、脱敏归档于 worktree `.superpowers/sdd/2026-09-24-issue-140-t28-miniprogram/`，task-report.md §8）。

### F3（medium）如实保留（环境外验收面）

真机验证维持 blocked（无设备，简报明示「未通过真实设备门槛则保留 blocked」）；t-button GUI 直点/me 页登出 GUI 为 T24 已知 automator 平台限制（本轮实测复核）。补足需真机或人工点击录屏，本环境不可得——不以伪造证据替代；相应业务链由 151 项单测 + 同链恢复入口真实 POST 组合覆盖（第 1 轮评审已实测复核认可）。

### F4（low）记录不修

api-client `AppendProgressInput` 类型不含 source 字段 vs 两端请求体硬编码 `source:{kind:'manual'}`（服务端 Go 结构有该字段且白名单钉死，行为正确）——既有类型滞后，非本任务引入；A2 注释指冻结合同请求体形状（与服务端结构一致）。
