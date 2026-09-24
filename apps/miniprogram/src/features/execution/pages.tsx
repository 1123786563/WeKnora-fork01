import { useEffect, useRef, useState, useCallback } from 'react';
import { View, Text } from '@tarojs/components';
import { useDidHide, useDidShow } from '@tarojs/taro';
import { timelineKindLabel } from '@weknora/mobile-core';
import type { AttentionDecisionReceipt, TaskDetailView, TaskListPage, TaskStartReceipt } from '@weknora/mobile-core';
import { Screen, Card, Action, Field, Notice, Section, ListRow, Badge, Empty, useData, DataBoundary, useAction, confirmAction } from '../../components/ui.tsx';
import { auth, executions } from '../../services/runtime.ts';
import { requireTaskOffice, openActiveMaterial, resolveTaskForRun } from '../../services/mobile-office.ts';
import { runStatusLabels, runStatusBadgeTone, connectionLabel, interruptionNotice, inboxVisibleItems, decisionReceiptText, materialEntryRow } from '../../services/office-views.ts';
import { navigate, routeParam } from '../../platform/navigation.ts';
import { errorMessage } from '../../core/errors.ts';
import { formatTime } from '../../core/format.ts';

const openTask=(taskId:string,runId:string)=>navigate('execution',{id:runId,task:taskId});

export function TasksPage(){
 const [filter,setFilter]=useState<''|'running'|'waiting_user'|'succeeded'>('');const [pages,setPages]=useState<TaskListPage[]>([]);const [lookupId,setLookupId]=useState('');const [receipt,setReceipt]=useState<string>();const action=useAction();
 const home=useData('tasks-home',()=>requireTaskOffice().home());
 const query=useData(`tasks:${filter}`,async()=>{const office=requireTaskOffice();const first=await office.tasks(filter?{status:filter}:{});setPages([first]);return first});
 const more=useAction();
 const pending=useData('tasks-pending',()=>requireTaskOffice().reconcilePending());
 return <Screen title='任务' tab><View className='wk-between'><Text className='wk-display'>工作正在向前。</Text><Action secondary onClick={()=>void navigate('agents')}>新任务</Action></View>
 <DataBoundary state={home}>{h=><>{h.needsMe.length>0&&<Card tone='warning'><Text className='wk-h3'>{h.needsMe.length} 项待你确认</Text><Action secondary onClick={()=>void navigate('approval')}>打开确认收件箱</Action></Card>}
 <Section title='正在运行'/>{h.running.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone={runStatusBadgeTone(item.runStatus)}>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}
 <Section title='最近完成'/>{h.recentlyCompleted.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone='neutral'>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}</>}</DataBoundary>
 <View className='wk-filters'>{([['','全部'],['running','运行中'],['waiting_user','待确认'],['succeeded','已完成']] as const).map(([id,name])=><Action key={id} secondary={filter!==id} onClick={()=>{setFilter(id);setPages([])}}>{name}</Action>)}</View>
 <DataBoundary state={query}>{first=><>{[...pages.flatMap(page=>page.items)].length?pages.flatMap((page,pi)=><View key={pi}>{page.items.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone={runStatusBadgeTone(item.runStatus)}>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}</View>):first.items.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone={runStatusBadgeTone(item.runStatus)}>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}
 {(pages.at(-1)?.nextCursor??first.nextCursor)&&<Action secondary loading={more.busy} onClick={()=>void more.run(async()=>{const page=await requireTaskOffice().moreTasks();setPages(p=>[...p,page])})}>加载更多</Action>}
 {!pages.length&&!first.items.length&&<Empty title='当前筛选下暂无任务'/>}</>}</DataBoundary>
 <DataBoundary state={pending}>{receipts=>receipts.filter(item=>!item.runId).map(item=><Card key={item.requestId} tone='warning'><Text className='wk-h3'>有一项提交仍待核对</Text><Text className='wk-muted'>{item.requestId}</Text><Action secondary loading={action.busy} onClick={()=>void action.run(async()=>{const after=await requireTaskOffice().reconcilePending();const admitted=after.find(r=>r.requestId===item.requestId&&r.runId);if(admitted){const resolved=await resolveTaskForRun(admitted.runId!);await openTask(resolved.taskId,admitted.runId!)}query.reload()})}>查询原请求</Action></Card>)}</DataBoundary>
 <Card><Field label='通过运行 ID 恢复' value={lookupId} onChange={setLookupId}/><Action secondary disabled={!lookupId.trim()} loading={action.busy} onClick={()=>void action.run(async()=>{const resolved=await resolveTaskForRun(lookupId.trim());await openTask(resolved.taskId,resolved.runId)})}>读取任务状态</Action><Text className='wk-muted wk-small'>服务端仍会验证任务归属，不会因知道 ID 而获得访问权限。</Text></Card>
 {receipt&&<Notice>{receipt}</Notice>}{action.error&&<Notice tone='danger'>{action.error}</Notice>}{more.error&&<Notice tone='danger'>{more.error}</Notice>}</Screen>;
}

