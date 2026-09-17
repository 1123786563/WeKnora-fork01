# Quiet Workbench · 视觉系统与设计令牌

版本2.0 / 2026-09-17。此为本项目本轮提出的移动产品风格，不宣称是上游品牌官方规范，也不是Happy逐像素复刻。

## 1. 风格原则

暖白的内容底与纯白卡片形成“可阅读的工作空间”；深青绿只用于明确行动与选中状态；柔和浅绿Hero承载任务入口，不使用大面积紫蓝渐变。琥珀表示需要注意的决定，红色保留给危险操作与错误；状态始终带文字/图标，不仅依赖颜色。

版式重心是用户下一步要做什么：第一层任务/审批，第二层执行与成果，第三层管理信息。不要用大段英文装饰抢占移动首屏，不把每张卡都加阴影、每个数字都做渐变，也不靠10px小字塞满信息。

深色主题不是颜色简单反转：背景分为深墨绿与高一层的卡片，主操作改成浅绿色底配深字，保持文字可读和动作层次。设计资产均以CSS/内联SVG实现；不打包任何字体文件。

## 2. 文件与单一事实来源

`tokens/tokens.json` 是令牌源；`tokens/tokens.css` 是Web输出；`tokens/native-tokens.ts` 是原生数值数据输出。JSON采用本地typed schema（$type/$value），**不声称符合某个尚未检验的DTCG版本**。修改源后运行 `python scripts/generate_tokens.py` 与 `python scripts/build.py`，禁止手改编译后的18页各自CSS。

令牌三层：基础色/尺寸 → 语义角色 → 组件映射。当前交付JSON包含语义颜色、spacing、radius、type、size、motion、elevation；组件映射固定在本规范，避免在每个业务页创建新的“特殊绿”。

## 3. 双主题颜色

| 语义令牌 | 浅色 | 深色 |
|---|---|---|
| `canvas` | `#EFF1EB` | `#0D1815` |
| `bg` | `#F6F7F3` | `#111F1B` |
| `surface` | `#FFFFFF` | `#1A2C25` |
| `surface-alt` | `#EEF2ED` | `#21382F` |
| `ink` | `#1B302B` | `#EAF4EF` |
| `muted` | `#60716B` | `#B4C8BE` |
| `subtle` | `#6B7A74` | `#A8BEB2` |
| `line` | `#DFE6DE` | `#365246` |
| `control-line` | `#869890` | `#759A85` |
| `brand` | `#08766A` | `#94E0BA` |
| `brand-hover` | `#065D54` | `#B0EECF` |
| `brand-soft` | `#E1F1E9` | `#244C39` |
| `on-brand` | `#FFFFFF` | `#113525` |
| `accent` | `#DDFAAD` | `#D5F9A9` |
| `hero` | `#DDEDE4` | `#274936` |
| `hero-ink` | `#153D30` | `#EAFAD4` |
| `warning` | `#865200` | `#FFD087` |
| `warning-soft` | `#FFF2D8` | `#47351F` |
| `danger` | `#AF3D32` | `#FFB0A5` |
| `danger-soft` | `#FFF0EC` | `#4A2A25` |
| `info` | `#365B99` | `#ADC9FF` |
| `info-soft` | `#EAF0FC` | `#273B55` |
| `purple` | `#6753A0` | `#D6C3FF` |
| `purple-soft` | `#F0EBFA` | `#3D3151` |
| `disabled` | `#748279` | `#9BB0A3` |
| `disabled-bg` | `#E6EAE4` | `#2A3A31` |
| `focus` | `#08766A` | `#94E0BA` |
| `scrim` | `#102A237A` | `#000000B8` |

### 3.1 颜色使用边界

| 用途 | 颜色对 | 约束 |
|---|---|---|
| 正文 | ink / surface或bg | 最重要内容不可用muted替代 |
| 辅助说明 | muted / surface或bg | 正文错误/权限信息不能降为装饰性浅灰 |
| 主按钮 | on-brand / brand | disabled单独语义，不靠整体opacity降低全部文本 |
| 选中、成功提示 | brand / brand-soft | 同时显示勾选或明确状态词 |
| 需要处理 | warning / warning-soft | 不表示操作必然危险或任务失败 |
| 错误/危险 | danger / danger-soft | 危险按钮需要后果说明与确认 |
| 输入框边界 | control-line / surface | line只作装饰分隔，不能承担唯一控件识别 |
| 焦点 | focus + 双层offset | 不被相邻背景或阴影吞没 |

选定语义对的计算结果在 `verification/token-contrast.json`。普通文字目标4.5:1，大字与必要图形按适用门槛；依据[E03/E04](../references/verified-sources.md)。该报告只是令牌组合检查，不等于完整WCAG认证。不得把四舍五入4.49写成满足4.5。

## 4. 字体与层级

| 角色 | 字号 / 行高 | 字重 | 场景 |
|---|---|---|---|
| caption | 12 / 18 | 400 | 时间、版本、非关键旁注 |
| label | 13 / 20 | 600 | 小标签、字段分组 |
| body-sm | 14 / 22 | 400 | 卡片辅助说明、操作后果 |
| body | 16 / 26 | 400 | 消息正文、输入、主要字段 |
| subtitle | 18 / 26 | 600 | 卡片标题、Sheet小标题 |
| title | 22 / 30 | 650 | 页面标题；RN映射600 |
| display | 28 / 38 | 700 | 首页任务入口，不滥用 |
| metric | 32 / 40 | 700 | 用量摘要，非所有计数 |

