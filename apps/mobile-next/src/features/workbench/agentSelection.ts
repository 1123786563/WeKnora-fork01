// M06 → M05 选择回传：模块级轻量 store（会话内导航栈共享）。
// M06 选中 Agent 时写 current 并 router.back()；M05（new-task）在回显时读取。
// 不持久化：空间 / 账号切换后失效是预期行为（能力目录属于当前空间）。
export interface AgentSelection {
  id: string;
  name: string;
}

export const agentSelection: { current: AgentSelection | null } = { current: null };
