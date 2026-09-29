# B5 iOS 模拟器增量复验报告（issue30-sweep）

- 复验员：iOS 复验员-B5（动态工作流子代理）
- 日期：2026-09-28 02:04–02:35（全流程约 31 分钟；构建本身约 20 分钟，未触及 45 分钟停止线）
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`（HEAD 8eff12283）
- 技能依据：`ios-dev` SKILL（先 `ios_preflight`，再构建/截图；idb 不可用 → 无 UI 自动化，仅 build/run/screenshot 路线）

## 0. 环境预检（ios_preflight，全部通过）

macOS darwin 27 arm64 / Xcode 27.0 (27A266a) / xcodebuild 可用 / simctl 可用 / iOS 27.0 运行时 11 台设备。
UI 后端：`backend=auto; idb unavailable` → 无 tap/swipe 能力，本报告全部交互用 deep link（`weknora://` scheme）+ `simctl io screenshot` 完成。

环境限制：本机未安装 Simulator.app GUI（`/Applications` 与 `Xcode.app/Contents/Applications` 均无，`open -a Simulator` 失败、MCP `ios_boot_simulator` 报 "Unable to find application named 'Simulator'"）。不阻塞：`simctl boot/install/launch/io screenshot` 全部走 CLI 正常工作。

## 1. 前提核查：已提交的 iOS 工程快照与 B5 依赖脱节（重要发现）

任务前提称"此前实测已生成并提交 apps/mobile/ios 工程"。实测事实：

1. `git ls-files apps/mobile/ios | wc -l` → **0**。根 `.gitignore:98`（`apps/mobile/ios/`）忽略整个目录。
2. 历史：`8d39806f4`（fix(mobile) 补齐 286 个文件，当前分支祖先）曾 force-add iOS 工程；随后 **`80fc3648a`（chore: 清理项目中冗余的第三方依赖、资源文件与配置文件）把 `apps/mobile/ios/` 全部删除**（`git log --diff-filter=D -- 'apps/mobile/ios/'` 实证，含 Podfile）。
3. T39 固化脚本 `apps/mobile/scripts/ios-release-build.sh:16-19` 注释明确设计意图："prebuild 重新生成 ios/（.gitignore 不入库）……漂移防护的唯一事实源"——即 iOS 工程不入库、靠 prebuild 重放是 #69 的既定管线。
4. **风险实测**：本地遗留的 `apps/mobile/ios/`（未跟踪副本）落后于 B5——prebuild 后、pod install 前 `ls ios/Pods | grep -i 'expo-audio|expo-network|ExpoAudio|ExpoNetwork'` 输出为空；旧 `Podfile.lock` 无 ExpoAudio/ExpoNetwork；`ios/WeKnora/Info.plist` 无 `NSMicrophoneUsageDescription`。若复验者直接对遗留工程跑 `xcodebuild` 增量构建，构建会照常 SUCCEEDED（Pods 工程由旧 lock 决定），**静默产出缺 expo-audio/expo-network 原生模块的包**，只在运行时 `require('expo-audio')` 处崩。因此本次按任务预案执行了重新 prebuild + pod install。

## 2. 构建流程与命令（全部实际执行）

### 2.1 prebuild 重新同步（02:04:33，秒级完成）

```
cd apps/mobile && npx expo prebuild -p ios        # 日志：01-prebuild.log
```

- 复核 `apps/mobile/plugins/ios-xcode27.js` 为幂等注入（唯一锚点判重：`# weknora_ios_xcode27_clamp`、`# weknora_ios_inhibit_warnings` 等），重复 prebuild 不丢 B4 修复（scene 生命周期、splash wordmark、部署目标钳制 16.0、inhibit_all_warnings、RCTNewArchEnabled=false）。
- prebuild 后 `ios/WeKnora/Info.plist:50-51` 写入 `NSMicrophoneUsageDescription = "WeKnora uses the microphone for voice dictation and live voice sessions."`（来自 `app.json:32` 的 expo-audio plugin 声明）。

### 2.2 pod install（02:04:58，59s）

```
cd apps/mobile/ios && pod install                # 证据：02-pod-install.log（会话回填）
```

- `Pod installation complete! There are 100 dependencies from the Podfile and 106 total pods installed.`
- 结果：`Podfile.lock:33 - ExpoAudio (55.0.18)`、`Podfile.lock:87 - ExpoNetwork (55.0.18)`。
- autolinking 独立核验：`node --eval "require('expo/bin/autolinking')" expo-modules-autolinking resolve --platform ios --json` → 21 模块，含 `expo-audio -> ExpoAudio`、`expo-network -> ExpoNetwork`。

### 2.3 xcodebuild Release 增量构建（02:07:33 → 02:27 BUILD SUCCEEDED，约 20 分钟）

