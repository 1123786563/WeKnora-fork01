/**
 * Vue frontend/src/stores/sessionActivityState.ts 同构复刻（shell 侧最小通路）。
 *
 * Vue 端数据通路：chat/index.vue 在打开会话时检测「最后一条 assistant 消息
 * 未完成」（onAfterMsgList 的 findLastMessage resume 分支），把该会话写入
 * sessionActivity store（isReplying/isStreaming watcher → update(id, true)）；
 * menu.vue 挂 5s setInterval 调 refresh() 轮询 detached 条目，消息完成/消失/
 * 连续 3 次失败即清除，侧栏行按 entries[id] 渲染 running spinner。
 *
 * React 端 chat 域（不可改动域）没有这枚标记的写入方，故由 shell 承担两职：
 * 活跃会话切换时跑一次同样的「末位未完成 assistant」检测（替代 chat 视图的
 * update），并按 menu.vue 的口径轮询全部条目直至清除。纯函数核心与 Vue
 * sessionActivityState 逐行对齐，便于单测锁定行为。
 */

export interface SessionActivityEntry {
  messageId: string;
  failures: number;
}

export type SessionActivityEntries = Record<string, SessionActivityEntry>;

export interface ActivityMessageLike {
  id: string;
  role: string;
  is_completed?: boolean;
}

/** chat/index.vue onAfterMsgList — 取「最后一条未完成 assistant」的 id（无则 ''）。 */
export function detectRunningMessageId(messages: readonly ActivityMessageLike[]): string {
  let found = '';
  for (const message of messages) {
    if (message.role === 'assistant' && message.is_completed !== true) found = message.id;
  }
  return found;
}

/**
 * Vue sessionActivityState.refresh 的单条目转移函数。
 * 返回 null 表示删除该条目（停止标记）；否则返回保留/更新后的条目。
 * - 记录过 messageId：该消息完成或消失 → 删除；否则续命（failures 清零）。
 * - 未记录 messageId：找不到未完成 assistant 视为请求被放弃，连续 3 次 → 删除。
 */
export function refreshSessionActivityEntry(
  entry: SessionActivityEntry,
  messages: readonly ActivityMessageLike[],
): SessionActivityEntry | null {
  const message = entry.messageId
    ? messages.find((item) => item.id === entry.messageId)
    : messages.find((item) => item.role === 'assistant' && item.is_completed !== true);
  if (message?.is_completed === true || (!message && entry.messageId)) return null;
  if (message) return { messageId: message.id, failures: 0 };
  if (entry.failures + 1 >= 3) return null; // 放弃的请求：从未产生 assistant 消息
  return { ...entry, failures: entry.failures + 1 };
}

/** Vue refresh 的 catch 分支：403/404 立删，其余错误按 failures 阈值清除。 */
export function refreshSessionActivityError(
  entry: SessionActivityEntry,
  status?: number,
): SessionActivityEntry | null {
  if (status === 403 || status === 404) return null;
  if (entry.failures + 1 >= 3) return null;
  return { ...entry, failures: entry.failures + 1 };
}
