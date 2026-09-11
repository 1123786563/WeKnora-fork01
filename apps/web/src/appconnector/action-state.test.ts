import test from 'node:test';
import assert from 'node:assert/strict';
import { actionMessage } from './action-state.ts';
test('unknown write is shown as reconciliation',()=>{
 assert.equal(actionMessage('unknown'),'正在核对外部执行结果');
 assert.equal(actionMessage('awaiting_approval'),'等待操作批准');
});
test('succeeded is terminal completion',()=>{
 assert.equal(actionMessage('succeeded'),'操作已完成');
});
test('in-flight and settled-processing states show in-progress copy',()=>{
 assert.equal(actionMessage('dispatched'),'操作处理中');
 assert.equal(actionMessage('queued'),'操作处理中');
 assert.equal(actionMessage('authorized'),'操作处理中');
 assert.equal(actionMessage('failed'),'操作处理中');
});
test('every A03 lifecycle state is covered by an explicit message',()=>{
 const states=['awaiting_approval','authorized','queued','dispatched','succeeded','failed','unknown'];
 for(const state of states){
  const message=actionMessage(state);
  assert.equal(typeof message,'string');
  assert.notEqual(message,'');
 }
});
