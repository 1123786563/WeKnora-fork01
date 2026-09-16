# React 前端 Code Agent 开发约束

适用于 React Web、Desktop renderer、Embed 及其共享包的开发与审查。Mobile 仅适用共享契约、领域逻辑和平台边界要求；React Native 保持原生 UI。任务明确要求的行为或设计变更优先于本文的保留基线要求。

开发流程见第 1–7 节；接入 CLI/Linter 或修改质量门禁时另读第 8 节。第 8 节是待实施方案，不能据此宣称工具已安装或检查已通过。

## 1. 开始前确定范围

1. 确认当前工作目录、分支和未提交修改，保护已有工作；不要默认切换到历史 React worktree。
2. 阅读根 `AGENTS.md`、`CONTEXT.md`、相关 ADR，以及任务关联的 GitHub Issue。Issue 操作遵循 `docs/agents/issue-tracker.md`。
3. 阅读目标页面、调用链、实际组件 API、主题入口和相关测试。样式迁移另读 `docs/plans/tailwind-shadcn-conventions.md` 与 `docs/plans/tailwind-shadcn-migration.md`，并核对它们的适用范围和实现假设。
4. 动手前列出：允许修改的文件或模块、复用组件、预期行为变化、受影响入口、验收场景。范围清楚后继续执行，常规实现选择无需重复确认。
5. 代码是当前实现证据，Issue 和已确认设计定义目标。发现冲突时说明差异；不要把历史描述当作当前实现，也不要以现有代码覆盖明确需求。

完成条件：本次范围、当前实现和目标行为已经对应到具体文件与验收项。

## 2. 目录职责与依赖边界

| 位置 | 职责 |
| --- | --- |
| `apps/web/src/main.tsx`、`routes.tsx` | 启动、页面装配、路由匹配、守卫和重定向 |
| `apps/web/src/platform/` | 应用外壳、会话与租户作用域、浏览器平台适配 |
| `apps/web/src/<业务目录>/` | Web 业务页面、专用组件、Hook 和局部逻辑 |
| `apps/desktop/` | Wails 平台适配与 renderer 入口 |
| `apps/embed/` | 独立嵌入入口与宿主交互 |
| `apps/mobile/` | React Native 客户端与原生交互 |
| `packages/contracts/` | 共享数据和接口契约 |
| `packages/api-client/` | API 访问、传输和错误处理 |
| `packages/domain/` | 纯业务规则、作用域、校验与状态计算 |
| `packages/core/` | 已有业务控制器；新增能力先核对其职责，避免成为杂物包 |
| `packages/design-tokens/` | 共享设计令牌 |
| `packages/ui/` | 自有基础 UI 控件、变体和主题映射 |
| `packages/views/` | 具有实际共享需求的业务视图 |
| `packages/i18n/` | 国际化资源与运行时能力 |

- 延续 React、TypeScript、Vite、Tailwind 4 和 `@weknora/ui`。版本、脚本、别名和导出以当前配置为准。
- 页面优先复用 API 客户端和平台适配层。纯领域逻辑保持框架无关；共享包不得反向依赖 `apps`。
- Web 专用内容留在业务目录，有实际复用需求再移入 `packages/views`，避免为假想复用提前抽象。
- Desktop 当前入口复用 Web；Web 页面、样式及平台调用改动需检查 Desktop 影响。共享视图改动还需检查 Embed 消费者。
- React Native 不引用 DOM UI、Web Tailwind 样式或浏览器专用依赖。
- 新增跨包 API 时同步检查 `package.json` exports、TypeScript 和受影响入口的构建解析。引用已公开的包路径，避免跨包相对路径绕过边界。
- 框架替换、路由迁移、全局状态体系或依赖升级属于独立架构变更，不能附带在单页任务中实施。

## 3. 页面与 React 状态组织

