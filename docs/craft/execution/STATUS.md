# CFT 执行状态（STATUS）

最后更新：2026-09-18 · 维护者：CFT 总控（单写）

状态语义：planned → ready → in_progress → implemented → verified → reviewed → merged；blocked 必须写明原因/影响/解除条件。

## 当前阶段：S00

| 任务 | 标题 | 状态 | 说明 |
|---|---|---|---|
| CFT-S00-T001 | 固定源码与验证入口 | **verified（自审）** | 基线报告 BASELINE.md；test:craft:shared 61、test:shared 573、go craft 103、playwright mock 6/6 全绿；修复 e2e harness 两处回归（recovery worker env、builtin 模型抢占 default）。证据 evidence/CFT-S00-T001/ |
| CFT-S00-T002 | 冻结命令、能力与事件适配合同 | ready | 依赖 T001 已满足；输入：04-API与状态契约.md + command-bridge.types.ts + 现有 api-client/contracts/后端 DTO；已知差异清单在 BASELINE §3 |
| CFT-S00-T003 | 继承品牌并建立Craft作用域令牌 | ready | 缺口：craft.css 86 处类名但色值全部硬编码，无 `.wk-craft` + `--craft-*` 别名 |
| CFT-S00-T004 | 共享组件与Craft宿主壳 | ready | 依赖 T003 |
| CFT-S00-T005 | 能力与发布门禁映射 | ready | 依赖 T002；现有 gate：WEKNORA_CRAFT_ENABLED + WEKNORA_CRAFT_KINDS |
| CFT-S00-T006 | assistant-ui锁版与最小真实挂载 | ready | **明确缺口**：@assistant-ui/react 未安装（W05 以纯 React 模拟同语义）；决策 D001 |
| CFT-S01-T007..T012 | S01 全部 | planned | 依赖 S00 产物 |
| CFT-S02/S03/S04/S05 | 后续阶段 | planned | 按 tasks.json 依赖图 |

## 环境阻塞

无。OpenCode 1.18.4、Playwright chromium、SQLite 全栈 harness 本机可用。

## 下一步

T002（合同冻结）→ T003（令牌）与 T006（assistant-ui）可并行推进（文件不冲突）。
