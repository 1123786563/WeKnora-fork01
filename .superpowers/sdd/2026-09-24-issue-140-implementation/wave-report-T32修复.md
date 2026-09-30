# Wave 13 — T32 修复轮 1/5：M1 门控差异 / M2 证据缺失 / M3 真机门槛（Issue #173 fix-resume）报告

- 状态：**DONE**（M1/M2 修复完成；M3 真机为环境门槛如实声明，交 T33 裁决，不伪造）
- BASE `5033b6d5f` → HEAD `df894023e`（本地 1 commit，未 push；复审 diff 范围 5033b6d5f..df894023e）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t32-miniprogram/WeKnora-fork01`（沿用 Wave 11 现场，续接无换基）
- 完整报告+证据：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t32-miniprogram/task-report.md`（§8 修复轮节）+ 本轮新增证据文件（见下）

## M1 主按钮 unknown 不封锁与 Web 门控差异 → 修复（TDD）

- **RED**：`tests/export-deletion.test.mjs` +3 用例（unknown 封锁+对账恢复 / unknown≠partial≠deleted / busy 矩阵+前置），`node --experimental-strip-types --test tests/export-deletion.test.mjs` → **tests 17 / pass 14 / fail 3**（ERR_MODULE_NOT_FOUND；RED-M1.txt 在档）。
- **GREEN**：新 `src/career/export-deletion.gating.ts`（`lifecycleGating` 纯函数逐条对齐 Web `ExportDeletionPage.tsx:296-298`：exportBlocked=busy‖unknown；deletionBlocked=删除 busy‖unknown‖exportBlocked；知悉 disabled=deletionBlocked；partial=已呈报确定回执不封锁）；`export-deletion.tsx` 两主按钮 disabled 接 gating + onTap 补 Web runExport/runDeletion 防重入守卫 + 知悉行封锁。恢复入口（用原请求对账导出/删除）即"重新对账"动作。
- **单测**：`pnpm --filter @weknora/miniprogram test` → **135/135**（132 基线+3 新增，含 57825 联调 dist 下 build-output 用例）。
- **真实 DevTools 复验（57825 隔离临时 DB，13/13 PASS，driver-fix1.cjs/driver-fix1-run.log 在档）**：构造 unknown 态（种子 intent=发起后中断等价现场）→ 数据面探针（Taro3 UI 树序列化进 page.data()，t-button 节点真实 disabled 属性）实测**两主按钮 disabled:true** → 边界可读但知悉 ☐ 点击不翻转（Web ack disabled 语义）→ 「用原请求对账导出」真实 `GET /exports/receipt`（server-tail.log 可见）→ 导出钮恢复/删除钮等知悉 → 勾选后全可用——**门控阶梯 2→1→0 完整可观察**；2 截图 sha256 两两不等并经图像模型视觉复核（shots-fix1/）。
- 工程新事实：automator 标签/穿透选择器对 t-button 均 0 命中（T24"不可触达"再证实）；**page.data() 数据面是 t-button 属性的可观察 seam**（probe-tbutton{1..4}-fix1.cjs 诊断过程在档）。

## M2 两份声称留档证据缺失 → 重取到确切路径+更正表述

- 评审属实：第 7 节原声称的 `server-tail.log`、`fixture-scrubbed.json` 从未落盘（原归档步骤未执行完成，已在 task-report §8 更正表述，不虚报原始来源）。
- **重取（原 57819 现场不可复现 → 本轮 57825 等价真实取证）**：
  - `.superpowers/sdd/2026-09-24-issue-140-t32-miniprogram/server-tail.log`：本轮真实服务器日志尾部（登录→open→boundary→**对账 GET /exports/receipt**），password/token 源日志即 `***`，Bearer/未打码密码过滤 0 命中。
  - `.superpowers/sdd/2026-09-24-issue-140-t32-miniprogram/fixture-scrubbed.json`：本轮 fixture setup **16/16 PASS** 的一次性 fixture（真实 HTTP 全链构造），TA/TB/passA/passB/grantUrl 脱敏。
- 一次性密钥 keys.env 与原 fixture.json 已删除；DevTools/服务器已停止（57825/9436 空闲）。

## M3 真机 blocked → 环境门槛声明（交 T33）

真机无设备为本环境永久门槛（T06/T24/T26/T28 同口径；Wave 13 裁决 progress.md:278）。本轮补齐实证 `device-gate-evidence.txt`：`which adb`→not found（exit 1）、`adb devices`→command not found（exit 127）、`xcrun devicectl list devices`→仅 simulated、USB 总线无 iPhone/iPad/Android。不伪造真机结果，维持 blocked 由主控 T33 统一裁决。

## 验证命令（全跑，真实输出）

| 命令 | 结果 |
| --- | --- |
| `pnpm --filter @weknora/miniprogram test` | ℹ tests 135 / pass 135 / fail 0 / skipped 0（最终 HEAD 复跑） |
| `pnpm --filter @weknora/miniprogram typecheck` | 13 errors，全部 account/pages.tsx CommercialSummary 基线豁免（`grep -vc` 排除该文件=0，零新增） |
| `pnpm --filter @weknora/miniprogram build:weapp` | Compiled successfully（WEKNORA_API_ORIGIN=http://127.0.0.1:57825 联调构建，gitignored） |
| `git diff --check` | 干净 |

## 提交列表

| commit | 内容 |
| --- | --- |
| `df894023e` | fix(miniprogram): M1 导出/删除主按钮对齐 Web 门控——unknown/对账中封锁+重新对账恢复（gating.ts 新增+页面接线+3 用例） |

