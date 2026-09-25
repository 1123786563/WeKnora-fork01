# Wave 3 — T02 补缺 wave 报告（F3 迟到响应运行时证据 + 截图19 定性更正）

- 任务：wave-3-t02-f3-fix-brief（Issue #145，fix-resume 轮）
- 执行者：frontend_validator（#140）
- 现场：`/Users/wuyongjun/.codex/worktrees/issue-140-t02-live-validation/WeKnora-fork01`，086d93ed1（clean，含已入库 checkpoint）→ 换基 `d88cd513a`（detach）→ 本轮 HEAD **`3e335ee3c`**（本地提交，未 push）
- 交付物：`docs/plans/issue-140/task-2-live-ios-validation.md`（4 处更正 + 新增 §10）+ 证据 3 件（21/22 截图 + http-probes-r3-late-response.txt）
- **生产代码零改动**（git diff 仅 docs；构建产物 apps/mobile/build 取证后已删除，worktree clean）

## 1. 两项处置结果

### 1.1 截图 19 定性更正 — 完成

- **更正依据（本轮实测）**：PIL 像素统计——截图 19 主色 `#ffffff 99.5%`、8 带亮度 250.9–255.0（全白屏）；对照登录门 16 = `#f2f2f2 94.0%`/顶带 231.9。R1 评审「实为全白屏」结论属实，报告原「登录门 + 空表单」定性错误。
- **更正位置（4 处，均只改截图 19 相关短语，未重写报告）**：§4 表 #5、§5 D-iOS-1、§6 R2 证据清单、§9 F4 行。
- **连带证据修复**：「凭据未持久化」原以 19 为旁证；本轮以新证据替换——截图 22（冷重启 terminate→launch→22s 稳定后 `#f2f2f2 94.0%` + AX 8 元素含 "Sign in to WeKnora" 双源确认登录门）+ 冷重启窗口服务器零请求。结论不变，证据基础变为有效。
- **成因复核**：本轮 launch 后 6s 仍 99.5% 白屏、~18s 后登录门才渲染——19 为「launch 后立即截图、UI 未及渲染」产物。

### 1.2 F3 迟到响应运行时证据 — 诚实结论：GUI 层不可构造 + 三源替代证据（已入库）

**结论（报告 §10.2）：「切换空间后迟到响应不可见」在真实 app 的 GUI 层在本环境不可构造；不可构造本身有本轮运行时证据，语义覆盖由单测 + 静态 seam + HTTP 时序旁证替代。**

**(a) 运行时复现（本轮全新环境，非引用旧轮）**：
- 拓扑：Lite 服务器 57807（全新 SQLite + fixture：a2@t2.io 属主 Tenant 1 + tenant_members(T3 contributor) + t02run0001 succeeded）+ 测试 TLS 代理 57808（自签 CA 仅注入模拟器）+ 重建 Release app（origin=https://127.0.0.1:57808，bundle 内联计数 1，GUI origin 预填实见）。
- GUI 登录（idb 键盘注入 a2@t2.io + 18 字符密码，18 圆点完整）：07:22:07 服务器 `POST /api/v1/auth/login` **200**（active_tenant=1、memberships 含 T3）。
- 此后 server.log 全量 grep：`/auth/me`=0、`/system/capabilities`=0、workbench app 请求=0（login 后唯一请求即 login 本身）；UI 终态截图 21 与 R2 基准 20 逐带一致（181/194/194/209.8/212.6/197.5/194/194 vs 180.3/194/194.1/209/212.4/197.5/194/194）→ Update-required 面。
- **⇒ 授权后 workbench UI 不可达（D-iOS-1 本轮完整复现）→ app 内无 Task 读取可发起、无空间切换入口 → 慢响应晚于空间切换的 GUI 时序无构造起点。**

