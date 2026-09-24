import { Text, View } from '@tarojs/components';
import { Screen, Card, Action, Notice, Empty, DataBoundary, useData, useAction } from '../../../components/ui.tsx';
import { auth } from '../../../services/runtime.ts';
import { listTaskArtifacts, taskArtifactDownloadPath } from '../../../services/workbench.ts';
import { openProtectedDocument } from '../../../platform/files.ts';
import { routeParam, navigate } from '../../../platform/navigation.ts';
import { formatBytes, formatTime } from '../../../core/format.ts';

const supported=(name:string,mime:string)=>/^application\/pdf$|officedocument\.wordprocessingml\.document$/i.test(mime)||/\.(pdf|docx)$/i.test(name);
/** Task-owned artifacts stay in the mini-program and only download on explicit tap. */
export default function ArtifactPage(){
 const run=routeParam('run');
 const query=useData(`task-artifacts:${run}`,()=>listTaskArtifacts(run),!!run);
 const action=useAction();
 return <Screen title='任务产物'>
  <Text className='wk-display'>把完成的工作，{ '\n' }留在手边。</Text>
  {!run&&<Empty title='缺少任务信息' body='请从任务详情重新打开产物列表。' action='返回任务' onAction={()=>void navigate('tasks')}/>}
  {run&&<DataBoundary state={query}>{items=><>
   {items.map(file=><Card key={`${file.index}:${file.id}`}>
    <Text className='wk-h3'>{file.name}</Text>
    <Text className='wk-muted wk-small'>{file.mime} · {formatBytes(file.size)}{file.createdAt?` · ${formatTime(file.createdAt)}`:''}</Text>
    <View className='wk-tdesign-scope'>
    {/*
      必须用 JSX 而非 createElement('t-button', …)（t-button.d.ts 有根因备注）：
      只有 JSX 产物 `_jsx("t-button", …)` 会被 Taro 构建期收集属性与事件，
      base.wxml 的 t-button 模板才会带上 bindtap="eh" 和属性绑定（D2 修复）。
    */}
    <t-button
     block
     disabled={action.busy || !supported(file.name,file.mime)}
     loading={action.busy}
     theme='primary'
     size='large'
     ariaLabel={supported(file.name,file.mime)?`打开或保存 ${file.name}`:`${file.name} 暂不支持`}
     customStyle='--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);'
     onTap={()=>void action.run(async()=>{
     const stamp=auth.scope.capture();
     const path=await taskArtifactDownloadPath(run,file);
     if(!auth.scope.isCurrent(stamp))throw new Error('SCOPE_CHANGED');
     await openProtectedDocument(path,file.name,true,stamp);
    })}>
     {supported(file.name,file.mime)?'打开或保存':'此格式暂不支持'}
    </t-button>
    </View>
   </Card>)}
   {!items.length&&<Empty title='暂时没有产物' body='任务可能仍在进行，或尚未生成 PDF / DOCX 文件。'/>}
  </>}</DataBoundary>}
  {action.error&&<Notice tone='danger'>{action.error} 授权过期时再次点击“打开或保存”会重新获取。</Notice>}
  {run&&<Action secondary onClick={query.reload}>重新读取</Action>}
  <Notice tone='info'>产物只在当前账号与空间的服务端权限允许时显示。授权链接仅在本次操作中使用；文件在本机临时打开后立即清理，不会写入持久缓存。</Notice>
  <Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>
 </Screen>;
}
