# W06 网页报告完整浏览器验收证据

首次完整浏览器验收（B 阶段交付证明）：真实浏览器（Playwright + Chromium）驱动真实全栈
（vite Web 应用、Go server、SQLite + 本地存储、锁定 OpenCode serve、nginx 独立 https 预览源），
完成创建→上传→主检索/理解→子生成→预览→修改→旧版下载（SHA 不变）→再次打开八项，
以及恶意 fixture、跨租户/非属主边界、切会话流隔离、刷新幂等安全断言。

## 1. 环境组合

| 组件 | 版本 / 身份 |
| --- | --- |
| 仓库 | .worktrees/craft-w06（分支 codex/craft-w06，BASE d9054bd0） |
| Go | go1.26.3 darwin/arm64 |
| Node / pnpm | v26.7.0 / 10.28.2 |
| Playwright | @playwright/test 1.63.0（pnpm-lock 锁定；浏览器 chromium-1243 本机缓存，无新增下载） |
| OpenCode serve | /Users/wuyongjun/.opencode/bin/opencode 1.18.4，sha256 `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`（与 docker/craft/runtime-config.json darwin-arm64-local-probe 一致；启动前 harness 复核） |
| 数据库 | SQLite（每次运行独立临时库，完整迁移链 000000–000045 自动应用） |
| 存储 | 本地文件存储（LOCAL_STORAGE_BASE_DIR 指向运行目录） |
| 预览源 | https://127.0.0.1:41877（deploy/craft/preview.conf 派生 + 运行时自签证书；明文端口 41878 返回 410） |
| Web | vite dev 127.0.0.1:41876（VITE_API_BASE_URL 指向 Go API） |
| Go API | 127.0.0.1:41875（WEKNORA_CRAFT_ENABLED=true、WEKNORA_CRAFT_KINDS=web、CRAFT_OPENCODE_BASE_URL 指向 serve） |

测试账号：每次运行随机生成口令，DB seed（bcrypt），经真实 POST /auth/login 换取 JWT 写入
storageState；凭据只存在于运行临时目录（`<run>/creds.txt`，0600），不入库不入提交。
角色矩阵：owner（tenant 1，属主）、viewer（tenant 1，非属主成员）、foreign（tenant 2，跨租户）。

主模型为受控 fixture（apps/web/e2e/craft-mocks/main-model.mjs，OpenAI 兼容：首轮
craft_delegate 工具调用、工具结果回传后终答）；子执行模型两套：
- **mock 模式**：sub-model.mjs（R07 craftMockProvider 的 Node 移植，确定性产出按月/季度/恶意页面）；
- **real 模式**：第二套 XDG 全隔离 serve，无 provider 配置、剔除凭据形环境变量 → opencode 内置
  免费模型（R07 方案，不加载、不消耗任何用户凭据）。

## 2. RED 证据（Step 1–2）

仅写入 spec/config/fixture 后、无栈运行：

```
$ cd apps/web && pnpm exec playwright test -c playwright.craft.config.ts e2e/craft-report.spec.ts
Error: page.goto: net::ERR_CONNECTION_REFUSED at http://localhost:5173/craft
1 failed / 5 did not run        # playwright 进程退出码 1
```

环境性失败形态（页面/依赖不存在），非断言或网络偶发；符合简报 RED 预期。

## 3. GREEN 命令与退出码

```
$ bash e2e/craft-stack.sh up mock     # 拉起全栈（build→mocks→serve→server→seed→login→nginx→vite）  exit 0
$ bash e2e/craft-stack.sh run mock     # Playwright 全量                                          exit 0
  6 passed (23.1s)   [1] 01 create/upload/generate/preview/modify/old-download  7.5s
                     [2] 02 reopen                                                    0.3s
                     [3] 03 malicious fixture blocked                                 4.7s
                     [4] 04 cross-tenant 404 / non-owner fail-closed                 0.3s
                     [5] 05 switching sessions isolation                               4.8s
                     [6] 06 reload idempotency (exactly 1 admission)                  5.0s
$ bash e2e/craft-stack.sh down mock   # 精确清理（记录 PID + nginx pid 文件）                     exit 0

$ bash e2e/craft-stack.sh up real     # 真实免费模型 serve                                          exit 0
$ bash e2e/craft-stack.sh run real --grep "01 create|02 reopen"                                   exit 0
  2 passed (1.4m)   # 真实模型两轮：月度生成 + 季度改写，各发布一个版本
$ bash e2e/craft-stack.sh run real --grep "04 cross"                                              exit 0
  1 passed (0.5s)
$ bash e2e/craft-stack.sh run real --grep "01 create"    # 独立复跑（真实模型第二轮样本）          exit 0
  1 passed (1.1m)
$ bash e2e/craft-stack.sh down real                                                                exit 0
```

