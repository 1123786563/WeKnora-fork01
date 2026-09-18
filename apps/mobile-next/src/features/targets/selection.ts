// M15 执行目标选择（模块级轻量共享）：new-task 选择目标后写回，targets 页展示当前项。
// 只保存服务端受权引用的 id/name；不承载任意节点地址输入（见 /targets 页说明）。
export interface TargetSelection {
  id: string;
  name: string;
}

export const targetSelection: { current: TargetSelection | null } = { current: null };
