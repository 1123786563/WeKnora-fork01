import {test} from 'node:test';
import assert from 'node:assert/strict';
import {usageLabel,fundingNote,asOfLabel,usageRows} from './usage.ts';

test('unknown consumption stays visible',()=>{
  assert.equal(usageLabel({knownCalls:3,unknownCalls:1,inputTokens:10,outputTokens:20,funding:'byok'}),'已记录 3 次调用，1 次用量待核对');
});

test('label states zero unknown plainly',()=>{
  assert.equal(usageLabel({knownCalls:5,unknownCalls:0,inputTokens:1,outputTokens:2,funding:'platform'}),'已记录 5 次调用，0 次用量待核对');
});

test('byok funding note says the space bears the model',()=>{
  assert.equal(fundingNote({knownCalls:1,unknownCalls:0,inputTokens:1,outputTokens:1,funding:'byok'}),'BYOK：本次模型调用由空间自有凭据承担，平台不计金额');
  assert.equal(fundingNote({knownCalls:1,unknownCalls:0,inputTokens:1,outputTokens:1,funding:'platform'}),'');
  assert.equal(fundingNote({knownCalls:2,unknownCalls:0,inputTokens:1,outputTokens:1,funding:'mixed'}),'含 BYOK 调用：部分模型调用由空间自有凭据承担，平台不计该部分金额');
});

test('as_of renders an explicit refresh stamp',()=>{
  const stamp = '2026-09-12T08:30:00Z';
  assert.equal(asOfLabel(stamp),'数据截至 2026-09-12 08:30 UTC（晚到用量修正后可刷新）');
});

test('usage rows split main and child runtimes',()=>{
  const rows = usageRows([
    {callId:'c1',attemptId:'a1',runtime:'main',delegationId:'',modelId:'m1',funding:'platform',status:'reported',inputTokens:10,outputTokens:20,cachedTokens:0,observedAt:'2026-09-12T08:00:00Z'},
    {callId:'c2',attemptId:'a2',runtime:'oc',delegationId:'dlg_1',modelId:'m2',funding:'byok',status:'unknown',inputTokens:0,outputTokens:0,cachedTokens:0,observedAt:'2026-09-12T08:01:00Z'},
  ]);
  assert.deepEqual(rows,[
    'main · m1 · platform · reported · in 10 / out 20',
    'oc（子执行 dlg_1）· m2 · byok · 用量待核对',
  ]);
});
