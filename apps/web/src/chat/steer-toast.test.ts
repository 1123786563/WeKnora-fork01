import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import { resolveChatCopy } from '@weknora/views';
import { resolveSteerSubmitFailure } from '@weknora/views/chat/composer';

import { steerFailureCopy, steerNoticeCopy } from './steer-toast.ts';

/*
 * R476-A2 — Vue chat/index.vue steer notices are scenario-keyed MessagePlugin
 * calls, not one operationFailed bucket (R475 A3 leftover):
 *   enqueue fail   → error  input.messages.steerFailed
 *   promote fail   → error  input.messages.steerPromoteFailed
 *   remove fail    → error  input.messages.steerRemoveFailed
 *   already_injected (enqueue/promote/remove answers) → info
 *                    input.messages.steerAlreadyInjected
 * These tests pin the scenario → copy-key dispatch and the Vue
 * `e?.message || t(key)` fallback contract (server message wins).
 */

test('each steer scenario dispatches to its own Vue toast copy in zh-CN', () => {
  const copy = resolveChatCopy('zh-CN');
  assert.equal(steerNoticeCopy(copy, 'enqueueFailed'), '追加失败，请重试');
  assert.equal(steerNoticeCopy(copy, 'promoteFailed'), '立即发送失败，请重试');
  assert.equal(steerNoticeCopy(copy, 'removeFailed'), '删除排队消息失败，请重试');
  assert.equal(steerNoticeCopy(copy, 'alreadyInjected'), '该消息已被当前回答接收');
});

test('steer scenario copy is locale-resolved, not the zh-only operationFailed bucket', () => {
  const en = resolveChatCopy('en-US');
  assert.equal(steerNoticeCopy(en, 'enqueueFailed'), 'Failed to append the message. Please try again.');
  assert.equal(steerNoticeCopy(en, 'promoteFailed'), 'Failed to send now. Please try again.');
  assert.equal(steerNoticeCopy(en, 'removeFailed'), 'Failed to remove the queued message. Please try again.');
  assert.equal(steerNoticeCopy(en, 'alreadyInjected'), 'This message has already been taken by the running answer.');
  assert.equal(steerNoticeCopy(en, 'enqueueFailed'), en.steerFailed, 'dispatch reads the table key, not a literal');
  assert.notEqual(steerNoticeCopy(en, 'enqueueFailed'), en.operationFailed, 'steer failures no longer collapse into operationFailed');
});

test('steerFailureCopy keeps the server-provided Error message and falls back per scenario', () => {
  const copy = resolveChatCopy('zh-CN');
  // Vue: MessagePlugin.error(e?.message || t('input.messages.steerFailed')).
  assert.equal(steerFailureCopy(copy, 'enqueueFailed', new Error('会话已结束')), '会话已结束');
  assert.equal(steerFailureCopy(copy, 'promoteFailed', new Error('boom')), 'boom');
  // A non-Error rejection (or an empty message) degrades to the scenario copy.
  assert.equal(steerFailureCopy(copy, 'enqueueFailed', 'plain string'), '追加失败，请重试');
  assert.equal(steerFailureCopy(copy, 'enqueueFailed', undefined), '追加失败，请重试');
  assert.equal(steerFailureCopy(copy, 'promoteFailed', null), '立即发送失败，请重试');
  assert.equal(steerFailureCopy(copy, 'removeFailed', new Error('')), '删除排队消息失败，请重试');
  assert.equal(steerFailureCopy(copy, 'alreadyInjected', new Error('ignored')), '该消息已被当前回答接收', 'info notices never take a cause message');
});

/*
 * Host wiring, asserted on the source (same pattern as
 * chat-route-page-steer-queue.test.ts): the steer handlers dispatch the
 * scenario notices — already_injected answers (enqueue/promote/remove) toast
 * the info copy, and the failure catches fall back per scenario instead of
 * the operationFailed bucket.
 */
test('ChatRoutePage steer handlers dispatch scenario notices instead of operationFailed', () => {
  const source = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(source, /steerNoticeCopy\(copy, 'alreadyInjected'\)/, 'already_injected answers surface the info notice');
  assert.match(source, /steerFailureCopy\(copy, 'promoteFailed', cause\)/, 'promote catch falls back to steerPromoteFailed');
  assert.match(source, /steerFailureCopy\(copy, 'removeFailed', cause\)/, 'remove catch falls back to steerRemoveFailed');
  assert.match(source, /steerFailureCopy\(copy, 'enqueueFailed', cause\)/, 'retry/enqueue failures fall back to steerFailed');

  // The steer chain (steer → promoteSteer → removeSteer → retrySteer → the
  // enqueue-result dispatcher) must not collapse into operationFailed.
  const steerChainStart = source.indexOf('function applySteerEnqueueResult');
  const steerChainEnd = source.indexOf('async function consumeSteerAfterTurn');
  assert.ok(steerChainStart > 0 && steerChainEnd > steerChainStart, 'steer handler region found');
  const steerChain = source.slice(steerChainStart, steerChainEnd);
  assert.doesNotMatch(steerChain, /copy\.operationFailed/, 'the steer chain no longer uses the operationFailed bucket');

  // Vue handleRemoveSteer inspects the DELETE answer: already_injected toasts
  // the info notice (the running answer took the message), a refused removal
  // keeps the chip.
  assert.match(steerChain, /status === 'already_injected'[\s\S]{0,400}steerNoticeCopy\(copy, 'alreadyInjected'\)/, 'remove already_injected answers toast the info notice');
});

/*
 * R478-A1 — the steer submit contract is pinned on an exported pure
 * function (explicit anchor) instead of the former 900-char source-window
 * regex, which silently depended on how much code sat between the submit
 * signature and its finally block (R476 was once over-matched that way).
 * resolveSteerSubmitFailure is the Vue Input-field.vue steer submit
 * fallback: the server Error message wins, input.messages.steerFailed is
 * the fallback — the send path's sendFailed must never leak in.
 */
test('the steer composer submit failure fallback resolves to steerFailed, not sendFailed', () => {
  const copy = resolveChatCopy('zh-CN');
  assert.equal(resolveSteerSubmitFailure(copy, new Error('网络中断')), '网络中断', 'the server-provided Error message wins');
  assert.equal(resolveSteerSubmitFailure(copy, 'plain string'), copy.steerFailed, 'a non-Error rejection degrades to the scenario copy');
  assert.equal(resolveSteerSubmitFailure(copy, undefined), copy.steerFailed);
  assert.equal(resolveSteerSubmitFailure(copy, new Error('')), copy.steerFailed, 'an empty Error message degrades too');
  assert.equal(resolveSteerSubmitFailure(copy, null), '追加失败，请重试', 'the fallback is input.messages.steerFailed');
  assert.notEqual(copy.steerFailed, copy.sendFailed, 'the two copy keys are distinct, so the fallback cannot pass as sendFailed by accident');
});

test('the SteerComposer submit wires the exported steer gates (line anchors, no source windows)', () => {
  const pageSource = readFileSync(new URL('../../../../packages/views/src/chat/page.tsx', import.meta.url), 'utf8');
  // R478-A1: single-line wiring anchors only — same pattern as the
  // ChatRoutePage assertions above; no fixed-width character windows.
  assert.match(pageSource, /resolveSteerAttachmentWarning\(attachments\)/, 'the attachment warning gate is the shared exported predicate');
  assert.match(pageSource, /resolveSteerSubmitFailure\(copy, cause\)/, 'a rejected steer enqueue falls back through the shared exported predicate');
});
