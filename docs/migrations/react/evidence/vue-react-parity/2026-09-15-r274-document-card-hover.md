# R274 文档卡片 hover 详情浮层（2026-09-15）

补齐 React 文档网格卡片缺失的 Vue hover 详情层：卡片 hover 300ms 后显示 Portal-like fixed 浮层，优先放在卡片右侧、空间不足时翻到左侧并限制在视口内；离开卡片立即清理延迟任务和浮层。浮层展示标题、失败状态、描述、来源、创建/更新时间、类型、标签及进入详情提示，使用项目 SVG 与共享 i18n。

验证：
- 文档 page-chrome：12/12（含右侧优先、左侧回退和视口 clamp 纯函数边界测试）
- `pnpm test:web`：905/905，0 失败、0 取消、0 跳过
- `pnpm typecheck:web`：通过
- `pnpm build:web`：通过；仅有既有大 chunk advisory
- `git diff --check`：通过

边界：当前为组件级渲染/定时器/定位实现，未在真实非空后端数据下完成 Vue/React 双端截图复核；处理时间线详情、完整卡片操作菜单和响应式/Wails/native 仍待补齐。
