// 领域标签映射（M03 工作台 / M04 会话列表 / M10 收件箱共用）。
// 与 app/(tabs)/index.tsx 中的本地实现保持同一语义：原始枚举与展示文案分离，
// 未知枚举一律显示"等待同步"，不得显示为成功。
// 页面无法从 app/ 目录互相导入，故集中到 src/features 供多页引用。

export function kindLabel(kind: string): string {
  if (kind === "tool_approval") return "工具批准";
  if (kind === "budget") return "预算";
  if (kind === "recovery") return "执行恢复";
  if (kind === "question") return "问题";
  if (kind === "connection") return "连接授权";
  return "需要确认";
}

export function runStatusLabel(status: string): string {
  switch (status) {
    case "queued": return "排队中";
    case "running": return "进行中";
    case "waiting_user": return "等待你处理";
    case "completed": case "succeeded": return "已完成";
    case "failed": return "失败";
    case "cancelled": case "canceled": return "已取消";
    default: return "等待同步";
  }
}

export type RunTone = "neutral" | "progress" | "attention" | "danger" | "success" | "unknown";

export function runStatusTone(status: string): RunTone {
  switch (status) {
    case "queued": return "neutral";
    case "running": return "progress";
    case "waiting_user": return "attention";
    case "completed": case "succeeded": return "success";
    case "failed": return "danger";
    default: return "unknown";
  }
}

/** ISO 时间 → 相对时间描述（"刚刚 / N 分钟前 / N 小时前 / N 天前"）；无效输入返回空串 */
export function formatRelative(iso: string): string {
  if (!iso) return "";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "";
  const diffMs = Date.now() - t;
  if (diffMs < 0) return "刚刚";
  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} 天前`;
  return `${Math.floor(days / 30)} 个月前`;
}

/** ISO 时间 → 当地时刻 "HH:MM:SS"（M08 最近同步 / 时间线用）；无效输入返回 null */
export function formatClock(iso: string | null): string | null {
  if (!iso) return null;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return null;
  const d = new Date(t);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
