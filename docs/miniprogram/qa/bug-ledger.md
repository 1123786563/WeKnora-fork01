# 缺陷台账（2026-09-18，分支 fix/miniprogram-qa-regression）

分级：P0 跨租户/凭据泄漏/未授权副作用；P1 安装/构建/核心流程不可用；P2 视觉/交互/可维护性。
来源：inherited 原有问题；integration 与 main 整合后；blocked-env 环境阻断；not-implemented 未实现。

## D1｜P2｜inherited｜测试文件吞掉模块导入错误

- 复现：三个测试文件（`tests/auth.test.mjs`、`tests/core.test.mjs`、`tests/transport.test.mjs`）
  使用 `try{import}catch{}` / `import(...).catch(()=>({}))`；模块求值失败时真实原因被吞，
  只剩"xxx must exist"，误导定位。
- 根因：测试编写模式。
- 修复：全部改为直接 `await import(...)`，任何求值错误使整个测试文件带原始栈失败。
- 回归：`pnpm --filter @weknora/miniprogram run test`（如再发生导入错误将直接红）。

## D2｜P2｜inherited｜CI 缺少小程序门禁

- 复现：`.github/workflows/frontend.yml` 的 react-workspace job 只覆盖
  shared/Web/Embed/Desktop；miniprogram 的 tokens/test/typecheck/build 从未在 CI 执行
  （历史构建坑 6683c149 一类只能靠人肉发现）。
- 根因：工作区接入 CI 时未同步。
- 修复：新增独立 `miniprogram` job：frozen 安装 → tokens:check → test → typecheck →
  build:weapp（占位 origin，明确标注不算真实联调）→ mini-regression。
- 回归：分支推送后 GitHub Actions `Frontend / WeChat miniprogram gates` 运行结果。

## D4｜P1｜inherited｜热路径 refresh 被明确拒绝后残留"僵尸会话"

- 复现（修复前证据 ✖）：`tests/auth.test.mjs`
  "definitive refresh rejection on the hot path clears the zombie session"。
  refresh 端点返回 401（refresh token 失效）后会话仍 `ready`、凭据仍在，
  之后每次操作都 401→refresh→401 无限循环，直到重启 App（冷启动 bootstrap 才会 clear）。
- 根因：`AuthCoordinator.refresh` 对 api.refresh 的失败不做凭证失效区分。
- 修复：`src/core/auth.ts` refresh 内捕获；仅当错误 `status===401`（明确凭证判定）
  时 `clear()`（清除存储/凭据/scope 并回 anonymous）；网络类失败保持会话可重试。
- 回归：上述测试 + "transient refresh failure keeps the session"（防误杀在线用户）。

## D5｜P1｜inherited｜服务端明确 unknown 后任务提交永久卡死

- 复现（修复前证据 ✖）：`tests/assembly.test.mjs`
  "startTask unknown after a lost response resubmits the SAME request id"。
  POST start 响应丢失 → intent 记为 unknown；再次点击触发 lookup，服务端明确回答
  unknown（`admission.go LookupRequest` 对 `ErrRecordNotFound` 的分支：无持久记录），
  但客户端仍抛"前一次任务仍待确认"，且 `PendingIntent.reset()` 拒绝非 rejected 状态
  → 该 scope 下永远无法再发起新任务。
- 根因：`workbench.startTask` 把"服务端明确无记录"与"结果仍在途"混为同一处理。
- 修复：`src/services/workbench.ts` startTask 中 lookup 返回 `unknown` 时不再抛错，
  继续**用同一 request_id** 重提交（`AdmissionCoordinator.Start` 对 request_id 幂等：
  记录已存在则按 request_hash 校验后返回原 run，冲突则 ErrConflict）；
  pending/dispatching 仍拒绝重复提交。
- 回归：上述装配测试 + "duplicate taps while a submission is unresolved never create
  a second intent"（防回归为重复 POST）。

## D6｜P2｜inherited｜core-js 目录导入阻断 Node 直测（装配测试前置修复）

- 复现：`node --experimental-strip-types` 下 `import('./src/services/runtime.ts')` 报
  `Directory import 'core-js/actual/url' is not supported resolving ES modules`。
  webpack 能解析目录，故 weapp 构建不报错——问题仅在被测试直接 import 源码时暴露。
- 根因：`src/platform/polyfills.ts` 使用目录形式导入。
- 修复：改为 `core-js/actual/url/index.js` 全路径（webpack 与 Node ESM 均可解析；
  修复后 weapp 构建复验通过，9.04s）。
- 回归：`tests/assembly.test.mjs` 整个文件即为其回归（导入失败会全红）。

## D7｜P2｜inherited｜auth 存储 key 未做 host 大小写归一化，同源变体间会话孤立且凭证残留

- 复现（修复前证据 ✖）：微信开发者工具 storage 实测存在
  `wk:auth:https://WeKnora-app.orb.local`（大写）凭证；改用小写 origin
  `https://weknora-app.orb.local` 构建后（URL host 大小写不敏感，同一后端），
  home 回到 anonymous 只剩"登录"按钮；`clear()/logout()` 只清当前 key，
  其他大小写变体的 bearer token 永久残留本机（凭证卫生）。
- 根因：`core/auth.ts` 构造器 `wk:auth:${origin}` 直接字符串拼接，host 未归一化。
- 修复：`normalizeApiOrigin()`（host 小写）+ 构造器归一化 + `migrateCaseVariants()`
  （同 host 变体的有效凭证迁移到规范 key、变体 key 一律删除，不同 host 不动）；
  `runtime.ts` 注入 origin 统一归一化（baseURL/信任校验/key 一致）；
  `ValueStore` 增加可选 `keys()`，Taro storage 用 `getStorageInfoSync` 实现。
- 回归：`tests/auth.test.mjs` 3 项（变体迁移恢复会话、logout 不动其他容器凭证、
  垃圾变体只删不迁移），修复前 3/3 红（41 tests / 38 pass），修复后 41/41 绿。
- 真实环境复验：切换 Up 栈 origin 构建后，storage 大写 `Up-WeKnora-app` key 自动迁移
  为 `up-weknora-app` 规范 key；旧 dev 栈 key 因不同 host 正确保留。

## D3｜P2｜inherited｜README 引用的验证文档不存在

- 复现：`apps/miniprogram/README.md` 链接 `docs/miniprogram/submission-v0.1.md` 404。
- 修复：补建 `docs/miniprogram/submission-v0.1.md`（验证层级声明 + not-implemented 边界 +
  qa/ 目录入口），qa/ 四文档随本分支建立。

## not-implemented 边界（非缺陷，记录防漂移）

- `GET /api/v1/workbench/executions`（任务列表读模型）无 Go 路由；装配契约测试
  "wire paths used by the miniprogram match the Go route table" 把实际发线的路径钉在
  既有 Go 路由集合内，列表端点出现时测试会提醒同步 UI 降级文案。
- 审批：仅支持安全拒绝（缺动作详情时批准按钮禁用）；支付渠道关闭；产物下载不猜链接。
