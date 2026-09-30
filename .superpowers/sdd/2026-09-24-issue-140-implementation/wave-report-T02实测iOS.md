# Wave 2 — T02 实测 iOS（validation）报告

- Subagent: 实现-T02实测iOS（frontend_validator，只读）
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t02-live-validation/WeKnora-fork01`（基线 `7cbad8941` detach → HEAD `0af444010`，本地提交，未 push）
- 正式报告：`docs/plans/issue-140/task-2-live-ios-validation.md`（+ 7 张证据截图 `task-2-evidence-ios/`，随报告本地提交）
- 角色为 validator：**生产代码零改动、无 TDD 适用项**（简报明令不改生产代码）；worktree 全程 clean，Metro 自动改写的 `apps/mobile/tsconfig.json` 已即时 `git checkout --` 还原。gitignored 生成物（apps/mobile/ios/、证书、/tmp harness）上的补丁仅用于取证，详见报告 §3。

## 结果速览（层级如实声明）

| 验收项 | 结果 | 层级 |
| --- | --- | --- |
| iOS 开发构建真实产物 | PASS（Release `xcodebuild` BUILD SUCCEEDED ×3；`expo run:ios` 因本机缺 Simulator.app 不可用，命令与报错已录） | 设备 |
| 应用真实启动 → 登录门 | PASS（origin 经 `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57803` 预填 + "Use WeKnora Cloud" 按钮） | GUI |
| 真实 GUI 登录 | PASS（键盘输入 → 应用自身 `POST /api/v1/auth/login` 200，经 TLS：自签 CA 仅注入模拟器 keychain） | GUI+HTTP |
| 受保护任务列表/详情 GUI 读取 | **BLOCKED — D-iOS-1**：登录 200 后运行时零后续请求（无 /auth/me、/capabilities、workbench），UI 停登录面，凭据不落盘；可稳定复现 ×3。属主侧 HTTP 层读取 200（列表+snapshot+overview） | HTTP PASS / GUI BLOCKED |
| 未认证拒绝 | PASS（401 ×3 端点；GUI=登录门） | HTTP+GUI |
| 跨 Tenant 拒绝 | PASS（B list 空、B snapshot 404、属主 200） | HTTP（第二账号 GUI 登录因 D-iOS-1 未做，已声明） |
| 会话过期 | PASS（同 DB 轮转 JWT_SECRET → 旧 token 401 `invalid or expired token`，重登 200） | HTTP 运行时 seam（GUI 层不可构造，已声明） |
| 空间切换不可见 | NOT VERIFIED（单 Tenant fixture + D-iOS-1；77/77 单测覆盖语义） | — |
| Android | 环境门槛维持（本任务范围内未尝试）；**但既往「无 adb」记录失实**：`~/Library/Android/sdk` 实存 adb 37.0.1 + emulator + AVD test36/test36-small（仅不在 PATH，0 设备连接）。是否补测由主控裁决 | 实勘 |

## 关键发现（详见正式报告 §5）

1. **D-iOS-1（阻断）**：真实 iOS 上登录成功后授权链路死寂；定位在 `mobile-runtime.ts` signIn 的 `passwordLogin`→`authenticate` 之间（persistCredential/SecureStore 写）。本机 ad-hoc 签名（0 个 Apple 签名身份）+ 空 entitlements 下 keychain 读正常、写无 SecItemAdd 活动亦无报错；注入 entitlements 会使 app 无法启动。建议在有真实开发证书的环境复跑后定性（环境 vs 运行时缺陷）。
2. **D-iOS-2**：简报建议的 `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=http://…` 被三层 HTTPS 强校验静默吞掉（登录屏/runtime/api-client）；本地联调需走 https 环回 + 模拟器受信自签 CA + TLS 代理（本报告方案，可复用）。
3. **D-iOS-3（UX 观察）**：登录失败（400）呈现为 fail-closed 的 "Update required" 面，用户无法区分密码错误与版本失配。
4. 环境注记：本机无 Simulator.app GUI；HID 长字符串注入会截断 RN 受控输入 state（自动化保真度限制，取证以服务器侧请求体为准）。

## 命令与输出摘要（详见正式报告 §1/§7）

