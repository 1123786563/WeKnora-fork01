import test from 'node:test';
import assert from 'node:assert/strict';
import { TaskOfficeError } from '@weknora/mobile-core';
import type { AttentionDecisionReceipt, AttentionInboxItem as InboxItem, TaskOffice } from '@weknora/mobile-core';
import { createAttentionInboxController, ATTENTION_RECEIPT_COPY } from './attention-inbox-view.ts';

const ITEM: InboxItem = {
  interactionId: 'i-1', runId: 'run-1', kind: 'tool_approval', argsHash: 'sha256:aa', expectedRevision: 4, createdAt: '2026-09-24T00:00:00Z',
};

function officeHarness(overrides?: {
  items?: InboxItem[];
  inboxError?: Error;
  decideReceipts?: Array<AttentionDecisionReceipt | Error>;
}): { office: Pick<TaskOffice, 'inbox' | 'decide'>; inboxCalls(): number; decideCalls(): number } {
  const decideReceipts = overrides?.decideReceipts ?? [{ status: 'delivery-unknown', interactionId: ITEM.interactionId, decisionId: 'd-1' } as AttentionDecisionReceipt];
  let inboxCalls = 0;
  let decideCalls = 0;
  let decideIndex = 0;
  return {
    inboxCalls: () => inboxCalls,
    decideCalls: () => decideCalls,
    office: {
      async inbox() {
        inboxCalls += 1;
        if (overrides?.inboxError) throw overrides.inboxError;
        return { items: overrides?.items ?? [ITEM] };
      },
      async decide() {
        decideCalls += 1;
        const step = decideReceipts[Math.min(decideIndex, decideReceipts.length - 1)]!;
        decideIndex += 1;
        if (step instanceof Error) throw step;
        return step;
      },
    },
  };
}

test('the controller loads the inbox, maps errors to copy, and appends honest receipt rows', async () => {
  const harness = officeHarness();
  const controller = createAttentionInboxController(harness.office);
  assert.deepEqual(controller.state(), { loading: true, receipts: [] });
  await controller.refresh();
  assert.deepEqual(controller.state().items, [ITEM]);
  assert.equal(controller.state().loading, false);

  // delivery-unknown 的 receipt 文案必须如实（AC2）：记录已落地 + 外部未知，绝不出现「完成/成功派发」。
  await controller.decide(ITEM, 'approve');
  const receipts = controller.state().receipts;
  assert.equal(receipts.length, 1);
  assert.equal(receipts[0].copy, ATTENTION_RECEIPT_COPY['delivery-unknown']);
  assert.match(receipts[0].copy, /已记录/);
  assert.match(receipts[0].copy, /未知/);
  assert.equal(/完成|成功/.test(receipts[0].copy) && !/不代表/.test(receipts[0].copy), false, 'receipt 文案不得宣称外部派发完成');
  // 决定后自动刷新收件箱（行应离开 pending）
  assert.equal(harness.inboxCalls(), 2);

  // 错误路径映射 TaskOfficeError 文案
  const failing = officeHarness({ inboxError: new TaskOfficeError('TASK_OFFICE_BACKEND') });
  const failingController = createAttentionInboxController(failing.office);
  await failingController.refresh();
  assert.match(failingController.state().error ?? '', /服务端暂时不可用/);
});

test('every receipt status maps to distinct honest copy', () => {
  for (const status of ['recorded', 'delivery-unknown', 'superseded', 'gone'] as const) {
    const copy = ATTENTION_RECEIPT_COPY[status];
    assert.ok(copy && copy.length > 0, status);
  }
  assert.match(ATTENTION_RECEIPT_COPY.recorded, /不代表外部操作已完成/);
});