M2/M3 无产品代码改动（证据文件落 `.superpowers/` 目录，gitignored 不入库，Wave 11 同款），故无对应 commit。

## 文件清单（本轮改动）

- `apps/miniprogram/src/career/export-deletion.gating.ts`（新）
- `apps/miniprogram/src/career/export-deletion.tsx`（接线）
- `apps/miniprogram/tests/export-deletion.test.mjs`（+3 用例）
- 证据（worktree `.superpowers/sdd/2026-09-24-issue-140-t32-miniprogram/`，不入库）：server-tail.log、fixture-scrubbed.json（M2）、device-gate-evidence.txt（M3）、shots-fix1/（2 png+manifest）、driver-fix1.cjs、fixture-fix1.cjs、driver-fix1-run.log、probe-tbutton{,2,3,4}-fix1.cjs、RED-M1.txt

## 自查与遗留

- 三 medium 逐项处置完毕；2 low（跨会话旧授权复验弱化、A2 恒真断言）不在本轮简报范围，未动。
- t-button GUI 手势链限制维持 T24 定论；本轮以数据面探针把 disabled 态变为可观察，属能力增强而非替代。
- T32 verified 由主控裁决；修复轮 1/5，若复审有新 finding 按轮次继续。

## 修复轮 2/5（第 1 轮复审 F1 medium 修复；F2/F3 low 记录不修）

- BASE `df894023e` → HEAD `1dcda5415`（本地 1 commit，未 push；复审 diff 范围 5033b6d5f..1dcda5415）

### F1（medium）partial 回执后恢复未决不回封锁 → 已修复（TDD）

- **RED**：`tests/export-deletion.test.mjs` +3 F1 用例（partial→ambiguous 重试回封锁→对账落定 / 对账失败 not_found 保持封锁至回执可读 / definite 拒绝与 SCOPE_CHANGED 不置未决），`node --experimental-strip-types --test tests/export-deletion.test.mjs` → **tests 20 / pass 17 / fail 3**（deletionOutcomeUnknown 未创建/函数缺失；RED-F1.txt 在档）。
- **GREEN**：
  - `export-deletion.gating.ts` 新增 `deletionOutcomeUnknown`（intent 存续且非已呈报 partial，**或上次恢复尝试未决**）与 `deletionRecoveryUnresolvedAfter`（对账失败且 intent 仍在→一律未决，对齐 Web `ExportDeletionPage.tsx:286` lookupDeletionReceipt catch→phase='unknown'；重试仅 ambiguous（code=outcome_unknown）未决、definite 拒绝不封锁，对齐 :267-268 runDeletion uncertain→unknown 与 definite→error 分支；intent 不在当前作用域不置未决，防无恢复入口的封锁死局）。
  - `export-deletion.tsx`：`delRecoveryUnresolved` 页面状态；`reconcileDeletion`/`retryDeletion` 统一包装收口全部三个恢复入口（顶部对账/重试+partial 卡重试）；`acceptDeletion` 收到确定回执即落定未决（对齐 Web acceptDeletion）。盲区修复：partial 回执后重试/对账以 ambiguous 告终时，旧 partial 回执不再解锁主按钮与知悉，不可再经 `deleteWholeSpace()` 换新 requestId 重复发起删除。
- **单测真实性说明**：F1 序列需服务端故障注入（partial+ambiguous 组合）——真实 DevTools 复验无法在无故障注入的实服务器上构造该态，故以真实 service+transport 装配的假后端单测为准（与 Wave 11 B3/C2 同级别证据）；第 1 轮 13/13 真实 DevTools 门控阶梯不受影响（该流程不经过 partial 态）。

### 受影响测试重跑（全跑，真实输出）

```
node --experimental-strip-types --test tests/export-deletion.test.mjs  → ℹ tests 20 / pass 20 / fail 0
pnpm --filter @weknora/miniprogram test        → ℹ tests 138 / pass 138 / fail 0 / skipped 0（135+3 F1）
pnpm --filter @weknora/miniprogram typecheck   → 13 errors（account/pages.tsx 基线豁免；排除该文件 grep -vc=0，零新增）
pnpm --filter @weknora/miniprogram build:weapp → Compiled successfully（dist/career/export-deletion.js 含 recoveryUnresolved 布线）
git diff --check                                → 干净
```

### 第 1 轮复审 low findings（记录不修）

- **F2（low）行号引用偏差**：属实——Web 门控三行实际在 `ExportDeletionPage.tsx:291-293`（本轮已实测 grep 确认），第 1 轮报告/task-report §8 引作 296-298；所引语义本身准确。本轮起新增文字均按 291-293 引用；原文未改（low 不修）。
- **F3（low）'deleting' 确定回执的 parity 分歧**：对账返回确定 'deleting'（进行中）回执时小程序按 unknown 封锁（'deleting'!=='partial'→deletionUnknown=true），Web acceptDeletion 对非 deleted 回执统一 phase='error' 不封锁。小程序多封锁不产生竞态且有恢复入口（对账/重试置顶），为安全方向的语义不对齐，记录待后续轮次裁量。

### 提交列表（本轮）

| commit | 内容 |
| --- | --- |
| `1dcda5415` | fix(miniprogram): F1 partial 后恢复未决回封锁——对齐 Web runDeletion/lookup 失败置 unknown（gating 两助手+页面状态与三入口收口+3 用例） |
