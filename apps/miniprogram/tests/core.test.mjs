import test from 'node:test';
import assert from 'node:assert/strict';
// 导入失败必须带出真实原因；任何模块求值错误直接失败整个测试文件。
const format = await import(`../src/core/format.ts`);
const scope = await import(`../src/core/scope.ts`);
const utf8 = await import(`../src/core/utf8.ts`);
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
test('intent request IDs are unique correlations, not credentials',()=>{
  assert.equal(typeof intent.requestId,'function');
  const first = intent.requestId();
  const second = intent.requestId();
  assert.match(first, /^mini-/);
  assert.notEqual(first, second);
});
test('route manifest has twenty-five screens and only four primary tabs',()=>{
  assert.ok(routing.ROUTES);
  assert.equal(Object.keys(routing.ROUTES).length,25); // T24 新增 career/discovery；T26 新增 career/application-material；T32 新增 career/export-deletion；T28 新增 career/progress-preparation；OCR high-9 补 career/rules-usage-reminders（此前注册于 app.config 却无路由键不可达）
  assert.equal(Object.values(routing.ROUTES).filter(x=>x.tab).length,4);
  assert.match(routing.pageUrl('document',{id:'a/b?c'}),/a%2Fb%3Fc/);
  assert.throws(()=>routing.pageUrl('not-a-page'));
});
