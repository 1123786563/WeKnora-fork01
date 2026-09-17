# React 无刷新导航设计方案

状态：建议稿，待评审。本文不代表已经批准实施。日期：2026-09-16。

## 目标与边界

Web 与复用 Web renderer 的 Desktop 在普通站内导航时不重新加载文档；同一身份、同一空间内的 PlatformShell 持续挂载，只更新内容区。既有 URL、查询参数、页面样式、认证流程、权限约束及业务功能保持兼容。

本次不引入 SSR、全局页面 KeepAlive、新的服务端数据缓存框架，也不迁移 Mobile 或 Embed 的独立路由。页面自身的数据加载仍允许发生。外部 OAuth、外部站点、下载与显式手动刷新不属于无刷新导航承诺。

## 当前实现证据

- `apps/web/src/main.tsx` 启动时解析一次 route，通过多个 root.render 分支装配页面；跨路径 popstate 显式 reload。登录、退出和切空间通过 location.assign 完成后续初始化。
- `apps/web/src/platform/PlatformShell.tsx` 菜单使用普通 a href；会话及命令面板使用 location.assign；通过改写 history.pushState/replaceState 更新菜单高亮。
- `apps/web/src/platform/adapters.ts` 导航只修改 history，并不驱动页面树更新。
- `apps/web/src/settings/SettingsPage.tsx` 手动管理 section、subsection、push/replace/back；部分值只在首次挂载读取。
- `apps/web/src/chat/ChatRoutePage.tsx` 会话状态初始读取 pathname，切会话时同时修改本地状态和 history；迁移必须统一两者。
- `apps/web/src/platform/scope-runtime.ts` 切空间更新凭据、角色与 scope，但 capabilitySnapshot 由 hydrate 更新。取消刷新后必须显式刷新新空间能力。
- `packages/domain/src/scope.ts` 已有 generation、AbortSignal、isCurrent；每次 switchScope 都推进 generation，因此普通导航不能无条件 hydrate 并取消全部在途任务。
- `apps/desktop/src/main.tsx` 先安装原生运行时，再导入 Web main；Desktop runtime 在启动前执行 URL 规范化。

以上为静态源码证据，尚未进行浏览器或 Wails 原生验证。

## 方案比较与建议

| 方案 | 优点 | 成本与限制 |
| --- | --- | --- |
| React Router Data Mode，推荐 | 统一导航实例、嵌套路由、错误边界；非组件代码可调用 router.navigate | 增加依赖，需要迁移 URL 状态与生命周期 |
| React Router Declarative Mode | 贴近现有组件取数，初始接入简单 | 非组件导航需要额外注入；后续统一导航能力需继续建设 |
| 扩展自研 history 路由 | 无新依赖，短期修改较少 | 长期自行承担订阅、并发、重定向、错误处理和导航语义维护 |

建议采用 Data Mode，使用 createBrowserRouter + RouterProvider。保留 Vite 构建与现有 API/controller 取数方式，本次不把业务请求全面搬进 loader。router 在 React 树外创建一次；不因菜单、空间或 render 变化重新创建。

依赖选用实施时经 engines、peerDependencies 和仓库 CI Node 版本验证的稳定精确版本并锁定 lockfile；本设计不假设仓库已经安装 React Router。

