# Wave 7 — T16-Web 报告：材料发布与授权下载 E2E（Issue #158）

- 实现者：frontend_implementer（T16-Web）；BASE `067aa1d0e` → HEAD `0e2439a3c`（`feat(web): publish and download verified material exports`，独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t16-web/WeKnora-fork01` 单本地提交，未 push）
- 详细报告（RED/GREEN 全文、逐相位证据、自查与局限）：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t16-web/task-2-report.md`
- E2E 归档：本目录 `t16-web-shots/`（截图 8 张 + evidence.log + e2e-run.mjs + server-startup.log + vite.log + testweb-full.log）

## 1. 交付物（文件所有权 = 简报声明范围 + 一行必要接线）

- modify：`apps/web/src/career/MaterialPage.tsx`（导出区扩展：发布/列表/下载/撤销/恢复，最小侵入不新建 ExportPage）+ `MaterialPage.test.tsx`（10→19 测试）+ `material.css`
- modify：`packages/api-client/src/career.ts`（冻结导出类型 + 严格解码器 + 五方法 publishMaterial/materialExports/materialExportSignedURL/materialExportDownload/revokeMaterialExport）+ `career.test.ts`（+2 聚焦测试）
- **所有权外一行**：`packages/api-client/src/client.ts:355` `createCareerApi(request, requestBinary)` —— 认证下载必须经二进制 transport 携带 Authorization（Wave 7 主控裁决的认证兑付语义）；单参调用签名不变，client.test.ts 17/17 无回归。未改后端、未改 career-core。

## 2. TDD 与验证（真实输出）

- RED：api-client `TypeError: refusing.publishMaterial is not a function`（20/22）、MaterialPage 9 失败（10/19，缺导出区与 css）；GREEN：api-client 22/22、MaterialPage 19/19、career 家族 145/145。
- `pnpm typecheck:web` TYPECHECK_PASS（0 错误）；`pnpm test:web` **2420/2420，0 fail 0 cancelled**（基线 2412 + 8 新 web 测试）；`pnpm build:web` `✓ built in 15.25s`；`git diff 067aa1d0e --check` exit 0。

## 3. 浏览器 E2E（后端端口 57813；T16 verified 关键验收）

Lite SQLite 一次性库（`WEKNORA_CAREER_EXPORT_SIGNING_KEY` 64hex）+ vite dev 57815 代理 + 真实 Chromium headless；走真实链路（注册→建档→JD→评估 ineligible+显式继续→申请 Task ready→材料 V1/V2）。**16/16 相位 OK**：发布 V1/V2 导出（staged→双验→`submittable（可用于投递）`、同正文摘要 `ad0fcfb4…`/`24259314…` 按版本区分、双文件行均「与导出正文摘要一致」）→ **认证下载 PDF（2356B）与 DOCX（2188B），路由拦截字节 SHA-256 与 signed-url 响应 digest 双端一致（match=true，Content-Type/Disposition 正确）** → 撤销前旧授权兑付 200 → 撤销 → **同授权立即 404 `export_grant_invalid`** → **V1 旧版本导出仍可下载且 digest 再次一致** → 存储篡改 DOCX → 服务端兑付摘要复检 500 拒绝（UI「下载未完成」）→ HTTP 探针（同 ID 同内容重放 200 同 exportId 无重复行；同 ID 异内容 409 idempotency_conflict；未认证 401）。

## 4. 声明与遗留

- 单格式 staged 失败无真实浏览器构造路径（后端确定性双验通过，无法经公共 API 注入）：组件级 stub 双测试覆盖（staged 保留错误不可投递 + 半发布声称被解码器与 UI 双门拒绝），如实声明；E2E 以存储篡改覆盖真实兑付完整性失败呈现。
- 导出回执无独立查询端点（`FindMaterialReceipt` 只解 material_edited/confirmed）→ UI 以同 requestId 幂等重放恢复（E2E 探针已证幂等）；后端合同缺口如实上报。
- 后端 Wave 6 遗留 2 low 转述不阻塞：认证兑付已获 Wave 7 调度员裁定成立（本客户端实现与裁决一致）；测试助手 Goexit 缺陷属后端测试侧。
- 服务器/vite 已停、端口 57813/57815 已释放；`/tmp/weknora-t16-browser.8704` 保留至集成评审。

## 5. 结论

T16 Web 面交付并完成 Web 下载 E2E 验收全链路（发布→双格式下载 SHA-256 比对→撤销立即失效→旧版本可下载→失败呈现→幂等探针）。T16 verified 由主控裁决。
