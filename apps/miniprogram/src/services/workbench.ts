import { client, apiOrigin } from './runtime.ts';
import { data, text, finiteInteger, record } from './views.ts';

/** Artifact metadata remains exposed here for the execution Artifact screen. */
export interface TaskArtifact {index:number;id:string;name:string;mime:string;version:string;size:number;sourceRun:string;createdAt:string}

/** Task-scoped artifact metadata is authorized by the server against the active user and space. */
export async function listTaskArtifacts(runId:string):Promise<TaskArtifact[]>{
 const value=record(data(await client.request({method:'GET',path:`/api/v1/workbench/executions/${encodeURIComponent(runId)}/artifacts`})));
 if(!Array.isArray(value.items))throw new Error('Task artifact collection requires items');
 return value.items.map(raw=>{const item=record(raw);const size=item.size;if(typeof size!=='number'||!Number.isFinite(size)||size<0)throw new Error('Invalid artifact size');
  return {index:finiteInteger(item.index),id:text(item.id),name:text(item.name,'未命名文件'),mime:text(item.mime,'application/octet-stream'),version:text(item.version),size,sourceRun:text(item.source_run),createdAt:text(item.created_at)};
 });
}
/** Mint a fresh short-lived URL for each explicit user action; never persist the grant. */
export async function taskArtifactDownloadPath(runId:string,item:Pick<TaskArtifact,'index'>):Promise<string>{
 const response=record(data(await client.request({method:'POST',path:`/api/v1/workbench/executions/${encodeURIComponent(runId)}/artifacts/${finiteInteger(item.index)}/signed-url`})));
 const url=text(response.url);let parsed:URL;
 try{parsed=new URL(url)}catch{throw new Error('Invalid signed artifact URL')}
 if(!apiOrigin||parsed.origin!==apiOrigin||parsed.pathname!=='/api/v1/workbench/artifacts/download'||!parsed.searchParams.has('signature'))throw new Error('Untrusted signed artifact URL');
 return `${parsed.pathname}${parsed.search}`;
}
