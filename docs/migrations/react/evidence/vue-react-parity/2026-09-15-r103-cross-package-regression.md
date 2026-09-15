# r103 跨包回归门禁

在 `codex/react-multiclient` 当前提交上并行运行：

- Shared：467/467
- Web：895/895
- Mobile：189/189
- Embed：7/7
- Desktop：2/2

所有测试进程退出码为 0。此前已独立验证 Web/Mobile/Embed/Desktop 类型检查与构建门禁；本轮未出现由 Vue 设置挂载修复或 iOS 验收准备引起的跨包回归。