运行产物目录（不入 git，保留至下次重启）：mock `/tmp/craft-w06-mock.4QPwFj`、
real `/tmp/craft-w06-real.fY9VZL`；截图在 `<run>/artifacts/`：
`01-preview-monthly.png`、`02-versions.png`、`03-malicious-blocked.png`、`04-viewer-invisible.png`。

回归命令（全部退出码 0）：

| 命令 | 结果 |
| --- | --- |
| go test ./internal/container/ ./internal/types/ ./internal/handler/session/ -count=1 | ok ×3 |
| go test ./internal/application/repository/ -count=1 | ok |
| pnpm run build:web / typecheck:web | 通过 |
| pnpm run test:shared | 110/110 pass |

## 4. B 阶段八项逐项结果（mock 模式，确定性断言）

| # | 验收项 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | 创建 | PASS | /craft 填 craft-goal → craft-create → POST /craft/sessions 201，工作台挂载 |
| 2 | 上传 | PASS | craft-upload 挂 craft-sales.csv → 真实附件入口上传（sha256 校验 cc19dfcd…）→ POST /craft/inputs 201（canonical resource:// ref）→ run 携带 input_refs |
| 3 | 主检索/理解 | PASS | run snapshot 查询含 `[已授权输入材料] inputs/<sha>/craft-sales.csv`；主模型 fixture 收到并委派 |
| 4 | 子生成 | PASS | craft_delegate → 容器真实执行链（工作区装配→输入落盘→R04 executor→锁定 serve）；craft_delegations 5/5 succeeded |
| 5 | 预览 | PASS | 独立 https 源真实 iframe：title=按月销售报告、总额 300、筛选东区→100、全部→300 |
| 6 | 修改 | PASS | 季度目标第二轮 → 新版本（不同 SHA），预览 title=季度销售汇总 |
| 7 | 旧版下载 | PASS | 首次下载 index.html sha256=59c6…（bytes 1324）；切回旧版本再下载 SHA 完全一致；新版本 SHA 不同（详表 §5） |
| 8 | 再次打开 | PASS | 离开再进入 /craft/:id：状态/两个版本/当前版本选择全部恢复 |

文案一致性：craft-main-status 终态文案 `已完成`（statusLabel('succeeded')）与主 Run 终态
（agent_runs.status=succeeded）逐字相同；断言在真实 SSE 流 + 快照刷新后成立。

真实模型轮（real 模式）：[1][2][4][5][6][7][8] 全部通过（1.4m + 1.1m 两轮）；真实模型读到落盘的
craft-sales.csv（生产装配的 workspace 输入落盘路径），产出真实销售报告页（title 销售数据季度分析报告，
含 300 总额），版本 checks 如实记录 entry=passed、build/preview=not_run。

## 5. 版本不可变性证据（mock 运行 /tmp/craft-w06-mock.4QPwFj 的 craft_version_files）

| 版本 | 会话/轮次 | 文件 | sha256 | bytes |
| --- | --- | --- | --- | --- |
| ver_f04d0cff… | 会话1 · 按月 | index.html | c669be213b98805f… | 1324 |
| ver_f04d0cff… | 会话1 · 按月 | report.md | f467b6119d2a7521… | 64 |
| ver_2463afd8… | 会话1 · 季度 | index.html | 119b56cffb160514… | 1324 |
| ver_2463afd8… | 会话1 · 季度 | report.md | 75d949cc7ca76fb1… | 66 |
| ver_38a979b0… | 会话1 · 恶意探测 | index.html | 0247f6bbfb68d4f3… | 1518 |
| ver_36d63072… | 会话B · 按月 | index.html / report.md | c669be21… / f467b611… | 1324 / 64 |
| ver_71ef7211… | 会话C · 按月 | index.html / report.md | c669be21… / f467b611… | 1324 / 64 |

测试内两次下载（首下载与切回旧版再下载）SHA 断言相等；新版下载 SHA 断言不同（mock 确定性内容）。
同内容跨会话同哈希（B/C 与会话1按月轮）证明内容寻址存储无重复对象。

## 6. 安全断言结果

