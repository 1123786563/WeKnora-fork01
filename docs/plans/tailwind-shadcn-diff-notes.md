
## 像素对比工具与首批结论

- tailwind-shadcn-diff.mjs（worktree 根）：canvas 像素差 + 差异包围盒，客观量化截图对比。
- 基础设施+组件首批（baseline vs after-foundation，阈值 0.1/255）：
  14/21 路由 0.000% 完全一致；其余 0.28%~2.9% 局部差异全部可归因：
  kb-settings=并行 i18n sweep+截断修复；configuration/administration/kb-wiki/kb-detail/
  kb-graph/settings-members=Button/Input/Switch 既有 utilities 声明开始生效 + i18n 文案。
  无布局/配色意外漂移。
