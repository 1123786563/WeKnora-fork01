# 原生依赖基线 · MX-002

基线 HEAD：`6a70c35a` → 本任务提交见 progress.md。环境：macOS darwin 25.6 arm64、Xcode 27.0（iOS 26.5 SDK）、Android SDK `~/Library/Android/sdk`（build-tools/emulator/cmdline-tools 齐备）、node v26.7.0、pnpm 10.28.2。

## 对齐前 → 对齐后（apps/mobile 实际解析版本）

| 包 | 对齐前 | 官方 SDK55 期望 | 对齐后 |
|---|---|---|---|
| react / react-dom | 19.3.0 / 19.3.0 | 19.2.0 | **19.2.0 / 19.2.0** |
| react-native | 0.83.1 | 0.83.10 | **0.83.10** |
| react-native-reanimated | 4.2.3 | 4.2.1 | **4.2.1** |
| react-native-keyboard-controller | 1.21.14 | 1.20.7 | **1.20.7** |
| react-native-screens | 4.22.0 | ~4.23.0 | **~4.23.0** |
| react-native-safe-area-context | 5.7.0 | ~5.6.2 | **~5.6.2** |
| react-native-svg | 15.12.1 | 15.15.3 | **15.15.3** |
| react-native-webview | 13.15.0 | 13.16.0 | **13.16.0** |
| @shopify/react-native-skia | 2.5.5 | 2.4.18 | **2.4.18** |
| @shopify/flash-list | 2.3.2 | 2.0.2（精确） | **2.0.2** |
| @react-native-community/datetimepicker | 8.4.4 | 8.6.0 | **8.6.0** |
| @react-native-community/slider | 5.0.1 | 5.1.2 | **5.1.2** |
| expo-location | ~55.0.0（解析 55.0.7） | ~55.1.14 | **~55.1.14** |
| @types/react | 19.1.17 | ~19.2.10 | **~19.2.10** |
| @react-navigation/native / native-stack | ^7.2.0 / ^7.14.7 | ^7.1.33 / ^7.14.5 | **^7.1.33 / ^7.14.5** |

renderer 配对：react-dom 19.2.0 === react 19.2.0；react 19.2.0 满足 react-native 0.83.10 peer `^19.2.0`。
packageManager：apps/mobile 由 pnpm@10.11.0 对齐根 pnpm@10.28.2。

## 复验清单（按分册步骤2）

- libsodium（react-native-libsodium）：声明版本不变，仍由现有 lockfile 解析；随整体安装复验通过（无解析错误）。
- 音视频：expo-av **未安装**（现状即无；语音任务 MX-029 前再引入，不在本轮擅自加依赖）。
- MMKV：react-native-mmkv 3.3.3 不在 bundledNativeModules 覆盖范围，保持现状。
- Unistyles：react-native-unistyles 3.1.1（官方矩阵无条目，保持）。
- Reanimated/Worklets：4.2.1 / 0.7.4（与期望一致）。
- 键盘：react-native-keyboard-controller 对齐 1.20.7。
- android/ ios/ 归属：基线为 CNG（目录不入库）；本任务运行 `expo prebuild --no-install` 双平台生成成功（ios exit 0 / android exit 0），目录被 gitignore，未执行任何删除行为，未运行带删除的 prebuild 脚本。
- SDK 大版本升级（SDK57）：**独立决策，未实施**。

## expo-doctor 剩余未解决项（逐条处置）

1. **lock file 检查失败**：monorepo 布局，锁文件在仓库根（pnpm workspace）。非缺陷；doctor 面向单 app 项目的检查假设。记录接受。
2. **Metro config watchFolders 不含 Expo 默认全部条目**：metro.config.js 为既有定制（服务 worktree/Android 调试链路，历史记忆有 serverRoot 定制）。mx-002 无该文件写锁，不擅改；记录为已接受偏差，若 Metro 链路出问题由对应任务重估。
3. **expo-modules-core / @expo/config-plugins 被直接安装**：既有声明（config plugin `withAndroidCleartextTraffic.js` 需要）；doctor 自述插件场景可忽略。记录接受，不删（删除会破坏 config plugin）。
4. **react 重复（19.2.0 移动端 vs 18.3.1 根 web-secure-encryption 解析）**：18.3.1 属根 workspace 其他包的依赖解析，不在 apps/mobile 原生构建图内。记录为已知项。

## 验证结果

| 验证 | 命令 | 结果 |
|---|---|---|
| 版本矩阵行为测试 | `pnpm run test:mobile-v2` | exit 0；tests 2 / pass 2 / fail 0（含 mx-001 回归） |
| 共享回归 | `pnpm run test:shared` | exit 0；tests 579 / pass 579 / fail 0 |
| 双平台 prebuild | `expo prebuild --no-install -p ios / -p android` | 均 exit 0，原生工程生成成功 |
| 原生 typecheck | `pnpm --filter @weknora/mobile typecheck` | **exit 2，3 个既有编译错误**（见下） |

### typecheck 失败项（继承缺陷，非本任务引入，有明确归属）

- `ConversationScreen.tsx(5)` TS2305 `createRequestID` 无导出 → G04 → **MX-006**
- `ConversationScreen.tsx(36,39)` TS2339 `approve/reject` 不在 ConversationCommands → G02 → **MX-005/MX-017**

MX-002 不越锁修复（两文件锁属 MX-017，决策 D-010）。上述错误在对齐前后完全相同（对齐前同样失败），证明非本任务回归。

### 未执行项（如实记录）

- xcodebuild / gradle 完整编译与真机/模拟器启动：**未执行**（工具链在位；完整构建属重操作，安排在集成检查点与 MX-034 双平台 E2E 执行，届时以同一 lockfile 出证）。
