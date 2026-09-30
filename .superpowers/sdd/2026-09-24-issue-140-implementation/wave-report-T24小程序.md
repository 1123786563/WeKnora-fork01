# Wave 4 — T24 小程序：微信小程序（Taro 4 + TDesign Miniprogram）C 找岗、建档与分享导入（Issue #164）

状态：**DONE**（真机门槛单列 blocked：无设备，T06 先例）。worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t24-miniprogram/WeKnora-fork01`，BASE `6a21e0df5` → HEAD `9ef4ee4c0`，本地 4 commits 未 push。完整报告+截图+服务器日志：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t24-miniprogram/`（task-report.md、shots/、live-server.log、live-driver.cjs）。

## RED / GREEN 证据

- **RED（行为级）**：先写 24 个用例（career-discovery 18 + career-platform 6，另 P7 后补）+ 空骨架（throw NOT_IMPLEMENTED）→ `node --experimental-strip-types --test tests/career-discovery.test.mjs tests/career-platform.test.mjs` → **tests 24 / pass 0 / fail 24**。
- **GREEN（三命令真实输出）**：
  - `pnpm --filter @weknora/miniprogram test` → `ℹ tests 87 / pass 87 / fail 0 / skipped 0`（62 基线全绿 + 25 新增）
  - `pnpm --filter @weknora/miniprogram typecheck` → 13 errors，全部为 account/pages.tsx CommercialSummary 既有基线，**零新增**
  - `pnpm --filter @weknora/miniprogram build:weapp` → 成功；`dist/career/discovery.{js,json,wxml,wxss}` 产出；`dist/career/discovery.json` usingComponents `"t-button":"/npm/tdesign/button/button"`（无 node_modules 段）；build-output D1/D2/F1 断言 PASS（含主包 <2MB）
  - `git diff --check` → 干净

## 关键行为（全部有可观察测试）

- **微信身份只关联不建第二档案**：service 无任何创建档案调用；open 经服务端 ClaimSpace 派生唯一档案（A1/A2 + DevTools 实测 A/B 两步）。
- **逐项确认建档**：propose/上传 → pending proposal → confirm_proposal 才进 facts（B1/B2 + DevTools B 三步）。
- **冲突回执 / 未知对账 / 空间切换失效**：C1（409+currentRevision 保真）/C2（outcome_unknown→原 requestId reconcile）/C3（旧响应丢弃）（+ D5 搜索侧）。
- **找岗消费 searches 冻结合同**：POST body 恰 {requestId,query,expectedRevision}（DisallowUnknownFields 兼容）；行含 checkedAt/qualification/link/uncertainty；coverage 如实（无来源不编造，D2/D3/D4）。
- **分享导入先核对后提交**：prepareSharedImport 纯本地预览（零网络）→ confirm 提交原文；负载缺失 → share_payload_missing 可恢复（E1/E2/P1-P4/P7 + DevTools D 两步）。
- **Web 档案变更同步可见**：changes 增量合并（G1 + DevTools E：上海→杭州）。

## 真实 DevTools 验证（12/12 PASS）

- 环境：CLI 2.02.2608070 + tourist appid + `cli auto` + miniprogram-automator（仓库外）+ 冷编译 settle-retry；Lite 服务器（go run ./cmd/server，SQLite/一次性密钥），Career 数据全部经公开 HTTP API 构造（如实声明构造层级）。
- **端口偏差**：57809/57810 被并行 T11-Web 任务先后占用（lsof 证据：node PID 49572/58131，cwd issue-140-t11-web；我的 bind FATAL 记录在临时 server.log）→ 按端口隔离约束改用 **57821**。
- 步骤：登录关联（真实 /auth/login）→ 同一份 Web 档案可见 → 逐步建档 pending → confirm 生效 → 一次性搜索（无来源如实 failed+no_vetted_sources）→ 未知搜索对账恢复（app 自身入口）→ 分享导入预览（发现并修复 query percent-encoding 缺陷，09a414c8b）→ Web 改档案同步可见（上海→杭州）→ 跨租户 B 登录只见自己空档案 → 匿名 401/B 读 A 回执 403。
- **已知限制（如实）**：① 真机无设备 → blocked；② t-button 的 GUI tap 不被 automator 触达（五种注入方式实测无效，wrapper 不等于组件节点）——事件交付以 build-output D2 产物断言 + T06 修复链（8996f0ef4，基线内）为据，业务由单测+DevTools 数据面组合覆盖；③ 搜索/导入的发起 POST 由同 token 等价请求代发（request-layer seam，T06 先例），渲染走 app 自身代码。

## 提交

| commit | 主题 |
| --- | --- |
| d69c251a1 | feat(miniprogram): link wechat identity to shared career desk |
| a4c15927a | feat(miniprogram): discover jobs and confirm facts on weapp |
| 09a414c8b | fix(miniprogram): decode percent-encoded share payloads on weapp entry |
| 9ef4ee4c0 | fix(miniprogram): keep act source.kind inside the frozen server whitelist |

## 文件清单

所有权内：`src/services/career.ts`（新）、`src/adapters/career-platform.ts`（新）、`src/career/discovery.tsx`（新）、`tests/career-discovery.test.mjs`+`tests/career-platform.test.mjs`（新；node strip-only 不能执行 .tsx，测试按工程惯例落 tests/*.test.mjs）。必要配套（报告声明）：`src/career/discovery.config.ts`、`app.config.ts`（career 分包）、`core/routes.ts`、`features/home/pages.tsx`（入口）、`tests/core.test.mjs`（路由守卫 20→21）、`packages/career-core/src/desk.ts`（parameter property→显式赋值等价重构，strip-only 兼容）。

## 自查与遗留

- 版本/缓存：构建显式 `cache:{enable:false}`；credential 不长存（登录链既有约束）；分享草稿仅内存；受控存储 `wk:career:` 前缀随登出清理（P5）。
- 原生例外：无（t-button=TDesign；展示复用项目 wk 设计系统属统一视觉）。
- 遗留：真机验证 blocked（无设备）；t-button GUI 自动化限制（如需 GUI 级证据需人工操作）。
