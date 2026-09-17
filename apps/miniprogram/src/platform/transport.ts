import type { HttpRequest, HttpResult, NativeMultipartFileRequest } from '@weknora/api-client';
import { Utf8Decoder } from '../core/utf8.ts';
export interface NativeResult { statusCode: number; header?: Record<string,unknown>; data: unknown }
interface NativeFailure { errMsg?: string }
interface HeaderEvent { header: Record<string,unknown>; statusCode?: number }
interface ChunkEvent { data: ArrayBuffer }
interface NativeTask {
  abort(): void;
  onHeadersReceived?(listener: (event: HeaderEvent) => void): void;
  offHeadersReceived?(listener: (event: HeaderEvent) => void): void;
  onChunkReceived?(listener: (event: ChunkEvent) => void): void;
  offChunkReceived?(listener: (event: ChunkEvent) => void): void;
  onProgressUpdate?(listener: (event: {totalBytesSent:number;totalBytesExpectedToSend:number}) => void): void;
}
interface NativeOptions {
  url:string; method:string; header:Record<string,string>; data?:unknown;
  responseType?:'text'|'arraybuffer'; dataType?:'json'|'text'; timeout:number; enableChunked?:boolean;
  success(result:NativeResult):void; fail(result:NativeFailure):void;
}
interface NativeUploadOptions {
  url:string; filePath:string; name:string; header:Record<string,string>; formData:Record<string,string>; timeout:number;
  success(result:NativeResult):void; fail(result:NativeFailure):void;
}
/** Injected so lifecycle/cancellation can be tested without the WeChat runtime. */
export interface WeappNetwork {
  request(options:NativeOptions):NativeTask;
  uploadFile(options:NativeUploadOptions):NativeTask;
}
export interface StreamMetadata { status:number|null; headers:Record<string,string> }
export class TransportFailure extends Error {
  code:string; status?:number;
  constructor(code:string,status?:number) { super(code);this.name='TransportFailure';this.code=code;this.status=status; }
}
function aborted():Error { const e=new Error('Request cancelled');e.name='AbortError';return e; }
function headers(input:Record<string,unknown> = {}):Record<string,string> {
  const out:Record<string,string>={};for(const [key,value] of Object.entries(input))out[key.toLowerCase()]=String(value);return out;
}
function normalize(result:NativeResult):HttpResult {
  if(!Number.isInteger(result.statusCode))throw new TransportFailure('INVALID_HTTP_STATUS');
  let body=result.data;
  if(typeof body==='string'){try{body=JSON.parse(body)}catch{/* Non-JSON is preserved for the shared error mapper. */}}
  return {status:result.statusCode,headers:headers(result.header),body};
}
const nativeHeaders=(request:HttpRequest)=>({...(request.body===undefined?{}:{'content-type':'application/json'}),...request.headers});
export function createWeappTransport(native:WeappNetwork) {
  function perform(request:HttpRequest,binary=false):Promise<HttpResult> {
    return new Promise((resolve,reject)=>{
      if(request.signal?.aborted){reject(aborted());return}
      let task:NativeTask|undefined,settled=false;
      const cleanup=()=>request.signal?.removeEventListener('abort',cancel);
      const done=(error?:unknown,value?:HttpResult)=>{if(settled)return;settled=true;cleanup();error?reject(error):resolve(value!)};
      const cancel=()=>{done(aborted());task?.abort()};
      try{
        task=native.request({url:request.url,method:request.method,header:nativeHeaders(request),data:request.body,
          responseType:binary?'arraybuffer':'text',dataType:binary?'text':'json',timeout:30_000,
          success:r=>{try{done(undefined,normalize(r))}catch(e){done(e)}},fail:()=>done(new TransportFailure('NETWORK_ERROR'))});
        if(!settled){request.signal?.addEventListener('abort',cancel,{once:true});if(request.signal?.aborted)cancel()}
      }catch(error){done(error)}
    });
  }
  function sendMultipartFile(request:NativeMultipartFileRequest):Promise<HttpResult> {
    return new Promise((resolve,reject)=>{
      if(request.method.toUpperCase()!=='POST'){reject(new TransportFailure('UPLOAD_METHOD_UNSUPPORTED'));return}
      if(request.signal?.aborted){reject(aborted());return}
      let task:NativeTask|undefined,settled=false;
      const cleanup=()=>request.signal?.removeEventListener('abort',cancel);
      const done=(error?:unknown,value?:HttpResult)=>{if(settled)return;settled=true;cleanup();error?reject(error):resolve(value!)};
      const cancel=()=>{done(aborted());task?.abort()};
      const header=Object.fromEntries(Object.entries(request.headers).filter(([k])=>k.toLowerCase()!=='content-type'));
      try{
        task=native.uploadFile({url:request.url,filePath:request.file.uri,name:'file',header,formData:request.fields,timeout:60_000,
          success:r=>{try{done(undefined,normalize(r))}catch(e){done(e)}},fail:()=>done(new TransportFailure('UPLOAD_FAILED'))});
        task.onProgressUpdate?.(p=>{if(!settled)request.onProgress?.({loaded:p.totalBytesSent,total:p.totalBytesExpectedToSend})});
        if(!settled){request.signal?.addEventListener('abort',cancel,{once:true});if(request.signal?.aborted)cancel()}
      }catch(error){done(error)}
    });
  }
  /**
   * Distinct from HttpTransport.sendStream: native headers may arrive before
   * statusCode. Metadata remains status:null until WeChat reports a real status.
   * Never fabricate 200 or replay a POST following an ambiguous interruption.
   */
  function stream(request:HttpRequest,onText:(text:string)=>void,onMetadata?:(meta:StreamMetadata)=>void):Promise<HttpResult> {
    return new Promise((resolve,reject)=>{
      if(request.signal?.aborted){reject(aborted());return}
      let task:NativeTask|undefined,settled=false,sawChunks=false,opened=false;
      let responseHeaders:Record<string,string>={},pending:ArrayBuffer[]=[],pendingBytes=0;
      const decoder=new Utf8Decoder();
      const isSSE=()=>/^text\/event-stream(?:\s*;|$)/i.test(responseHeaders['content-type']??'');
      const cleanup=()=>{clearTimeout(timer);request.signal?.removeEventListener('abort',cancel);task?.offHeadersReceived?.(onHeaders);task?.offChunkReceived?.(onChunk);pending=[]};
      const done=(error?:unknown,value?:HttpResult)=>{if(settled)return;settled=true;cleanup();error?reject(error):resolve(value!)};
      const fail=(error:unknown)=>{done(error);task?.abort()};
      const cancel=()=>fail(aborted());
      const emit=(data:ArrayBuffer)=>{const text=decoder.push(data);if(text)onText(text)};
      const flush=()=>{if(!isSSE())return;for(const chunk of pending)emit(chunk);pending=[];pendingBytes=0};
      const onHeaders=(event:HeaderEvent)=>{
        if(settled)return;
        try{
          if(Number.isInteger(event.statusCode)&&(event.statusCode!<200||event.statusCode!>=300))throw new TransportFailure(`HTTP_${event.statusCode}`,event.statusCode);
          responseHeaders=headers(event.header);opened=true;onMetadata?.({status:Number.isInteger(event.statusCode)?event.statusCode!:null,headers:responseHeaders});flush();
        }catch(error){fail(error)}
      };
      const onChunk=(event:ChunkEvent)=>{
        if(settled)return;sawChunks=true;
        try{
          if(event.data.byteLength>4*1024*1024)throw new TransportFailure('STREAM_CHUNK_TOO_LARGE');
          if(isSSE())emit(event.data);
          else if(!opened){pendingBytes+=event.data.byteLength;if(pendingBytes>65_536)throw new TransportFailure('STREAM_HEADERS_MISSING');pending.push(event.data)}
          // Known non-SSE chunks are never forwarded to the event consumer.
        }catch(error){fail(error)}
      };
      const timer=setTimeout(()=>fail(new TransportFailure('STREAM_TIMEOUT')),115_000);
      try{
        task=native.request({url:request.url,method:request.method,header:{...nativeHeaders(request),accept:'text/event-stream'},data:request.body,
          responseType:'arraybuffer',dataType:'text',enableChunked:true,timeout:110_000,
          success:r=>{
            if(settled)return;
            try{
              if(!Number.isInteger(r.statusCode))throw new TransportFailure('INVALID_HTTP_STATUS');
              if(r.statusCode<200||r.statusCode>=300)throw new TransportFailure(`HTTP_${r.statusCode}`,r.statusCode);
              responseHeaders={...responseHeaders,...headers(r.header)};
              if(!isSSE())throw new TransportFailure('EXPECTED_EVENT_STREAM',r.statusCode);
              flush();if(!sawChunks){if(r.data instanceof ArrayBuffer)emit(r.data);else if(typeof r.data==='string')onText(r.data)}
              decoder.finish();onMetadata?.({status:r.statusCode,headers:responseHeaders});done(undefined,{status:r.statusCode,headers:responseHeaders,body:undefined});
            }catch(error){done(error)}
          },fail:()=>done(new TransportFailure('STREAM_INTERRUPTED'))});
        if(!task.onChunkReceived||!task.onHeadersReceived){fail(new TransportFailure('Native chunk callbacks unavailable'));return}
        task.onHeadersReceived(onHeaders);task.onChunkReceived(onChunk);
        if(!settled){request.signal?.addEventListener('abort',cancel,{once:true});if(request.signal?.aborted)cancel()}
      }catch(error){done(error)}
    });
  }
  return {send:(r:HttpRequest)=>perform(r),sendBinary:(r:HttpRequest)=>perform(r,true),sendMultipartFile,stream};
}
