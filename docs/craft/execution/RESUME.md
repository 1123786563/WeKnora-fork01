# CFT 续跑入口（RESUME）

最后更新：2026-09-18（T024 完成后；累计 verified 24/36：S00-S03 全闭，首个产品里程碑达成）

## 已完成（verified，自审；证据目录 docs/craft/evidence/<task-id>/）

| 任务 | commit | 核心交付 |
|---|---|---|
| CFT-S00-T001 | cd551bca | 基线 + e2e harness 两处回归修复；全入口实跑证据 |
| CFT-S00-T002 | e0ddd84b | contracts command-ports 冻结合同（D003 裁决） |
| CFT-S00-T003 | d9490802 | .wk-craft 令牌作用域 + 对比度修正 |
| CFT-S00-T006 | b4baf8f1 | @assistant-ui/react 0.15.20 真挂载 |
| CFT-S00-T005 | 9aa4783f | 能力投影全矩阵 + 服务端 capabilities |
| CFT-S00-T004 | 8f347c7a | shell 组合层（焦点委托共享 Sheet） |
| CFT-S01-T008 | efb0e369 | core command-bridge（顺序所有者 + 意图级幂等键） |
| CFT-S01-T007 | 6f0bbb4b | 重放稳定组合契约 + thread.tsx 工具/交互卡 |
| CFT-S01-T009 | d3198f20 | library/templates 新页 + kind 选择器接服务端能力 |
| CFT-S01-T010 | 1a47857d | 对话列切真实 assistant-ui Thread（composer=false 保留锚点）；spec05 定位器更新 |
| CFT-S01-T011 | 587dc369 | domain version-selection 四断言 + versions.tsx 恢复确认 + sources 撤权占位 |
| CFT-S01-T012 | d186b567 | CraftDecisionCard 四层送达状态 + 终态零控件 + StatusNotice 非失败横幅 |
| CFT-S02-T013 | 1646a846 | 协议 pin 3 测试 + CRAFT_LIVE=1 真实 serve 两轮 PASS |
| CFT-S02-T014 | b0cb7666 | TestDelegationIdentity*（幂等/冲突/重启读回） |
| CFT-S02-T015 | b2a3961a | **修复真实缺口**：预检读失败盲重发 prompt → 拒绝重提交转 unknown；3 注入测试 + live 50.8s 复验 |
| CFT-S02-T016 | 6c15ace9 | TestCraftWorkspaceGuard* 四断言矩阵（单写者/stale 零写入/stale fence 全链拒绝/读不占写槽） |
| CFT-S02-T017 | 5864d627 | TestCraftSourceGuard*（跨租户拒绝/共享按资源 ACL/注入文档纯数据结构白名单） |
| CFT-S02-T018 | e455743f | TestCraftExecutionBudget*（拒绝零请求/重放去重/重试如实/unknown 计数非 0） |
| CFT-S03-T019 | d22f605a | 发布链 pin（admission 失败零上传零发布/事务失败留 staging 同内容重试恢复） |
| CFT-S03-T020 | cbde19fd | 预览续期策略 pin（v2 发布后 v1 重取票仍绑定 v1）+ 修复 T009 漏跑断言 |
| CFT-S03-T021 | ab7e82d0 | TestCraftDelegateMerge*（failed 定案无载荷/unknown 不持久化/succeeded 只携事实） |
| CFT-S03-T022 | 44e72409 | 缺源拒绝恢复+下载仍可用 pin（其余三断言既有 C05 套件） |
| CFT-S03-T023 | f7d34946 | StopStatus 五态矩阵（受理≠终止）；410/SIGKILL 双窗/stale 冲突既有 |
| **CFT-S03-T024** | **b95a202d** | **里程碑**：mock 6/6 + real 3/3（真实免费模型 2.1m）；十反例全映射已执行测试 |

测试基线：test:craft:shared **108 pass**；playwright mock **6/6**；go craft 103 + handler/service 全绿；typecheck:shared/web 通过。

## 进行中

- 无。

## S04 就绪态（T025-T030，Office 三类型；按 S02/S03 差距核实模式）

- 后端生成器已有（internal/craft 的 document/spreadsheet/slides 各 11/15/11 测试 + manifest 验证器 + T019 admission pin）；T025-T030 按任务卡逐项核对 + pin；浏览器级"生成→修改→查看→历史下载"验收需全栈 harness 开 WEKNORA_CRAFT_KINDS 对应类型再跑。
- Gate 现状：默认只开 web；document/spreadsheet/slides 关闭中（任务验收过了才放开，不谎称 36/36）。
- **环境注意**：本机今天多起外部清理（playwright 缓存、nginx 二进制）。e2e 前检查 `nginx -v` 与 `ls ~/Library/Caches/ms-playwright/`；playwright 重装用 `PLAYWRIGHT_DOWNLOAD_HOST=https://cdn.npmmirror.com/binaries/playwright pnpm --filter @weknora/web exec playwright install chromium-headless-shell`。

## S05 提示（T031-T036）

- 生命周期/审计/灰度回滚/视觉五宽度基线（1440/1280/1024/768/390）/安全故障注入总回归/证据收敛；T035 跑 Mimosa 完整扫描（hook 持续 scanner_enobufs）。
- 测试基线：test:craft:shared 108；go craft 103+ 全绿；e2e mock 6/6 + real 3/3；live serve 两轮绿。

## 未提交改动 / 锁 / 进程

- 工作树干净；无活跃 stack（均已 teardown）；台账锁总控。主工作区（main）其他会话改动不要动。

## 下一条可执行命令

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/craft-cft
# S04 从 T025 开始：读 docs/design/weknora-craft-hifi/docs/tasks/CFT-S04-T025.md，
# 核对 internal/craft/document.go + skills 清单（docs/testing/craft 与 W01/W06 报告），
# 按 S02/S03 模式 pin；验证基线: pnpm run test:craft:shared（108）+ go craft 全绿
```

## 缺失环境 / 待验证

- 无缺失（本机环境修复记录见 evidence/CFT-S03-T024 §三）。待验证：Office 三类型浏览器级验收（S04）、视觉五宽度（T034）、Mimosa 完整扫描（T035；hook 一直报 scanner_enobufs 未出完整结论，本轮未宣称安全）。
