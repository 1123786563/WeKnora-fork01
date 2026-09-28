# 交互面板 parity 增补规格（可点击面 × 弹出面板对齐）

- 日期：2026-09-24
- 状态：进行中
- 基线：docs/specs/2026-09-21-tdesign-react-migration-design.md（已完成，T16 三轮 0.09%）
- 验收标准：沿用同构迁移口径——每个面板扫描项 0.00%（pixdiff 容差 8、1280×720 稳态截图）；豁免须像素级归因+根因代码级定位（台账 #16-#22 判例）

## 目标

1. 盘点双端全部**可点击元素**及其触发的**弹出面板**（dropdown/context menu/popover/dialog/drawer/select popup/date picker/confirm/tooltip 等）。
2. 为每个面板建立扫描项（auto-scan.mjs actions 机制：clickAria/clickCss/clickText + click 序列），逐项收敛 0.00%。
3. 已覆盖的 14 个 ix-* 项不重做；同一面板多处触发只扫一次代表入口。

## 非目标

- 破坏性/写操作流不实测（删除确认类只打开面板不确认；表单不提交）。
- hover tooltip 仅在既有序列工具（稳态门+相位同步）能确定性冻结时纳入。
- 键盘导航深测（另行）。

## 方法

Phase I 盘点：headless 双端爬取（Vue :5174 / React :5175，parity 凭据），按页枚举可点击元素→分类（导航/操作/面板触发器），产出**面板矩阵**（页面×触发器×面板类型×双端存在性）+ 建议扫描项清单（含选择器兜底链）。矩阵入库 docs/migrations/react/evidence/vue-react-parity/panel-matrix/。

Phase II 分批收敛：按页面组分批把扫描项加进 auto-scan.mjs（每批一提交），逐面板收敛 0.00%（playbook SOP+豁免纪律），批完成=批内面板全零。

Phase III 收官：全量（60 静态+全部新交互项）三轮稳定+守护 automation 更新基线。

## 风险

- 面板打开动画相位（沿用 syncAnimPhase/noFreeze 机制）；
- 面板内容数据态依赖（fixture 数据保证双端同态）；
- 触发器选择器双端不一致（迁移后应同构，异构即差异本身）。
