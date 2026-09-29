import { useEffect, useState } from 'react';
import { View, Text } from '@tarojs/components';
import { Screen, Card, Action, Notice, Empty, Badge, useSession, useAction, confirmAction } from '../../components/ui.tsx';
import { auth } from '../../services/runtime.ts';
import { navigate } from '../../platform/navigation.ts';
import { memberSpaces } from '../../services/views.ts';
// 静默登录页：进入即 wx.login 换平台会话，失败保留“立即登录”重试，不再渲染账密表单。
export function LoginPage(){
 const action=useAction();const start=()=>void action.run(async()=>{await auth.wxLogin();await navigate('workspace')});
 useEffect(()=>{start()},[]);
 return <Screen title='WeKnora' publicPage><View className='wk-login-hero'><View className='wk-orbit'/><Text className='wk-eyebrow'>WORK, WITH A LITTLE MORE SPACE</Text><Text className='wk-display'>你的随身{ '\n' }AI 工作台</Text><Text className='wk-muted'>将使用你的微信身份自动登录平台账号</Text></View><Card><Text className='wk-h2'>微信快捷登录</Text><Text className='wk-muted'>同一账号，同一空间。登录即表示同意平台的数据使用与服务说明。</Text><Action loading={action.busy} onClick={start}>立即登录</Action>{action.error&&<Notice tone='danger'>{action.error}</Notice>}</Card><Action secondary onClick={()=>void navigate('states',{kind:'privacy'})}>隐私与数据使用说明</Action></Screen>;
}
export function WorkspacePage(){
 const session=useSession(),action=useAction();const choices=memberSpaces(session.memberships);const [selected,select]=useState(session.tenantId??'');
 // A verified current tenant remains selectable even if an old server omits memberships.
 if(session.tenantId&&!choices.some(s=>String(s.id)===session.tenantId))choices.unshift({id:Number(session.tenantId),name:session.tenantName,role:'当前空间'});
 return <Screen title='选择工作空间'><Text className='wk-eyebrow'>YOUR SPACE, YOUR CONTEXT</Text><Text className='wk-display'>在哪个空间开始？</Text><Text className='wk-muted'>知识、任务和用量，都属于当前空间。</Text><View className='wk-spacer'/>{choices.map(space=><Card key={space.id} onClick={()=>select(String(space.id))} tone={selected===String(space.id)?'mint':'white'}><View className='wk-between'><View><Text className='wk-h2'>{space.name}</Text><Text className='wk-muted'>{space.role}</Text></View><Badge tone={selected===String(space.id)?'success':'neutral'}>{selected===String(space.id)?'已选择':'可进入'}</Badge></View></Card>)}{!choices.length&&<Empty title='还没有工作空间' body='接受空间邀请后，再选择空间进入。' action='处理邀请' onAction={()=>void navigate('invitations')}/>}<Action loading={action.busy} disabled={!selected} onClick={()=>void action.run(async()=>{if(selected!==session.tenantId){if(!await confirmAction('切换工作空间','未提交的当前页面输入不会带入新空间。后台任务不会被取消。'))return;await auth.switchTenant(Number(selected))}await navigate('home')})}>进入工作空间</Action>{action.error&&<Notice tone='danger'>{action.error}</Notice>}<Action secondary onClick={()=>void navigate('invitations')}>接受邀请</Action></Screen>;
}
