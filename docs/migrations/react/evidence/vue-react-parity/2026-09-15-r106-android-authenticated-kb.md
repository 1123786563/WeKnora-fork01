# r106 Android 认证后知识库列表验收

## 环境与步骤

- Release APK：`apps/mobile/android/app/build/outputs/apk/release/app-release.apk`
- AVD：`test36-small`（Android 36）
- 后端：本机 `uimig-server`，模拟器通过 `http://10.0.2.2:8080` 访问
- 账号：`parity-test@local.dev`
- 步骤：清除应用数据 → 设置 Server address → 输入账号密码 → Sign in

## 结果

- 登录请求成功，应用从认证页进入 `knowledge/index`。
- 知识库列表成功加载真实后端数据：`Parity KB Demo`、描述 `parity test data`、`文档 · 0 项`。
- 首屏同时显示“新对话”“共享空间”“系统设置”“退出登录”“新建知识库”及全部/我创建的/收藏/最近访问筛选。
- 设备截图：`/tmp/android-authenticated4.png`。

## 证据边界

本证据关闭 Android Release 安装、启动、认证和真实 KB 列表加载门禁；KB 详情、文档编辑/上传、Wiki/FAQ、图谱、聊天以及写操作仍需逐项设备验收。