- Page 负责路由参数、权限接入和页面组合；独立交互区域提取为业务组件。
- 请求生命周期、订阅和异步操作按职责提取专用 Hook 或 controller；转换、校验和状态计算提取为纯函数。
- 拆分以职责和状态所有权为依据。避免继续向大型页面堆积功能，也避免按机械行数拆出无意义文件。
- 区分服务端数据、表单草稿、URL 状态和局部 UI 状态，每种数据保留明确事实来源。
- render 可计算的派生值直接计算；Effect 用于外部同步，并清理请求、订阅、计时器和事件监听。
- 服务端状态或复杂表单库先核对当前依赖；确需新增时解释适用范围和迁移成本，先在一个模块验证。
- 列表使用稳定业务标识作为 key；保持输入控件受控方式一致，避免组件重建导致输入或焦点丢失。

## 4. 组件与 Tailwind 约束

- 基础控件统一通过 `@weknora/ui` 使用。先检查已有控件，再在 UI 包补充能力；业务页面不复制第二套基础组件。
- 以仓库实际 Props 和导出为准，不能假设等同官方 shadcn API。修改公共 API 时检查所有消费者，保持兼容或完成明确的迁移。
- 新组件使用项目现有 `cn`、CVA 和适用的无头组件能力，并适配现有主题、交互和可访问性契约。
- 使用 shadcn CLI 前检查相关工作区的 `components.json`、别名、样式入口和生成落点，审查生成差异；不得通过默认初始化覆盖现有配置或控件。
- 通用布局优先 Tailwind，通用颜色、字体、圆角优先语义令牌。维护 `design-tokens`、`packages/ui/src/theme.css` 与既有变量之间的映射。
- 保留用户字体、主题属性和局部主题变量。新增主题能力沿用现有机制，不另建平行体系。
- Web 当前刻意未启用 Preflight；启用 reset 必须作为明确变更评估，并提供页面回归证据。
- 检查 Tailwind 源码扫描覆盖实际消费者；新增共享源码位置后验证生产 CSS 中所需 utilities 已生成。
- 动态样式使用完整类名映射，禁止拼接 `bg-${color}-500` 等不可静态识别的类名；运行时连续值可使用受控 CSS 变量。
- 精确对齐允许必要的任意值；复杂动画、富文本和第三方样式允许保留 CSS，并说明原因。
- 排查 cascade layer、选择器优先级和加载顺序后再修复样式冲突，避免新增大量 `!important` 掩盖问题。
- 删除类名前搜索事件逻辑、测试、父级选择器和其他消费者。DOM 钩子可保留，即使其样式已迁移到 utilities。
- 共享样式仅在最后一个消费者迁移且回归通过后删除；任务外的全局样式清理单独处理。

## 5. 视觉与业务契约

- 未明确要求设计变更时，保持页面布局、品牌色、字体层级、信息架构、路由、业务逻辑和全部功能。
- Vue 对齐任务以指定 Vue 页面、状态和交互为基线。shadcn/ui 是实现基础，默认外观不构成项目视觉规范。
- 新增文案走现有 i18n 机制；检查长译文、长标题及中英文输入。
- 保持 API 契约、校验时机、错误语义、上传、流式恢复、工具审批和提交去重行为。
- 请求处理取消、过期响应和失败；重试写操作前确认幂等性，不能无条件自动重试外部副作用。
- 缓存键与异步结果绑定现有用户/租户 scope。切租户、退出或权限变化后，旧请求不得回写新作用域，旧数据不得继续展示。
- 权限 UI 与操作路径同时处理授权状态及后端拒绝；隐藏按钮或 CSS 控制不能代替后端授权。
- 凭据通过现有凭据适配层处理，不写入 URL、日志或新增的普通持久化状态。

## 6. 验收与交付

按实际受影响功能选择场景；每个不适用或受阻项说明原因。

| 范围 | 必须检查的场景 |
| --- | --- |
| 数据页面 | 加载、空数据、失败、无权限、成功、禁用和提交中 |
| 表单 | 校验时机、重复提交、失败保留输入、成功反馈 |
| 弹层 | 焦点进入与恢复、Tab、Esc、遮罩关闭、嵌套层级和 Portal 样式作用域 |
| 布局 | 长文本、滚动容器、窄屏、主题、字体与键盘操作 |
| 路由 | 直达、刷新、前进后退、重定向和权限守卫 |
| 异步流程 | 卸载、切换资源或租户后的过期响应，以及任务相关的断流恢复 |

