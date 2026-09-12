import test from 'node:test';
import assert from 'node:assert/strict';
import { refundMessage } from './refund-state.ts';
test('remote success with pending revocation is not retried payment',()=>{
 assert.equal(refundMessage('revocation_pending'),'退款已完成，权益调整处理中');
 assert.equal(refundMessage('refund_unknown'),'退款结果核对中');
});
test('completed and rejected refunds get distinct terminal messages',()=>{
 assert.equal(refundMessage('completed'),'退款与权益调整已完成');
 assert.equal(refundMessage('rejected'),'退款申请未通过');
});
test('in-flight and unrecognised states fall back to processing',()=>{
 assert.equal(refundMessage('requested'),'退款处理中');
 assert.equal(refundMessage('reviewing'),'退款处理中');
 assert.equal(refundMessage('pending'),'退款处理中');
 assert.equal(refundMessage('failed_confirmed'),'退款处理中');
 assert.equal(refundMessage('not_created_confirmed'),'退款处理中');
 assert.equal(refundMessage('anything-else'),'退款处理中');
});
