# R272 跨包回归（2026-09-15）

文档文件夹卡片和知识库卡片视觉调整后，跨包回归通过：

- `pnpm test:shared`：469/469
- `pnpm test:mobile`：190/190
- `pnpm test:desktop`：2/2
- `pnpm test:embed`：7/7

共享、移动端、桌面渲染器和 Embed 的测试均为 0 失败；原生编译、安装、启动及真实后端业务链仍需独立验收。
