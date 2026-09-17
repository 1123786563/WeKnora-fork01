import { useEffect, useState, useRef, useCallback, useSyncExternalStore, type ReactNode } from 'react';
import Taro, { useDidHide, useDidShow } from '@tarojs/taro';
import { View, Text, Button, Input, Textarea, Image } from '@tarojs/components';
import { auth } from '../services/runtime.ts';
import { errorMessage } from '../core/errors.ts';
import { navigate, back } from '../platform/navigation.ts';
import { t } from '../i18n/zh-CN.ts';
import type { SessionView } from '../core/auth.ts';
export const useSession=():SessionView=>useSyncExternalStore(auth.subscribe,auth.snapshot,auth.snapshot);
import sparkIcon from '../assets/spark.png';
import bookIcon from '../assets/book.png';
import tasksIcon from '../assets/tasks.png';
import userIcon from '../assets/user.png';
import arrowIcon from '../assets/arrow.png';
import checkIcon from '../assets/check.png';
import shieldIcon from '../assets/shield.png';
import uploadIcon from '../assets/upload.png';
const iconMap: Record<string,string> = {spark: sparkIcon, book: bookIcon, tasks: tasksIcon, user: userIcon, arrow: arrowIcon, check: checkIcon, shield: shieldIcon, upload: uploadIcon};
export function Icon({name='spark'}:{name?:string}){return <Image className='wk-icon' src={iconMap[name] ?? sparkIcon} mode='aspectFit'/>}

