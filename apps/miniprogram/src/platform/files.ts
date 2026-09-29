import Taro from '@tarojs/taro';
import type { NativeFileSource } from '@weknora/api-client';
import { auth, apiOrigin, currentBearerToken } from '../services/runtime.ts';
import type { ScopeStamp } from '../core/scope.ts';
const MAX_BYTES=20*1024*1024; // Conservative client memory/transfer guard; server may impose a lower bound.
const MAX_DOWNLOAD_ERROR_BYTES=4096;
async function downloadErrorCode(filePath:string):Promise<string|undefined>{
 try{
  const {size}=await Taro.getFileInfo({filePath}) as {size:number};
  if(size<1)return undefined;
  const body=Taro.getFileSystemManager().readFileSync(filePath,'utf8',0,Math.min(size,MAX_DOWNLOAD_ERROR_BYTES));
  if(typeof body!=='string')return undefined;
  const parsed=JSON.parse(body) as {code?:unknown};
  return typeof parsed.code==='string'?parsed.code:undefined;
 }catch{return undefined}
}
// D3（task-6-live-validation）：downloadFile 落下的临时文件（http://tmp/…）在真实
// DevTools 运行时 unlink/unlinkSync 一律 permission denied，文件残留而 UI 承诺“临时
// 打开后立即清理”。运行时临时目录不归应用管理；应用可写可删的只有 USER_DATA_PATH。
// 因此受保护产物先复制成私有副本再打开，副本在打开结束后立即删除——UI 的清理承诺
// 对“我们创建的文件”真实成立。
// 注意：真实运行时的 Taro 对象没有 env 字段（@tarojs/api 的 Taro 无 env，weapp 插件
// 的 initNativeApi 也不复制 wx.env；类型声明里的 Taro.env.USER_DATA_PATH 是假的），
// USER_DATA_PATH 只能从 weapp 全局 wx.env 读取。
function userDir():string{
 const env=(globalThis as {wx?:{env?:{USER_DATA_PATH?:string}}}).wx?.env;
 const dir=env?.USER_DATA_PATH;
 if(!dir)throw new Error('本机存储目录不可用');
 return dir;
}
function toUserCopyPath(name:string):string{
 // openDocument 靠文件扩展名识别类型：随机后缀必须放在扩展名之前，副本必须以
 // 原始扩展名结尾（实测 DevTools 对无 .pdf 结尾的同内容文件报 filetype not supported）。
 const dot=name.lastIndexOf('.');
 const ext=dot>0?name.slice(dot+1).toLowerCase():'';
 const stem=(dot>0?name.slice(0,dot):name).replace(/[^A-Za-z0-9._-]+/g,'-').replace(/^[-.]+/,'')||'artifact';
 const suffix=`${Date.now().toString(36)}${Math.random().toString(36).slice(2,8)}`;
 return `${userDir()}/wk-open-${stem}-${suffix}${ext?`.${ext}`:''}`;
}
function copyFile(fs:ReturnType<typeof Taro.getFileSystemManager>,srcPath:string,destPath:string):Promise<void>{
 return new Promise((resolve,reject)=>fs.copyFile({srcPath,destPath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error('本机文件准备失败'),{cause:e}))}));
}
function unlinkFile(fs:ReturnType<typeof Taro.getFileSystemManager>,filePath:string):Promise<void>{
 return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error(`临时副本清理失败：${filePath}`),{cause:e}))}));
}
export async function chooseDocument():Promise<NativeFileSource>{
 const result=await Taro.chooseMessageFile({count:1,type:'file'});const f=result.tempFiles[0];if(!f)throw new Error('未选择文件');
 if(f.size>MAX_BYTES)throw new Error('客户端单文件上限为 20 MiB');
 return {uri:f.path,name:f.name,type:'application/octet-stream',size:f.size};
}
export async function openProtectedDocument(path:string,name:string,showMenu=false,expectedScope?:Readonly<ScopeStamp>):Promise<void>{
 if(!path.startsWith('/api/v1/')||path.includes('://')||path.includes('..'))throw new Error('不可信下载路径');
 const extension=name.split('.').pop()?.toLowerCase();if(!extension||!['pdf','doc','docx','xls','xlsx','ppt','pptx'].includes(extension))throw new Error('此格式请使用授权 Web 工作台查看');
 const stamp=expectedScope??auth.scope.capture();if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
 // Refresh once through the shared authorized transport. Capture scope before this await so
 // a response for the prior identity can never authorize a download in the newly active scope.
 const accessToken=await currentBearerToken();if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
 const controller=auth.scope.controller();
 const fs=Taro.getFileSystemManager();
 let userCopy='';let failure:{failed:boolean;error:unknown}={failed:false,error:undefined};
 try{
  const tempFilePath=await new Promise<string>((resolve,reject)=>{
    const task=Taro.downloadFile({url:apiOrigin+path,header:{Authorization:`Bearer ${accessToken}`},timeout:60000,
    success:r=>{
     // Completed HTTP responses can still materialize an error body locally. Own it before
     // validating status so the failure path reads its error code like a success path would.
     if(r.statusCode!==200){
      void downloadErrorCode(r.tempFilePath).then(responseCode=>{
       const code=r.statusCode===401
        ?responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID'
        :undefined;
       reject(Object.assign(new Error('下载失败'),{status:r.statusCode,...(code?{code}:{})}));
      });
      return;
     }
     resolve(r.tempFilePath);
    },fail:reject});
   const stop=()=>task.abort();controller.signal.addEventListener('abort',stop,{once:true});
   task.onProgressUpdate(p=>{if(p.totalBytesWritten>MAX_BYTES)task.abort()});
  });
  if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
  const info=await Taro.getFileInfo({filePath:tempFilePath}) as {size:number};if(info.size>MAX_BYTES)throw new Error('文件超出本端查看上限');
  if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
  userCopy=toUserCopyPath(name);
  await copyFile(fs,tempFilePath,userCopy);
  if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
  await Taro.openDocument({filePath:userCopy,showMenu});
 }catch(e){failure={failed:true,error:e}}
 finally{
  auth.scope.release(controller);
  if(userCopy){
   // 清理失败不静默：UI 承诺立即清理，违背承诺必须可见；已有业务失败时保留原始错误。
   try{await unlinkFile(fs,userCopy)}
   catch(cleanupError){if(!failure.failed)failure={failed:true,error:cleanupError}}
  }
 }
 if(failure.failed)throw failure.error;
}
