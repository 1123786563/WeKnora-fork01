# Wave 9 — T18-Web 投递确认与版本回看（Issue #159）报告

- 实现者：frontend_implementer（T18-Web，独立 worktree `issue-140-t18-web`）
- BASE `10e2f30da` → HEAD `efce8c755`（`feat(web): confirm submissions with bound versions`，本地提交未 push）
- 详细报告：worktree `.superpowers/sdd/2026-09-24-issue-140-t18-web/task-2-report.md`；E2E 归档：本目录 `t18-web-shots/`（截图 10 张 + evidence.log + server-startup.log + e2e-run.mjs + 失败轮残片 shots-run1-failed/）

## 1. 交付物（文件所有权 = 简报声明范围）

- create：`apps/web/src/career/SubmissionPage.tsx`（+11 测试 +submission.css）——渠道/声明时间/版本选择或显式未知、本人确认文案、投递记录时间线、版本只读回看、typed conflict/原 requestId 恢复/revision 冲突/跨租户错误态
- modify：`packages/api-client/src/career.ts`（T18 冻结类型+严格解码+三方法 recordSubmission/applicationSubmissions/submissionReceipt；显式未知标记排他；单列表≤1 防线）+ `career.test.ts`（2 聚焦测试）
- modify：`apps/web/src/career/ApplicationPage.tsx`（「投递确认与回看」按需入口，key 隔离跨申请）+ `ApplicationPage.test.tsx`（1 接线测试）；`MaterialPage.tsx`（可选 `onMaterialId` 上报材料指针，1 effect）

冻结：投递记录时间线在 SubmissionPage 内呈现，与 ProgressPage 时间线**并存**。

## 2. TDD 与验证（真实输出）

- RED：api-client `TypeError: api.recordSubmission is not a function`（25/27）；SubmissionPage `ERR_MODULE_NOT_FOUND`；ApplicationPage 接线（stash 验证）16/17。
- GREEN：api-client 27/27；SubmissionPage 11/11；ApplicationPage 17/17；career 目录合计 127/127。
- `pnpm typecheck:web` 通过；`pnpm test:web` **2442/2442，0 fail 0 cancelled**（基线 2430+12 新）；`pnpm build:web` `✓ built in 25.27s`；`git diff --check` exit 0。
- 孤儿 runner：仅主仓库 issue106 会话的活跃 runner（未触碰）；web 测试无端口绑定。

## 3. 浏览器 E2E（后端端口 57815；T18 verified 关键验收）

Lite SQLite 一次性库 + vite dev 57816 代理 + 真实 Chromium headless；全真实链路（注册→档案确认→JD→评估 ineligible+显式继续→申请 A→材料 V1→发布 submittable 导出），B/C 为同岗位不同批次真实创建。**13/13 相位 OK**：

1. 面板打开：文案「系统不代投、不发送邮件、不填写外部表单；点击下载或发布导出不会被视为投递」；版本下拉三态（V1 导出 / 显式未知）。
2. 记录投递（渠道 web、声明时间 18:30→`occurredAt=10:30Z`、绑定 V1 导出）→ 时间线恰 1 条记录（渠道/确认者/请求编号/版本 V1（材料+导出）/备注）。
3. **回看绑定的材料版本**：点「回看版本 V1」→ 只读区呈现 V1 正文+固定证据。
4. **未知版本显式 unconfirmed**：B 批申请选「未知版本」→ 记录行「版本未确认（显式未知）——确认时未绑定材料版本」，无回看按钮。
5. **重复确认被拒**：typed conflict alert「已有投递记录…不会产生第二条」，记录仍 1 条。
6. **未知回执恢复**：route 中断一次 POST → 「原请求编号 <uuid>」→ 查回执 404 提示 → 原编号重试成功，恰 1 条。
7. 跨租户：账户 B 打开同 URL → 「当前空间不可访问」无泄露（截图核验；该相位脚本 log 两行格式瑕疵已在 task-2-report §4 注明，判定以截图+HTTP 探针为准）。
8. HTTP 探针：A 列表 200（1 条 confirmed+V1 绑定）；B 跨租户 403 无泄露；同 ID 同内容重放 200 同 submissionId；同 ID 异内容 409 idempotency_conflict；**新 ID 重复确认 409 submission_already_confirmed**；回执 200 同 submissionId；B 回执 403。

第一轮 E2E 曾因脚本按钮子串匹配把已开面板点关而失败（脚本缺陷，修正后全量重跑通过；残片保留）。

## 4. 提交列表

| SHA | 说明 |
| --- | --- |
| `efce8c755` | feat(web): confirm submissions with bound versions |

## 5. 已知局限（后端 4 low 转述自 wave-report-T18后端.md §7，不阻塞）

一申请一记录 UNIQUE 硬约束（重投语义走 progress resubmitted）；SubmissionBoundVersion 对未知版本返回 ErrSubmissionNotFound（下游先读 versionConfirmed，本面板不受影响）；PG 迁移 000204 无 PG 实例仅 sqlite 实证；bindProgressApplication 复用未重命名。前端侧：已有记录时表单仍开放（呈现 typed conflict 真路径+常驻警示）；版本回看为面板内只读渲染非共享 MaterialPage DOM；E2E 相位 7 脚本 log 格式瑕疵（已注明）。

## 6. 结论

T18 Web 面交付并完成 Web E2E 验收全链路（记录投递→时间线回看→版本回看→显式未知→重复拒绝→原 requestId 恢复→跨租户拒绝→HTTP 幂等/typed conflict 探针）。T18 verified 由主控裁决（解锁 T19/T26）。
