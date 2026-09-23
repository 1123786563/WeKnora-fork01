# TDesign 同构迁移全量验收（T16 终验，2026-09-23）

- 工作树：`.worktrees/tdm-int` @ `feat/tdesign-react-migration` `c3e5eca4f`（T15 删旧栈后 HEAD）
- 环境：Vue `:5174`（frontend/，主仓）+ React `:5175`（本树 apps/web）+ 后端 `:8084`；1280x720，pixdiff 容差 8
- 工件：`auto-scan/<stamp>/`（report.json / report.md / 双端 PNG / diff PNG），本次三轮全量报告位于主仓证据目录

## 1. 三轮全量扫描汇总

| 轮次 | stamp (UTC) | 成功 | >1% | 平均差异 | 非零项 | 与既档豁免值偏差 |
|---|---|---|---|---|---|---|
| R1（T15 后首轮） | 2026-09-23T14-28-11 | 60/60 | 1 | **0.09%** | 9 | 0 |
| R2 | 2026-09-23T14-50-42 | 60/60 | 1 | **0.09%** | 9 | 0 |
| R3 | 2026-09-23T15-00-07 | 60/60 | 1 | **0.09%** | 9 | 0 |

- 三轮 60 项逐页 diff 全部逐位一致（含全部 0% 项）；**零波动项、零异常页、零新增残差**。
- R2 首跑（14-44-17）因 `:5175` dev server 中途掉线仅 9/60，按既定方式重启（本树 `VITE_DEV_PROXY_TARGET=http://localhost:8084 pnpm dev --port 5175 --strictPort`）后重跑为上表 R2；掉线为环境事件，非代码回归，废跑报告保留于 auto-scan 目录备查。
- 51/60 页三轮均 0.00%（含 T15 删除 tailwindcss 管线与 `@weknora/ui` 后全部核心路由、settings 27 分区中 20 个、ix-* 交互项中 13 个）。

## 2. 豁免台账终版（9 项，全部三轮逐位复现）

像素数为 R1 PNG 复算值（`pixdiff.py` 重跑，与轮内值一致）；bbox 与《tdesign-migration-playbook.md》§6 DOM 差异台账逐项吻合。

| # | 扫描项 | diff% | 像素数 | bbox | 根因 | 取证/引用 |
|---|---|---|---|---|---|---|
| 1 | settings-general | 5.439 | 50122 | x357-1116 y149-685 | React-only 套餐与额度卡（SP14 T1 商业功能，Vue GeneralSettings.vue 无此块）：93px 卡体 + 109px 下推传导（末行出视口）；卡外行级取证 selects/previews/labels/radio 逐元素几何全 0 | `981cf5cfe`（迁移提交内置行级取证与豁免声明）；卡片来源 `6a8c8f1a0`；本轮独立复核：React 表单整体下移 109px、卡体 y149-250，与提交记录完全一致 |
| 2 | settings-members | 0.084 | 772 | x797-1085 y304-547 | tdesign-react Input autoWidth 宽度测量取整（offsetWidth+1）→ t-pagination 页大小 select 宽 +1.156px，两表合计 ~772px；库差异无 seam | 台账 #16（task-12a 实证）；`ebb2d12a2`（members 迁移，残余归因）、`21d3b14bc`/`a8c9e4543`（复审修复链） |
| 3 | settings-models | 0.003 | 28 | x1038-1049 y83-94 | 「模型测试」按钮 play-circle 图标描边边缘 AA 权重差（vue 侧恒浅一级）；headed 系统 Chrome 复核 28→8px，真机不可见 | 台账 #16 勘误（T12b 评审 Important-1，元素指认纠正后重取证）；工件 `auto-scan/2026-09-22T10-28-36/headed-modeltest-*.png` |
| 4 | settings-integration-im | 0.002 | 14 | x423-430 y127-130 | chevron-down sprite `<use>` 12px 视口光栅 stroke 亚像素相位 → 左斜边单级灰阶差（219↔204）；headed GPU 光栅化 14→12px 仍在同斜边，真机不可见；无页面 seam | 台账 #18（T12c 实证，S1 评审勘误坐标即 x423-430/y127-130） |
| 5 | settings-integration-embed | 0.002 | 14 | x431-438 y127-130 | 同 #4，同族 chevron 斜边 AA（embed 位坐标 x431-438/y127-130） | 台账 #18（T12c 实证） |
| 6 | ix-kb-settings | 0.003 | 31 | 弹窗四角弧线 | Dialog 圆角 12px 弧线 AA 阶梯逐角错位（引擎栅格伪影）；双端弹窗盒逐边一致、computed 逐属性一致、逐端自比 0px、排除试验（半径 11-13/双弧线/clip-path 不动差值） | 台账 #19（S3 评审 Important-2 独立复算）；`a5da455a9`（kb-settings 迁移 0.216%→0.003%） |
| 7 | login | 0.001 | 5 | x1014-1017 y32-62 | 右上角 4px 宽竖条 AA 残噪（≤1px DOM 对齐后的亚像素尾差）；R492 修复轮已将轮播相位噪声（5.4%）修至 DOM ≤1px | `scan-history.md` 2026-09-21T14-40-48 条目（login/register DOM 对齐 ≤1px）；台账 #21/#22（表单 margin/img 光栅相位修复） |
| 8 | register | 0.001 | 5 | x1014-1017 y32-62 | 同 #7（同源 auth 布局，同 bbox 同像素数） | 同 #7 |
| 9 | redirect-integrations | 0.002 | 14 | x423-430 y127-130 | 重定向落点为 integrations 首屏（im tab），差值即 #4 的 im chevron 斜边 AA 同 bbox 传导——bbox/像素数与 #4 逐位相同即为实证 | 台账 #18 传导（本轮 bbox 逐位比对确认） |