1. 从当前根与包内 `package.json` 选择已有命令，执行受影响模块测试、Web 类型检查和构建。只修改文档时检查差异、路径和规则一致性，无需执行应用测试。
2. UI/views 或共享逻辑变更追加相关共享测试；检查消费者，补充受影响 Desktop、Embed、Mobile 的对应门禁。
3. 视觉与交互变更提供浏览器证据；对齐任务在相同视口、数据、主题与页面状态下对照。
4. 测试验证可观察行为，不通过降低断言或删除失败测试制造通过。选择器调整保持断言语义。
5. 分开报告静态检查、单元测试、浏览器、真实后端和原生端证据。构建通过不等于页面或多端验收通过。
6. 环境不可用时记录 `blocked-env`、受阻场景和恢复条件，继续独立可完成的工作；mock 结果不能充当真实集成证明。
7. 交付包含改动文件与原因、实际验证结果、未验证项和残余风险。所有任务要求均有对应证据或明确阻塞状态后，才能作出相应完成声明。

## 7. 历史文档使用注意

历史迁移手册可能包含旧 worktree 路径、旧组件 API 或旧实现决定。尤其需要读取当前 Desktop 入口和 Dialog 源码：Desktop 的 Web 复用关系、Dialog 的 Portal 与焦点行为均应从当前实现核对。发现过时描述时明确标注，任务涉及该约定时同步修正适用范围，避免继续传播旧假设。

## 8. CLI、Linter 与 CI 门禁方案

### 8.1 状态与选择

2026-09-16 静态检查：Web 已配置 TypeScript strict，根脚本有类型检查、测试及构建入口；尚未发现根目录与 Web 的统一 ESLint 门禁。本节仅记录方案，未安装工具、创建 lint 配置或运行门禁。执行接入任务时重新检查 manifests、lockfile 和 CI，并按实际结果更新此段。

优先采用 ESLint 作为统一静态检查入口，将 `@shadcn/lint`、React Hooks、typescript-eslint 和 JSX a11y 放入同一套 flat config；依赖图与浏览器验证保持独立命令。比并行维护多套重复 lint 更便于定位规则和管理例外。

