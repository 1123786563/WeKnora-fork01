# Wave 1 — T06 实测补齐：微信开发者工具真实环境验证（validation）

你是 frontend_validator。本简报自包含：只读本简报 + 文中引用的文件，不依赖任何父会话。**禁止派发子 agent。** 你只验证、只记录，**不改任何生产代码**；发现的缺陷写进报告，由主控另行派发修复。

## 1. 背景与目标

T06/#148（微信小程序 Task 产物下载）代码已全部集成到集成分支：实现与三轮修复以 `4b9b550c2`,`a1b33eec0`,`0c68e2754`,`7d492a9d3` 合入；在当时的集成 HEAD，`pnpm --filter @weknora/miniprogram test` 53/53 PASS、`build:weapp` PASS（台账记载）。**唯一缺口是真实环境验收**——主计划 Task 5 第 3 步："用真实微信开发者工具/设备下载一个受保护 Task Artifact，比对内容摘要和权限拒绝。缺真机时该验收单列 blocked。"

你的目标：在真实微信开发者工具（本机 GUI/模拟器，非单测 mock）中完成并取证：
1. 小程序构建产物在开发者工具中加载运行；
2. 对一个受保护 Task Artifact 执行 列表 → 下载 → 本地打开（PDF/DOCX 或 fixture 使用的 MIME）；
3. SHA-256 三方比对：小程序实际下载的字节 vs 服务器本地存储文件 vs API 返回的 `digest`；
4. 权限拒绝：另一 Tenant 用户访问同一 run 的 signed-url 得到非 200（404/401），以及过期 grant（如 `ttl_seconds=1` 后重试）被拒；
5. （可观察即取证）下载临时文件的受控删除行为。

## 2. Worktree 与基线

```bash
cd /Users/wuyongjun/.codex/worktrees/issue-140-t06-live-validation/WeKnora-fork01
git status --short        # 必须 clean（调度员 2026-09-25 实测为空，停在 6d67b51ae）
git checkout --detach 21df162a24d73d9ec68d012458577050c18fd59f
```
- 该 worktree 是此前被 429 中断的实测 agent 现场，停在旧基线 `6d67b51ae`，无未提交价值（实测 status 为空），直接 checkout 到当前集成 HEAD `21df162a2` 后开展。全程本地 commit 允许（仅证据/报告文件），绝不 push、绝不改集成分支。
- 严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`（尤其 `.worktrees/issue30-sweep`）。

## 3. 环境实勘（调度员 2026-09-25 已核实）

- 微信开发者工具 CLI：`/Applications/wechatwebdevtools.app/Contents/MacOS/cli`（存在可用）。常用：`cli open --project <dir>`、`cli auto --project <dir> --auto-port <port>`（自动化，需工具开启服务端口；若 `cli auto` 不可用则用 `cli open` + 手动/其他可复核手段，并在报告中如实写明采用了哪种方式）。
- 小程序工程：`apps/miniprogram`（Taro 4.2.1）。构建：`pnpm --filter @weknora/miniprogram build:weapp`，产物在 `apps/miniprogram/dist`。
- API base：`apps/miniprogram/config/index.ts` 明确"本地回环的 http（localhost/127.0.0.1 带端口）仅用于开发"——把小程序指向本地服务器是受支持的路径，先读该文件确认配置方式（编译 env / project.config.json）。
- 已知基线事实：`pnpm --filter @weknora/miniprogram typecheck` 有 13 个 `CommercialSummary`（account/pages.tsx）预存错误，基线即红，与你无关，不要修。

## 4. 本地受保护 Artifact 服务器（fixture 模式，沿用 T04 先例）

先完整阅读两份先例（在集成 worktree 同路径下）：
- `docs/plans/issue-140/task-4-live-http-validation.md` —— 受保护 artifact fixture 的完整复现步骤（sessions/agent_runs/artifact_versions 行、`local://tenant/fixture.txt` object key、digest/size 与实际字节一致）。
- `docs/plans/issue-140/task-3-live-http-validation.md` —— Lite 服务器启动方式。