台账口径：#1 为功能性豁免（React-only 商业功能，功能本体保留）；#2 为库级无 seam 差异；#3-#9 为渲染引擎/库光栅化伪影级豁免（真机 GPU 光栅化复核均收敛或不可见），均已在 §6 台账登记且经评审（S1/S3/T12b/T12c）。

## 3. 构建体积对比（vite build，同机同版本 vite 7.3.6 背靠背构建）

口径：基线 = `0603053b2`（T15 前）临时 worktree `/tmp/tdm-base` 构建产物；当前 = 本树 `c3e5eca4f` 产物。均为 `tsc -b && vite build` 全量 dist。

| 维度 | 0603053b2（基线） | c3e5eca4f（当前） | Δ |
|---|---|---|---|
| dist 文件数 | 360 | 360 | 0 |
| dist 总体积 | 14.06 MiB | 14.09 MiB | **+32.5 KiB（+0.23%）** |
| JS 合计 | 11530.6 KiB（222 文件） | 11529.3 KiB（222 文件） | -1.3 KiB（中性） |
| CSS 合计（raw） | 1681.4 KiB（59 文件） | 1715.2 KiB（59 文件） | +33.8 KiB |
| 主 CSS（raw / gzip） | 521.8 / 61.0 KiB | 552.4 / **59.1 KiB** | raw +30.6 KiB；**gzip -1.9 KiB** |
| 主入口 index-*.js（raw / gzip） | 2980.0 / 741.2 KiB | 2980.0 / 741.2 KiB | 0（routes chunk 同为 377,846 字节） |
| 字体/图标/静态资源 | 1206.5 KiB | 1206.5 KiB | 0（逐字节同尺寸） |

要点：
- T15 删除项（`packages/ui`/`@weknora/ui`、tailwindcss 管线）为**已无引用死代码**，JS 体积净中性（主入口与 routes chunk 尺寸不变），证明删除未触及任何生效产物路径。
- CSS raw +33.8 KiB 来自删除 tailwind 工具类管线后页面级语义化 parity 规则（settings.td.css 等平移块）进入主 CSS；**传输口径（gzip）净 -1.9 KiB**，线上体积不劣化。

## 4. 验收结论

**通过（PASS）。**

1. 三轮全量扫描 60/60 成功、平均差异稳定 0.09%、60 项逐页逐位一致，无波动项、无新增残差、无回归。
2. 9 项非零值全部落在既档豁免值上（逐位相等），豁免台账终版每项均有像素数、bbox、根因与取证 commit 引用，与 §6 DOM 差异台账及历轮评审记录完全互证。
3. 构建体积中性（dist +0.23%，主入口 JS 与 gzip CSS 均持平或更小），T15 死代码删除无产物级副作用。
4. 依据 `docs/plans/2026-09-21-tdesign-react-migration.md` §验收（:595）：3 轮 60 项中除台账豁免项外全部 0.00% —— 本轮为终态达成。

过程备注：R2 首跑遇 `:5175` 掉线（环境事件，废跑 14-44-17 保留备查），按既定方式重启后三轮全绿；除此之外无任何阻塞。