```
xcodebuild -workspace apps/mobile/ios/WeKnora.xcworkspace -scheme WeKnora \
  -sdk iphonesimulator -configuration Release \
  -derivedDataPath apps/mobile/ios/build          # 完整日志：03-xcodebuild-release.log（7.4MB，33,969+ 行）
```

- 结果：`** BUILD SUCCEEDED **`。产物 `WeKnora.app/WeKnora` 22M、`main.jsbundle` 3.7M（时间戳 02:26-02:27）。
- 与 T39 固化脚本一致性：`apps/mobile/scripts/ios-release-build.sh:35-37` 的 xcodebuild 参数与本命令等价（脚本额外带 `-destination 'generic/platform=iOS Simulator'`）。
- 耗时注记：其中约 15 分钟（02:11–02:26）消耗在 metro `export:embed` 等待 watchman 对主仓库的初始 crawl——`.worktrees` 目录 23G、26 个 worktree，watchman 日志 02:26:25 "crawl complete"。编译本身（ExpoAudio 127 行、ExpoNetwork 115 行、WeKnora 173 行构建日志活动）很快。这是环境性延迟，非工程缺陷。
- B5 新 pod 确入链接面：构建日志含 "Explicit dependency on target 'ExpoAudio' in project 'Pods'"、"-lExpoAudio/-lExpoNetwork" 链接标志。

### 2.4 产物核验（B5/#69 验收点：expo-audio/expo-network + 麦克风文案）

| 验收点 | 实测证据 | 结论 |
|---|---|---|
| ExpoAudio 原生模块入包 | `nm WeKnora.app/WeKnora | grep -c ExpoAudio` → 1414 符号（含 `__DATA__TtC9ExpoAudio11AudioModule` ObjC 类符号）；静态链接入主二进制（Frameworks/ 无独立 .framework，Podfile 未启用 use_frameworks，属预期） | ✅ |
| ExpoNetwork 原生模块入包 | 同上 `grep -c ExpoNetwork` → 83 符号 | ✅ |
| 麦克风权限文案随包 | `plutil -extract NSMicrophoneUsageDescription raw WeKnora.app/Info.plist` → "WeKnora uses the microphone for voice dictation and live voice sessions." | ✅ |
| 依赖声明 | `apps/mobile/package.json:15` expo-audio ~55.0.18、`:20` expo-network ~55.0.18（B5 已提交层） | ✅ |

## 3. 安装、启动与页面验证

模拟器：iPhone 18 Pro（iOS 27.0，UDID 0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC），`simctl boot` 后为 Booted 态。

```
xcrun simctl install <UDID> .../Release-iphonesimulator/WeKnora.app   # INSTALL_OK（02:29:06）
xcrun simctl launch <UDID> com.weknora.mobile                          # pid 8712（02:29:44）
```

插曲：首次 launch 用了臆测的 `org.weknora.WeKnora` 报 FBSOpenApplicationServiceErrorDomain code=4；经 `plutil -extract CFBundleIdentifier` 核实真实 id 为 `com.weknora.mobile` 后成功。

### 3.1 截图证据（均在 `ios-evidence/b5-recheck/`）

| 截图 | 路由/来源 | 实测内容（视觉模型逐字复核） |
|---|---|---|
| 04-first-screen.png | 冷启动首屏 | DeploymentLoginScreen："Sign in to WeKnora"、部署地址输入框（weknora.example.com）、Email/Password——应用壳+JS bundle+路由全部正常 |
| 06-voice-room.png | `weknora://tasks/voice?taskId=recheck-demo` | #57 Voice Room 结构性空态：「当前部署或设备不支持语音房（需要麦克风与授权通道）。」+ 顶部常驻文案「语音确认的文字以『调整指令』进入任务时间线；高风险审批只能在行动收件箱完成，语音无法代替审批。」（AC2 结构性无审批能力的 UI 面）+ 按钮「开始说话 / 离开语音房 / 重试写入任务」 |
| 07-task-detail.png | `weknora://tasks/detail?taskId=recheck-demo&runId=recheck-run` | 「无法读取该任务 / 请先登录并激活空间，再打开任务详情。」+ 重试——B2-F13 错误码分流正常 |
| 08-inbox.png（附 08-inbox-crop.png 放大） | `weknora://inbox` | 「请先登录并激活空间，再查看行动通知。」——未授权分支文案（inbox.tsx default export 分支） |
| 09-tasks.png（附 09-tasks-crop.png 放大） | `weknora://tasks` | 「请开始新的任务或从行动通知进入。」——未登录空态引导 |

像素量化佐证（PIL）：08/09 非白像素 0.52%，与 07（0.69%，含确认文字）同量级——页面确有渲染内容，非白屏崩溃；全图视觉模型对浅灰小字漏检，放大裁剪后逐字确认。

### 3.2 启动日志报错（05-launch-errors.log）

