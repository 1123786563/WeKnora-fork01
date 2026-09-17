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