- `pnpm --filter @weknora/mobile test` → 77/77 pass, 0 fail；`typecheck` → 0 error。
- `xcodebuild … Release … build` → BUILD SUCCEEDED；`grep -c "127.0.0.1:57803" main.jsbundle` = 1。
- 服务器/代理：Lite @57802（env 全集同 T06），TLS 代理 @57803；两用户（Tenant 3 属主 / Tenant 2 他人）+ sessions/agent_runs fixture（succeeded）。
- 拒绝矩阵：401（无认证 list/snapshot/career）、404（跨 Tenant snapshot）、空列表（跨 Tenant list）、401 过期（轮转后旧 token）。
- 清理：进程/端口（57802/57803/8081）全停，app 已卸载，/tmp 一次性目录（DB/密钥/证书）已删，CA 私钥随目录销毁。

## 提交

- `0af444010` `docs(mobile): record T02 live iOS validation`（报告 + 证据；基于 `7cbad8941`）

## 自查与遗留

- 自查：观测表每项均附层级声明；未伪造任何设备证据；GUI 结论以服务器日志/CFNetwork/AX/像素带四源交叉。
- 遗留（交主控裁决）：① D-iOS-1 的签名环境复跑与定性派发；② Android 是否因 SDK 实存而重新开测；③ D-iOS-2 文档修正；④ T02 verified 与否——iOS 侧完成「构建/启动/GUI 登录 200」，但受保护 Task 的 GUI 读取被 D-iOS-1 阻断（HTTP 层全过），Android 缺项如前。

## Round 1 评审修复（提交 `086d93ed1`，基于 `0af444010`）

R1 结论「核心 GUI 观测失实」（F1 high）+ F2/F3/F4（medium）全部修复；F5/F6（low）按指示仅记录不修：

- **F1（high）**：接受评审 OCR——截图 12/14 为 **UpgradeRequiredScreen**（catch→safe() 路径，`mobile-runtime.ts:302-314`），非登录面；报告「结论先行/观测表 #5/D-iOS-1」已改写，初版「promise 挂起」假说撤回。R2 独立复核：全新安装 + 重新 GUI 登录（`a2@t2.io`→服务器 200），提交后截图 18/20 顶部像素带与已知 Update-required 基准（截图 15）一致（5.4/11.5/19.7/17.0 vs 5.2/10.8/19.4/16.3），与登录门（04）显著不同；下半部为 iOS 保存密码对话框（表单提交成功旁证，评审 F2 指出）。屏幕定性方法学（OCR+像素带，AX 受系统对话框干扰不可靠）已在 D-iOS-1 声明。
- **F2（medium）**：HTTP 证据可复核化——P1–P5 全量重跑并入库脱敏原始记录 `task-2-evidence-ios/http-probes-r2.txt`（P1 401×3；P2 属主列表/snapshot/overview 200 含 fixture；P3 跨 Tenant 空列表+404；P4 空间切换；P5 轮转过期；token/密码只上线不入文）。
- **F3（medium）**：空间切换 seam 补测 PASS——属主 A 双租户（`tenant_members` 直插）经 `POST /api/v1/auth/switch-tenant {tenant_id:3}`（access-token bearer + body refresh）换签后：旧 Task 空列表（200 `{"items":[]}`）、`t02run0001/snapshot` 404；对照 Tenant-1 token 仍可见 fixture。观测表 #10 由 NOT VERIFIED 更新为 seam PASS；迟到响应 GUI 不可见仍声明未验证。
- **F4（medium）**：`a2@t2.io` GUI 填入态补齐——截图 17（a2@t2.io + 12 位密码填入态）、18（提交 200 后）、19（冷重启回登录门=凭据未持久化）、20（二次登录终态）。
- **F5/F6（low）**：维持 R1 记录（Release 取证口径；Android SDK 实存交主控裁决）。
- **R2 重跑校验（命令与真实输出）**：`pnpm --filter @weknora/mobile test` → `# tests 77 / # pass 77 / # fail 0 / # cancelled 0 / # skipped 0`；`pnpm --filter @weknora/mobile typecheck` → exit 0（无输出无报错）。
- 提交列表更新：`0af444010`（R1 报告）→ `086d93ed1`（R1 评审修正 + R2 证据），基线 `7cbad8941`，未 push。
