# T33/#172 五环境真实闭环与发布门槛（终局验收）任务报告

- 轮次：2026-09-26（Asia/Shanghai）。角色：frontend_validator + backend_validator 合一（终局验收员）。
- 现场：独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01`，BASE **`d6b3e1bb5`**（detach；功能票全部集成后的集成 HEAD）。**生产代码零改动**（diff 仅 verification/ 两文档 + 本目录证据）。
- 隔离：Lite 服务器 127.0.0.1:57828（SQLite 一次性 DB + 一次性 JWT/AES/导出签名密钥）；Vite 57829→57828；iOS TLS 代理 57830→57828；微信 DevTools auto-port 9433。
- 说明：本任务为 validation/closure，**无生产代码/测试代码变更，TDD RED→GREEN 不适用**（简报第 4 节工作清单为验证与文档）；验证命令照跑并记录真实输出如下。

## 1. 本轮各环境新跑链路（证据：evidence/ 相对本 worktree `.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/`）

### 1.1 Web（真实浏览器，完整七步链 + 失败态 + 越权）——全通

- 拓扑：`go run ./cmd/server`（DB_DRIVER=sqlite/一次性密钥/STORAGE=local 临时目录）@57828；`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57828 pnpm exec vite --port 57829`；Playwright Chromium。
- 链路（全部真实回执）：注册 t33a@t33.io(201)→登录(200)→档案（提案毕业时间=2026-06→确认→**修订 2/确认 1 条**）→岗位（粘贴 JD→opportunity `3e57ca16…`/snapshot `f0a759b5…`）→评估（`34d83e35…` 三值「待确认」+职位/档案证据指针）→申请（`de3fa49d…`+Task `ef1125b7…`/Run `349d0617…`）→材料（`beb6dd4f…` V1 不可变+导出 `9a9eb655…` PDF/DOCX 双核验 submittable）→**PDF 真实落盘 2642B，SHA-256 `a65d6bd4…` 与页面摘要一致**→投递（`06817966…` 渠道招聘网站绑定 V1）→进展（事件 `e32f4d90…` 已投递→阶段确定性投影「已投递」事件修订 1）。
- 失败态：①一次性找岗「上海 前端开发 实习」→诚实失败（searchId `936c6f6e…`「暂无已核验来源，本次未抓取任何数据」+同编号恢复入口）；②未配签名密钥下载→诚实失败 `career export signing key not configured`，补配重启（同 DB/JWT）后成功。
- 越权（HTTP）：t33b@t33.io 读 A 岗位快照 404 not_found / 申请 403 forbidden / 材料 403 / 进展 404；B 自身 open 200 空档案；匿名 401。→ evidence/web-cross-tenant-probes.txt、web-network-requests.log、web-chain-final.png、career-material-v1.pdf。

### 1.2 iOS（T02 模式抽查：构建/启动/GUI 登录 + HTTP 边界链）——构建与登录链路通；D-iOS-1 复现（如实）

- Release 构建（prebuild+pod install+xcodebuild，origin=https://127.0.0.1:57830）→ **BUILD SUCCEEDED**，main.jsbundle origin 内联 grep=1；ad-hoc 重签+安装+启动→自身登录门（origin 预填，AX 8 元素）。
- GUI 登录（idb 逐字符注入 a33@t.io）：**服务端 auth_tokens 表 22:52:53/22:54:17 两次真实签发 ⇒ login 经 TLS 200**；UI 终态 Update required 面——**D-iOS-1 在集成 HEAD 完整复现**（登录成功后授权链路异常回退，后续请求不发；与 T02/T02补缺一致）。
- HTTP 边界：属主读 career-linked 受保护 Task（列表+快照 200）；跨租户 B→404；匿名→401。→ evidence/ios/（ios-spotcheck.md+2 截图）、ios-http-probes.txt。
- Career 链 App 内不可达（如实边界）：D-iOS-1 + Expo 端 Career UI 六票（T05/T23/T25/T27/T29/T31）随 T01 blocked 未实现。

### 1.3 微信小程序（DevTools）——4/4 PASS

- `WEKNORA_API_ORIGIN=http://127.0.0.1:57828 pnpm --filter @weknora/miniprogram build:weapp` EXIT=0（origin 内联 dist/common.js）；CLI touristappid + `cli auto --auto-port 9433` + miniprogram-automator（仓库外）。
- 抽查：真实 /auth/login（同 Web 账号）→home；/career/discovery **Web 创建的已确认事实「毕业时间：2026-06 · 修订 2」在微信端可见（同源跨端）**；一次性找岗入口+对账/空间切换失效说明在页；/career/application-material 修订 2/已确认 1 条/岗位与资格评估/不可变版本说明呈现。截图 4 张 + 驱动真实输出 → evidence/wx-devtools-spotcheck.md、wx-0*.png、wx-driver.cjs（口令已净化）。
- 边界：真机 blocked；t-button GUI 触达限制（T24 定论）由 172 单测覆盖。

