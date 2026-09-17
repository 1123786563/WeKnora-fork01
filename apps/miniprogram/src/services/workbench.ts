import { client, executions, auth, stream, apiOrigin } from './runtime.ts';
import { createServerSentEventParser, executionEventsRequest, type StartExecutionInput } from '@weknora/api-client';
import { parseExecutionEvent } from '@weknora/contracts';
import { data, rows, identifier, text, finiteInteger, record } from './views.ts';
import { PendingIntent, requestId } from '../core/intent.ts';
import { scopeKey } from '../core/scope.ts';
import { installSnapshot, appendEvent, type RunProjection } from '../core/execution.ts';
import { storage } from '../platform/storage.ts';
export interface TaskCard {runId:string;title:string;status:string;updatedAt:string}
export interface Interaction {id:string;kind:string;argsHash:string;revision:number;decisionId:string;action:string}
export interface Target {id:string;kind:string;state:string}
export async function targets():Promise<Target[]>{return rows(data(await client.request({method:'GET',path:'/api/v1/execution-targets'}))).map(r=>({id:identifier(r.id),kind:text(r.kind),state:text(r.state)})).filter(t=>t.state==='active')}
/** Proposed list read contract; 404/501 are surfaced, never mapped to an empty list. */
export async function listTasks(cursor='',status=''):Promise<{items:TaskCard[];nextCursor:string}>{
 const q=new URLSearchParams({limit:'20'});if(cursor)q.set('cursor',cursor);if(status)q.set('status',status);
 const value=record(data(await client.request({method:'GET',path:`/api/v1/workbench/executions?${q}`})));
 if(!Array.isArray(value.items))throw new Error('Task collection requires items');
 return {items:value.items.map(r=>{const row=record(r);return {runId:identifier(row.run_id),title:text(row.title,'Agent 任务'),status:text(row.run_status),updatedAt:text(row.updated_at)}}),nextCursor:text(value.next_cursor)};
}
export async function interactions(runId:string):Promise<Interaction[]>{return rows(data(await client.request({method:'GET',path:`/api/v1/workbench/executions/${encodeURIComponent(runId)}/interactions`}))).map(r=>({id:identifier(r.id),kind:text(r.kind),argsHash:text(r.args_hash),revision:finiteInteger(r.expected_revision),decisionId:text(r.decision_id),action:text(r.action)}))}
export async function rejectInteraction(item:Interaction):Promise<void>{
 if(item.kind!=='tool_approval'||item.decisionId||!item.argsHash)throw new Error('不可重复或跨类型决策');
 const key=`wk:decision:${scopeKey(auth.scope.capture())}:${item.id}`;
 const previous=storage.read(key);const id=typeof previous==='string'?previous:requestId();storage.write(key,id);
 await client.request({method:'POST',path:`/api/v1/workbench/executions/interactions/${encodeURIComponent(item.id)}/decisions`,body:{id:item.id,decision_id:id,kind:item.kind,action:'reject',args_hash:item.argsHash,expected_revision:item.revision}});
 storage.remove(key);
}
export function pendingIntent():PendingIntent{return new PendingIntent(storage,scopeKey(auth.scope.capture()),requestId)}
const recentKey=()=>`wk:recent-runs:${scopeKey(auth.scope.capture())}`;
export function recentRuns():string[]{const r=storage.read(recentKey());return Array.isArray(r)?r.filter((v):v is string=>typeof v==='string').slice(0,15):[]}
export function rememberRun(runId:string):void{storage.write(recentKey(),[runId,...recentRuns().filter(id=>id!==runId)].slice(0,15))}
export async function startTask(input:Omit<StartExecutionInput,'request_id'>):Promise<string>{
 const intent=pendingIntent(),old=intent.current();
 if(old&&old.state!=='rejected'){
  const found=await executions.lookup(old.requestId);intent.reconcile(found);
  if(found.state==='admitted'&&found.run_id){rememberRun(found.run_id);intent.acknowledge();return found.run_id}
  // lookup 明确返回 unknown 表示服务端没有该请求的持久记录（admission.go 对
  // ErrRecordNotFound 的分支），不是"仍在途"。此时用同一 request_id 重提交是
  // 安全的：若记录实际已存在，Start 会按幂等键返回原 run；换新 ID 反而会
  // 违背"同一意图保留相同 request_id"。pending/dispatching 仍需等待。
  if(found.state!=='unknown')throw new Error('前一次任务仍待确认，请先查询原请求');
 }
 if(old?.state==='rejected')intent.reset();const entry=intent.begin();
 try{const ack=await executions.start({...input,request_id:entry.requestId});intent.reconcile({state:'admitted',run_id:ack.run_id});rememberRun(ack.run_id);intent.acknowledge();return ack.run_id}
 catch(error){intent.reconcile({state:'unknown'});throw error}
}
export async function watchExecution(runId:string,signal:AbortSignal,update:(value:RunProjection)=>void):Promise<void>{
 let projection=installSnapshot(await executions.snapshot(runId,signal),runId);if(signal.aborted)return;
 update(projection);rememberRun(runId);
 if(['succeeded','failed','canceled'].includes(projection.execution.run_status))return;
 const request=executionEventsRequest(runId,String(projection.cursor));
 const parser=createServerSentEventParser(frame=>{projection=appendEvent(projection,parseExecutionEvent(JSON.parse(frame.data)));update(projection)});
 await stream({...request,url:apiOrigin+request.path,headers:request.headers??{},signal},chunk=>parser.push(chunk));
 // Do not advance the cursor from an incomplete frame on disconnection. Resnapshot next time.
}