参考：[官方接入](https://reactrouter.com/start/data/installation)、[布局及嵌套路由](https://reactrouter.com/start/data/routing)。

## 组件与模块职责

| 文件/模块 | 责任 |
| --- | --- |
| `apps/web/src/main.tsx` | OIDC 回调预处理、主题、运行时创建、唯一一次根挂载 |
| 新增 `apps/web/src/app/create-app-runtime.ts` | 创建稳定 client、凭据协调器、scopeRuntime、sessionStore；不读取固定的初始 route 来决定长期行为 |
| 新增 `apps/web/src/app/session-store.ts` | 可订阅会话快照与恢复、登录、退出、切空间事务；React 通过 useSyncExternalStore 消费 |
| 新增 `apps/web/src/app/router.tsx` | 唯一运行时路由树、布局、懒加载边界和 404；接收 runtime，不循环导入 main |
| 新增 `apps/web/src/app/RouteGate.tsx` | 消费最新会话、location 和原有纯 guardRoute；等待必要恢复、重定向或放行 |
| 新增 `apps/web/src/app/PlatformLayout.tsx` | Shell + 内容区 Suspense/ErrorBoundary + Outlet；管理路由焦点和内容滚动 |
| 现有 `apps/web/src/routes.tsx` | 迁移期保留 URL 兼容解析和纯权限策略，最终去掉重复页面匹配和 reload 策略 |
| 现有 `apps/web/src/platform/adapters.ts` | 注入 navigate/replace 实现；不自行写 history |
| 各业务 RoutePage | 用参数与 search 驱动页面，向共享视图传入导航回调 |

运行时依赖方向：main 创建 runtime → 创建 router(runtime) → 将 router.navigate 注入平台适配层 → 挂载 RouterProvider。共享 packages/views 通过 props/现有平台契约导航，不依赖 Web router 或 apps 源码。

路由层次：公共登录/注册/邀请入口、引导入口、Craft 独立布局、Platform 布局为同一个 router 的不同分支。Platform 布局同时覆盖 `/platform/*` 和 `/knowledgeBase/*`；不得把 Craft 强行包进侧栏。Web 对 `/embed/*` 保留独立入口提示，不接入 Web 身份上下文。Craft 现有匿名可达语义保持，并审计 `features/craft/routes.tsx` 的内部 URL 管理，不嵌套第二个浏览器 router。

Suspense 放在 Shell 内容区内，防止页面代码加载使整个侧栏消失。同空间普通导航不对 Shell 使用 pathname/location.key。切身份或空间时允许重建空间相关 Shell 子树以清除旧数据；主题等不含空间数据的 Provider 保持稳定。

## 会话、权限与空间切换

会话快照至少包含 phase、identity、tenant、role、capabilities、systemAdmin、liteMode/edition、revision 和 error。phase 区分 restoring、anonymous、ready、switching、error；凭据仍由现有凭据适配层持有。快照在无更新时保持引用稳定。

首次打开受保护路径先恢复身份和权限，再挂载业务页面。普通导航同步重新执行 guardRoute，使用已恢复快照；不为每次菜单点击重建 client 或 scope。权限刷新触发点为登录完成、切空间完成、已知权限变更、恢复窗口焦点及权限拒绝后的单次重验；并发恢复合并为同一请求。重验不自动重试写操作。

身份恢复和导航需带会话 revision；旧恢复请求不得覆盖退出、切空间后的快照。同一身份/空间且权限未变的重验不推进 scope generation；权限发生变化时刷新快照并失效相关请求与页面状态。每次导航和快照变化都重新判定访问，包括仅 search 变化进入受限设置分区的情况。后端继续承担最终授权。

切空间流程：进入 switching 并阻止重复提交 → 隔离旧空间页面和在途结果 → 执行现有切换 API → 持久化新凭据并切换 scope → 在新请求上下文下调用 auth.me 更新角色和 capabilities → 发布 ready → replace 到知识库列表。服务端切换尚未成功的失败可恢复旧空间并重新加载；新凭据已经持久化但 hydrate 失败时保持隔离错误页并提供重试/退出，不渲染旧空间、不自动重试切换写操作。

退出及刷新令牌失败：先失效所有会话请求、清理凭据和空间状态，再 replace 登录页；旧的 auth.me 和导航结果不得使会话复活。401 按现有 refreshCoordinator 处理；403 重验权限并展示拒绝或重定向；普通网络错误显示可重试错误，避免一律清空登录态。

OIDC hash 回调解析、待接受邀请、next 安全校验、lite autoSetup、无空间引导必须从现有 bootstrap 迁移为显式流程。一次性接受邀请等写动作有去重标识，不因组件重渲染或开发模式 effect 重跑而重复执行。

## 导航及页面状态契约

1. 菜单使用 NavLink；普通站内链接使用 Link；命令面板、登录完成、详情跳转及非 React 代码统一调用导航适配器。保留键盘激活、Ctrl/Cmd 点击、中键、新标签和复制链接语义。
2. 用户主动进入页面通常 push；兼容别名、权限重定向、默认参数规范化用 replace。保留既有设置分区中明确的 push/replace 行为，不把每次输入都塞入历史栈。
3. URL 是资源 ID、设置 section/subsection、Wiki tab/slug、文档 knowledge_id 和可分享筛选项的来源。通过 useParams/useSearchParams/useLocation 派生，不只在 useState 初始化时读取；查询更新保留无关参数。
4. 设置关闭保留现有 close 模式；有可信应用内返回位置则返回，无可用位置则进入知识库列表，防止直接打开设置时退离应用。
5. 聊天在 A → B、前进后退、新建 → 获得 sessionId 时分别处理状态。切到其他会话取消旧 UI 流订阅并过滤迟到消息；新建会话取得 ID 后的 URL 更新不得卸载当前执行、丢附件或中断流。不能简单给聊天页面统一加 sessionId key。离开聊天时释放页面订阅，返回按现有持久化/恢复协议恢复，不把前端卸载等同于停止后端 Agent。
6. 知识库/文档 ID 变化时重新加载对应资源并隔离旧响应；从副作用中进行 Wiki URL 规范化，移除 render 阶段 history 写入。
7. 普通页面离开后允许卸载，原有持久化草稿继续生效；不承诺新建全局草稿缓存。已有未保存退出提示须覆盖客户端导航和浏览器离开两条路径。
8. 页面切换关闭菜单和命令面板，更新标题并把焦点移到内容标题。侧栏滚动保持；新页面内容通常回顶，后退按历史条目恢复内容滚动；设置同页分区不无条件触发全页面滚动。
9. 移除 Shell 对 history 的改写和业务页自建 popstate 监听。启动前 OIDC/Desktop URL 规范化允许直接 replaceState；运行期站内跳转不再直接写 history。外链、OAuth、下载、显式整页恢复逐项登记为例外。

## 兼容与发布边界

第一项实施产物是现有 resolveRoute/routeRedirect/guardRoute 的完整兼容矩阵，包括 `/creatChat`、`/platform/chat`、`/knowledgeBase` 系列、系统管理别名、旧 integrations、邀请参数、Craft、开发专用路径、未知路径与异常编码。不因菜单不可见而省略历史地址。原路径权限先检查，规范化后目标权限再次检查，避免管理地址变成普通设置 URL 后丢失限制。

Desktop 保留启动前 bridge、API base 与 URL 规范化顺序；优先共用 browser history 路由，在真实 Wails 中验证。若原生宿主无法支持深链或刷新，明确阻塞并另行评估适配，不静默改成 hash URL。检查实际生产静态服务的 HTML fallback，确保手动刷新和直达深链成功，API、静态资源 404 不被吞成 index.html。

变更在隔离分支按步骤完成，不发布同时存在新旧两套路由所有者的中间状态。通过单一已验证版本发布；失败时回退完整前端版本及 lockfile。无后端数据迁移，不保留长期双路由开关。

## 实施分解与依赖

下表为设计级工作包；正式逐步骤实施计划在方案确认后展开。每项要求先加入可观察行为的失败用例，再实现、回归、审查并形成独立提交。

| 工作包 | 内容与主要文件 | 前置 | 完成证据 |
| --- | --- | --- | --- |
| T0 兼容基线 | routes.test.ts、导航入口清单、现有守卫/设置/聊天测试；确认 router 依赖兼容 | 无 | 路径矩阵完整；当前菜单文档导航可复现 |
| T1 会话运行时 | create-app-runtime、session-store、scope-runtime 及测试 | T0 | 恢复去重、退出竞争、切空间失败、权限刷新、旧响应隔离测试 |
| T2 路由和外壳 | router、RouteGate、PlatformLayout、main、adapters、Shell | T1 | 两个代表页面无刷新往返，Shell 挂载身份不变，守卫在 URL/权限变化时执行 |
| T3 设置 URL | SettingsPage 及其导航消费者和测试 | T2 | section/subsection 直达、push/replace、前进后退、关闭和权限拒绝通过 |
| T4 聊天与资源 URL | ChatRoutePage、知识库/文档/Wiki 路由、Craft 与相关共享视图 | T2 | 会话 A/B、创建会话流连续性、资源换 ID、迟到结果隔离通过 |
| T5 全入口收口 | 其他站内链接、认证/邀请/引导、命令面板、Desktop 导航；删除旧刷新路径 | T3、T4 | 原始导航调用分类清零或列入有依据例外；兼容矩阵全部通过 |
| T6 综合验收 | Web 浏览器、真实后端、Wails、生产深链和回退检查 | T5 | 分层证据完整，阻塞项明确，无用构建结果代替运行验证 |

T3 与 T4 在接口冻结后可并行；router、main、session-store、adapters 和 lockfile 由单一集成责任人串行修改。各业务工作包向集成者提交路由注册需求，不同时修改中央路由文件。

## 验收与证据

- 浏览器记录顶层 document 请求和启动实例标识；每次主菜单点击均无新文档加载，Shell DOM/挂载身份保持，URL、高亮与内容一致。不能只验证没有调用 reload。
- 直达、刷新、前进后退、别名、异常编码、未知路径、query/hash 保留均进入预期页面；重定向不形成循环或污染返回历史。
- 匿名、已登录、无空间、viewer/admin/owner、系统管理员、能力关闭、过期凭据分别验证；拒绝访问时目标业务页面不挂载，不提前发出其数据请求。
- 同一会话普通导航不失效全局 scope；切空间/退出后旧请求、流事件和旧权限不可回写。新空间 hydrate 失败必须保持隔离状态。
- 懒加载失败和业务加载失败显示局部错误，侧栏仍可用；首屏初始化错误有恢复路径。
- 自动化使用现有 Node test/tsx/JSDOM 体系，并新增独立路由浏览器测试；不要把只覆盖 Craft 的 Playwright 配置当作全站验收。
- 基础命令：`pnpm test:web`、`pnpm typecheck:web`、`pnpm build:web`、`pnpm test:desktop`、`pnpm typecheck:desktop`、`pnpm build:desktop-renderer`；共享包变动执行对应共享测试，并按消费者追加 Embed 门禁。
- 浏览器、真实后端、生产部署深链、Wails 原生启动分别记录结果。缺少环境标记 blocked-env；单元 mock、类型检查与构建不证明这些场景已验收。

## 本次交付状态

已完成当前入口、菜单、路由策略、scope、设置、聊天和 Desktop 启动链的静态核对及设计。未安装依赖、修改业务实现、运行验收或发布；本方案仍待用户评审。
