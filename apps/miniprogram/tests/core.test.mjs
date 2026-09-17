import test from 'node:test';
import assert from 'node:assert/strict';
// 导入失败必须带出真实原因；任何模块求值错误直接失败整个测试文件。
const format = await import(`../src/core/format.ts`);
const scope = await import(`../src/core/scope.ts`);
const utf8 = await import(`../src/core/utf8.ts`);
const execution = await import(`../src/core/execution.ts`);
const intent = await import(`../src/core/intent.ts`);
const routing = await import(`../src/core/routes.ts`);

test('integer formatting never rounds values larger than Number.MAX_SAFE_INTEGER', () => {
  assert.equal(typeof format.formatCredits, 'function');
  assert.equal(format.formatCredits('90071992547409931'), '90,071,992,547,409,931');
  assert.equal(format.formatMoney('90071992547409931'), '900,719,925,474,099.31');
  assert.equal(format.formatMoney('1'), '0.01');
  assert.throws(() => format.formatMoney('NaN'));
});
test('scope transition aborts old requests and refuses stale commits', () => {
  assert.equal(typeof scope.ScopeGuard, 'function');
  const g = new scope.ScopeGuard({ origin:'https://api.example.test',userId:'u',tenantId:'1' });
  const before=g.capture(); const c=g.controller(); let saved=false;
  g.switchTo({origin:'https://api.example.test',userId:'u',tenantId:'2'});
  assert.equal(c.signal.aborted,true);
  assert.equal(g.commit(before,()=>{saved=true}),false);
  assert.equal(saved,false);
  assert.notEqual(scope.scopeKey(before),scope.scopeKey(g.capture()));
});
test('UTF-8 decoder works for every byte split, including surrogate-pair characters', () => {
  assert.equal(typeof utf8.Utf8Decoder,'function');
  const bytes=new TextEncoder().encode('中文😀\n任务 €𠀀');
  for(let split=0;split<=bytes.length;split++){
    const d=new utf8.Utf8Decoder();
    assert.equal(d.push(bytes.slice(0,split))+d.push(bytes.slice(split))+d.finish(),'中文😀\n任务 €𠀀');
  }
  const d=new utf8.Utf8Decoder(); let out=''; for(const b of bytes)out+=d.push(Uint8Array.of(b));
  assert.equal(out+d.finish(),'中文😀\n任务 €𠀀');
});
test('UTF-8 decoder rejects malformed and truncated input rather than inventing characters',()=>{
  assert.equal(typeof utf8.Utf8Decoder,'function');
  assert.throws(()=>new utf8.Utf8Decoder().push(Uint8Array.of(0xc0,0xaf)));
  const d=new utf8.Utf8Decoder();d.push(Uint8Array.of(0xf0,0x9f));assert.throws(()=>d.finish());
});
const run={schema_version:1,run_id:'r',session_id:'s',revision:4,driver:'platform',run_status:'running',execution_status:'running',settlement_status:'pending',seq:7,capabilities:{}};
const event=seq=>({schema_version:1,run_id:'r',attempt_id:'a',seq,type:'progress',occurred_at:'2026-09-17T00:00:00Z',payload:{summary:'working'}});
test('snapshot watermark starts replay correctly and rejects gaps or foreign runs',()=>{
  assert.equal(typeof execution.installSnapshot,'function');
  const p=execution.installSnapshot({execution:run,watermark:7,events:[event(7)]},'r');
  const next=execution.appendEvent(p,event(8));assert.equal(next.cursor,8);
  assert.equal(execution.appendEvent(next,event(8)),next);
  assert.throws(()=>execution.appendEvent(next,event(10)),/gap/i);
  assert.throws(()=>execution.appendEvent(next,{...event(9),run_id:'other'}),/run/i);
});
test('snapshot installation cannot include events ahead of its watermark',()=>{
  assert.equal(typeof execution.installSnapshot,'function');
  assert.throws(()=>execution.installSnapshot({execution:run,watermark:6,events:[event(7)]},'r'));
});
test('event cursor is not committed when persistence fails',()=>{
  assert.equal(typeof execution.persistEvent,'function');
  const p=execution.installSnapshot({execution:run,watermark:7,events:[]},'r');
  assert.throws(()=>execution.persistEvent(p,event(8),()=>{throw Error('quota')}));
  assert.equal(p.cursor,7);
});
test('pending submission reuses the same id and unknown lookup never releases the intent',()=>{
  assert.equal(typeof intent.PendingIntent,'function');
  const m=new Map(); const store={read:k=>m.get(k),write:(k,v)=>m.set(k,v),remove:k=>m.delete(k)};
  let n=0; const p=new intent.PendingIntent(store,'tenant1',()=>`req-${++n}`);
  assert.equal(p.begin().requestId,p.begin().requestId);
  p.reconcile({state:'unknown'});assert.equal(p.begin().requestId,'req-1');
  p.reconcile({state:'admitted',run_id:'r'});assert.equal(p.current().runId,'r');
  assert.throws(()=>p.reset(),/active/i);
  p.acknowledge();assert.equal(p.begin().requestId,'req-2');
});
test('route manifest has twenty screens and only four primary tabs',()=>{
  assert.ok(routing.ROUTES);
  assert.equal(Object.keys(routing.ROUTES).length,20);
  assert.equal(Object.values(routing.ROUTES).filter(x=>x.tab).length,4);
  assert.match(routing.pageUrl('document',{id:'a/b?c'}),/a%2Fb%3Fc/);
  assert.throws(()=>routing.pageUrl('not-a-page'));
});