**(b) 单测（本轮实跑）**：`npx tsx --test 'packages/mobile-core/src/**/*.test.ts'` → **149/149 pass**；`mobile-runtime.test.ts` 40/40，F3 语义 4 用例：ok 33 `a late authorized response after a scope change is dropped`（RUNTIME_SCOPE_CHANGED）、ok 34 `a late tenant verification cannot override a completed later switch`、ok 37 `a stream opened before a scope change is dropped…`、ok 12 `late responses after sign out are ignored`。

**(c) 静态 seam**：`packages/mobile-core/src/runtime/mobile-runtime.ts` epoch 守卫（scope 变化递增 epoch :137/:143/:498；`current()` :148；状态发布重检 :290）+ `scope-lease.ts` 可撤销 lease。

**(d) HTTP 层时序旁证（脱敏入库 http-probes-r3-late-response.txt，观察层级=传输层）**：07:24:28 属主读取挂起（测试代理人为延迟 6000ms，harness 层制造时序、未改生产代码）→ 07:24:30 挂起期间 switch-tenant→3 成功、新作用域列表 `[]`/snapshot 404 → 07:24:34 迟到的 Tenant 1 响应到达（携带 t02run0001/succeeded，晚于切换 ~4s）。证明迟到响应会晚于空间切换到达传输层；app 内不可见性由 (b)(c) 覆盖，不冒充 GUI 证据。

## 2. 验证命令与真实输出摘要（RED/GREEN 口径说明）

本轮为**补证据 + 更正文档任务，无生产代码/测试代码变更，TDD RED→GREEN 不适用**（简报第 2 节明确「不改任何生产代码」）；验证命令照跑如下：

| 命令 | 输出 |
| --- | --- |
| `npx tsx --test 'packages/mobile-core/src/**/*.test.ts'` | `# tests 149 / # pass 149 / # fail 0` |
| `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` | `# tests 40 / # pass 40 / # fail 0`（含 ok 12/33/34/37 四个 F3 用例） |
| `pnpm --filter @weknora/mobile test` | `# pass 77 / # fail 0`（17.9s） |
| `pnpm --filter @weknora/mobile typecheck` | exit 0（tsc --noEmit 0 error） |
| `xcodebuild … -configuration Release … build`（EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57808） | `** BUILD SUCCEEDED **`（exit 0）；产物 bundle `grep -ac '127.0.0.1:57808' main.jsbundle` = 1、57803 = 0 |
| GUI 登录（idb tap/text） | server.log `method=POST path=/api/v1/auth/login … status_code=200`（07:22:07，email a2@t2.io） |
| 登录后请求统计 | `grep -c auth/me`=0；`grep -c system/capabilities`=0；login 后 gin 请求仅 login 1 条 |

## 3. 提交与文件清单

- 提交：`3e335ee3c docs(mobile): close T02 round-2 evidence gaps`（基 d88cd513a，本地，未 push）
- 文件：`docs/plans/issue-140/task-2-live-ios-validation.md`（+35/−4）；新增 `task-2-evidence-ios/21-r3-post-login-update-required.png`、`22-r3-cold-relaunch-stable-login-gate.png`、`http-probes-r3-late-response.txt`

## 4. 清理与自查

- 清理：server 57807 / TLS 57808 / idb_companion 停止（端口复核 down）；模拟器内 app 卸载；/tmp/wk-t02-r3（DB/密钥/证书/harness）删除；凭据不入库（transcript 脱敏）；worktree git status clean。
- 自查：①生产代码零改动 ✓（diff 仅 docs）；②只改简报声明所有权（报告 + 证据目录）✓；③未 push ✓；④端口隔离 57807/57808 ✓；⑤未派发子 agent ✓；⑥主仓库与 .worktrees/issue30-sweep 未触碰 ✓。
- 遗留：D-iOS-1（授权链 fail-closed）未解，GUI 层迟到响应证据维持不可构造，修复依赖可用签名环境复跑（报告 D-iOS-1 修复派发建议不变）；T02 终态由主控裁决。

## 5. 评审包

`docs/plans/issue-140/reviews/` 由 review-package 脚本生成（见提交返回路径）；若脚本失败以 git log/diff 兜底文件为准。
