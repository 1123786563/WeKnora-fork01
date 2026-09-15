# R275 仅有子文件夹时保持文档网格（2026-09-15）

Vue 在当前目录没有直接文档但存在子文件夹时仍渲染 `DocumentCardView` 的文件夹卡片。React 原先以 `items.length > 0` 作为唯一网格挂载条件，会错误地进入空状态；现以“文档或当前目录子文件夹任一存在”为条件，保留文件夹进入交互，真正空目录仍显示 Vue 空状态。

验证：
- 文档 page-chrome：13/13
- `pnpm test:web`：906/906，0 失败、0 取消、0 跳过
- `pnpm typecheck:web`：通过
- `pnpm build:web`：通过；仅有既有大 chunk advisory
- `git diff --check`：通过

边界：本项覆盖渲染条件纯函数和组件级回归；真实非空文件夹后端数据、双端截图、响应式及 Wails/native 仍待补齐。
