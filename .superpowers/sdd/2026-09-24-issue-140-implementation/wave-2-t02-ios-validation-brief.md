# Wave 2 — T02 iOS 实测补齐（validation）

你是 frontend_validator。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 你只验证、只记录，**不改任何生产代码**；发现缺陷写进报告由主控另行派发修复。

## 1. 背景与目标

T02/#145（Expo iOS/Android 受认证 Task 薄切片）代码已集成（13a8e1dc9/75eb5dcfe/f8f15fedc）；单测/typecheck 过。缺口（台账载明）："native iOS and Android authenticated Task read and live cross-Tenant denial absent"。Android 无 adb/emulator **永久不可测**（既有裁定，如实记录为环境门槛，不伪造）。你的目标：在真实 iOS 模拟器上完成并取证主计划 Task 2 验收的可达成部分：

1. iOS 开发构建真实启动（Expo 55 / RN 0.83.10，模拟器 **iPhone 18 Pro，UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`**，iOS 27 runtime）。
2. 真实 GUI 登录（邮箱密码，应用自身认证流）后读取一个受保护 Task（任务列表 + 详情）。
3. 跨 Tenant 拒绝：另一 Tenant 用户凭据对同一 Task 不可见/被拒（GUI 或其触发的 HTTP 层均可，如实声明层级）。
4. 未认证读取被拒（登录门前状态）。
5. 会话过期/空间切换后旧 Task 数据与迟到响应不可见（若 GUI 难以构造，至少在应用运行时以真实凭据过期重放验证 scope epoch 行为，如实声明层级）。

## 2. Worktree 与基线

```bash
cd /Users/wuyongjun/.codex/worktrees/issue-140-t02-live-validation/WeKnora-fork01
git status --short   # 必须 clean（调度员 2026-09-25 实核为空，停在 6d67b51ae）
git checkout --detach 7cbad8941
```
验证对象是**已集成代码**（集成 HEAD 7cbad8941）。报告与证据文件本地 commit 允许；绝不 push、绝不改集成分支、严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 3. 构建与运行环境（实勘事实）

- 工程在 `apps/mobile`（package.json 仅 test/typecheck 脚本——构建用 expo CLI：`npx expo run:ios --configuration Debug` 或 prebuild + xcodebuild，自选并在报告记录确切命令与版本；`expo --version`/`xcodebuild -version` 记录）。
- API origin 注入机制：构建期环境变量 `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN`（在 apps/mobile 源码里 grep 此名称即可定位读取处，调度员实核其存在）。构建时将其值设为 `http://127.0.0.1:57802`（iOS 模拟器与宿主共享 loopback）。
- 指定模拟器：`xcrun simctl boot 0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`（已 boot 则跳过）。
- pnpm 依赖先 `pnpm install --frozen-lockfile`。

## 4. 本地服务器与 fixture（沿用既有模式，端口 57802）

先读 `docs/plans/issue-140/task-6-live-validation.md` 的 Reproduction outline（服务器启动 env 全集：DB_DRIVER=sqlite、新 DB_PATH、RETRIEVE_DRIVER=sqlite、STREAM_MANAGER_TYPE=memory、STORAGE_TYPE=local、LOCAL_STORAGE_BASE_DIR、SERVER_HOST=127.0.0.1、SERVER_PORT=57802、DISABLE_REGISTRATION=false、APP_EXTERNAL_URL、生成的 JWT_SECRET/SYSTEM_AES_KEY/WEKNORA_ARTIFACT_SIGNING_KEY）。注册两个用户（各含独立 Tenant）；按 T04/T06 fixture 方式在迁移后的 SQLite 插入 owner 的 `sessions`/`agent_runs`（`succeeded`）行，使任务列表非空。不共享任何真实账号/服务/模型；结束停止进程并删除临时目录。T03 先例：`task-3-live-http-validation.md`（Lite 启动细节）。

## 5. 验收判定基准（主计划 Task 2 verbatim 摘录）

- "在现有 Task Office 集成测试增加未认证、另一 Tenant、会话过期和切换空间后延迟响应场景"——单测已覆盖（74 mobile tests），你验证真实运行时行为。
- "在 iOS 和 Android 开发构建上各取一条真实受保护 Task 读取证据。设备不可用时明确留下环境门槛，不将单测作为设备验收。"——iOS 你完成；Android 记录环境门槛（无 adb/emulator，命令实勘输出贴进报告）。
- DAG 节点 T02 证据要求："iOS、Android 各提供实际构建、登录与受保护 Task 读取记录。未登录、跨 Tenant、空间切换和端能力探针结果可复现。"

## 6. 全局约束（verbatim 摘录）

- 你是 validator：生产代码零改动，唯一可写文件是证据/报告文档。
- 不自动投递、不绕过登录/CAPTCHA；测试数据库、端口（57802）隔离。
- 严禁 push、merge、deploy、GitHub 操作；禁止派发子 agent。
- 报告只写事实与实测输出；模拟器构建失败、登录流无法建立等任何环节受阻时如实 blocked 附命令与输出，绝不伪造设备证据。
- 凭据/密钥 disposable：结束清理，报告不保留 token。

## 7. 产出

报告写入本 worktree `docs/plans/issue-140/task-2-live-ios-validation.md`：环境（命令+版本+UDID）、观测表（每项 PASS/blocked + 关键输出）、Android 环境门槛记录、未验证项/局限（如实声明验证层级：GUI vs 运行时 seam vs HTTP）、复现提纲。本地提交 `docs(mobile): record T02 live iOS validation`。最终消息报告 HEAD SHA 与各项证据结果。T02 是否 verified 由主控依据你的证据裁决（Android 缺项意味着大概率维持 running/blocked 口径而非 verified；你的价值是把 iOS 侧证据补齐做实）。
