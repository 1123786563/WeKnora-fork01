# T01：Expo 鸿蒙原生兼容性闸口

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

验证现有 Expo／React Native 工程是否能以可维护的适配方式在鸿蒙原生设备运行受认证 Task；形成可复现的技术裁定，供后续移动求职 Ticket 使用。

## Acceptance criteria

- [ ] 明确区分鸿蒙原生应用与运行 Android 兼容包，记录目标系统版本、工具链、Expo SDK、RN 与原生模块兼容矩阵
- [ ] 若有可维护的 Expo 鸿蒙路径，构建可运行包，在鸿蒙设备读取受保护 Task 并验证登录、空间失效、文件、分享、通知和存储
- [ ] 若无可维护路径，给出最小复现、失败证据、受影响 Ticket 和可选技术裁定；不得把 Android 包、WebView 或概念演示记为鸿蒙原生验收通过
- [ ] UI 只映射 TDesign Mobile React 视觉规范到原生组件，不直接导入移动 Web 组件
- [ ] 不改 Career 业务合同；共享 Mobile Runtime 改动先经契约验证再与 iOS／Android 分支集成

## Blocked by

None (can start immediately)。

## Ownership and interfaces

- 主责边界：Expo 鸿蒙适配可行性、原生构建与端能力探针；不接管 iOS／Android 页面或 Career 后端。
- 消费接口：现有 Expo App、Mobile Runtime、Task 公共读取与认证。
- 交付接口：鸿蒙原生可运行路径及能力矩阵，或带复现证据的阻塞裁定。
- 与 T02 在独立 Worktree 并行；共享契约冻结后由主控集成。

## Verification evidence

- 记录构建命令、系统和 SDK 版本、设备、受保护 Task 的真实读取及越权拒绝；失败亦附原始日志位置。
- 产出鸿蒙 native、Android 兼容包和 WebView 的可辨别证据，避免将后两者冒充前者。

## Failure and unknown results

官方 Expo 路径未覆盖鸿蒙时，本 Ticket 必须以真实适配试验决定；没有可维护路径则将 T05 及移动端含鸿蒙的后续 Ticket 标记 blocked，提出架构变更供用户裁定，不擅自宣称三端完成。

## Parallel scheduling

- 理论前沿：G0；可与 T02 iOS／Android、T03 Career/Identity、T04 Artifact、T06 小程序并行。
- 独立 Worktree，鸿蒙构建产物和模拟器资源独占。
