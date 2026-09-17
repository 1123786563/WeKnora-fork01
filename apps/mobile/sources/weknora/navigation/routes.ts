/**
 * 产品路由模型（MX-009）。
 * - 四个底部入口固定：工作台 / 会话 / 资源 / 我的（设计 M 系列唯一 ground truth）。
 * - M01–M18 每页唯一 screen id 与导航路径；详情页挂在对应 Tab 之上（返回栈按 Tab 分组）。
 * - 深链恢复不得绕过 ProductAuthProvider/ScopeProvider：shell 入口统一经 PRODUCT_ENTRY_GUARDS
 *   声明的守卫链（实现由 MX-010/011 接线；本模型只声明顺序，不实现鉴权）。
 */

export type ProductTabId = 'workbench' | 'sessions' | 'resources' | 'profile';

export interface ProductTab {
  id: ProductTabId;
  /** 底部入口文案（固定四项，不得增删改名） */
  label: string;
  /** Tab 内根 screen id */
  rootScreen: ProductScreenId;
}

export const PRODUCT_TABS: readonly ProductTab[] = [
  { id: 'workbench', label: '工作台', rootScreen: 'M03' },
  { id: 'sessions', label: '会话', rootScreen: 'M04' },
  { id: 'resources', label: '资源', rootScreen: 'M11' },
  { id: 'profile', label: '我的', rootScreen: 'M17' },
] as const;

export type ProductScreenId =
  | 'M01' | 'M02' | 'M03' | 'M04' | 'M05' | 'M06' | 'M07' | 'M08' | 'M09'
  | 'M10' | 'M11' | 'M12' | 'M13' | 'M14' | 'M15' | 'M16' | 'M17' | 'M18';

export interface ProductRoute {
  screen: ProductScreenId;
  name: string;
  /** 导航路径（产品组前缀 /product；legacy Happy 路由保持原 / 前缀互不占用） */
  path: string;
  /** 挂载 Tab（详情返回栈归属；null=全屏流程页，返回到来源 Tab） */
  tab: ProductTabId | null;
}

export const PRODUCT_ROUTES: readonly ProductRoute[] = [
  { screen: 'M01', name: '登录', path: '/product/login', tab: null },
  { screen: 'M02', name: '空间选择', path: '/product/spaces', tab: null },
  { screen: 'M03', name: '工作台', path: '/product/(tabs)/workbench', tab: 'workbench' },
  { screen: 'M04', name: '会话列表', path: '/product/(tabs)/sessions', tab: 'sessions' },
  { screen: 'M05', name: '新建任务', path: '/product/tasks/new', tab: 'sessions' },
  { screen: 'M06', name: 'Agent 选择', path: '/product/tasks/agents', tab: 'sessions' },
  { screen: 'M07', name: '对话', path: '/product/runs/[runId]', tab: 'sessions' },
  { screen: 'M08', name: '执行详情', path: '/product/runs/[runId]/detail', tab: 'sessions' },
  { screen: 'M09', name: '操作审批', path: '/product/runs/[runId]/approvals/[interactionId]', tab: 'sessions' },
  { screen: 'M10', name: '收件箱', path: '/product/inbox', tab: null },
  { screen: 'M11', name: '资源', path: '/product/(tabs)/resources', tab: 'resources' },
  { screen: 'M12', name: '知识详情', path: '/product/knowledge/[kbId]', tab: 'resources' },
  { screen: 'M13', name: '连接详情', path: '/product/connections/[id]', tab: 'resources' },
  { screen: 'M14', name: '任务成果', path: '/product/runs/[runId]/artifacts/[artifactId]', tab: 'sessions' },
  { screen: 'M15', name: '执行目标', path: '/product/targets/[id]', tab: 'resources' },
  { screen: 'M16', name: '语音输入', path: '/product/runs/[runId]/voice', tab: 'sessions' },
  { screen: 'M17', name: '我的', path: '/product/(tabs)/profile', tab: 'profile' },
  { screen: 'M18', name: '空间用量', path: '/product/usage', tab: 'profile' },
] as const;

/** 深链恢复守卫链（声明顺序；实现接线归 MX-010/011）。 */
export const PRODUCT_ENTRY_GUARDS: readonly string[] = ['ProductAuthProvider', 'ScopeProvider'] as const;

/** 唯一性不变量：screen id 与 path 各自唯一（模块加载时自检，模型违规即崩）。 */
const screenIds = new Set(PRODUCT_ROUTES.map((route) => route.screen));
const paths = new Set(PRODUCT_ROUTES.map((route) => route.path));
if (screenIds.size !== PRODUCT_ROUTES.length) throw new Error('product route model: duplicate screen id');
if (paths.size !== PRODUCT_ROUTES.length) throw new Error('product route model: duplicate path');
for (const tab of PRODUCT_TABS) {
  const root = PRODUCT_ROUTES.find((route) => route.screen === tab.rootScreen);
  if (!root || root.tab !== tab.id) throw new Error(`product route model: tab ${tab.id} root screen mismatch`);
}

/** 按 screen id 查路由（导航唯一入口；页面不手写路径字符串）。 */
export function routeFor(screen: ProductScreenId): ProductRoute {
  const route = PRODUCT_ROUTES.find((item) => item.screen === screen);
  if (!route) throw new Error(`unknown product screen ${screen}`);
  return route;
}