export function Badge({children,tone='neutral'}:{children:ReactNode;tone?:'neutral'|'success'|'warning'|'danger'|'info'}){return <Text className={`wk-badge wk-badge--${tone}`}>{children}</Text>}
export function Action({children,onClick,secondary=false,danger=false,disabled=false,loading=false}:{children:ReactNode;onClick?:()=>void;secondary?:boolean;danger?:boolean;disabled?:boolean;loading?:boolean}){
 return <Button className={`wk-button ${secondary?'wk-button--secondary':''} ${danger?'wk-button--danger':''}`} disabled={disabled||loading} loading={loading} onClick={onClick}>{children}</Button>;
}
export function Card({children,tone='white',onClick}:{children:ReactNode;tone?:'white'|'mint'|'dark'|'warning';onClick?:()=>void}){return <View className={`wk-card wk-card--${tone}`} onClick={onClick}>{children}</View>}
export function Section({title,action,onAction}:{title:string;action?:string;onAction?:()=>void}){return <View className='wk-section'><Text className='wk-section-title'>{title}</Text>{action&&<Button className='wk-text-button' onClick={onAction}>{action} ›</Button>}</View>}
export function Notice({children,tone='info'}:{children:ReactNode;tone?:'info'|'warning'|'danger'}){return <View className={`wk-notice wk-notice--${tone}`}><Text>{children}</Text></View>}
export function Empty({title=t('empty'),body,action,onAction}:{title?:string;body?:string;action?:string;onAction?:()=>void}){return <View className='wk-empty'><View className='wk-emblem'><Icon name='spark'/></View><Text className='wk-h2'>{title}</Text>{body&&<Text className='wk-muted'>{body}</Text>}{action&&<Action secondary onClick={onAction}>{action}</Action>}</View>}
export function Field({label,value,onChange,placeholder='',multiline=false,password=false,type='text'}:{label:string;value:string;onChange:(value:string)=>void;placeholder?:string;multiline?:boolean;password?:boolean;type?:'text'|'number'}){
 return <View className='wk-field'><Text className='wk-label'>{label}</Text>{multiline?<Textarea className='wk-textarea' value={value} placeholder={placeholder} maxlength={8000} autoHeight={false} adjustPosition onInput={e=>onChange(e.detail.value)}/>:<Input className='wk-input' value={value} placeholder={placeholder} password={password} type={type} maxlength={500} onInput={e=>onChange(e.detail.value)}/>}</View>;
}
export function ListRow({title,subtitle,onClick,icon='book',suffix}:{title:string;subtitle?:string;onClick?:()=>void;icon?:string;suffix?:ReactNode}){return <View className='wk-listrow' onClick={onClick}><View className='wk-avatar'><Icon name={icon}/></View><View className='wk-grow'><Text className='wk-row-title'>{title}</Text>{subtitle&&<Text className='wk-muted wk-small'>{subtitle}</Text>}</View>{suffix??<Text className='wk-chevron'>›</Text>}</View>}
export function Screen({title,children,tab=false,publicPage=false}:{title:string;children:ReactNode;tab?:boolean;publicPage?:boolean}){
 const session=useSession();const window=Taro.getWindowInfo();const capsule=Taro.getMenuButtonBoundingClientRect();
 const top=window.statusBarHeight??20,navHeight=Math.max(44,(capsule.top-top)*2+capsule.height);
 return <View className='wk-screen'><View className='wk-header' style={{paddingTop:`${top}px`,paddingRight:`${Math.max(96,window.windowWidth-capsule.left+12)}px`}}><View className='wk-header-line' style={{height:`${navHeight}px`}}>{!tab?<Button className='wk-back' onClick={()=>void back()} ariaLabel='返回'>‹</Button>:<View className='wk-brand-icon'><Icon/></View>}<View className='wk-grow'><Text className='wk-header-title'>{title}</Text><Text className='wk-header-caption'>{session.tenantName||'同一账号，同一工作空间'}</Text></View></View></View><View className='wk-main'>{publicPage||session.phase==='ready'?children:session.phase==='loading'||session.phase==='switching'?<Empty title={session.phase==='switching'?'正在切换工作空间':t('loading')}/>:session.phase==='error'?<Empty title='连接未完成' body={session.error} action={t('retry')} onAction={()=>void auth.bootstrap()}/>:<Empty title='让工作，在微信里继续' body='登录已有平台账号，查看你的知识与任务。' action={t('login')} onAction={()=>void navigate('login')}/>}</View></View>;
}
export interface DataState<T>{data:T|undefined;loading:boolean;error:string|undefined;reload:()=>void}
export function useData<T>(key:string,load:(signal:AbortSignal)=>Promise<T>,enabled=true):DataState<T>{
 const session=useSession();const [result,set]=useState<{data:T|undefined;loading:boolean;error:string|undefined}>({data:undefined,loading:false,error:undefined});
 const fn=useRef(load);fn.current=load;const active=useRef<AbortController>();const mounted=useRef(true);const run=useRef(0);
 const reload=useCallback(()=>{
  active.current?.abort();const ticket=++run.current;
  if(!enabled||auth.snapshot().phase!=='ready'){set({data:undefined,loading:false,error:undefined});return}
  const stamp=auth.scope.capture(),c=new AbortController();active.current=c;set({data:undefined,loading:true,error:undefined});
  fn.current(c.signal).then(data=>{if(mounted.current&&!c.signal.aborted&&ticket===run.current&&auth.scope.isCurrent(stamp))set({data,loading:false,error:undefined})},error=>{if(mounted.current&&!c.signal.aborted&&ticket===run.current&&auth.scope.isCurrent(stamp))set({data:undefined,loading:false,error:errorMessage(error)})});
 },[key,enabled,session.phase,session.tenantId,session.userId]);
 useEffect(()=>{mounted.current=true;reload();return()=>{mounted.current=false;active.current?.abort()}},[reload]);
 useDidShow(reload);useDidHide(()=>{active.current?.abort();run.current++});
 return {...result,reload};
}
export function DataBoundary<T>({state,children}:{state:DataState<T>;children:(data:T)=>ReactNode}){if(state.loading)return <Empty title={t('loading')}/>;if(state.error)return <Empty title='暂时无法读取' body={state.error} action={t('retry')} onAction={state.reload}/>;return state.data===undefined?null:<>{children(state.data)}</>}
export function useAction(){
 const [busy,setBusy]=useState(false),[error,setError]=useState<string>();const running=useRef(false),mounted=useRef(true);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 const run=async(work:()=>Promise<void>)=>{if(running.current)return;running.current=true;setBusy(true);setError(undefined);try{await work()}catch(e){if(mounted.current)setError(errorMessage(e))}finally{running.current=false;if(mounted.current)setBusy(false)}};
 return {busy,error,run};
}
export async function confirmAction(title:string,content:string):Promise<boolean>{return (await Taro.showModal({title,content,confirmText:'确认',cancelText:'取消',confirmColor:'#183E33'})).confirm}
