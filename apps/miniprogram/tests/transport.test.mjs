import test from 'node:test';
import assert from 'node:assert/strict';
// 导入失败必须带出真实原因；任何模块求值错误直接失败整个测试文件。
const mod=await import('../src/platform/transport.ts');
function native(){
 const n={opts:null,headers:null,chunk:null,aborts:0,request(opts){n.opts=opts;return {abort(){n.aborts++},onHeadersReceived(fn){n.headers=fn},offHeadersReceived(){n.headers=null},onChunkReceived(fn){n.chunk=fn},offChunkReceived(){n.chunk=null}}},uploadFile(opts){n.upload=opts;return {abort(){n.aborts++},onProgressUpdate(fn){n.progress=fn}}}};return n;
}
const req={method:'GET',url:'https://api.example.test/api/v1/test',headers:{}};
test('request preserves real status and normalizes headers',async()=>{
const n=native(),t=mod.createWeappTransport(n);const p=t.send(req);
 n.opts.success({statusCode:403,header:{'Content-Type':'application/json'},data:{error:'forbidden'}});
 const r=await p;assert.equal(r.status,403);assert.equal(r.headers['content-type'],'application/json');
});
test('cancellation aborts native task and rejects once',async()=>{
const n=native(),t=mod.createWeappTransport(n),c=new AbortController();const p=t.send({...req,signal:c.signal});c.abort();
 await assert.rejects(p,e=>e.name==='AbortError');assert.equal(n.aborts,1);
});
test('native upload carries filePath and fields without browser FormData',async()=>{
const n=native(),t=mod.createWeappTransport(n);
 const p=t.sendMultipartFile({...req,method:'POST',file:{uri:'wxfile://a',name:'a.pdf',type:'application/pdf'},fields:{channel:'mini'}});
 assert.equal(n.upload.filePath,'wxfile://a');assert.equal(n.upload.name,'file');assert.equal(n.upload.formData.channel,'mini');
 n.upload.success({statusCode:200,header:{},data:'{"success":true}'});assert.deepEqual((await p).body,{success:true});
});
test('stream metadata may be unknown before the real final HTTP status',async()=>{
const n=native(),t=mod.createWeappTransport(n);let text='',meta;
 const p=t.stream(req,s=>{text+=s},m=>{meta=m});
 n.headers({header:{'Content-Type':'text/event-stream; charset=utf-8'}});
 const bytes=new TextEncoder().encode('data: 中文😀\r\n\r\n');for(const b of bytes)n.chunk({data:Uint8Array.of(b).buffer});
 assert.equal(meta.status,null);assert.equal(text,'data: 中文😀\r\n\r\n');
 n.opts.success({statusCode:200,header:{'Content-Type':'text/event-stream'},data:new ArrayBuffer(0)});
 assert.equal((await p).status,200);
});
test('non-SSE data never reaches the event consumer',async()=>{
const n=native(),t=mod.createWeappTransport(n);let count=0;
 const p=t.stream(req,()=>count++);n.headers({header:{'Content-Type':'application/json'}});
 n.chunk({data:new TextEncoder().encode('{"error":"expired"}').buffer});
 n.opts.success({statusCode:401,header:{'Content-Type':'application/json'},data:{error:'expired'}});
 await assert.rejects(p,e=>e.status===401);assert.equal(count,0);
});
test('stream rejects data loss when callback protocol is unsupported',async()=>{
const t=mod.createWeappTransport({request:()=>({abort(){}}),uploadFile:()=>({abort(){}})});
 await assert.rejects(t.stream(req,()=>{}),/chunk/i);
});

test('cancel remains AbortError when native abort synchronously calls fail',async()=>{
  const controller=new AbortController();
  const native={request:o=>({abort(){o.fail({errMsg:'abort'})}}),uploadFile(){throw new Error('unused')}};
  const pending=mod.createWeappTransport(native).send({method:'GET',url:'https://example.test',headers:{},signal:controller.signal});
  controller.abort();await assert.rejects(pending,{name:'AbortError'});
});
test('known non-2xx stream headers reject before forwarding any content',async()=>{
 const n=native(),t=mod.createWeappTransport(n);let forwarded=false;
 const p=t.stream(req,()=>{forwarded=true});
 n.headers({statusCode:401,header:{'Content-Type':'text/event-stream'}});
 // Complete the native request as well so the unfixed implementation settles.
 n.opts.success({statusCode:401,header:{'Content-Type':'text/event-stream'},data:new ArrayBuffer(0)});
 await assert.rejects(p,e=>e.status===401);assert.equal(forwarded,false);assert.equal(n.aborts,1);
});
