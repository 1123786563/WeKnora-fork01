# Wave 16 — T33 修复轮 1/5 报告：两处矩阵证据缺口（Issue #172，fix-resume）

- 角色：frontend_validator + backend_validator 合一（终局验收员修复轮）。轮次：2026-09-27（Asia/Shanghai）。
- 现场：独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01`，基线 `92718d338`（续接既有 checkpoint，clean），本轮单 commit **`fbee27c8a`**（`docs(verification): close launch matrix evidence gaps`）。未 push/merge。
- **生产代码零改动**（diff 范围 92718d338..fbee27c8a：launch-matrix.md + T33 目录证据/日志/报告，8 文件 +1358 行）。
- TDD：本轮为纯验证文档修复（M1 重跑取证 + M2 矩阵补格），无生产/测试代码变更，RED→GREEN 不适用（与终局轮同一先例口径）；简报验证命令全部实跑并落档。

## 1. M1：§0 六个回执 ID 无入库证据（缺口→补证→指针）

- **缺口**：launch-matrix §0 六个截断标识（提案回执 `d2e5d8bb…`/确认回执 `441a6c02…`/机会导入 requestId `c666ff58…`/导出正文摘要 `8a333f7f…`/PDF SHA-256 `a65d6bd4…`/Task(session) `ef1125b7…`）当轮响应原文未落档；当轮一次性 DB+密钥已销毁，**原始 ID 不可复现——如实标注，不虚报**。
- **补证**：同 worktree 重开隔离 Lite 服务器（127.0.0.1:57828，一次性 SQLite+一次性 JWT/AES/导出签名密钥（≥32B hex 合规）+一次性 local 存储），python3 stdlib 探针以全新一次性账号复走同一七步链+失败态（与 Web 前端同一 HTTP 契约）。**EXIT=0，17×HTTP 200**；原文脱敏落档 `evidence/web-receipts-r2.txt`；PDF 落盘 `evidence/career-material-v1-r2.pdf`（PDF 1.7/1 页/2211B；shasum `e74104c4…`==导出回执 `files[pdf].fileDigest`，自洽）。
- **指针更新**：矩阵新增 §0.1（六 ID→r2 回执映射 + 不可复现声明）；§0 原表保持历史记录。附加真实实证：同 requestId CAS 重试恢复成功（步 8）、二次投递 409 `submission_already_confirmed`（步 7b）、找岗诚实失败 `no_vetted_sources`（步 9）。

## 2. M2：分享导入/通知权限无专门格（缺口→补证→指针）

- **缺口**：主计划 T33 验收明文五项（分享导入/PDF 下载/通知权限/跨端同步/越权）中两项缺矩阵专门格。
- **补证**：矩阵新增 **§2A** 两张分环境专门格（层级如实标注：一级=集成仓库已入库归档 wave-report；二级=本轮 r2 指针）：
  - **§2A.1 分享导入**：微信=✅ T24 先核对后提交（DevTools 两步+percent-encoding 修复 `09a414c8b`，一级=`wave-report-T24小程序.md`；本轮未重开 DevTools，按简报允许路径引用归档+层级标注）；Web=not-applicable（能力事实）；iOS/鸿蒙/Android=blocked（引 §1.2/§1.4/§1.5）。
  - **§2A.2 通知权限**：微信=真机弹层 **blocked（如实）**（tourist appid 无模板可配；模拟器真实调用失败不弹层→页面如实 unavailable+站内回落、delivered 恒 false；C2 单测钉死拒绝路径，一级=`wave-report-T30小程序.md`）；Web=✅ 站内待办 T20 verified（冻结正文/去重/退订后可读+重订阅/live 4 项，一级=`wave-report-T20-Web.md`）；iOS/鸿蒙/Android=blocked。
- PDF 下载/跨端同步/越权三项既有指针复核在位（§0 文件下载行、§1.3 同源跨端、§0 越权行+§2）。

## 3. 四门复跑（同 HEAD 92718d338→fbee27c8a 工作区实跑，gate-logs/）

| 门 | 命令 | 结果 |
| --- | --- | --- |
| Go career | `go test ./internal/modules/career/... -count=1` | `ok 108.538s` exit=0 |
| Web 类型 | `pnpm typecheck:web` | exit=0 |
| 小程序类型 | `pnpm --filter @weknora/miniprogram typecheck` | exit 2：13 errors 全部 `features/account/pages.tsx` 既有基线（该文件外 0）——零新增 |
| 小程序测试 | `pnpm --filter @weknora/miniprogram test` | tests 172 / pass 172 / fail 0 / skipped 0（≥167+5 基线，5 个 build-output 条件跳过项实际执行通过） |
| diff 卫生 | `git diff --check` | exit=0 |

**零回归确认**（四门与终局轮口径一致或更好）。

## 4. 提交与文件清单

- commit：`fbee27c8a`（单 commit，diff 范围 `92718d338..fbee27c8a`，复审用）。
- 文件：`docs/plans/issue-140/verification/launch-matrix.md`（M）；`.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/web-receipts-r2.txt`、`evidence/career-material-v1-r2.pdf`（A）；`gate-logs/{go-career,typecheck-web,mp-typecheck,mp-test}-r2.log`（A）；同目录 `task-report.md` §5 修复轮节（M）。
- 隔离：端口 57828 独占（用毕释放）；临时目录 `/tmp/wk-t33fix-20260927-004238` 用后清理；一次性密钥未落盘（进程内联）。

## 5. 自查与遗留

- 如实声明：M1 原六 ID 的原始响应体不可复现（一次性环境已销毁）——以等价链路（同 HEAD 同契约同链路）全文落档替代并在矩阵声明；M2 微信侧为一级归档引用+层级标注（本轮未重开 DevTools/无真机），符合修复简报允许路径。
- 编辑过程一处 §2 标题误删已当场恢复，最终结构复核 0/1/2/2A/3/4 完整。
- 3 low（全量 test:web 欠账、证据计数/空文件瑕疵、mp typecheck 13 基线）不在本轮 2 medium 范围，维持终局轮处置建议（parked/待空闲补跑），未动。
- T33 verified 终裁权在主控 Step 4；本报告不代裁。
