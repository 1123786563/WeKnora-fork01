# T19 Web — 面试准备与来源修订（task-2-report）

- BASE `ff5b2e843` → HEAD `491088d44`
  - `1bfd4d679` `feat(web): review sourced interview preparations`
  - `491088d44` `fix(web): show preparation success notice and read the saved revision back from the material domain`
  - 本地提交，未 push/merge。
- 独立 worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t19-web/WeKnora-fork01`（detached，基线 ff5b2e843）。
- E2E 归档（截图 + evidence + 脚本 + 服务器日志）：集成 worktree `.superpowers/sdd/2026-09-24-issue-140-implementation/t19-web-shots/`；本目录留有同轮拷贝与失败轮残片。

## 1. 交付物（文件所有权 = 简报声明范围）

| 文件 | 变更 |
| --- | --- |
| `packages/api-client/src/career.ts` | T19 preparations 冻结合同：`PreparationFocus/Kind/Status/FailureCode/Anchor/SnapshotRef/Sources/Receipt`、`GeneratePreparationInput`、`PreparationList`、`decodePreparationReceipt`、`decodePreparationList`、client 方法 `generatePreparation` / `applicationPreparations` / `preparationReceipt`（严格解码：draft ⇔ materialId ⇔ 非空 body ⇔ 无 failureCode；failed 必带 code∈{generation_failed, claim_unconfirmed}；generating 无 code；非 draft 无 reviewRisks；anchor sha256 校验；draft 全量快照引用）。 |
| `packages/api-client/src/career.test.ts` | +4 测试：编码三方法路径、输入拒绝、发明 payload/空白成功产物拒绝（20 个 inventor）、typed failure/generating 态可解码。 |
| `apps/web/src/career/PreparationPage.tsx` | 新页面：生成（focus 选择）→ 版本锚定可见（“基于实际投递版本 V{n}”+提交/材料/导出/摘要）→ 来源链（岗位快照+SHA-256、已确认事实键、档案修订）→ 正文主张级追溯（已链接确认事实 / 待补充占位）→ 修订草稿（编辑段落/正文/主张 → `editMaterial` 保存 → 从材料域 `material()` 读回修订后正文呈现）→ 未确认投递版本 typed 提示态 → outcome_unknown 恢复（查回执/原请求编号重试）→ preparation_generation_failed 明确失败可恢复 → revision_conflict 重读 → 跨租户不可访问 → 零自动发送文案。 |
| `apps/web/src/career/PreparationPage.test.tsx` | +10 用户可观察行为测试（锚定可见/来源链/零自动发送/未知提示态/修订保存/失败可恢复同 ID 重试/失败行无正文/修订冲突/跨租户/样式）。 |
| `apps/web/src/career/preparation.css` | TDesign 浅色 token + `#07c05f` 品牌绿（生成/修订/来源行）。 |
| `apps/web/src/career/ApplicationPage.tsx` | 最小入口：`preparationOpen` 状态 + “面试准备与来源” 切换按钮 + `PreparationPage`（key=applicationId）。 |

未改后端、未改 career-core 合同；无缺口需要上报。

## 2. RED → GREEN 证据

### RED（先写失败测试并运行）

- api-client（`pnpm exec tsx --test packages/api-client/src/career.test.ts`）：
  - `✖ preparation client encodes generate, list and receipt recovery paths — TypeError: api.generatePreparation is not a function`
  - `✖ preparation client refuses blank identifiers, invented focuses and negative revisions — TypeError: refusing.generatePreparation is not a function`
  - `✖ preparation decoder rejects blank products that disagree with their status — TypeError: api.generatePreparation is not a function`
  - `✖ preparation decoder keeps the typed failure state decodable … — TypeError: api.preparationReceipt is not a function`
- Web（`node --import tsx --test src/career/PreparationPage.test.tsx`）：
  - `Error [ERR_MODULE_NOT_FOUND]: Cannot find module '.../apps/web/src/career/PreparationPage.tsx'`（11 个测试全部未运行）。

### GREEN