启动模式（要点，细节以两份文档为准）：
```bash
# 独立临时目录 + 隔离 SQLite；本轮端口分配 57801（T03 用过 62628、T04 用过 54512，57801 无冲突）
DB_DRIVER=sqlite DB_PATH=<tmp>/wk-t06.db RETRIEVE_DRIVER=sqlite STREAM_MANAGER_TYPE=memory \
STORAGE_TYPE=local LOCAL_STORAGE_BASE_DIR=<tmp>/storage SERVER_HOST=127.0.0.1 SERVER_PORT=57801 \
DISABLE_REGISTRATION=false APP_EXTERNAL_URL=http://127.0.0.1:57801 \
（按 T03/T04 文档生成 JWT/AES/WEKNORA_ARTIFACT_SIGNING_KEY）go run ./cmd/server
```
注册两个用户（各含独立 Tenant），用第一个用户的 bearer + `X-Tenant-ID` 走 signed-url API；DB 内按 T04 fixture 方式插入 owner 的 `sessions`/`agent_runs`/`ready artifact_versions` 行（PDF/DOCX 字节或固定 fixture 字节，记录精确 SHA-256）。**不共享任何真实账号/服务/模型；结束后停止进程并删除临时目录。**

小程序侧登录态：小程序有自己的认证流（OIDC/登录页）。若小程序内完整登录流不可行，允许的最小替代是用开发者工具自动化在请求层注入该用户有效凭据（与 T04 "fixture bypass Task generation、真实走 router/中间件/handler"同等口径），但必须在报告中如实声明注入层级；若连这一层都无法可靠建立，如实 blocked。

## 5. 主计划 Task 5 验收原文（判定基准，verbatim）

- 增加产物列表→下载→本地打开的合同测试，覆盖无权限、过期链接、网络未知、空间切换。（已由 53/53 单测覆盖，你不需要重写）
- 接线 Taro 下载与文件打开适配器，按用户动作取授权，不长期缓存 URL；下载前后检查 scope epoch，受控删除临时文件。（你验证其在真实环境的行为）
- **用真实微信开发者工具/设备下载一个受保护 Task Artifact，比对内容摘要和权限拒绝。缺真机时该验收单列 blocked。**
- 提交独立分支，reviewer 检查错误提示、权限、临时文件。（评审另派，你只产出证据）

## 6. 全局约束（verbatim 摘录）

- 不自动投递、发送邮件、跨站填表、绕过 CAPTCHA/登录或读取邮箱推断进展。
- 所有实现者只修改任务所有权内的文件；你是 validator：**生产代码零改动**，唯一可写文件是证据/报告文档。
- 严禁 push、merge、deploy、GitHub 操作；禁止派发子 agent。
- 报告只写事实与实测输出；**环境不满足（例如工具服务端口无法开启、GUI 无法自动化、登录流无法建立）时如实写 blocked 并附命令与输出证据，绝不伪造通过**——T06 是否标 verified 由主控依据你的证据裁决，不由你宣布。
- 测试数据库、端口（本轮 57801）、构建目录与并行任务隔离，不得占用其他端口。

## 7. 产出

- 报告写入本 worktree：`docs/plans/issue-140/task-6-live-validation.md`，结构参照 `task-4-live-http-validation.md`（观测表 + 复现提纲 + 明确的未验证项/局限声明；不保留任何凭据、token、signed URL 或完整私有响应体）。
- 本地提交：`git add docs/plans/issue-140/task-6-live-validation.md && git commit -m 'docs(miniprogram): record T06 live wechat validation'`。
- 最终消息报告：HEAD SHA、每项证据的实测结果（PASS/blocked + 关键输出摘要）、服务器端口与清理确认。
