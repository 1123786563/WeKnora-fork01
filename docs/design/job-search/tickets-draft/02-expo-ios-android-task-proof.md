# T02：Expo iOS／Android 受认证 Task 薄切片

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#145](https://github.com/1123786563/WeKnora-fork01/issues/145)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

复用仓库现有 Expo／React Native 工程，让同一位已登录用户在 iOS、Android 打开同一条获授权的现有 Task，固定可供求职页面复用的原生运行边界。

## Acceptance criteria

- [ ] iOS、Android 开发构建均在真实模拟器或设备打开受保护 Task；使用现有 Expo 工程及 Mobile Runtime
- [ ] 未登录和跨 Tenant 读取被拒绝，切换空间后旧 Task 数据和迟到响应不可见
- [ ] 两端验证导航、文件选择／下载、系统分享、通知与安全存储；缺口有明确 Adapter 记录
- [ ] UI 使用原生 React Native 组件映射 TDesign Mobile React 的视觉 token 和交互规范，不直接导入移动 Web 组件
- [ ] 记录构建命令、SDK／依赖版本、模拟器或设备及行为证据

## Blocked by

None (can start immediately)。

## Ownership and interfaces

- 主责边界：现有 Expo／React Native 移动运行面、iOS／Android Task 入口及平台 Adapter；不改 Career 后端业务规则。
- 消费接口：现有 WeKnora 认证、Tenant 范围与 Task 公共读取。
- 交付接口：可复用的 Expo 原生页面边界、双目标端能力矩阵与受认证 Task 读取示例。
- 与 T01 的鸿蒙验证使用独立 Worktree；共享 Mobile Runtime 契约先冻结再集成。

## Verification evidence

- iOS、Android 各提供实际构建、登录与受保护 Task 读取记录。
- 未登录、跨 Tenant、空间切换和端能力探针结果可复现。

## Failure and unknown results

任一目标失败时保留版本、错误和复现步骤；不得用 WebView 或另一目标通过替代。

## Parallel scheduling

- 理论前沿：G0；与 T01 鸿蒙可行性、T03 Career/Identity、T04 Artifact、T06 小程序下载并行。
- 独立 Worktree，隔离模拟器、端口和构建目录。
