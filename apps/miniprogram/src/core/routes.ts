export const ROUTES = {
  login:{path:'subpackages/auth/login/index',title:'登录',tab:false},
  workspace:{path:'subpackages/auth/workspace/index',title:'选择工作空间',tab:false},
  home:{path:'pages/home/index',title:'工作台',tab:true},
  agents:{path:'subpackages/agent/list/index',title:'发现 Agent',tab:false},
  agent:{path:'subpackages/agent/detail/index',title:'发起任务',tab:false},
  chat:{path:'subpackages/chat/conversation/index',title:'对话',tab:false},
  tasks:{path:'pages/tasks/index',title:'任务',tab:true},
  execution:{path:'subpackages/execution/detail/index',title:'执行详情',tab:false},
  approval:{path:'subpackages/execution/approval/index',title:'审批确认',tab:false},
  artifact:{path:'subpackages/execution/artifact/index',title:'任务产物',tab:false},
  knowledge:{path:'pages/knowledge/index',title:'知识',tab:true},
  kb:{path:'subpackages/knowledge/detail/index',title:'知识库详情',tab:false},
  upload:{path:'subpackages/knowledge/upload/index',title:'添加知识',tab:false},
  document:{path:'subpackages/knowledge/document/index',title:'文档与引用',tab:false},
  me:{path:'pages/me/index',title:'我的',tab:true},
  usage:{path:'subpackages/account/usage/index',title:'用量与套餐',tab:false},
  checkout:{path:'subpackages/account/checkout/index',title:'确认订单',tab:false},
  order:{path:'subpackages/account/order/index',title:'订单与履约',tab:false},
  invitations:{path:'subpackages/account/invitations/index',title:'空间邀请',tab:false},
  states:{path:'subpackages/account/states/index',title:'状态说明',tab:false},
} as const;
export type PageId = keyof typeof ROUTES;
export function pageUrl(page: PageId, params: Record<string,string|number|undefined> = {}): string {
  const route=ROUTES[page]; if (!route) throw new Error('Unknown page');
  const query=Object.entries(params).filter(([,v])=>v!==undefined).map(([k,v])=>`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`).join('&');
  if(route.tab && query) throw new Error('Tab routes cannot carry query parameters');
  return `/${route.path}${query?`?${query}`:''}`;
}
