# CFT 证据索引（36 任务全量）

基线：`.worktrees/craft-cft` @ craft/cft-execution · 更新：2026-09-18（T036 收敛时）
每目录含 EVIDENCE.md（commit/命令/退出码/断言对照/未验证项/回退）。

| 任务 | commit | 一句话结论 | 证据目录 |
|---|---|---|---|
| CFT-S00-T001 | cd551bca | 基线报告 + e2e harness 双回归修复（recovery worker/builtin 模型）；全入口实跑 | CFT-S00-T001/ |
| CFT-S00-T002 | e0ddd84b | 命令端口冻结合同（7 测试；D003 裁决） | CFT-S00-T002/ |
| CFT-S00-T003 | d9490802 | .wk-craft 令牌作用域 + 对比度修正（4 测试） | CFT-S00-T003/ |
| CFT-S00-T006 | b4baf8f1 | assistant-ui 0.15.20 锁版真挂载（5 JSDOM 测试） | CFT-S00-T006/ |
| CFT-S00-T005 | 9aa4783f | 能力投影全矩阵 + 服务端 capabilities | CFT-S00-T005/ |
| CFT-S00-T004 | 8f347c7a | shell 组合层（焦点委托共享 Sheet，4 测试） | CFT-S00-T004/ |
| CFT-S01-T008 | efb0e369 | 命令桥（意图级幂等键；修复换键缺口） | CFT-S01-T008/ |
| CFT-S01-T007 | 6f0bbb4b | 重放稳定组合契约 + 工具/交互卡（4 测试） | CFT-S01-T007/ |
| CFT-S01-T009 | d3198f20 | library/templates 新页 + 类型选择器接服务端 gate | CFT-S01-T009/ |
| CFT-S01-T010 | 1a47857d | 对话列切真实 assistant-ui Thread（锚点保留） | CFT-S01-T010/ |
| CFT-S01-T011 | 587dc369 | 版本/来源独立性四断言 + 恢复确认 | CFT-S01-T011/ |
| CFT-S01-T012 | d186b567 | 送达四层状态卡 + 非失败横幅 | CFT-S01-T012/ |
| CFT-S02-T013 | 1646a846 | 协议 pin 3 测试 + CRAFT_LIVE 真实 serve 两轮 PASS | CFT-S02-T013/ |
| CFT-S02-T014 | b0cb7666 | 委派身份幂等/冲突/重启读回 | CFT-S02-T014/ |
| CFT-S02-T015 | b2a3961a | **修复盲重发缺口** + 注入证明 POST 恰 1 次 | CFT-S02-T015/ |
| CFT-S02-T016 | 6c15ace9 | 工作区守卫矩阵（单写者/stale/读不占槽） | CFT-S02-T016/ |
| CFT-S02-T017 | 5864d627 | 资料守卫（跨租户/资源 ACL/注入纯数据） | CFT-S02-T017/ |
| CFT-S02-T018 | e455743f | 预算×计量矩阵（unknown 非 0） | CFT-S02-T018/ |
| CFT-S03-T019 | d22f605a | 发布链（admission 零上传/事务失败留 staging） | CFT-S03-T019/ |
| CFT-S03-T020 | cbde19fd | 预览续期绑定同版本（v2 后 v1 仍 v1） | CFT-S03-T020/ |
| CFT-S03-T021 | ab7e82d0 | 委派归并（failed 无载荷/unknown 不终结） | CFT-S03-T021/ |
| CFT-S03-T022 | 44e72409 | 恢复守卫（缺源拒恢复、下载恒可用） | CFT-S03-T022/ |
| CFT-S03-T023 | f7d34946 | StopStatus 矩阵（受理≠终止）+ 竞态既有 | CFT-S03-T023/ |
| **CFT-S03-T024** | **b95a202d** | **里程碑**：mock 6/6 + real 3/3 + 十反例全映射 | CFT-S03-T024/ |
| CFT-S04-T025 | c26f493c | 文档链 pin（中文/表格/引用 OOXML 读回） | CFT-S04-T025/ |
| CFT-S04-T026 | d8ff8f7b | document 浏览器级 4/4（CRAFT_KINDS 开放实跑） | CFT-S04-T026/ |
| CFT-S04-T027 | fac1f39e | 表格链 pin（真重算/预览=存储/政策隔离） | CFT-S04-T027/ |
| CFT-S04-T028 | 250ead8a | spreadsheet 浏览器级 4/4 | CFT-S04-T028/ |
| CFT-S04-T029 | ec4c92c7 | 演示稿链 pin（页数/溢出拒绝/单页隔离） | CFT-S04-T029/ |
| CFT-S04-T030 | 46c6a198 | slides 浏览器级 4/4（S04 关闭） | CFT-S04-T030/ |
| CFT-S05-T031 | 288e8c8a | 生命周期四断言重执行（保守清理） | CFT-S05-T031/ |
| CFT-S05-T032 | adad3c9f | 审计面（载荷键封闭+文本有界+unknown 可定位） | CFT-S05-T032/ |
| CFT-S05-T033 | 57f80a72 | Gate 矩阵 + 回滚文档 + 商业门禁实跑拒绝 | CFT-S05-T033/ |
| CFT-S05-T034 | b333dbdb | 五宽度视觉基线（11 截图 + 零溢出断言） | CFT-S05-T034/ |
| CFT-S05-T035 | 907c4eab | Mimosa 完整深扫（219 存量/本轮 0）+ 矩阵复跑 | CFT-S05-T035/ |
| CFT-S05-T036 | （本提交） | 证据收敛：本索引 + 旧计划替代标记 | CFT-S05-T036/（本文件） |

## 发布范围与类型 Gate（与证据一致）

- **web**：mock 全量 6/6 + real 模式 3/3（真实 OpenCode 免费模型）+ 十反例映射——证据可支撑"受控试用→生产"的发布评审（生产放行仍需在发布 commit 重新生成 release-evidence，见 release-gates.md）。
- **document / spreadsheet / slides**：模拟环境浏览器级生成→修改→查看→历史下载全通过（各 4/4 spec + 生成链 pin）；生产 KINDS 默认关闭，逐类型开放由 T033 灰度流程决定。

## 未验证项（显式列出，不划完成）

1. real 模式的 03/05/06 spec（UI 语义断言由 mock 全量覆盖——W06 决策沿用，如需可在生产证据轮补跑）。
2. 生产环境灰度演练（真实部署 KINDS 切换）与生产 release-evidence 重生成（HEAD 已偏离，检查器正确拒绝）。
3. Mimosa 219 项存量发现的安全债治理（本轮文件 0 发现；属仓库整体债务，移交后续轮次）。
4. 逐像素高保真 diff（本轮为结构/布局/对比度基线；需固定浏览器/字体环境）。
5. 暗色主题（设计边界外）。
