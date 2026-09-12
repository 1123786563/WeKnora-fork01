# /platform/settings live 基线截图（2026-09-12，Round 13）

- 同一真实账号（parity-test@local.dev，owner）、1440x900、zh-CN 下双端对比：
  - Vue（settings-vue-live.png）：全屏抽屉式设置，分组导航（账户/空间/模型/发布集成/数据与扩展，含图标），常规设置为本地偏好（语言/主题/字体/字号），全中文。
  - React（settings-react-live.png）：平台外壳内嵌平铺英文 "Sections" 列表 + 泛化只读 dump，无分组/图标/抽屉形态。
- 与 Round 8 settings 审计结论一致（#14 导航形态、#5 i18n、#1 缺 section、#13 GeneralSettings 功能差异）。
- settings 实施子代理进行中：i18n 键已就绪（1038 键），功能修复优先，视觉形态（抽屉/分组/图标）列为下一迭代。
