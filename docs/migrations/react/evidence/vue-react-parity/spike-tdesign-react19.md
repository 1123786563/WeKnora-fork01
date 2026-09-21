# Phase 0 Spike 闸门判定证据 — React 19 × tdesign-react 1.18.3（Task 3）

- 日期：2026-09-21（扫描 stamp `2026-09-21T16:03`）
- 分支：`feat/tdesign-react-migration`（BASE=5c91ef07c）
- 环境：Vue dev :5174 / React dev :5175 / 后端 :8084 全在线；1280×720 headless chromium；pixdiff 容差 8（口径与 spec §8 一致）
- 脚本：`scripts/parity/tmp-spike-tdesign.mjs`（一次性，复用 auto-scan.mjs 的 chromium 解析 / login() / 双 context localStorage 注入；因非 PASS 未删除，供裁决后复跑）
- 产物目录：`docs/migrations/react/evidence/vue-react-parity/spike/`（本文件同级）

## 判定结论：FAIL（按 brief 原始口径）→ 已 BLOCKED 上报，等待 controller 裁决

三项判定标准（plan Task 3 Step 2 / brief）：

| # | 判定项 | 标准 | 实测（raw） | 结论 |
|---|---|---|---|---|
| 1 | 运行时兼容 | Message/Notification 双端弹出、无 React 报错 | 双端全弹出，0 个 React 渲染错误 | **PASS** |
| 2 | 组件像素 | spike-tdesign 静态页差异 < 0.5% | **2.542%**（dialog 打开态 2.914%） | **FAIL** |
| 3 | 图标像素 | data-spike-icons 区域 ≈ 0% | **5.814%** | **FAIL** |

决策树走向：命令式 API **未**触雷（T2 的 `react-19-adapter` 引入已解决，本轮复验 Message/Notification 双端正常）→ 不进入 patch-package / React 18 降级分支；但像素两项 raw 未达标 → 按预案**不清理 spike 现场、不判 PASS**，BLOCKED 上报。

## 关键归因：raw 差异 100% 来自「主题/全局 CSS 上下文缺失」，而非组件库或 React 19 兼容性

Task 3 时点 React 端只引入了 `tdesign-react/dist/tdesign.css`（Task 1），**尚未平移 Vue 端主题 token（Phase 1 Task 4）与 app 级 body 全局样式**。两端 spike 页继承的全局上下文不同：

| 上下文项 | Vue 端（spike 页 computed） | React 端（spike 页 computed） | 像素影响 |
|---|---|---|---|
| 主按钮背景 | `rgb(7,192,95)`（品牌绿 #07c05f，theme.css `--td-brand-color-4`） | `rgb(0,82,217)`（TDesign 默认蓝） | 整块按钮 100% 差异 |
| body 字体栈 | `-apple-system, system-ui, …` 14px | `"PingFang SC", "Microsoft YaHei", "Arial Regular"` 16px（tdesign.css 默认） | 全页文字栅格化不同 |
| body 文字色 | `rgba(0,0,0,0.9)` | `rgb(0,0,0)` | 文字 + 图标 currentColor 逐像素差 ~10% alpha（delta≈25 > 容差 8） |

（React 端 raw computed 值取自独立诊断探针；spike-result.json 内 `contextProbe.react` 一行误在 /login 页采样，仅该行无效，截图与 diff 数字不受影响。`contextProbe.reactNormalized` 证明注入生效：与 Vue 端逐项一致。）

**对照组实验（归因，非判定口径）**：向 React 页注入 `frontend/src/assets/theme/theme.css` + Vue 端 body font/size/color（仅测量用 addStyleTag，未改任何应用代码）后重测：

| 对比项 | raw（判定口径） | normalized（上下文对齐后） |
|---|---|---|
| spike-tdesign 静态页 | 2.542%（23431 px） | **0.000%**（0 px） |
| spike-tdesign-dialog 打开态 | 2.914%（26857 px） | **0.199%**（1836 px） |
| data-spike-icons 区域 | 5.814%（1719 px） | **0.014%**（4 px） |

即：静态页在等主题上下文下**逐像素 0.00%**，证明 tdesign-react 1.18.3 与 tdesign-vue-next 1.20.7 的组件渲染（按钮/输入/选择器/Tabs/Switch/Tooltip/表格/图标）完全同构；raw 2.542% 可 100% 归因于 Task 4（主题 token）+ app 级 body 样式这两块**计划内 Phase 1 工作**。

### 图标项专项结论（spec §7-4：icons-react 0.6.11 vs icons-vue-next 0.4.4）

