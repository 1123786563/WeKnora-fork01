# vue-react-parity Round 1 基线证据（2026-09-12）

- 分支 `codex/react-multiclient`，HEAD `b2d0cf6`。
- 静态检查：`pnpm run test:shared` 224/224 通过；`pnpm run test:web` 113/113 通过。
- 工作区保留既有未提交修改：apps/mobile/expo-env.d.ts、docs/migrations/react/{reuse-manifest.csv,runtime-baseline.md,version-matrix.md}、docs/superpowers/* 新文件。
- 建立文档：docs/migrations/react/vue-react-parity-matrix.md、vue-react-parity-progress.md。
- 本轮开始逐页闭环，首两页：/login(+/register、/onboarding/workspace) 与 /platform/knowledge-bases；差异分析报告见后续 evidence 文件。
