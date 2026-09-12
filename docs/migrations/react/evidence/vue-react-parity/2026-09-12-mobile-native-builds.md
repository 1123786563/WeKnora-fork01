# iOS/Android 原生构建验证（2026-09-12，Round 55）

- `npx expo export --platform ios` → Hermes iOS bundle 3.2MB（dist-ios）。
- `npx expo export --platform android` → Hermes Android bundle 3.3MB（dist-android）。
- 双平台 Metro/Hermes 编译全部成功；typecheck:mobile 已通过（85/85 测试）。
- 限制（如实登记）：Expo 导出为构建级验证，不等于原生功能验收——模拟器/真机交互验收（聊天流、上传、推送等）需 Xcode/Android Studio 环境与设备，当前会话无 GUI 设备接入，留待后续补齐。