- 20 个同名图标两端 SVG `outerHTML` 逐字节一致（viewBox/stroke/d 相同），元素矩形逐个相同（首个：24,456.78,20,20，间距 20px 无位移）。
- raw 5.814% 完全由 `currentColor` 继承色差造成（Vue `rgba(0,0,0,0.9)` vs React `rgb(0,0,0)`，笔画像素 delta≈25 全部超容差）；对齐后 **0.014%（4 px 抗锯齿残差）≈ 0%**。
- 图标形状/几何本身无差异 → icons-react 0.6.11 与 Vue 端 icons 0.4.4 同名图标 SVG 同源成立。

### normalized 残差 0.199% 的定位（唯一真实组件差异）

Dialog 打开态残差 = 一个 60×32px 块（x719-778, y299-330）+ 1px：**Dialog footer 取消按钮默认 variant 不同**——Vue `t-dialog__cancel` 为 `t-button--variant-base`（灰底 #e7e7e7），React 为 `t-button--variant-outline`（白底）。属 1.20.7 vs 1.18.3 库版本错位的默认值差异（与 T2 已知 DOM 差异同族，零布局占位），建议进 Task 10 playbook DOM 差异台账，迁移时对 cancel 显式传 variant 补齐。Dialog 几何（400,144,480,220）、header、close 按钮两端一致。

## 运行时兼容明细（判定项 1）

- React 19.3 + `tdesign-react/es/_util/react-19-adapter`（T2 情报①）：MessagePlugin/NotificationPlugin 正常弹出，`message-spike`/`notification-spike` 文本均出现，无 `reactRender is not a function`、无 `ReactDOM.render is no longer supported`、无任何 pageerror。
- Vue 端同项全部通过（NotifyPlugin 导出名差异已由 T2 情报②处理）。
- Dialog 打开态双端 `dialog-content-spike` 可见（vue/react 均 true）。
- 全程 HTTP≥400 仅 `/api/v1/auth/auto-setup` 403，**双端同样出现**（app 启动探测，与 spike 无关，非 React 侧回归）。

## T2 情报引用与验证状态

| T2 情报 | 本轮验证 |
|---|---|
| ① react-19-adapter 必需且 dev 下可用 | 复验成立（生产构建单例问题仍留 Phase 1 重验） |
| ② Vue 端 NotifyPlugin 导出名 / Switch :default-value | 未复现问题，两端行为一致 |
| ③ 库间已知 DOM 差异（Switch 根标签/Table 附加节点/Dialog 关闭态不挂载/Tabs 面板容器） | 全部零视觉占位，normalized 0.000% 静态页证实其不影响像素 |
| Tabs 面板容器差异需重点看间距 | normalized 0.000%，无间距差异 |

## 截图产物（`docs/migrations/react/evidence/vue-react-parity/spike/`）

- `spike-tdesign-{vue,react}.png` + `spike-tdesign-diff.png`（raw 静态 2.542%）
- `spike-tdesign-dialog-{vue,react}.png` + `spike-tdesign-dialog-diff.png`（raw 弹窗 2.914%）
- `spike-icons-{vue,react}.png` + `spike-icons-diff.png`（raw 图标区 5.814%）
- `spike-tdesign-norm-*` / `spike-tdesign-dialog-norm-*` / `spike-icons-norm-*`（对照组 0.000% / 0.199% / 0.014%）
- `spike-result.json`（全量数字：diff_pct/diff_pixels/bbox/bands/hot_cells/运行时探针/console 捕获）

## 裁决请求（controller）

raw 口径 FAIL 的根因是**计划排序**：Task 4（主题 token 平移）在 Phase 1，而像素判定在 Phase 0 末——主按钮绿 vs 蓝 + 全页字体栈 + 图标 currentColor 三项上下文差异在 Task 3 时点结构性无法消除（仅主按钮+危险按钮色块即 >0.5%）。可选走向：

1. 接受 normalized 口径为闸门证据（组件同构 0.000% + 图标 0.014% + 运行时 PASS）→ 判定实质 PASS，执行 Task 3 Step 4 清理，继续 Task 4；
2. 先执行 Task 4（+React 端 body 全局样式对齐）后原样复跑 `node scripts/parity/tmp-spike-tdesign.mjs`（现场已保留，raw 应收敛到 ≈0.2% 以内）再判 PASS；
3. 其他裁决。

本任务未执行任何清理（spike 页/路由/tmp 脚本均保留），未触碰决策树的 patch / React 18 降级分支。