### 1.4 鸿蒙原生 / 1.5 Expo Android——BLOCKED（实勘复证）

- 本轮实测（evidence/android-harmony-live-probe.txt，21:35）：hdc/hvigor/ohpm 与 DevEco-Studio 全缺失（T01 依据维持）；adb 1.0.41/emulator/AVD(test36,test36-small) 存在但 **`adb devices` 0 设备**、本工程无 .apk（修正简报「无 adb/emulator」表述：工具在、设备无）。
- 按主计划约束两格 **blocked，不计通过、不宣称完成**；Android 构建/启动未尝试（简报范围外，交主控裁决）。

## 2. 完整门（同 HEAD 实跑，gate-logs/）

| 门 | 结果 |
| --- | --- |
| `go test ./internal/modules/career/... -count=1`（主计划命令①） | **ok 109.788s** exit=0 |
| `pnpm typecheck:web`（②） | **exit=0** |
| `pnpm --filter @weknora/mobile typecheck`（③） | **exit=0** |
| `pnpm --filter @weknora/miniprogram typecheck`（④） | **exit 2：13 errors，全部 features/account/pages.tsx CommercialSummary 既有基线（该文件外 0）**——与 T30/T32 记录一致，零新增（evidence/mp-typecheck.log） |
| Go 6 包门（补 workbench/database/router/architectureguard/handler） | 全部 ok（230.8s/128.0s/5.9s/2.2s/handler 4.5+2.1+101.8s）exit=0 |
| `pnpm --filter @weknora/miniprogram test`（172 门） | **tests 172 / pass 167 / fail 0 / skipped 5**（5 skipped=build-output 条件跳过；本轮 build:weapp 实跑通过） |
| Web 聚焦门 `node --import tsx --test --test-concurrency=4 'src/career/*.test.tsx' 'src/career/*.test.ts'` | **174/174 pass 0 fail** |
| 全量 `pnpm test:web`（Wave 13/14 欠账闭环） | **未通过（争用下失败，待空闲补跑）**：21:39:47 启动（load≈68/10 核）；2h09 无进展（0% CPU/无子进程）终止时缓冲尾吐出真实断言失败 `deepStrictEqual ['Bob',undefined,undefined] vs ['Bob',undefined]`、`Exit status 1`（测试名因 tail 截断丢失）；含 Bob 的四个测试文件孤立复跑（含并发 4）75/75 全过不可复现→争用/顺序相关；取证 evidence/testweb-stuck-forensics.txt + testweb-full-truncated-output.log；聚焦门 174/174 为本轮 Web 测试证据 |

## 3. 终态裁决材料（供主控裁，不代裁；详表见 launch-matrix.md §3）

- T01 鸿蒙闸口：blocked 维持（本轮复勘工具链仍缺）。
- T01 传导链 T05/T23/T25/T27/T29/T31（Expo 移动 Career UI 六票）：随 T01 blocked，未派发实现（无 wave 报告/提交）——建议主控在「等待鸿蒙 spike」与「降范围改 Android/iOS-only 另立票」之间裁。
- T02：iOS verified-with-gate（HTTP 边界全过+构建/登录链复证；D-iOS-1 单列）；Android 侧建议裁「永久缺项/降范围」（本机长期 0 设备）。
- T28-F3（真机/t-button GUI）：parked。T30 残项（真机订阅弹层/模板未配置/3 medium）：parked。
- 各票遗留 low（T21 F2 注释措辞、T24 t-button、T26 三 seam GUI 层、T32 基线 13 type errors 等）：parked，不阻塞。
- 生产空 allowlist：发布门槛硬缺口（source-coverage.md §1/§5）。

## 4. 发布门槛结论（如实）

