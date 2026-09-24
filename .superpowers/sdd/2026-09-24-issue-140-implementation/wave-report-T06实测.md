# Wave 1 — T06 实测（微信开发者工具真实环境验收）报告

- Agent: frontend_validator（只读验证，零生产代码改动）
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t06-live-validation/WeKnora-fork01`（基线 `21df162a24d73d9ec68d012458577050c18fd59f` detached，调度员指示的复用现场，实测 status 干净）
- 提交：`13efe84dd docs(miniprogram): record T06 live wechat validation`（仅新增 `docs/plans/issue-140/task-6-live-validation.md`，未 push）
- HEAD：`13efe84dd`；评审包：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-implementation/review-21df162a2..13efe84dd.diff`（1 commit，11890 bytes）

## RED/GREEN 说明

本任务是 validation（零生产代码改动），无实现型 RED→GREEN。等效"RED"是缺陷证据、"GREEN"是通过证据，全部为真实命令输出：

## 服务器侧（隔离 Lite，127.0.0.1:57801，全部 PASS）

- 启动：`DB_DRIVER=sqlite … SERVER_PORT=57801 … go run ./cmd/server`，`GET /api/v1/career/open` 未认证 → **401**（就绪信号，与 T03 同口径）。
- 注册两用户 → 201/201；登录 → 200/200；tenant 1（owner A）/ tenant 2（B）。
- fixture：`sessions` + `agent_runs`(succeeded) + `messages`(artifacts: `local://1/t06-live-fixture.pdf`, 403B) + `artifact_versions`(ready, digest=93e9aed7…70c)；存储文件 SHA-256 `93e9aed76cc5593d06c663f7f5b5daa9496ff736810bfb34ab470858d560070c`。
- 探针（真实 curl 输出）：A 列表 200（1 项）/A 签发 200/免凭证下载 200 403B 且 SHA-256 与存储文件一致；versions 路由 200 且 `digest` 与字节一致（**三方比对 PASS**）；B 列表 **404**、B 签发 **404**、未认证 **401**；`ttl_seconds=1` 2 秒后兑换 **401 `artifact_grant_expired`**；篡改 `message_id` **401 `artifact_grant_invalid`**；撤销后重放 **404**。

## 真实微信开发者工具（CLI 2.02.2608070，模拟器基础库 3.16.3，touristappid）

- 对照实验（极简原生工程）：reLaunch/元素读取全通 → 本机自动化链路健康（冷编译 ~45s，需 settle 后再发命令）。
- **缺陷 D1**：官方 `build:weapp` 产物在真实工具无法启动——`dist/subpackages/execution/artifact/index.json` 的 `t-button` 引用 `/npm/.pnpm/tdesign-miniprogram@1.17.0/node_modules/...`（未产出 dist/npm，且文件放到位仍因路径含 node_modules 被拒），DevTools 日志 `getAppJSON error … 未找到组件` + `simulator launch catch error`，白屏。
- 本地解锁（仅 gitignored 构建产物）：拷 `miniprogram_dist` 到 `dist/npm/tdesign` + 改页面 JSON 引用后，**应用启动**：首页渲染 → GUI 邮箱密码登录（真实 `/auth/login` 200，会话入 storage）→ 任务 tab 显示 fixture 任务 → 详情页 → 产物页卡片（`t06-live-fixture.pdf · application/pdf · 403 B`）——以上全部为应用自身代码路径（截图 shot-20~24/30~31/40~41）。
- **缺陷 D2**：产物页唯一动作按钮“打开或保存”无效——`dist/base.wxml` 的 `<t-button>` 模板**无任何事件绑定**（onTap 被 Taro 编译丢弃）；同页“重新读取”（原生按钮）点击在服务器产生 GET（阳性对照），t-button 死寂；模板级补 `bindtap="eh"` 仍无法送达 React 处理器（事件注册在编译期缺失）。
- **请求层 seam（已声明，简报允许的最小替代）**：在真实运行时用应用自有登录凭据复现应用同序 API：mint(wx.request 200) → wx.downloadFile(带 Authorization，200，403B) → getFileInfo(403B) → **wx.openDocument 成功**；下载字节 SHA-256 = 存储文件 = API digest（**真实运行时三方比对 PASS**）。
- **缺陷 D3**：`unlinkSync(http://tmp/…pdf)` 抛 `permission denied`，删除后文件仍存在（getFileInfo 403B）；应用 catch 吞错，与页面“立即清理”承诺不符（模拟器口径；无真机）。

## 结论（供主控裁决，不由本 agent 宣布 verified）

- 服务端签名授权/权限拒绝/摘要链：真实环境全部通过。
- 小程序 GUI：加载（需 D1 解锁）、登录、任务列表、产物列表通过；**GUI 触发下载被 D2 阻断**；下载/打开/摘要经请求层 seam 通过；临时文件受控删除 D3 失败。
- D1/D2/D3 均为集成代码在真实运行时的缺陷（单测 53/53 与 webpack 构建无法暴露），建议主控据此决定 T06 verified 与否并派发修复。

## 文件清单

- 新增：`docs/plans/issue-140/task-6-live-validation.md`（唯一交付文档，含观测表/缺陷/局限/复现提纲；不含凭据、token、signed URL、完整私有响应体）。
- 仓库外（均不提交）：`/tmp/t06-auto/*`（自动化脚本、截图、probe 日志）、gitignored `project.private.config.json` 与 `dist/` 本地补丁（报告已如实声明）。

## 清理确认

- 微信开发者工具：`cli quit` + 进程清理完成；隔离服务器（57801）已停止；`/tmp/weknora-t06-live.*`（DB/存储/密钥/日志）已删除。端口 57801 无监听。

## 自查与遗留

- 生产代码零改动（`git status` 除报告外干净；提交仅 1 个 docs 文件）。
- 未验证项如实列出：真机、B 用户 GUI 二次登录、openDocument 视觉渲染（截图空白，fixture 为手写最小 PDF）、GUI 层过期提示映射（被 D2 阻断）。
- 基线预存 13 个 `CommercialSummary` typecheck 错误与本次无关。
- 注：commit hook 提示 Mimosa 完整扫描结论缺失（scanner_enobufs），本任务未宣称项目安全。
