# R251 — Desktop renderer regression

日期：2026-09-15

## 验证范围

本项检查当前 React UI 共享层在桌面渲染入口的静态契约与测试回归，不替代 Wails 启动、登录、真实后端或逐页视觉验收。

## 结果

- `pnpm test:desktop`: 2/2 passed。
- `pnpm typecheck:desktop`: passed。
- 覆盖桌面 API root 校验、Wails bridge 状态与历史 deep-link 映射。

## 未覆盖

未声称桌面运行时逐页与 Vue 对齐；Wails 实际启动、同条件截图、窗口缩放、键盘/弹层交互和真实后端业务流程仍需单独验证。
