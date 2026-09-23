# ADR-0018：求职移动端使用 Expo，小程序使用 TDesign Miniprogram

状态：已决定（用户于 2026-09-24 修订）。适用范围：WeKnora 求职专业 Agent；Web 沿用 React/TDesign。

## 决定

移动 App 沿用现有 Expo／React Native 工程，iOS 与 Android 使用原生页面。移动界面的颜色、层级、组件状态和交互参照 TDesign Mobile React；由于该项目是移动 Web 的 React 组件库，原生页面使用 React Native 组件映射设计 token，不直接依赖其 DOM／CSS 组件。微信小程序沿用现有 Taro 4 工程，并通过页面 usingComponents 接入 TDesign Miniprogram 原生小程序组件。该小程序页面仅承诺 Weapp 目标；组件与 Taro 4 当前构建链的兼容性须在首个小程序 Ticket 中实际验证。业务状态和服务接口仍由 Career Desk 与 Career Office 共享，文件、分享、通知、存储和导航按平台适配。

鸿蒙保留为正式交付目标，但 Expo 官方文档只明确 iOS、Android、Web。不得把 Expo 已支持 iOS／Android 推断为可直接构建鸿蒙原生 App。先单列兼容性闸口，验证可维护的 Expo／React Native 鸿蒙原生适配、真实运行包、认证 Task 读取和端能力。若无可行路径，受影响 Ticket 阻塞并带证据提交技术裁定；Android 兼容包、WebView 不算鸿蒙原生交付。

## 依据

- [Expo 官方文档](https://docs.expo.dev/index.html)列出的通用目标为 Android、iOS、Web；[额外平台支持](https://docs.expo.dev/modules/additional-platform-support/)说明 Expo Modules 首要支持 iOS 和 Android。
- [TDesign Mobile React](https://github.com/Tencent/tdesign-mobile-react)自述定位为 React 18 移动 Web 应用组件库。
- [TDesign Miniprogram](https://github.com/Tencent/tdesign-miniprogram)是微信小程序组件库；[Taro 原生模块文档](https://docs.taro.zone/docs/hybrid)说明可通过 usingComponents 引用原生组件，并提示这种页面不能自动跨端转换。当前仓库小程序已使用 Taro 4.2.1。
- 用户明确修订此前 Taro 4 四目标方案，以本 ADR 和同步后的批准 Spec 为准。

## 后果

1. Ticket DAG 将 Expo iOS／Android 受认证 Task 薄切片与鸿蒙原生可行性闸口拆成可并行的前置 Ticket；小程序文件和组件能力另行推进。
2. Expo 原生视觉适配与小程序 TDesign Miniprogram 是两个独立 UI 实现边界，共享业务合同但不共享具体组件源码。
3. Web、iOS、Android、鸿蒙原生、小程序五环境分别验证认证、文件、分享、通知、缓存与完整求职流程；鸿蒙闸口失败时不得声称通过。
