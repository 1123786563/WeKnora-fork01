import test from 'node:test';
import assert from 'node:assert/strict';
import { createServerSentEventParser } from './stream.ts';
import { createChatAttachmentsApi } from './attachments.ts';
import { createKnowledgeDocumentsApi } from '../knowledge/documents.ts';
import { createWeKnoraClient } from '../client.ts';

test('mini: CRLF split between chunks cannot prematurely dispatch an SSE frame', () => {
  const events: {data:string}[] = [];
  const parser = createServerSentEventParser(event => events.push(event));
  parser.push('data: one\r'); parser.push('\ndata: two\r');
  assert.equal(events.length, 0);
  parser.push('\n\r'); parser.push('\n');
  assert.deepEqual(events.map(event => event.data), ['one\ntwo']);
});
test('mini: every character split produces identical SSE frames', () => {
  const wire = ': heartbeat\r\nid: 12\r\nevent: answer\r\ndata: 你好\r\ndata: 🌱\r\n\r\n';
  for(let at=0;at<=wire.length;at++) {
    const events:unknown[]=[];const parser=createServerSentEventParser(e=>events.push(e));
    parser.push(wire.slice(0,at));parser.push(wire.slice(at));
    assert.deepEqual(events,[{id:'12',event:'answer',data:'你好\n🌱'}]);
  }
});
test('mini: incomplete input is not dispatched by push',()=>{
  const events:unknown[]=[];const parser=createServerSentEventParser(e=>events.push(e));
  parser.push('data: {"partial":');assert.equal(events.length,0);
});

test('mini: native knowledge upload does not instantiate FormData',async()=>{
  const Original=globalThis.FormData;
  globalThis.FormData=class {constructor(){throw new Error('FormData must not be constructed')}} as unknown as typeof FormData;
  try {
    let called=false;
    const api=createKnowledgeDocumentsApi(async input=>{
      called=true;assert.equal(input.body,undefined);assert.equal(input.nativeFile?.uri,'wxfile://file');
      assert.equal(input.multipartFields?.channel,'miniprogram');
      return {success:true,data:{id:'document-1'}};
    });
    await api.upload('kb-1',{file:{uri:'wxfile://file',name:'a.pdf',type:'application/pdf'},channel:'miniprogram'});
    assert.equal(called,true);
  } finally {globalThis.FormData=Original;}
});
test('mini: native chat attachment preserves native file and fields',async()=>{
  const Original=globalThis.FormData;
  globalThis.FormData=class {constructor(){throw new Error('FormData must not be constructed')}} as unknown as typeof FormData;
  const sentinel=new Error('request inspected');
  try {
    const api=createChatAttachmentsApi(async input=>{
      assert.equal(input.body,undefined);assert.equal(input.nativeFile?.uri,'wxfile://file');
      assert.equal(input.multipartFields?.agent_id,'agent-1');throw sentinel;
    });
    await assert.rejects(api.upload('session-1',{file:{uri:'wxfile://file',name:'a.pdf',type:'application/pdf'},agentId:'agent-1'}),error=>error===sentinel);
  } finally {globalThis.FormData=Original;}
});
test('mini: shared client selects native transport before creating browser body',async()=>{
  const Original=globalThis.FormData;
  globalThis.FormData=class {constructor(){throw new Error('FormData must not be constructed')}} as unknown as typeof FormData;
  try {
    let uploads=0;
    const client=createWeKnoraClient({baseURL:'https://test.invalid',transport:{
      async send(){throw new Error('wrong transport')},
      async sendMultipartFile(input){uploads++;assert.equal(input.fields.channel,'miniprogram');return {status:200,headers:{},body:{success:true}}},
    }});
    await client.request({method:'POST',path:'/upload',nativeFile:{uri:'wxfile://file',name:'a.pdf',type:'application/pdf'},multipartFields:{channel:'miniprogram'}});
    assert.equal(uploads,1);
  } finally {globalThis.FormData=Original;}
});
test('mini: native upload without native transport fails closed',async()=>{
  const client=createWeKnoraClient({baseURL:'https://test.invalid',transport:{async send(){throw new Error('must not fall back')}}});
  await assert.rejects(client.request({method:'POST',path:'/upload',nativeFile:{uri:'wxfile://file',name:'a.pdf',type:'application/pdf'}}),/Native multipart transport is unavailable/);
});
