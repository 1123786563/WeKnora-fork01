import test from 'node:test';
import assert from 'node:assert/strict';
import type { OrderView } from '@weknora/contracts';
import { orderMessage, PURCHASE_STATE_LABEL } from './order-state.ts';

function order(overrides: Partial<OrderView> = {}): OrderView {
  return { id: '1', payment: 'pending', fulfillment: 'pending', amount_fen: '100', currency: 'CNY', ...overrides };
}

// (#82 Task 15 / D15-f) The shared purchase-state vocabulary: ONE label map
// covering the CLOSED product state set (spec L169), consumed by the Billing
// row suffix — no page keeps a private copy of the words anymore.
test('PURCHASE_STATE_LABEL covers the closed purchase state set', () => {
  assert.equal(PURCHASE_STATE_LABEL.awaiting_payment, '待付款（权益未开放）');
  assert.equal(PURCHASE_STATE_LABEL.paid_awaiting_activation, '已付款待激活');
  assert.equal(PURCHASE_STATE_LABEL.active, '已生效');
  assert.equal(PURCHASE_STATE_LABEL.canceled, '该购买已取消，请重新发起购买');
  assert.equal(PURCHASE_STATE_LABEL.absent, '');
});

test('payment success never implies credits delivered',()=>{
 assert.equal(orderMessage({id:'1',payment:'paid',fulfillment:'pending',amount_fen:'100',currency:'CNY'}),'已付款，权益处理中');
});

// 行为与边界 counterexamples: 初购/充值/升级/续费 share the same state machine;
// paid 未到账, closed, and fulfilled-precedence are the boundaries that must stay honest.

test('fulfilled orders report benefits delivered regardless of payment state', () => {
  assert.equal(orderMessage(order({ payment: 'paid', fulfillment: 'fulfilled' })), '权益已生效');
  assert.equal(orderMessage(order({ payment: 'pending', fulfillment: 'fulfilled' })), '权益已生效');
  assert.equal(orderMessage(order({ payment: 'closed', fulfillment: 'fulfilled' })), '权益已生效');
});

test('paid with fulfillment still processing never claims benefits delivered', () => {
  assert.equal(orderMessage(order({ payment: 'paid', fulfillment: 'processing' })), '已付款，权益处理中');
});

test('closed orders are reported closed', () => {
  assert.equal(orderMessage(order({ payment: 'closed', fulfillment: 'pending' })), '订单已关闭');
});

test('unpaid pending orders wait for payment', () => {
  assert.equal(orderMessage(order()), '等待付款');
});