**公开门槛未达成**：①来源覆盖=空（生产 allowlist 设计性为空，无授权核验记录）且运营核验五项（来源/模型/微信能力/个人信息/收费）逐项未完成（需人工运营介入）；②鸿蒙原生与 Expo Android blocked；③iOS App 内授权后链路存在 D-iOS-1。可发布面：Web 与微信（DevTools）端到端抽查链全通、完整门全绿（小程序 typecheck 13 项既有基线除外，零新增）。

## 5. 全量 test:web 补记（终值）

**未通过（争用下失败，待空闲补跑）**——启动 21:39:47 时 load≈68/10 核（evidence/cpu-contention-before-testweb.txt）；运行 2h09 后 runner 无进展（0% CPU、无子测试进程、无 TCP；evidence/testweb-stuck-forensics.txt），kill 时缓冲尾吐出真实失败：`ERR_ASSERTION deepStrictEqual actual ['Bob', undefined, undefined] vs expected ['Bob', undefined]`，`ERR_PNPM_RECURSIVE_RUN_FIRST_FAIL … Exit status 1`（完整截断输出：evidence/testweb-full-truncated-output.log）。失败测试名因采集管道 `tail -12` 截断丢失；含 Bob 的四个候选文件（organizations/tenant-members/experts/administration）孤立复跑（含 --test-concurrency=4）**75/75 全过，不可复现**。诚实分类：**争用/顺序相关失败 + runner 挂起**（与 T03 报告所述「full Web test script runner-stability correction」历史一致）。本轮以聚焦门 career 174/174 作为 Web 测试证据；全量门按简报 fallback 如实标注「待空闲补跑」，不以部分结果冒充全量通过。

## 6. 清理

- DevTools `cli quit`、idb_companion 停止、TLS 代理与 Lite 服务器停止、/tmp/weknora-t33-closure 与 /tmp/wk-t33-automator 删除、模拟器内 app 卸载、模拟器钥匙串自签 CA 移除（尽力）、端口复核。凭据全部 disposable 且已净化（wx-driver.cjs 口令 REDACTED）。

## 7. OCR 状态记录（不自行宣称）

简报第 5 节：OCR 覆盖为主控侧流程（集成分支已有 OCR WIP 在跑）。本轮 validator 未运行 OCR，其状态以主控 Step 4 流程为准。

## 5. 修复轮 r2（2026-09-27，fix-resume 1/5，HEAD 92718d338 之上）——两项 medium 闭环

本节为 Wave-15 评审 2 medium 的修复记录；生产代码零改动（diff 仅 launch-matrix.md + 本目录证据/报告）。TDD RED→GREEN 不适用（纯验证文档修复，无生产/测试代码变更）；验证命令照跑见 §5.3。

### 5.1 M1：§0 六个回执 ID 无入库证据 → 等价链路重跑全文落档

- **缺口**：launch-matrix §0 引用的六个截断标识（提案回执 `d2e5d8bb…`、确认回执 `441a6c02…`、机会导入 requestId `c666ff58…`、导出正文摘要 `8a333f7f…`、PDF SHA-256 `a65d6bd4…`、Task(session) `ef1125b7…`）当轮响应原文未落档；当轮一次性 SQLite DB+一次性密钥已销毁，**原始 ID 响应体不可复现（如实标注，不虚报）**。
- **补证**：同 worktree（HEAD `92718d338`）重开隔离 Lite 服务器（`go run ./cmd/server` @127.0.0.1:57828；DB_DRIVER=sqlite 一次性 DB `/tmp/wk-t33fix-20260927-004238/weknora.db`；STORAGE_TYPE=local 一次性目录；一次性 JWT/AES/`WEKNORA_CAREER_EXPORT_SIGNING_KEY`（≥32 字节 hex，符合 `rendering.go:66-82` 约束）；GIN_MODE=release）。python3 标准库探针（密码经环境变量传入）以全新一次性账号复走同一七步链+失败态，与 Web 前端同一 HTTP 契约（`routes_career.go`）。
- **结果**：EXIT=0，17×HTTP 200。逐字响应原文（token/密码脱敏，refresh_token 一并脱敏）落档 **evidence/web-receipts-r2.txt**；PDF 落盘 **evidence/career-material-v1-r2.pdf**（`file`=PDF 1.7/1 页；`shasum -a 256`=`e74104c4e7b7c6ac0d00219d0c524acbe938f13e5248394e87ec92cb8193f849`，与导出回执 `files[pdf].fileDigest` 相等——自洽）。
- **六 ID→r2 回执映射**（指针已更新入矩阵 §0.1）：提案→步 1b/1d（同 requestId 回执重放）；确认→步 1c（修订 0→1→2）；机会导入→步 2；正文摘要→步 5c `contentDigest`；PDF SHA→步 6b；Task(session)→步 4 `taskId`/`runId`。附加实证：步 8 一次真实 revision 冲突→**同 requestId 重试成功**（未知回执恢复语义）；步 7b 二次投递诚实拒绝 409 `submission_already_confirmed`；步 9 找岗诚实失败 `no_vetted_sources`（searchId 原文）。
- 过程如实记录：探针迭代 5 次才全通（material CAS 语义=profile 头、channel 合法词汇=web、download URL 前缀、签名密钥 hex 长度、每轮新账号）；每次失败响应均真实服务器返回，最终成功轮全文落档。

