# D10 修复证据：iOS 跟随系统暗色（T41 #71 follow-up，2026-10-03）

环境：iPhone 18 Pro 模拟器（0A38DB71，iOS 27.0），本地 Release 构建（xcodebuild Release-iphonesimulator，main.jsbundle 内嵌修复代码），UpgradeRequired 面（T39 轮 origin 缓存残留，环境因素非产品回归；根布局修复作用于全部路由面）。luma 口径同 T41：1x1 降采样加权亮度。

## 根因（双色探针实证）

- `src/app/_layout.tsx` 根布局未对任何层设置背景；全部路由 screen（Home/TaskDetail/UpgradeRequired 等）均为无样式 RN 原语。
- **react-native-screens 的原生 screen 容器不透明且默认白底、不随 traitCollection 变化**——盖住根布局任何背景（探针 `contentStyle:{backgroundColor:'#FF0000'}` 全屏生效，luma 50.7，见 d10-diag-contentstyle-probe-red.png；仅给 SafeAreaView 设背景则不可见，luma 仍 ~220 白）。
- Android 之所以跟随：`AppTheme` parent=`Theme.AppCompat.DayNight.NoActionBar`（android/.../values/styles.xml）原生夜间跟随，无需 JS 层。
- 顺带排除：`ios Info.plist UIUserInterfaceStyle=Automatic`（非法值）——装机删除该键后行为不变，非根因，未改动。

## 修复形状

`_layout.tsx`：`useColorScheme()` → `nativeTokens.colors[scheme].bg` 同时挂到根 `SafeAreaView` style 与 Stack `screenOptions.contentStyle`（后者是生效点，前者兜底安全区外露）。dark palette 复用既有 `packages/design-tokens`，无新主题系统。聚焦测试：`apps/mobile/src/app-smoke.test.tsx`（dark/light/null 三态断言 root+screens 背景）。

## luma 判读（appearance light → dark → light 往返）

| 状态 | 修复前（T41 leg4，同面） | 修复后 | 判定 |
|---|---|---|---|
| light | 239 | **243.6**（d10-1） | light 不变（白→token 浅底 #F6F7F3，视觉不可辨） |
| dark | 214（仅系统 chrome 变暗） | **27.7**（d10-2） | 跟随：token 暗底 #111F1B 理论 luma ≈27.7，文字 OCR 全文可读（iOS 默认 label 色随暗色变白） |
| light 恢复 | — | **243.6**（d10-3） | 往返无残留 |

## 未主题化面如实登记

所有路由 screen 本身仍为无样式原语（无 surface/ink/muted 等 token 分层），本轮只把画布级背景接到 token：暗色下文字靠系统默认 label 色（可读）；卡片段落级暗色设计属后续产品工作。Android 侧本修复使其 screen 容器由 AppCompat 夜灰改为 token 暗底（仍跟随）；本会话无 Android 模拟器（adb 不可用），Android 活体复验待补。