| 断言 | 结果 | 说明 |
| --- | --- | --- |
| 恶意 fixture：父站 cookie 读/写 | BLOCKED | sandbox 无 allow-same-origin → SecurityError |
| 恶意 fixture：parent.document DOM/cookie | BLOCKED | 跨源 opaque origin → SecurityError |
| 恶意 fixture：外部 fetch（主站 origin + 外部 origin） | BLOCKED | 预览源 CSP connect-src 'none' → TypeError |
| 跨租户：GET 会话工作区 / 版本文件 | 404 | 租户不可见（fail-closed） |
| 非属主成员（viewer）：读 / POST runs | 404 / 404 | 会话按属主作用域读取，表面不可见即不可写；writeSession 的 403 分支需"可读非属主"（分享）路径，分享未接线，W03 服务级测试已覆盖该分支 |
| 切会话旧 stream 不追加 | PASS | 会话A生成中切到B再回A：A 对话区无 B 提示词；B 完成后对话区仅含 B 自身内容 |
| 刷新生成中页面 POST runs 恰 1 次 | PASS | 生成中 reload×2 后 agent_runs 该会话恰 1 行、craft_session_requests create 恰 1 行（DB 计数，非 UI 文案） |

## 7. 部署装配（协调器授权扩展）

- `internal/container/craft_runtime.go`（新增）：CRAFT_OPENCODE_BASE_URL 驱动的真实执行链
  （R01 Client→R04 Executor→CraftDelegateService）；未设置时保持 R05 fail-closed（默认关闭语义不变，
  有测试钉死）。含：工作区 OC 会话装配（CAS）、W03 workspace 输入清单落盘（内容寻址、逐字节 sha256 校验）、
  每委派独立输出目录 + serve 相对 output 指针软链（不改写任务——R04 幂等 PrepareTask 约束）、
  W01 收集器发布不可变版本 + artifact.published 事件、delegation.* 事件写入运行事件流（浏览器 SSE 投影）。
- `internal/container/craft_runtime_test.go`（新增）：fail-closed 默认、指针作用域/防覆盖、
  产物源跟随指针/穿越拒绝、事件 payload 契约，4 组测试。
- `internal/container/container.go`：executor provider 替换为 newCraftRuntimeExecutor；registerCraftHTTPHandlers
  的 Invoke 移至全部依赖注册之后（原位置从未真实启动过服务器——dig 顺序缺陷，W06 首启发现）。
- docker/craft/* 与 deploy/craft/*：**零改动**——预览源直接由 deploy/craft/preview.conf 派生（运行时 sed
  替换端口/证书/上游），无需新增部署文件。

### 集成修复（超出简报 Files 清单，逐条报备）

浏览器验收首启暴露的四个跨任务集成缺口，均为最小修复：

1. `internal/types/model.go`：ModelParameters.Scan 只接受 []byte，SQLite 驱动返回 string →
   模型参数整体静默丢失（base_url 为空打到 api.openai.com）。修复：同时接受两种类型。
2. `internal/handler/session/agent_run.go`：SSE 流在 Run 终态时直接关闭，从不发送终态 run 帧 →
   工作台永远收不到"已完成"。修复：关闭前发送终态 run 投影帧（一行 + 注释）。
3. `apps/web/src/features/craft/routes.tsx`：上传关联后以附件 id 而非关联响应的 canonical
   resource:// ref 提交 run → 后端 400（input ref not associated）。修复：使用 addInput 返回的 ref。
4. `internal/container/craft_runtime.go` 内的执行器消息 id 适配（adoptExecutorMessageID）：R02 存储的
   PrepareTask 默认填充 UUID，而 R04 执行器校验 msg_ 环窗格式 → 首次委派即 unknown。适配在委派派发前
   一次性改写为执行器格式（重试不换 id）。该缺口本质属 R02↔R04，报告留档供上游回归。

## 8. 边界与限制（如实）

- 预览 verdict（W02 RecordPreviewVerdict）无 HTTP 入口，浏览器验收无法回写 preview check → 版本 checks
  如实显示 preview=not_run（build 亦 not_run：无构建步骤）。
- 交互 decide 后端路由仍未注册（W04 报告 §6 既知）：组件只记录决定并告警，绝不静默批准。
- 本地单 serve 的输出指针为串行委派设计；并发委派共享指针有竞态（容器化部署按会话独立沙箱，
  无此问题）——已在 craft_runtime.go 注释与本文件留档。
- real 模式仅运行 01/02/04（真实模型生成/预览/下载路径）；03/05/06 为 UI 语义断言，由 mock 全量覆盖
  （真实模型轮次时长 1.4m/1.1m，全部通过留档）。
- 单元测试凭据不入库不入提交；运行目录（含 SQLite、存储、日志、截图）在 /tmp，重启即失。
