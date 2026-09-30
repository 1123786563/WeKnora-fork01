# Wave 11 — T32 小程序：微信小程序全空间导出与完整删除（Issue #173 implement）报告

- 状态：**DONE**（真机门槛单列 blocked：无设备，T06/T24/T26 先例）
- BASE `ff5b2e843` → HEAD `5033b6d5f`（本地 1 commit，未 push；live 验证未发现产品代码缺陷，无修复轮提交）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t32-miniprogram/WeKnora-fork01`
- 完整报告+证据：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t32-miniprogram/task-report.md`（9 张可区分截图+manifest、driver/fixture/probe 脚本、脱敏服务端日志、fixture-scrubbed）

## RED → GREEN（TDD）

- RED：`node --experimental-strip-types --test tests/export-deletion.test.mjs` → **tests 14 / pass 0 / fail 14**（exportWholeSpace/deleteWholeSpace/saveSpaceExportPackage 等全部缺失）。
- GREEN（简报四命令照跑，真实输出）：
  - `pnpm --filter @weknora/miniprogram test` → `ℹ tests 132 / pass 132 / fail 0 / skipped 0`（118 基线 + 14 新增；dist 构建后 build-output 用例一并执行）
  - `pnpm --filter @weknora/miniprogram typecheck` → 13 errors（全部 account/pages.tsx CommercialSummary 基线豁免，**零新增**；过程中曾引入 1 个新错误并已修复）
  - `pnpm --filter @weknora/miniprogram build:weapp` → `Compiled successfully`；dist/career/export-deletion.* 产出；t-button 引用 `/npm/tdesign/button/button`
  - `git diff --check` → 干净

## 提交列表

| commit | 内容 |
| --- | --- |
| `5033b6d5f` | feat(miniprogram): T32 全空间导出与完整删除——同源合同、边界强制确认与三 seam 恢复（service 五路由全链+删除 intent 存续至终态、adapter 本端留存 seam、页面、14 用例、taro-stub writeFile/setClipboardData、路由 22→23） |

## 文件清单

所有权内（简报 Files + 调度员补充 service 扩展）：
- `apps/miniprogram/src/career/export-deletion.tsx`（新）+ `export-deletion.config.ts`（新）
- `apps/miniprogram/src/adapters/career-platform.ts`（扩展：spaceExportPayload 完整序列化不截断、saveSpaceExportPackage 写 USER_DATA_PATH+落盘字节复算摘要、copySpaceExportToClipboard 等效可取得流程）
- `apps/miniprogram/src/services/career.ts`（扩展：exportWholeSpace/spaceExportReceipt/deletionBoundary/deleteWholeSpace + pending/reconcile/retry + clearCareerCaches；与 Web ExportDeletionPage 同一批 api-client 解码器=同源同版本）
- 测试落位：计划点名 `export-deletion.test.tsx`，harness 为 `node --test tests/*.test.mjs`（strip-only 不支持 JSX）→ 落位 `apps/miniprogram/tests/export-deletion.test.mjs`（T24/T26 同款偏差）

必要配套（T26 先例同款，逐条声明）：
- `apps/miniprogram/src/app.config.ts`（career 分包 +export-deletion）、`src/core/routes.ts`（+careerLifecycle）、`src/career/discovery.tsx`（入口行，页面可达无死代码）、`tests/core.test.mjs`（路由 22→23）、`tests/helpers/taro-stub.mjs`（writeFile/setClipboardData 契约+状态）

## 行为验收要点（证据层级）

- **导出内容与其他端一致**：同一后端包（同一冻结合同+同一批解码器）；单测 A1/A2 断言 POST body 恰为 requestId+expectedRevision 两字段、归档六段包含性（事实/历史/岗位快照/申请事件/材料版本/投递记录）；DevTools 驱动在真实服务器上渲染同清单（真实计数+digest 头+固定修订）。
- **保存/打开等效可取得流程**：平台无浏览器式下载→①完整 JSON 写 USER_DATA_PATH 真实文件（路径/字节/落盘字节复算 sha256 如实呈现）②完整内容剪贴板复制（DevTools 实测 3923 字符完整 JSON 回读）——限制如实说明、**不静默截断**（单测 A3/A4+驱动 D 组双重钉死）。
- **删除说明保留规则和外部平台边界**：删除前必须先「查看删除边界」（12 段空间内计数/2 项外部不可撤回/4 项保留披露）+勾选知悉+二次确认 modal；t-button 在未满足前禁用。
- **部分失败可恢复不称完全删除**：服务端合同（deleted⇔步骤全 done）+解码器双向校验；**部分失败是成功响应但未到终态——intent 特意存续至 deleted**（与通用 writeRecoverable 的差异，单测 B3 钉死同 requestId 恢复+重放原 revision）。
- **删除后重进不恢复**：app 内退出登录→重新登录同一账号→「还没有求职档案」空态（截图+图像复核）；服务端复核 open→facts=0/proposals=0。
- **旧链接拒绝+本地缓存清理双重验证**：删除前导出回执删除后 404 not_found（app 内复验提示+服务端探针）；删除前签发的材料下载授权 URL 删除后 404；clearCareerCaches 恰清 wk:career:* 键（单测 D2+驱动 getStorageInfoSync 复核存储无 career 键）。
- **三 seam 可观察**：冲突回执（E3 revision_conflict+currentRevision、E2 跨租户 403 typed）、未知对账（C1/C2 原编号对账/安全重放）、空间切换（E1 SCOPE_CHANGED 无残留 intent）。
- **真实 DevTools（57819，全新隔离临时 DB+一次性密钥）**：app 驱动 **26/26 PASS**（登录→入口→恢复态→导出对账→保存→复制→边界→知悉→**app 内真实 POST /deletions**→完成态+旧授权复验+缓存清理→重进不恢复→跨租户拒绝）；服务端探针 setup **16/16** + verify **6/6**；9 截图 9 个不同 sha256；关键截图（删除回执/重进空态）经图像模型视觉复核。

## 自查与遗留

- live 驱动四轮迭代全部为测试侧问题（fixture progress 修订域/文本分段空白/长页滚动方式），**产品代码零缺陷零改动**。
- 真机验证 **blocked**（无设备）：真机差异点已记录（USER_DATA_PATH 前缀 wxfile://、剪贴板权限弹窗、留存文件在文件管理中的可见性）。
- t-button GUI tap 不被 automator 触达（T24 定论）：主按钮业务链由恢复入口驱动的**同一真实 POST 链**+132 单测+22 服务端探针组合覆盖。
- 工程新事实入册：progress 的 expectedRevision 域=每应用事件计数；签名下载 TTL≤15min；长页滚动用 Element.scrollIntoView（pageScrollTo 不可靠）；app 内登出吊销服务端会话（复验需重登）。
- 环境已清理：57819/9435 空闲、DevTools/服务器停止、keys.env/fixture.json 删除、脱敏归档完成。
- T32 verified 由主控裁决（小程序链 T24→T26→T32 收尾，仅剩 T28/T30）。