export function ExecutionPage(){
 const runId=routeParam('id'),taskIdParam=routeParam('task');const [view,setView]=useState<TaskDetailView>();const [error,setError]=useState<string>();const [instruction,setInstruction]=useState('');const action=useAction();const handle=useRef<ReturnType<ReturnType<typeof requireTaskOffice>['open']>>();
 const stop=useCallback(()=>{handle.current?.close('page-hidden');handle.current=undefined},[]);
 const reload=useCallback(async()=>{stop();setError(undefined);
  try{
   const office=requireTaskOffice();
   const taskId=taskIdParam||(await resolveTaskForRun(runId)).taskId;
   const opened=office.open({taskId,runId});handle.current=opened;
   const off=opened.updates(next=>setView(next));
   const initial=await opened.hydrate();setView(initial);
   return()=>off();
  }catch(e){setError(errorMessage(e));return undefined}
 },[runId,taskIdParam]);
 useEffect(()=>{const off=reload().then(cleanup=>cleanup).catch(()=>undefined);return()=>{void off.then(cleanup=>cleanup&&cleanup());stop()}},[reload]);useDidShow(()=>{void reload()});useDidHide(stop);
 return <Screen title='执行详情'>{view?<><Card tone='mint'><View className='wk-between'><Badge tone={runStatusBadgeTone(view.runStatus)}>{runStatusLabels[view.runStatus]??view.runStatus}</Badge><Text className='wk-muted wk-small'>{connectionLabel(view.connection)}</Text></View><Text className='wk-display'>每一步，都有迹可循。</Text><Text className='wk-mono'>{runId}</Text><View className='wk-meta'><Text>任务：{view.title||view.taskId}</Text><Text>生命周期：{view.lifecycle}</Text><Text>执行：{view.executionStatus}</Text><Text>结算：{view.settlementStatus}</Text><Text>已确认事件：#{view.cursor}</Text></View></Card>
 {view.attention==='required'&&<Card tone='warning'><Text className='wk-h2'>任务需要你的确认</Text><Text className='wk-muted'>核对动作与影响后，再决定如何继续。</Text><Action onClick={()=>void navigate('approval',{run:runId})}>查看待处理事项</Action></Card>}
 {view.interruption&&<Notice tone='warning'>{interruptionNotice(view.interruption.reason)}</Notice>}
 <Section title='执行时间线'/><View className='wk-timeline'>{view.timeline.map(entry=><View className='wk-step' key={entry.seq}><Text className='wk-h3'>{timelineKindLabel(entry.kind)}</Text><Text className='wk-muted wk-small'>{formatTime(entry.occurredAt)} · #{entry.seq}</Text></View>)}</View>
 <Card><Field label='追加指令' value={instruction} onChange={setInstruction} multiline placeholder='补充要求，不会重建任务'/><Action loading={action.busy} disabled={!instruction.trim()||view.runStatus==='succeeded'||view.runStatus==='failed'||view.runStatus==='canceled'} onClick={()=>void action.run(async()=>{await executions.command(runId,{action:'steer',text:instruction.trim(),expected_revision:view.revision});setInstruction('');const cleanup=await reload();cleanup&&cleanup()})}>提交追加指令</Action><Text className='wk-muted wk-small'>追加/取消命令暂经授权 API 直发；统一的 Task 意图通道属后续 Issue。</Text></Card>
 <View className='wk-grid'><Action secondary onClick={()=>void navigate('artifact',{session:view.taskId,run:runId})}>查看材料</Action><Action danger disabled={view.runStatus==='succeeded'||view.runStatus==='failed'||view.runStatus==='canceled'} loading={action.busy} onClick={()=>void action.run(async()=>{if(!await confirmAction('取消这个任务？','这会向服务端发送取消命令，已发生的消耗不会因此自动退还。'))return;await executions.command(runId,{action:'cancel',expected_revision:view.revision});const cleanup=await reload();cleanup&&cleanup()})}>取消任务</Action></View></>:<Empty title='正在读取服务端快照' body='不会为了恢复页面重新启动任务。'/>}{error&&<Notice tone='warning'>{error}</Notice>}{action.error&&<Notice tone='danger'>{action.error}</Notice>}<Action secondary onClick={()=>void reload()}>重新同步快照</Action></Screen>;
}

export function ApprovalPage(){
 const runId=routeParam('run');const query=useData(`inbox:${runId}`,async()=>{const view=await requireTaskOffice().inbox();return inboxVisibleItems(view.items,runId||undefined)});const [receipt,setReceipt]=useState<string>();const action=useAction();
 const decide=async(item:{interactionId:string;runId:string;kind:'tool_approval'|'budget'|'recovery';argsHash:string;expectedRevision:number;createdAt:string},receiptFor:(r:AttentionDecisionReceipt)=>string)=>{const result=await requireTaskOffice().decide({item,action:'reject'});setReceipt(receiptFor(result));query.reload()};
 return <Screen title='审批确认'><Text className='wk-display'>先看清影响，{ '\n' }再作出决定。</Text><Notice tone='warning'>当前接口尚未提供动作名称、目标资源和风险摘要，不能只凭参数哈希批准外部写操作。请在 Web 核验完整动作；此版本仅支持安全拒绝。</Notice>
 <DataBoundary state={query}>{items=><>{items.map(item=><Card key={item.interactionId} tone='warning'><View className='wk-between'><Text className='wk-h3'>{item.kind==='tool_approval'?'工具操作确认':item.kind==='budget'?'预算扩展':'执行恢复'}</Text><Badge tone='warning'>待处理</Badge></View><Text className='wk-mono'>事项：{item.interactionId}</Text><Text className='wk-muted wk-small'>运行：{item.runId} · 版本：{item.expectedRevision} · 决策与参数哈希绑定</Text>{item.kind==='tool_approval'?<Action secondary loading={action.busy} onClick={()=>void action.run(()=>decide(item,decisionReceiptText))}>拒绝本次操作</Action>:<Text className='wk-muted wk-small'>此类事项需要扩展/恢复参数上下文，请到 Web 工作台处理。</Text>}</Card>)}{!items.length&&<Empty title='当前没有待处理事项'/>}</>}</DataBoundary>
 {receipt&&<Notice>{receipt}</Notice>}{action.error&&<Notice tone='danger'>{action.error}</Notice>}<Action secondary onClick={()=>void navigate('tasks')}>返回任务</Action></Screen>;
}

export function ArtifactPage(){
 const run=routeParam('run');const [content,setContent]=useState<string>();const action=useAction();const [receipt,setReceipt]=useState<string>();
 const query=useData(`materials:${run}`,async()=>{const material=openActiveMaterial();if(!material)throw new Error('材料面板尚未就绪');return material.index({runId:run})},!!run);
 const terminal=useData(`terminal:${run}`,async()=>{const material=openActiveMaterial();if(!material)throw new Error('材料面板尚未就绪');return material.open({kind:'terminal',runId:run})},!!run);
 return <Screen title='任务材料'><Text className='wk-display'>把完成的工作，{ '\n' }留在手边。</Text>
 <DataBoundary state={query}>{index=><>{index.materials.map(entry=>{const row=materialEntryRow(entry);return <Card key={entry.materialId}><ListRow title={row.title} subtitle={row.subtitle} onClick={()=>void action.run(async()=>{const material=openActiveMaterial();const view=await material!.open({kind:entry.kind,runId:run,materialId:entry.materialId});if(view.kind==='artifact'||view.kind==='test-report'||view.kind==='diff')setContent('text' in view?(view.text??JSON.stringify({preview:view.preview})):JSON.stringify({preview:view.preview}))})}/><Action secondary loading={action.busy} onClick={()=>void action.run(async()=>{const material=openActiveMaterial();const grant=await material!.act({kind:'download',runId:run,materialId:entry.materialId});if(grant.kind==='grant')setReceipt(`签名链接（短时效）：${grant.url}`)})}>复制下载链接</Action></Card>})}{!index.materials.length&&<Empty title='暂时没有材料' body='任务可能仍在进行，或者没有生成可下载产物。'/>}</>}</DataBoundary>
 <DataBoundary state={terminal}>{view=>view.kind==='terminal'?<Card><Text className='wk-h3'>终端输出（只读）</Text>{view.lines.slice(0,80).map(line=><Text key={line.seq} className='wk-mono wk-small'>{line.stream==='stderr'?'[stderr] ':''}{line.text}</Text>)}{view.nextCursor!==undefined&&<Text className='wk-muted wk-small'>已截断显示前 80 行；完整输出请到 Web 工作台。</Text>}</Card>:null}</DataBoundary>
 {content&&<Card><Text className='wk-h3'>预览</Text><Text className='wk-prose' selectable>{content}</Text></Card>}
 {receipt&&<Notice>链接为短时效签名授权，请复制到浏览器打开；小程序不直接执行下载内容。HTML、脚本不在小程序中执行。</Notice>}
 {action.error&&<Notice tone='danger'>{action.error}</Notice>}
 {run&&<Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>}</Screen>;
}
