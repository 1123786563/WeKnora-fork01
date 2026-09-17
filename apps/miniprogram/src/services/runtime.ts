import '../platform/polyfills.ts';
import Taro from '@tarojs/taro';
import { createWeKnoraClient, createExecutionsApi, createServerSentEventParser, parseChatEvent, buildChatStreamRequest } from '@weknora/api-client';
import type { HttpRequest, HttpResult, NativeMultipartFileRequest } from '@weknora/api-client';
import type { ChatStreamEvent } from '@weknora/contracts';
import { AuthCoordinator } from '../core/auth.ts';
import { storage, clearPrivateCache } from '../platform/storage.ts';
import { createWeappTransport, type WeappNetwork } from '../platform/transport.ts';
const origin=__API_ORIGIN__;
const network:WeappNetwork={
  request:options=>Taro.request(options as Parameters<typeof Taro.request>[0]),
  uploadFile:options=>Taro.uploadFile(options as Parameters<typeof Taro.uploadFile>[0]),
};
const native=createWeappTransport(network);
let coordinator:AuthCoordinator;
function authenticated<T extends HttpRequest>(request:T):T{
  if(!origin)throw new Error('API_ORIGIN_MISSING');
  if(!request.url.startsWith(origin+'/'))throw new Error('Untrusted API origin');
  const credential=coordinator?.credential();
  return {...request,headers:{...request.headers,...(credential?.kind==='bearer'?{Authorization:`Bearer ${credential.accessToken}`}:{})}};
}
const raw=createWeKnoraClient({baseURL:origin,transport:{send:r=>native.send(authenticated(r))}});
coordinator=new AuthCoordinator(origin,storage,raw.auth);
export const auth=coordinator;
async function scoped<T extends HttpRequest>(input:T,send:(request:T)=>Promise<HttpResult>):Promise<HttpResult>{
  if(auth.snapshot().phase!=='ready')throw new Error('AUTH_REQUIRED');
  const stamp=auth.scope.capture(),controller=auth.scope.controller();
  const cancel=()=>controller.abort();
  if(input.signal?.aborted)controller.abort();else input.signal?.addEventListener('abort',cancel,{once:true});
  try{
    const request={...input,signal:controller.signal};
    let result=await send(authenticated(request));
    if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
    if(result.status===401&&input.method==='GET'){
      await auth.refresh(stamp);
      if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
      result=await send(authenticated(request));
    }
    if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
    return result;
  }finally{input.signal?.removeEventListener('abort',cancel);auth.scope.release(controller)}
}
export const client=createWeKnoraClient({baseURL:origin,transport:{
  send:r=>scoped(r,native.send),sendBinary:r=>scoped(r,native.sendBinary),
  sendMultipartFile:r=>scoped(r as NativeMultipartFileRequest,native.sendMultipartFile),
}});
export const executions=createExecutionsApi(client.request);
export async function stream(input:HttpRequest,onText:(text:string)=>void):Promise<void>{
  if(auth.snapshot().phase!=='ready')throw new Error('AUTH_REQUIRED');
  const stamp=auth.scope.capture(),controller=auth.scope.controller();
  const cancel=()=>controller.abort();
  if(input.signal?.aborted)controller.abort();else input.signal?.addEventListener('abort',cancel,{once:true});
  try{
    await native.stream(authenticated({...input,signal:controller.signal}),text=>{
      if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');onText(text);
    });
  }finally{input.signal?.removeEventListener('abort',cancel);auth.scope.release(controller)}
}
export async function chatStream(options:Parameters<typeof buildChatStreamRequest>[0],onEvent:(event:ChatStreamEvent)=>void):Promise<void>{
  const req=buildChatStreamRequest(options),parser=createServerSentEventParser(frame=>onEvent(parseChatEvent(frame)));
  await stream({...req,url:origin+req.path,headers:req.headers??{}},text=>parser.push(text));parser.finish();
}
export const apiOrigin=origin;
export async function logout():Promise<void>{await auth.logout();clearPrivateCache()}
export function stopSubscriptions():void{auth.scope.abortAll()}
