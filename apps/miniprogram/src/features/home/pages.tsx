import { useState } from 'react';
import { View, Text, Textarea, Button } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Section, ListRow, Badge, Empty, useData, DataBoundary, useAction, useSession } from '../../components/ui.tsx';
import { client, executions, auth } from '../../services/runtime.ts';
import { navigate, routeParam } from '../../platform/navigation.ts';
import { rows, identifier, text, record } from '../../services/views.ts';
import { startTask, pendingIntent, rememberRun } from '../../services/workbench.ts';
let memoryDraft='';let draftScope='';
function scopeDraft():string{const scope=JSON.stringify(auth.scope.capture());if(draftScope!==scope){memoryDraft='';draftScope=scope}return memoryDraft}
export function HomePage(){
 const session=useSession();const [prompt,setPrompt]=useState(scopeDraft());const agents=useData('home-agents',()=>client.configuration.agents.listWithState());
 const recent=useData('home-sessions',signal=>client.sessions.list({page:1,pageSize:4,signal}));
 return <Screen title='WeKnora' tab><Button className='wk-space-button' onClick={()=>void navigate('workspace')}>{session.tenantName||'选择工作空间'} ⌄</Button><View className='wk-hero'><View className='wk-orbit'/><Text className='wk-eyebrow'>MAKE ROOM FOR WHAT MATTERS</Text><Text className='wk-display'>把任务交给 AI，{ '\n' }把时间留给重要的事。</Text><Text className='wk-muted'>查资料、整理文档，或者继续一项工作。</Text></View><Card><Textarea className='wk-intent' value={prompt} onInput={e=>setPrompt(e.detail.value)} maxlength={8000} placeholder={'今天想完成什么？\n例如：整理本周工作，生成一份周报'}/><View className='wk-between'><Text className='wk-muted wk-small'>使用空间已配置的 Agent</Text><Button className='wk-send-square' onClick={()=>{memoryDraft=prompt;draftScope=JSON.stringify(auth.scope.capture());void navigate('agents')}}>↑</Button></View></Card><Section title='为你准备的 Agent' action='查看全部' onAction={()=>void navigate('agents')}/><DataBoundary state={agents}>{result=><View className='wk-grid'>{result.items.slice(0,2).map(item=><Card key={item.id} tone='mint' onClick={()=>void navigate('agent',{id:item.id})}><Text className='wk-agent-glyph'>✦</Text><Text className='wk-h3'>{item.name}</Text><Text className='wk-muted wk-small'>{text(item.description,'打开助手，开始一项工作')}</Text></Card>)}{!result.items.length&&<Empty title='还没有可用 Agent' body='请由空间管理员在 Web 端配置。'/>}</View>}</DataBoundary><Section title='继续上次的工作' action='任务中心' onAction={()=>void navigate('tasks')}/><DataBoundary state={recent}>{result=><Card>{rows(result).map(item=><ListRow key={identifier(item.id)} title={text(item.title,'未命名会话')} subtitle='继续已有会话' onClick={()=>void navigate('chat',{id:identifier(item.id)})}/>)}{!rows(result).length&&<Empty title='从第一个问题开始' body='你的会话会保存在当前平台账号中。'/>}</Card>}</DataBoundary></Screen>;
}
export function AgentsPage(){
 const [search,setSearch]=useState('');const query=useData('agents',signal=>client.configuration.agents.listWithState({signal}));
 return <Screen title='发现 Agent'><Text className='wk-display'>为工作，找个好帮手。</Text><Field label='搜索 Agent' value={search} onChange={setSearch} placeholder='名称或用途'/><DataBoundary state={query}>{result=>{const items=result.items.filter(a=>`${a.name} ${text(a.description)}`.toLowerCase().includes(search.toLowerCase()));return <>{items.map(a=><Card key={a.id}><ListRow title={a.name} subtitle={text(a.description,'已配置的空间助手')} icon='spark' onClick={()=>void navigate('agent',{id:a.id})}/><Badge tone={result.disabledOwnAgentIds.includes(a.id)?'warning':'success'}>{result.disabledOwnAgentIds.includes(a.id)?'当前不可用':'可访问'}</Badge></Card>)}{!items.length&&<Empty title='没有匹配的 Agent' body='换个关键词再试一次。'/>}</>}}</DataBoundary></Screen>;
}
export function AgentPage(){
 const id=routeParam('id'),session=useSession();const [prompt,setPrompt]=useState(scopeDraft()),[budget,setBudget]=useState('200');const action=useAction();
 const query=useData(`agent:${id}`,signal=>client.configuration.agents.listWithState({signal}));
 const submit=()=>action.run(async()=>{
  const previous=pendingIntent().current();
  if(previous){const found=await executions.lookup(previous.requestId);pendingIntent().reconcile(found);if(found.state==='admitted'&&found.run_id){rememberRun(found.run_id);pendingIntent().acknowledge();await navigate('execution',{id:found.run_id});return}if(found.state==='rejected')pendingIntent().reset();else throw new Error('原请求尚未确定，请在任务中心继续查询')}
  if(!/^\d+$/.test(budget)||!Number.isSafeInteger(Number(budget)))throw new Error('预算须为可表示的非负整数');
  const stamp=auth.scope.capture();const chat=await client.sessions.create({title:prompt.trim().slice(0,60)});
  if(!auth.scope.isCurrent(stamp))return;
  const run=await startTask({session_id:identifier(record(chat).id),agent_id:id,target_id:'platform',workspace_ref:'',text:prompt.trim(),budget_upper:Number(budget)});
  if(auth.scope.isCurrent(stamp))await navigate('execution',{id:run});
 });
 return <Screen title='发起任务'><DataBoundary state={query}>{result=>{const agent=result.items.find(a=>a.id===id);if(!agent)return <Empty title='Agent 不存在或没有访问权限'/>;return <><Card tone='mint'><Text className='wk-agent-glyph'>✦</Text><Text className='wk-display'>{agent.name}</Text><Text className='wk-muted'>{text(agent.description,'在已配置的能力范围内完成你的工作。')}</Text><Badge tone='success'>{session.tenantName}</Badge></Card><Field label='希望完成什么？' value={prompt} onChange={setPrompt} multiline placeholder='描述目标、要求和需要你确认的步骤'/><Card><Field label='本次任务预算上限（Credits）' value={budget} onChange={setBudget} type='number'/><Text className='wk-muted wk-small'>执行目标：平台托管。预算是否被实际强制执行取决于服务端账本适配的部署状态。</Text></Card><Notice>任务会在后端执行。离开小程序不等于取消任务；写入外部系统的操作应单独确认。</Notice><Action loading={action.busy} disabled={!prompt.trim()||!budget||result.disabledOwnAgentIds.includes(id)} onClick={()=>void submit()}>发起任务</Action><Action secondary onClick={()=>void navigate('chat',{agent:id})}>改用对话模式</Action>{action.error&&<Notice tone='danger'>{action.error}</Notice>}</>}}</DataBoundary></Screen>;
}
