# Native E2E · 双平台产品链（MX-034）

## 状态：blocked-env（如实记录，不以降级替代记通过）

`tests/mobile-v2/e2e/mx-034.test.ts`（frozen）已就位：真实探测三要素（dev client 构建产物标记、平台设备/模拟器、后端环境变量），任一缺失即抛 `blocked-env`。**当前缺失项**：

| 要素 | 状态 |
|---|---|
| dev client 构建（iOS `.e2e-ios-build.ok` / Android `.e2e-android-build.ok`） | **未构建**（本地未执行 prebuild+xcodebuild/gradlew 全链构建；产物不入库） |
| Android 设备/adb | adb 不在 PATH（SDK 目录存在待核） |
| 受控产品测试账户（E2E_PRODUCT_ACCOUNT）+ E2E_WEKNORA_ORIGIN | **未授权设置** |

## 运行配方（解除阻塞后逐步执行）

```bash
# 1. 后端栈（docker compose WeKnora-app/postgres/redis 已在运行——核对 origin 可达）
export E2E_WEKNORA_ORIGIN=https://<docker-origin>   # 不写入仓库/证据
export E2E_PRODUCT_ACCOUNT=<受控账户>                # 仅环境变量
# 2. 构建 dev client（每平台一次）
cd apps/mobile
pnpm exec expo prebuild -p ios && xcodebuild ...     # 成功后：touch ../../.e2e-ios-build.ok
pnpm exec expo prebuild -p android && ./android/gradlew assembleDebug  # 成功后：touch ../../.e2e-android-build.ok
# 3. 启动模拟器并安装 dev client + Maestro
maestro test tests/mobile-v2/maestro --platform ios
maestro test tests/mobile-v2/maestro --platform android
# 4. frozen 链
pnpm exec tsx --test tests/mobile-v2/e2e/mx-034.test.ts
```

## 套件内容（Maestro 配置已交付）

- `maestro/core.yaml`：M01 登录→M02 空间→M03 工作台→M05 新建→M07 对话→M08 详情→M09 审批→M17 我的→退出。
- `maestro/scope.yaml`：切空间（旧数据不可见/迟到不覆盖）+ 杀进程冷启动恢复（request_id 对账同一 Run）。

## 已验证的等价层（不替代 native-e2e，如实分层）

- native-component（挂载）：MX-008/009/013/020 挂载套件 9/9。
- 恢复/并发/安全注入：MX-033 套件 7/7（进程边界等价注入）。
- 跨语言字节合同：MX-004 frozen（Go 字节→TS parser）。
- **以上不记为 native-e2e 通过**；解除三要素后按配方执行并回填本文件。

## 记录要求（执行时回填）

- build（xcodebuild/gradlew SHA）、OS 版本（模拟器 runtime）、后端 SHA（WeKnora-app 镜像）、数据库与服务来源。
- 冷启动通知、前后台切换、杀进程、权限拒绝各场景逐项记录。
- 浏览器原型截图不替代原生截图；18 页视觉对比在 MX-035 出证。

---

# 第四轮设备实测更新（2026-09-18）

## 本轮设备会话完成项（真实执行，非模拟）

| 项 | 结果 |
|---|---|
| 后端栈 | 本地 Docker WeKnora 栈（:8080）确认运行；受控测试账户 `e2e_tester` 经 `/api/v1/auth/register`+`/login` 创建并登录成功（tenant 10005） |
| Android dev client | **构建成功**（gradle assembleDebug → app-debug.apk）→ headless 模拟器 test36 安装+启动成功（emulator-5554） |
| iOS dev client | **BUILD SUCCEEDED**（xcodebuild 第七轮产出 WeKnora.app；经历 Pods deployment-target 15.1→16.0 与 RevenueCat PaywallColor Swift6 memberwise-init 补丁） |
| Metro | 本 worktree Expo dev server 起 **8083**（8081 被 react-multiclient worktree 占用，勿动他人进程）；设备经 DevLauncher 连接成功，bundle 5 次真实打包 |
| 产品树可达 | 修复后设备真实进入 `sources/app` 产品链（英文 server 选择屏出现=根布局重定向生效） |

## 设备实测发现并修复的真实缺陷（三项）

1. **app.config.ts 遮蔽 router root**：`app.config.ts`（841d4319 后引入）无 `router.root`，遮蔽 `app.config.js`（368eb42d）的 `root:'./sources/app'` → Expo 解析到上游 desktop 骨架 `apps/mobile/app/`，产品链整体失联（深链 Unmatched+索引不重定向）。修复：app.config.ts 显式声明同 root（含注释说明遮蔽史）。
2. **入口 Unistyles 初始化顺序**：expo-router require-context 使 `(app)/...` 先于 `_layout.tsx` 求值（ASCII '('<'_'），`artifacts/[id].tsx:16` 模块级 StyleSheet 在 configure 前崩溃（红屏）。修复：`apps/mobile/index.ts` 先 import `./sources/unistyles.ts`。
3. **SecureStore key 非法字符（Android）**：`weknora:selected-origin` 等含 `:` 的 key 抛 `Invalid key`（Uncaught promise ×2，origin 无法持久化）。修复：全部 key `:`→`.`（origin-storage + 三处 OIDC key 文件）。

## 当前剩余阻塞（如实，未定位）

- **server 屏 Continue 后无导航**：输入合法 origin 点 Continue 停留原屏，无 JS/SecureStore 报错。怀疑 `save()`（write→setMobileHost→router.replace('/(app)/login')）链某环未完成（候选：auth 冷启动 adapter.read 挂起致 loading 常真，或 gate 与 replace 竞态）。下一轮：加临时导航日志/直接核验 `weknora.selected-origin` 持久化结果。
- iOS app 已产出未安装（iPhone 17 Pro 已 booted；iOS 模拟器用 `http://localhost:8083` 直连 Metro）。
- MX-035 设备矩阵待导航链打通后采集。

## 已获原生过程截图（artifacts/e2e/，真实设备；非 18 页矩阵）

android-legacy-login-light（desktop 骨架登录页——遮蔽缺陷实证）、android-server-entry、android-login-filled、android-postlogin-legacyhome-light、android-M01-login-light、android-M03-product-light、android-step1/probe1-3。

## 复现（接续）

```bash
# Metro：cd apps/mobile && EXPO_ROUTER_APP_ROOT=./sources/app pnpm exec expo start --port 8083 --dev-client
# Android：DevLauncher → New dev server → http://10.0.2.2:8083
# iOS：xcrun simctl install "iPhone 17 Pro" apps/mobile/ios/build/DerivedData/Build/Products/Debug-iphonesimulator/WeKnora.app && xcrun simctl launch "iPhone 17 Pro" com.weknora.mobile → DevLauncher → http://localhost:8083
```
