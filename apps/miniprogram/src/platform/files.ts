import Taro from '@tarojs/taro';
import type { NativeFileSource } from '@weknora/api-client';
import { auth, apiOrigin } from '../services/runtime.ts';
import type { ScopeStamp } from '../core/scope.ts';
const MAX_BYTES=20*1024*1024; // Conservative client memory/transfer guard; server may impose a lower bound.
export async function chooseDocument():Promise<NativeFileSource>{
 const result=await Taro.chooseMessageFile({count:1,type:'file'});const f=result.tempFiles[0];if(!f)throw new Error('未选择文件');
 if(f.size>MAX_BYTES)throw new Error('客户端单文件上限为 20 MiB');
 return {uri:f.path,name:f.name,type:'application/octet-stream',size:f.size};
}
export async function openProtectedDocument(path:string,name:string,showMenu=false,expectedScope?:Readonly<ScopeStamp>):Promise<void>{
 if(!path.startsWith('/api/v1/')||path.includes('://')||path.includes('..'))throw new Error('不可信下载路径');
 const extension=name.split('.').pop()?.toLowerCase();if(!extension||!['pdf','doc','docx','xls','xlsx','ppt','pptx'].includes(extension))throw new Error('此格式请使用授权 Web 工作台查看');
 const credential=auth.credential();if(credential.kind!=='bearer')throw new Error('AUTH_REQUIRED');
 const stamp=expectedScope??auth.scope.capture();if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');const controller=auth.scope.controller();let filePath='';
 try{
  filePath=await new Promise<string>((resolve,reject)=>{
    const task=Taro.downloadFile({url:apiOrigin+path,header:{Authorization:`Bearer ${credential.accessToken}`},timeout:60000,
    success:r=>{
     // Completed HTTP responses can still materialize an error body locally. Own it before
     // validating status so the finally block removes it just like a successful download.
     filePath=r.tempFilePath;
     if(r.statusCode!==200){reject(Object.assign(new Error('下载失败'),{status:r.statusCode,...(r.statusCode===401?{code:'ARTIFACT_GRANT_EXPIRED'}:{})}));return}
     resolve(r.tempFilePath);
    },fail:reject});
   const stop=()=>task.abort();controller.signal.addEventListener('abort',stop,{once:true});
   task.onProgressUpdate(p=>{if(p.totalBytesWritten>MAX_BYTES)task.abort()});
  });
  if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
  const info=await Taro.getFileInfo({filePath}) as {size:number};if(info.size>MAX_BYTES)throw new Error('文件超出本端查看上限');
  if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
  await Taro.openDocument({filePath,showMenu});
 }finally{auth.scope.release(controller);if(filePath){try{Taro.getFileSystemManager().unlinkSync(filePath)}catch{/* temporary file may already be removed */}}}
}