`simctl spawn log show --last 5m --predicate 'process == "WeKnora"'` 过滤 error/fatal/crash/exception 后仅 3 条：

1. UIIntelligenceSupport XPC "No such process"（模拟器系统服务缺失，iOS 27 模拟器通用噪音）
2. 同上 XPCRichError
3. PointerUI "non-launching port is incompatible"（系统噪音）

**无应用级错误、无崩溃、无 JS 异常记录。** 冷启动 → 登录页渲染全程健康。

## 4. #69 T39 验收矩阵的可模拟器化部分（本次覆盖范围）

| T39 验收项 | 本次状态 |
|---|---|
| Release 构建管线可复现（固化脚本） | ✅ 脚本存在且参数经本次手动等效执行验证（ios-release-build.sh:35-37） |
| expo-audio/expo-network 原生依赖入包 | ✅ 见 §2.4（前提是重跑 prebuild+pod install，遗留工程不含） |
| 麦克风权限文案 | ✅ 随包 Info.plist 实测 |
| 真机 Release 安装包安装+运行 | ✅ 模拟器等效面：install→launch→5 页面渲染、无崩溃（真机本身未做，见 §5） |
| 冷启动恢复 / 撤销不可复活 / 弱网重续 三组真实部署集成证据 | ❌ 不可模拟器化：需登录态+真实后端部署+网络损伤注入；模拟器未登录态无法进入这些路径（脚本 emit-acceptance-record.ts 的 opt-in 通道亦需真实凭据） |
| 麦克风授权轮次 / 真实语音会话（W30） | ❌ 同上：需要授权态+后端转写代理；本次仅验证到语音房结构性空态 UI（AC2 文案面） |

## 5. 无法验证项及原因（blocked-env，如实声明）

1. **登录态后的全部页面**（任务列表数据、审批收件箱条目、交付回执、语音房会话轮次）：应用需要真实部署地址+账号；模拟器内无可用后端，weknora.example.com 为占位地址。
2. **三组逆境集成证据**（冷启动恢复/撤销不可复活/弱网重续）：依赖 #69 定义的真实部署环境与网络损伤，超出模拟器未登录复验能力。
3. **麦克风权限弹窗实测**：未登录态语音房呈结构性空态，不触发录音请求；且模拟器无 GUI 窗口、idb 不可用，无法驱动授权弹窗交互。
4. **UI 自动化**（tap/swipe/type）：idb backend 不可用（ios_preflight 报告），全部页面导航改用 deep link 完成，功能等效但交互细节未覆盖。
5. **真机（物理设备）安装**：无连接设备，未执行。

## 6. 发现（findings）

1. **[important] 遗留 iOS 工程静默缺 B5 原生模块的增量构建陷阱**：`80fc3648a` 删除入库快照后，本地未跟踪的 `apps/mobile/ios/`（含旧 Podfile.lock/旧 build/）不会因 B5 的 package.json 变更而自动失效——直接 `xcodebuild` 增量构建照常成功但产物缺 ExpoAudio/ExpoNetwork，仅运行时 `require('expo-audio')` 才暴露。本次以 prebuild+pod install 重放规避并实证（§1.4/§2.2）。建议：T39 脚本已有 prebuild 步骤属正确防线，但任务描述中"此前已生成并提交 apps/mobile/ios 工程"的说法与 git 现状（0 文件入库）不符，后续批次任务书应改为"运行 ios-release-build.sh 重放"。
2. **[minor] 任务前提描述与仓库现状不符**：如上，"已提交 apps/mobile/ios 工程"实际为"曾提交（8d39806f4）后被删（80fc3648a），现行设计为不入库+脚本重放"。
3. **[minor] watchman 初始 crawl 拖长首次 Release 构建 ~15 分钟**：主仓库 `.worktrees/`（23G、26 个 worktree）导致 fsevents watcher 初始爬取 02:11–02:26（watchman 日志 "crawl complete"）。对 45 分钟预算构成真实挤压。缓解建议：watchman 配置忽略 `.worktrees`，或验收机预热 watchman 后再计时。
4. **[minor] 本机 Simulator.app GUI 缺失**：MCP `ios_boot_simulator`/`ios_screenshot` 路线不可用（"Unable to find application named 'Simulator'"），改用 simctl CLI 全程完成（boot/install/launch/openurl/io screenshot 均正常）。环境限制，非工程缺陷。

## 7. 结论

B5 集成后的移动端 iOS Release 线在重放 T39 管线（prebuild → pod install → xcodebuild Release）后**构建成功、安装启动正常、无启动报错**；B5 新增的 expo-audio/expo-network 原生模块与麦克风权限文案确认入包；#57 语音房、任务详情、行动收件箱、任务列表四个 B5 相关页面在未登录态全部渲染为设计的引导/空态分支。无法在模拟器复现的部分（登录态、真实部署集成、麦克风授权轮次、真机）已在 §5 如实声明。