- api-client：`ℹ tests 35 / pass 35 / fail 0`（含 4 个新 preparation 测试）。
- Web：`ℹ tests 10 / pass 10 / fail 0`。
- 修正轮（E2E 发现的两处 UI 缺陷，单测同步）：
  1. 成功消息被 `writePhase !== 'idle'` 条件隐藏（acceptReceipt 置回 idle）→ 改为 message 存在即渲染（error 用 alert 语义）。
  2. 修订保存后 preparation receipt 是生成时快照（后端冻结语义），列表行不随材料草稿更新 → 保存成功后从材料域 `GET /materials/:id` 读回修订后正文，`[aria-label="修订后草稿"]` 区呈现。

## 3. 验证命令与真实输出（全在 HEAD `491088d44` 复跑）

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck:web` | 通过（无输出错误，exit 0） |
| `pnpm test:web`（全量 node26） | `ℹ tests 2459 / pass 2459 / fail 0 / cancelled 0`（基线 2449 + 新增 10） |
| `pnpm build:web` | `✓ built in 10.11s`（仅 chunk 大小警告，先例同款） |
| `git diff --check`（含暂存） | 干净（exit 0） |
| `pnpm typecheck:shared`（career.ts 属其输入清单，额外） | 通过（exit 0） |
| `pnpm exec tsx --test packages/api-client/src/career.test.ts` | `35/35 pass` |
| `pnpm test:shared`（额外） | api-client 段全绿；唯一失败 `packages/design-tokens/src/craft.test.ts` — **基线预存**：读 `packages/ui/src/theme.css`，该文件在 ff5b2e843 就不存在（`git ls-tree ff5b2e843 -- packages/ui/src/theme.css` 为空），与本任务无关。简报验证口径（typecheck:web/test:web/build:web/diff--check）不受影响。 |

## 4. 浏览器 E2E（后端端口 57818；T19 verified 关键验收）

- 环境：Lite SQLite 一次性库 `/tmp/weknora-t19-browser.B2t0Li`（随机 40 字符 JWT、`DISABLE_REGISTRATION=false`、`go build` 产物、CWD=worktree）监听 `127.0.0.1:57818`；未认证 `GET /api/v1/career/open` → 401。vite dev 代理 `127.0.0.1:57820`（`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57818`；57819 被其他会话进程占用，按隔离规则避开）。浏览器：仓库 `@playwright/test` 真实 Chromium headless。跑毕服务器与 vite 已停止，57818/57820 已确认释放；临时目录保留至集成评审。
- 层级声明：全真实链路（注册→档案确认(2026)→JD 导入（仅限2027届）→评估 ineligible+显式继续→创建申请→材料 V1→编辑确认 V2→发布导出 V2（双格式核验 submittable）→投递绑定 V2→生成面试准备），非预置 fixture；B/C 为同岗位不同批次真实 UI 创建。
- **15/15 相位 OK**（evidence.log 逐相位时间戳）：
  1. `register-A` → 平台登录落地。
  2. `career-confirm-graduation` → 确认事实“毕业时间=2026”。
  3. `jd-import` → 岗位+快照 URL。
  4. `evaluate-and-apply-A` → 申请 A，Task ready。
  5. `material-draft-confirm-V1-edit-confirm-V2-publish-export` → V2 发布导出 submittable（材料章节含带事实键主张“2026 年毕业/毕业时间”+占位主张“实习经历待补充”）。
  6. `submission-record-bound-V2` → 投递记录绑定 **版本 V2**。
  7. `preparation-panel-open` → 面板文案含 **不自动发送/不代为承诺**；生成表单就绪。
  8. `preparation-generate-anchored-V2` → 行含 **“基于实际投递版本 V2（材料 … · 导出 … · 提交 …）”**——版本锚定可见。
  9. `preparation-sources-review`（**关键验收：查看来源**）→ 来源链含 岗位快照+SHA-256、投递版本 V2+提交+内容摘要、**已确认事实：毕业时间**、档案修订、材料草稿 id；主张级 `2026 年毕业（已链接确认事实：毕业时间）`、占位 `实习经历待补充（缺失/待补充，不得补造）`。
  10. `preparation-revise-draft-save`（**关键验收：修订草稿**）→ 编辑器加载草稿正文 → 改标题/正文/主张 → 保存 → `修订已保存` + `[aria-label="修订后草稿"]` 从材料域读回：`本人修订` 正文、`2026 年毕业（本人补充：含两次内推复盘）（已链接确认事实：毕业时间）`、占位保留；列表行锚定 V2 保持。
  11. `preparation-cover-letter-focus` → 第二份草稿（求职信）同样锚定 V2；列表 2 条。
  12. `application-B-unknown-version-prompt`（B 批申请，无投递记录）→ `[role=alert]` `未确认实际投递版本：… 系统不会自行改用最新材料版本。请先记录投递…`；无草稿产物；全文无“最新版本”暗示。
  13. `application-C-unknown-outcome-recovery`（C 批申请：先记录投递绑定 V2 → Playwright route 中断一次 POST）→ `暂时无法确认准备是否已生成（原请求编号 <uuid>）`，无空白成功产物 → 「查询准备回执」→ 「用原请求编号重试」→ 生成成功且恰 1 条、锚定 V2（同 ID 重试实证）。
  14. `cross-tenant-preparation-denied` → 账户 B（清会话后注册登录）打开 A 空间申请 URL → 无准备面板、无来源链/锚定内容泄露（截图核验）。
  15. `http-replay-and-receipt-probes`（节点级 fetch，真实头）：A 列表 200（2 条，均 anchor.version=2）；**B 跨租户列表 403、无 preparations 字段**；B 批申请生成 → **409 preparation_version_unknown（submissionRecorded=false）**；A 同 requestId 同内容重放 POST → 200 同 receipt；回执查询 → 200 anchorVersion=2；同 requestId 不同 focus → **409 idempotency_conflict**。
- 截图 12 张：`01-material-V2-submittable-export` `02-submission-bound-V2` `03-preparation-panel-open` `04-preparation-generated-anchored-V2` `05-preparation-source-chain` `06-preparation-revision-editor` `07-preparation-revision-saved` `08-preparation-cover-letter` `09-unknown-version-prompt` `10-unknown-outcome-recoverable` `11-unknown-outcome-recovered` `12-cross-tenant-denied`。
- 迭代史（非产品缺陷的脚本/UI 修正，残片保留）：run1 成功消息渲染条件缺陷（本仓 UI，已修）；run2 材料无主张致 claim 断言无法命中（脚本增强）；run3 修订后内容呈现语义（本仓 UI，改为材料域读回）；run4/5 脚本 `phase` 不返回值与跨租户注册上下文（脚本修正）。最终轮 15/15。

## 5. 简报行为要求逐条自查

- 版本锚定可见：✓（单测 + E2E 相位 8/11；HTTP probe anchorVersion=2）。
- 未知投递提示态不猜最新：✓（相位 12 + HTTP 409 typed；提示文案引导先记录投递或显式未知口径，无“最新版本”字样）。
- 来源链可追溯：✓（相位 9：事实/快照/版本引用三层 + 主张级事实键）。
- 修订草稿保存：✓（相位 10：editMaterial + 材料域读回；不覆盖既有草稿语义按后端冻结呈现——receipt 是生成快照，材料草稿演进）。
- 失败保留可恢复无空白成功产物：✓（相位 13 outcome_unknown 同 ID 恢复；`preparation_generation_failed` 明确失败态同 ID 重试；失败行无正文/无修订入口——单测覆盖）。
- 零自动发送：✓（面板声明文案；无发送类控件——单测断言按钮集合）。
- TDesign 浅色 + `#07c05f`：✓（preparation.css，单测断言 token 与品牌色）。

## 6. 已知局限

- 修订流经既有 `editMaterial`（materials 域），preparation receipt 的正文是生成时快照；面板通过材料域读回呈现修订后正文，列表行保留生成快照与锚定（按后端冻结语义如实呈现，UI 有说明文案）。
- `PreparationVersionUnknownError.submissionRecorded` 两分支（未记录/已记录未确认）在 UI 合并为一个提示态文案（errors.ts 透传不含该字段，且其不在本任务文件所有权内）；HTTP probe 已实证后端两字段齐全。
- test:shared 的 `packages/design-tokens/src/craft.test.ts` 在基线 ff5b2e843 即失败（缺 `packages/ui/src/theme.css`），与本任务无关，未修（不在所有权内）。
