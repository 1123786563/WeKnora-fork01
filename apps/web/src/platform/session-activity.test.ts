// session-activity.ts 纯核心单测 —— 与 Vue frontend/src/stores/sessionActivityState.ts
// （refresh 转移规则）及 chat/index.vue onAfterMsgList（末位未完成 assistant 检测）
// 逐行为对齐：这些规则决定侧栏会话行 running spinner 的显隐与清除。
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  detectRunningMessageId,
  refreshSessionActivityEntry,
  refreshSessionActivityError,
} from './session-activity.ts';

const msg = (id: string, role: 'user' | 'assistant', isCompleted: boolean) => ({ id, role, is_completed: isCompleted });

test('detectRunningMessageId 取最后一条未完成 assistant（chat/index.vue findLastMessage）', () => {
  // API 返回旧→新：中间夹着已完成 assistant 时只认末位未完成的那条。
  assert.equal(detectRunningMessageId([
    msg('u1', 'user', true),
    msg('a1', 'assistant', true),
    msg('a2', 'assistant', false),
    msg('u2', 'user', true),
  ]), 'a2');
  // 全部完成 → 空串（会话不在生成态）。
  assert.equal(detectRunningMessageId([msg('u1', 'user', true), msg('a1', 'assistant', true)]), '');
  // is_completed 缺省按未完成处理（与 Vue !message.is_completed 同口径）。
  assert.equal(detectRunningMessageId([{ id: 'a9', role: 'assistant' }]), 'a9');
  assert.equal(detectRunningMessageId([]), '');
});

test('refresh：记录的 messageId 完成后清除标记', () => {
  const entry = { messageId: 'a2', failures: 0 };
  assert.equal(refreshSessionActivityEntry(entry, [
    msg('u1', 'user', true),
    msg('a2', 'assistant', true),
  ]), null);
});

test('refresh：记录的 messageId 从消息列表消失后清除标记', () => {
  // 会话被清空/消息被删：携带 messageId 找不到目标即删（Vue: !message && entry.messageId）。
  assert.equal(refreshSessionActivityEntry({ messageId: 'gone', failures: 0 }, [msg('u1', 'user', true)]), null);
});

test('refresh：未完成消息续命并回填 messageId、清零 failures', () => {
  assert.deepEqual(refreshSessionActivityEntry({ messageId: 'a2', failures: 2 }, [
    msg('u1', 'user', true),
    msg('a2', 'assistant', false),
  ]), { messageId: 'a2', failures: 0 });
  // 无 messageId 条目在轮询中发现未完成 assistant → 绑定其 id。
  assert.deepEqual(refreshSessionActivityEntry({ messageId: '', failures: 0 }, [
    msg('a1', 'assistant', false),
  ]), { messageId: 'a1', failures: 0 });
});

test('refresh：无 messageId 且无未完成 assistant 视为放弃的请求，连续 3 次失败清除', () => {
  const first = refreshSessionActivityEntry({ messageId: '', failures: 0 }, [msg('u1', 'user', true)]);
  assert.deepEqual(first, { messageId: '', failures: 1 });
  const second = refreshSessionActivityEntry(first!, [msg('u1', 'user', true)]);
  assert.deepEqual(second, { messageId: '', failures: 2 });
  const third = refreshSessionActivityEntry(second!, [msg('u1', 'user', true)]);
  assert.equal(third, null, '第三次失败达到阈值即清除');
});

test('refresh 错误分支：403/404 立即清除，其余错误计 3 次后清除', () => {
  assert.equal(refreshSessionActivityError({ messageId: 'a1', failures: 0 }, 403), null);
  assert.equal(refreshSessionActivityError({ messageId: 'a1', failures: 0 }, 404), null);
  assert.deepEqual(refreshSessionActivityError({ messageId: 'a1', failures: 0 }, 500), { messageId: 'a1', failures: 1 });
  assert.equal(refreshSessionActivityError({ messageId: 'a1', failures: 2 }, undefined), null);
});