`@shadcn/lint` 支持 Tailwind v4 和自有组件，可用于 `@weknora/ui`。Oxlint 也是替代方案，但官方当前注明其 JS 插件 API 处于 alpha；若后续 lint 性能成为瓶颈，先验证所需规则兼容性，再决定迁移。接入时固定相容版本，并核对 Node、TypeScript、ESLint 与插件的版本要求。[官方接入说明](https://github.com/shadcn-ui/lint#get-started)

### 8.2 检查分层

| 层 | 工具 | 强制目标与边界 |
| --- | --- | --- |
| 类型 | TypeScript `tsc` | Props、接口、导出与类型兼容；使用实际包配置，检查共享包是否完整纳入覆盖 |
| React | `eslint-plugin-react-hooks` | Hook 调用与依赖规则；逐项校准规则，不为消除提示盲目添加 Effect 依赖 |
| 异步与类型安全 | `typescript-eslint` | 优先评估 `no-floating-promises`、`no-misused-promises`；类型感知规则配置对应 TS 项目 |
| 设计系统 | `@shadcn/lint` | 组件样式契约、主题颜色和 Tailwind 类检查；按下一节分级 |
| 静态可访问性 | `eslint-plugin-jsx-a11y` | 标签、ARIA 和交互语义；为自有控件配置必要映射 |
| 架构 | `dependency-cruiser` | 禁止反向依赖、平台污染和循环依赖；按实际 TS 路径、包 exports 配置解析 |
| 行为 | 现有测试框架 | 异步竞争、租户切换、校验、权限拒绝、流式状态和提交去重 |
| 浏览器 | Playwright + axe | 焦点、键盘、弹层、真实布局和部分可访问性；视觉对齐另做截图对照 |

官方资料：[React Hooks](https://react.dev/reference/eslint-plugin-react-hooks)、[异步规则](https://typescript-eslint.io/rules/no-floating-promises/)、[JSX a11y](https://github.com/jsx-eslint/eslint-plugin-jsx-a11y)、[依赖检查](https://github.com/sverweij/dependency-cruiser)、[浏览器可访问性](https://playwright.dev/docs/accessibility-testing)。

### 8.3 shadcn 规则策略

| 规则 | 目标政策 |
| --- | --- |
| `shadcn/no-restyle` | UI 消费端按组件定义契约；Button 的内部间距、颜色和形状由 variant/size 管理，页面可控制约定的外部布局 |
| `shadcn/no-raw-colors` | 已治理业务目录设为 error；设计令牌定义及有依据的视觉例外单独处理 |
| `shadcn/no-unknown-classes` | 主题解析和外部类清单验证后设为 error；UI 实现目录也保留 |
| `shadcn/require-static-classes` | 已治理消费端设为 error；UI 实现目录关闭此规则，允许调用自有 CVA 变体函数 |
| `shadcn/no-arbitrary-values` | 初期关闭或限域试行；保留必要精确值，不为通过检查改变视觉基线 |
| `shadcn/no-inline-styles` | 限域试行；为运行时定位、图表尺寸和 CSS 变量保留明确例外 |

以上规则均需经试运行后生效，不直接启用全部严格策略。[规则与渐进接入](https://github.com/shadcn-ui/lint/blob/main/docs/adoption.md)

识别自有组件可使用如下配置片段。它不是完整可执行配置；接入时仍需补齐 parser、files、主题发现和 overrides。

```js
settings: {
  shadcn: {
    componentImports: ["^@weknora/ui(/|$)"],
    note: "遵循 frontend.md；公共组件变体在 packages/ui 中维护。",
  },
}
```

配置中的组件契约使用仓库实际组件名与 Props。`packages/ui/src/**` 作为实现区，不沿用消费端的 `no-restyle` 禁令；仍检查其中的类名和适用的 token 规则。[自有组件配置](https://github.com/shadcn-ui/lint#settings)

需要显式验证的限制：

- `require-static-classes` 只覆盖识别到的组件和转发包装组件，不覆盖全部原生 JSX 元素；全面检查动态 Tailwind 拼接需要额外 AST 规则。它不能解析的合法导入常量也可能被报告，先检查原因再决定例外。[规则边界](https://github.com/shadcn-ui/lint/blob/main/docs/rules/require-static-classes.md)
- `no-unknown-classes` 的主题加载失败会导致降级检查。门禁必须把主题解析失败视为未完成检查；先用合法语义类和故意拼错的类验证识别能力。外部样式与无样式 DOM 钩子采用精确清单和用途说明，避免放行整个前缀。类合法也不证明生产扫描、层叠或显示位置正确。[主题与类检查](https://github.com/shadcn-ui/lint/blob/main/docs/rules/no-unknown-classes.md)
- 任意值替换建议可能是近似颜色或字号，不代表视觉等价；保留主题变量语义和必要精确值。[任意值规则](https://github.com/shadcn-ui/lint/blob/main/docs/rules/no-arbitrary-values.md)

### 8.4 仓库专用硬规则

依赖图规则优先于字符串扫描：解析实际导入目标，才能覆盖相对路径和别名绕过。

- `packages/**` 不依赖 `apps/**`。
- `packages/domain/**` 不依赖 React、React DOM、UI 或 views；浏览器全局变量另用 ESLint 限制。
- `packages/ui/**` 不依赖业务页面、业务 API 客户端或平台应用。
- `apps/mobile/**` 不依赖 Web/Embed/Desktop 源码和 DOM UI 包。
- Desktop 到 Web 的既有入口复用作为明确允许边，不误判为所有 app 间引用都合法。
- 业务页面直接网络调用与凭据存取按目录限制；允许的 API、认证和平台适配位置先从当前调用链确定。
- 公共包新增循环依赖应失败；历史环路单独记录和治理。

这些规则需在配置中实现后才构成硬门禁。新增自定义规则或架构配置要有最小正反例：合法导入通过、违规导入失败、别名和相对路径均可识别。不要用只验证实现字符串存在的测试替代规则行为测试。

### 8.5 历史问题与例外管理

采用“全范围扫描 + 逐目录收紧”，避免只 lint 修改行：共享主题、导出和配置变化可能影响未修改的消费者。

1. 首次扫描输出报告，区分真实问题、误报、必要例外和历史债务。扫描任务不自动重写整个仓库。
2. 已治理目录设为 error 并要求零违规；未治理目录保留有边界的债务记录，新增目录默认严格。
3. ESLint bulk suppressions 可作为过渡机制，但不能把按文件/规则的数量抑制等同于逐条违规身份追踪；减少旧问题与引入新问题可能抵消数量。需要严格禁止新增违规时，对修改文件执行不抑制检查，或增加稳定诊断指纹比较。
4. 普通业务任务不得通过重新生成全量基线、扩大忽略目录、降低级别或加入文件级 disable 获得通过。必要例外记录规则、文件或组件、原因及复查条件。
5. CI 不自动生成 suppression；清理债务时缩减抑制，并检查无用禁用指令。lint 配置、基线和 CI 的变更单独说明，审查它们是否降低覆盖。

ESLint 提供已有 error 的批量抑制和清理机制；warn 不参与该抑制。是否采用以及如何防止数量抵消，须在接入时验证。[官方 suppression 文档](https://eslint.org/docs/latest/use/suppressions)

### 8.6 统一命令与 CI

以下是建议脚本名，当前不可假设存在；接入后以 `package.json` 为唯一命令来源。

| 建议入口 | 责任 |
| --- | --- |
| `lint:frontend` | ESLint 检查 Web、Desktop、Embed 及相关共享包；明确排除生成物和 vendor，RN 使用独立配置 |
| `lint:architecture` | 依赖图边界与循环检查 |
| `check:frontend` | 聚合静态门禁、受影响类型检查、测试和构建，任一失败返回非零退出码 |
| `test:frontend:e2e` | 需要浏览器环境的关键交互和可访问性检查，独立报告 |

- 本地与 CI 使用相同脚本、固定依赖和 lockfile；CI 不执行自动修复。
- 硬门禁失败阻止合并；观察期的 warn 单独报告。只有受检范围已清理或合理抑制后才使用 `--max-warnings 0`，避免名义 warn 实际阻断全部旧代码。
- 本地 hook 仅用于快速反馈，CI 才是不可依赖开发者手动跳过的检查入口；分支规则需要将对应 job 设为 required。
- 路径过滤必须包含共享包、主题、配置和 lockfile。共享基础设施变化触发所有受影响入口；配置变更触发完整静态检查。
- 浏览器测试固定视口、主题、数据和动画条件；截图更新需解释业务或视觉原因，不能批量接受差异代替审查。
- E2E 缺少必需服务时明确失败或报告阻塞，由合并策略决定是否允许；不得以静默跳过显示为完整验收通过。

### 8.7 接入顺序与完成标准

1. 清点版本和入口，接入 ESLint 与插件；完成主题解析、自有组件识别及正反例验证。
2. 选择一个业务模块试点，治理违规和必要例外，建立零违规样板；再扩展到共享 UI 和其他业务目录。
3. 加入架构检查，验证合法平台复用和非法跨层依赖均被正确识别。
4. 接入 CI 聚合入口与历史债务策略，确认故意制造的违规确实让 job 失败。
5. 为关键路由、弹层和租户切换补充行为与浏览器证据，随后更新本节实施状态。

工具已安装、lint 无输出或文档已写入均不等于接入完成。完成证据至少包含：受检文件范围、插件/主题正常加载、正反例结果、真实扫描结果、例外清单，以及本地和 CI 的一致退出状态。视觉一致性、后端授权、跨租户隔离与原生运行仍按第 6 节单独验收。
