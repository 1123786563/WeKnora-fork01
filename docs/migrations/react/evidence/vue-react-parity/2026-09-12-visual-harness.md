# Vue/React 截图对比环境（2026-09-12 建立）

- 工具：`playwright-core`（安装于 `.parity-tools/`，已被 gitignore，复用全局 ms-playwright 缓存的 chromium-1243）。
- 启动脚本（后续逐页使用）：`.parity-tools/shoot.js`，参数：URL、输出路径、视口（默认 1440x900，deviceScaleFactor 1）。
- 固定条件：视口 1440x900 / 390x844（移动核对）、DPR 1、语言 zh-CN、等待网络空闲与字体加载后截图。
- Vue 基准 dev server：`cd frontend && npm run dev`（隔离端口）；React Web：`pnpm --filter @weknora/web dev`。
- 排除项规则：仅允许固定时间戳等动态数据；任何排除须在本目录 evidence 中逐项说明。

## shoot.js

见 `.parity-tools/shoot.js`（本轮已创建并冒烟通过 chromium launch）。
