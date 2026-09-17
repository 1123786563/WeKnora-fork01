import { useEffect, useRef, useState } from 'react';
import { View, Text } from '@tarojs/components';
import { useDidHide } from '@tarojs/taro';
import { initialChatStreamState, reduceChatStream } from '@weknora/domain/chat/reducer';
import { groupChatReferences } from '@weknora/domain/chat/references';
import { responseType } from '@weknora/contracts';
import { Screen, Card, Action, Field, Notice, Section, ListRow, Empty, useData, DataBoundary, useAction } from '../../components/ui.tsx';
import { client, auth, chatStream } from '../../services/runtime.ts';
import { navigate, routeParam } from '../../platform/navigation.ts';
import { rows, record, text, identifier } from '../../services/views.ts';
import { chooseDocument } from '../../platform/files.ts';
export default function ChatPage(){
 const agent=routeParam('agent'),kb=routeParam('kb'),doc=routeParam('document');
 const [sessionId,setSessionId]=useState(routeParam('id')),[input,setInput]=useState(''),[question,setQuestion]=useState(''),[answer,setAnswer]=useState(initialChatStreamState),[interrupted,setInterrupted]=useState(false),[attachments,setAttachments]=useState<{id:string;name:string}[]>([]);
 const action=useAction();const active=useRef<AbortController>();const current=useRef(initialChatStreamState());
 const history=useData(`messages:${sessionId}`,signal=>client.sessions.messages(sessionId,{limit:30,signal}),!!sessionId);
 useDidHide(()=>{active.current?.abort();setInterrupted(true)});useEffect(()=>()=>active.current?.abort(),[]);
 const ensureSession=async()=>{if(sessionId)return sessionId;const value=await client.sessions.create({title:input.trim().slice(0,60)||'新对话'});const id=identifier(record(value).id);setSessionId(id);return id};
 const send=()=>action.run(async()=>{
  if(!input.trim())return;const stamp=auth.scope.capture();const id=await ensureSession();if(!auth.scope.isCurrent(stamp))return;
  const query=input.trim();setInput('');setQuestion(query);setInterrupted(false);current.current=initialChatStreamState();setAnswer(current.current);
  const c=new AbortController();active.current=c;
  try{
   await chatStream({sessionId:id,mode:agent?'agent':'knowledge',signal:c.signal,body:{query,agent_id:agent||undefined,agent_enabled:!!agent,knowledge_base_ids:kb?[kb]:[],knowledge_ids:doc?[doc]:[],attachment_ids:attachments.map(a=>a.id),channel:'miniprogram'}},event=>{
    if(!auth.scope.isCurrent(stamp)||c.signal.aborted)return;
    if(responseType(event)==='thinking')return; // Do not retain private reasoning on this client.
    const next=reduceChatStream(current.current,event);
    current.current={...next,thinking:'',toolCalls:Object.fromEntries(Object.entries(next.toolCalls).map(([key,tool])=>[key,{id:tool.id,name:tool.name,status:tool.status}]))};
    setAnswer(current.current);
   });
   if(auth.scope.isCurrent(stamp))setAttachments([]);
  }catch(error){if(auth.scope.isCurrent(stamp))setInterrupted(true);throw error}
 });
 const refs=groupChatReferences(answer.references).flatMap(g=>g.items);
 return <Screen title='知识 / Agent 对话'><Notice>{kb?'当前范围：已选择的知识库':agent?'当前范围：Agent 已配置资源':'当前范围由后端及会话配置决定'}</Notice>{sessionId&&<DataBoundary state={history}>{messages=><View>{rows(messages).map(message=><View key={identifier(message.id)} className={`wk-message ${message.role==='user'?'wk-message--user':''}`}><Text className='wk-message-author'>{message.role==='user'?'你':'AI 助手'}</Text><Text className='wk-prose' selectable>{text(message.content)}</Text></View>)}</View>}</DataBoundary>}{!sessionId&&!question&&<Empty title='从一个好问题开始' body='答案会尽量附上来源；重要内容请结合原文核实。'/>}{question&&<View className='wk-message wk-message--user'><Text className='wk-message-author'>你</Text><Text className='wk-prose'>{question}</Text></View>}{(answer.answer||action.busy)&&<View className='wk-message'><Text className='wk-message-author'>AI 助手</Text><Text className='wk-prose' selectable>{answer.answer||'正在处理…'}</Text>{Object.keys(answer.toolCalls).length>0&&<Text className='wk-muted wk-small'>正在使用已授权工具 · {Object.keys(answer.toolCalls).length} 项</Text>}</View>}{refs.length>0&&<><Section title='回答来源'/>{refs.map(ref=><Card key={ref.key}><ListRow title={ref.title} subtitle={ref.snippet} onClick={ref.knowledgeId?()=>void navigate('document',{id:ref.knowledgeId,kb:ref.knowledgeBaseId}):undefined} suffix={<Text>{ref.knowledgeId?'查看原文 ›':'外部来源'}</Text>}/></Card>)}</>}{Object.keys(answer.approvals).length>0&&<Notice tone='warning'>当前对话需要工具授权。请在 Web 工作台核对完整动作信息，不会在这里自动批准。</Notice>}{interrupted&&<Notice tone='warning'>连接已中断。请先重新读取历史；不要为恢复回答重复发送原问题。</Notice>}{sessionId&&<Action secondary onClick={()=>{setQuestion('');setAnswer(initialChatStreamState());history.reload()}}>重新读取服务端历史</Action>}{action.error&&<Notice tone='danger'>{action.error}</Notice>}<Card><Field label='你的问题' value={input} onChange={setInput} multiline placeholder='继续提问，让想法更清晰…'/>{attachments.map(a=><Text className='wk-badge' key={a.id}>{a.name} · 临时附件</Text>)}<View className='wk-grid'><Action secondary disabled={action.busy} onClick={()=>void action.run(async()=>{const stamp=auth.scope.capture(),file=await chooseDocument(),id=await ensureSession();const uploaded=await client.chat.attachments.upload(id,{file,...(agent?{agentId:agent}:{})});if(auth.scope.isCurrent(stamp))setAttachments(v=>[...v,{id:identifier(record(uploaded).id),name:file.name}])})}>添加附件</Action>{action.busy?<Action secondary onClick={()=>{active.current?.abort();setInterrupted(true)}}>停止接收</Action>:<Action disabled={!input.trim()} onClick={()=>void send()}>发送</Action>}</View><Text className='wk-footnote'>AI 生成内容，请结合引用核实。临时附件不会自动存入知识库。</Text></Card></Screen>;
}
