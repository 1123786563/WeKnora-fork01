# #69（T39 iOS）/ #70（T40 Android）模拟器验收轮实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 依用户裁定（2026-10-02：支付宝豁免 R-82a；**app 端测试以模拟器为准**），把 #69 的 8 项 blocked-env 与 #70 的 Release/签名/七工作流在模拟器+本地授权部署栈上尽实，出双票验收判词。

**Architecture:** 复用 #31 轮资产：授权 HTTPS `https://192-168-3-33.nip.io:8443`（nginx）+ Casdoor（:18000，t01live 用户）+ 后端（main HEAD）+ iPhone 18 Pro sim（booted）+ AVD test36。权威验收面：`docs/plans/issue30-sweep/ios-evidence/t39-acceptance.md`（人工路径九流清单+blocked 五项）与 `docs/plans/issue30-sweep/android-evidence/android-release.md`（Release/签名/七工作流表）。

**Tech Stack:** Expo iOS Release（simctl）、Android Gradle assembleRelease + 本地 keystore、emulator adb、既有 t39/ios-acceptance-run.sh 门。

## Global Constraints

- 用户裁定口径：模拟器证据=app 端验收证据；**真机/APNs/FCM送达/TestFlight 类不可模拟项如实保持 blocked-env**（不得伪造、不得降断言）。
- 验收门（emit-acceptance-record.ts / t39-record.json 语义）保持：evidenced 必须带产物、blocked-env 必须带理由。
- 密钥不入库；栈共享不误伤（Lago/casdoor/onyx 等在跑容器不动）。
- 不 push、不动 GitHub（判词后由主控统一）；提交带 `(T39 #69)` / `(T40 #70)`。
- 工作区：`.superpowers/sdd/2026-10-02-issue69-70/`。

---

### Task 1（#69/T39）: iOS 模拟器授权面九流尽实
- [ ] 起栈核验（nginx:8443/后端/Casdoor/sim booted）；重装当前 HEAD Release 包。
- [ ] 逐流执行人工路径清单 1-9（登录/切租户/发任务 SSE/后台恢复/离线草稿/通知本地路径/分享面板/语音拒权/登出冷启动），可自动化的自动化（simctl io 截图+日志；输入可用 XCUITest/adb 对应手段或 idb 若可用），不可自动化的以可复现脚本+截图留证。
- [ ] blocked 五项重判：①真机→按裁定以模拟器为准（真机挂账注明）；②APNs→保持 blocked（如实）；③授权面自动化→凭据已备，尽实；④弱网→跑 opt-in 真实部署 harness（凭据已备）或 sim 级手段，宿主 sudo pf 不做；⑤分享面板→sim 实测。
- [ ] 重发 `t39-record.json`（门 exit 0）+ 报告增补段；提交证据与记录。

### Task 2（#70/T40）: Android Release + 七工作流矩阵
- [ ] `assembleRelease`（本机 SDK/Gradle 可用；产物 apk）+ 本地 release keystore 签名（工程验收口径，keystore 不入库）。
- [ ] emulator test36 安装 Release 包 → 七工作流逐行验证（系统返回/后台权威同步/通知两路径〔FCM 送达保持 blocked〕/file URI 录音上载链/Keystore 落盘+备份排除/麦克风两路径/弱网离线恢复），截图+日志留证 `android-evidence/`。
- [ ] runbook 表逐行回填证据路径；未满足项保持 blocked 带理由。

### Task 3: 双票判词
- [ ] 两票独立抽查（opus，各一）：证据↔验收点对照、门语义合规；判词「#69/#70 closure-ready（真机/推送类边界内）」或阻塞清单。
- [ ] 台账（t39-acceptance.md/android-release.md）终局段；GitHub 关票由主控按用户授权执行。

## Self-Review
覆盖：#69 九流+五 blocked 重判→Task 1；#70 Release+七工作流→Task 2；判词→Task 3；无占位。