字体采用系统无衬线回退。中文优先系统苹方/系统中文字体；浏览器可用Noto Sans CJK回退但不提供字体文件。数字可用tabular-nums；长ID单独等宽并允许断行。原生允许系统字体缩放，正文不设置关闭font scaling。HTML Aa按钮是125%视觉演示，原生200%动态字体另验。

## 5. 间距、尺寸与层次

基准画板390×844；验证320–1440宽度的重排。移动内容边距20，窄屏按布局规则缩到16而不是缩字。间距尺度0/2/4/6/8/12/16/20/24/28/32/40/48/64。卡片内部16–20，章节间24，图标文字间8–12。

主要触控目标48，图标44，按钮50，输入框最小48。圆角：xs6、sm10、control12、card20、hero24、sheet28、pill999。Sheet头部圆角大于卡片，文档正文不加圆角装饰。

阴影仅用于浮层与桌面预览手机边界，普通列表靠颜色与间距区分。JSON的x/y/blur/opacity是设计参数；RN映射为平台支持的shadow/elevation，不直接传Web box-shadow字符串。Safe Area与键盘高度来自系统；图稿中的顶部状态栏和底部home indicator是演示装饰，原生不得重复绘制两份系统状态栏。

## 6. 动效与反馈

120ms用于按压与细微反馈，180ms主题/内容过渡，240ms用于Sheet。reduced-motion时为0并保留静态反馈；流式文本不逐字过度动画。骨架不带虚假真实进度，运行圆点/波形不能让用户以为已经有真实网络或录音。

操作即时呈现busy并禁止重复提交；成功文案根据所获得事实：“决定已记录”“停止请求已提交”“原请求已找到”，不泛化为“任务完成”。Toast不承担唯一错误信息，字段错误与页面错误有持久位置。

## 7. 组件规格

| 组件 | 变体 / 关键props | 必须验证 |
|---|---|---|
| Button | primary / secondary / danger / ghost；busy、disabled、icon、label | 文字完整、44/48触控、键盘焦点、busy防重复 |
| FormField | label、hint、error、required、value、onChange | label与输入关联；错误用文字；密码不缓存 |
| Sheet | title、description、frozenObject、actions、onDismiss | 焦点trap、背景inert、返回不执行、scope变化失效 |
| StateBadge | run/interaction/settlement分域；label、tone、icon | 不把三个状态压成一个绿点 |
| CapabilityReason | supported / unavailable / forbidden、reason | 不可用必有理由，无默认允许 |
| TaskHero | title、helper、primaryAction | 全页只有一个最强主操作 |
| RunCard | title、agent、status、asOf、onOpen | stale与unknown可分辨，不暴露越权摘要 |
| Message | stableId、role、parts、streaming | 未知part安全回退；不能渲染未经处理脚本 |
| ResourcePill | ref、name、status、onRemove | 附件扫描中不可提交；名字长可截但可查看全文 |
| BottomTabs | selected、badge、onNavigate | 4入口不动态变换顺序；屏幕阅读器有选中态 |
| RiskNotice | severity、impact、target | 后果清晰，不用颜色代替说明 |
| EmptyState | title、description、action | 当前scope空，不借用其他空间数据 |

建议组件路径 `apps/mobile/sources/weknora/ui/*`，外层屏幕目录按任务索引落地。原生优先复用已有可解耦Happy组件，先套语义props，不重建一整套第三方UI依赖。

## 8. React Native映射例

以下为集成形状示例，组件代码须接入实际Unistyles版本与已有工程，不能把它当本轮已编译原生组件：

```tsx
const theme = resolveNativeTheme(mode);
const colors = theme.colors;
const buttonStyle = {
  minHeight: theme.size['button-height'],
  borderRadius: theme.radius.control,
  backgroundColor: colors.brand,
  paddingHorizontal: theme.spacing['20'],
};
// Native Text使用colors['on-brand']与16号正文；不导入HTML/CSS。
// 应用入口注册主题，Screen通过已有主题hook消费；不要每屏监听系统主题。
```

状态颜色由业务状态投影得到tone，再由主题解析颜色；严禁在API DTO放hex颜色。RN shadows、字重650→600、字体scaling和hitSlop属于适配层，不污染共享纯业务包。所有真实原生可访问性和动态字体门槛由MX-035验证。

## 9. 视觉交付验收

视觉目标以 `index.html` 和18个逐页HTML为主，截图是对应状态的冻结参考。开发截图比较须固定设备尺寸、字体、主题、语言与mock fixture；变化需记录原因。允许平台原生输入/权限弹窗的合理差异，不要求复制浏览器评审侧栏。

高保真定义：完整视觉层级、双主题、文本/图标/间距、可点主流程和可复现异常状态。它不等于所有后台接口已经联调，也不等于全部Happy高级功能已经在18页里实现。