### 5.2 M2：分享导入/通知权限无矩阵专门格 → §2A 两表补齐

- **缺口**：主计划 T33 验收明文要求「分享导入、PDF/DOCX 下载、通知权限、跨端同步与越权检查有可复现记录」，矩阵缺分享导入与通知权限专门格。
- **补证（矩阵新增 §2A，两表分环境专门格，层级如实标注）**：
  - **分享导入（§2A.1）**：微信=✅ T24「先核对后提交」（prepareSharedImport 零网络预览→confirm 提交；percent-encoding 修复 `09a414c8b`；E1/E2/P1-P4/P7+DevTools 两步）——证据层级一级=集成仓库归档 `wave-report-T24小程序.md`，本轮 r2 未重开 DevTools（修复简报允许路径：引用归档+本轮补充指针，如实标注层级）；Web=not-applicable（能力事实：无微信分享入口，等价入口粘贴 JD 已实证）；iOS/鸿蒙/Android=blocked（引 §1.2/§1.4/§1.5 依据）。
  - **通知权限（§2A.2）**：微信=真机弹层 **blocked（如实）**（tourist appid 无模板可配，`REMINDER_SUBSCRIBE_TEMPLATE_IDS=[]`；模拟器内真实调用失败不弹层→页面如实 unavailable+站内回落，delivered 恒 false；拒绝路径由单测 C2 钉死非未实现）——一级=`wave-report-T30小程序.md`；Web=✅ 站内待办 T20 verified（隐私正文冻结模板、去重、退订后可读+重订阅、站内为事实源；live 4 项+HTTP 实证）——一级=`wave-report-T20-Web.md`；iOS/鸿蒙/Android=blocked（依据同上）。

### 5.3 修复轮四门复跑（同 HEAD，零回归确认；gate-logs/）

| 门 | 命令 | 结果 |
| --- | --- | --- |
| Go career | `go test ./internal/modules/career/... -count=1` | **ok 108.538s** exit=0（gate-logs/go-career-r2.log） |
| Web 类型 | `pnpm typecheck:web` | **exit=0**（gate-logs/typecheck-web-r2.log） |
| 小程序类型 | `pnpm --filter @weknora/miniprogram typecheck` | **exit 2：13 errors，全部 `features/account/pages.tsx` CommercialSummary 既有基线（该文件外 0）**——与 T30/T32 基线一致零新增（gate-logs/mp-typecheck-r2.log） |
| 小程序测试 | `pnpm --filter @weknora/miniprogram test` | **tests 172 / pass 172 / fail 0 / skipped 0**（5 个 build-output 条件跳过项本轮实际执行通过；≥ 167+5 基线）（gate-logs/mp-test-r2.log） |
| diff 卫生 | `git diff --check` | **exit=0** |

### 5.4 本轮文件清单（均在本任务所有权内）

- `docs/plans/issue-140/verification/launch-matrix.md`（§0.1 + §2A 两表 + 头部修复轮行；§2 标题在编辑中一度误删已恢复，结构复核 0/1/2/2A/3/4）
- `evidence/web-receipts-r2.txt`（新）、`evidence/career-material-v1-r2.pdf`（新）
- `gate-logs/go-career-r2.log`、`typecheck-web-r2.log`、`mp-typecheck-r2.log`、`mp-test-r2.log`（新）
- 本报告追加节；隔离临时目录 `/tmp/wk-t33fix-20260927-004238`（仓库外，用后清理，一次性密钥未落盘）
