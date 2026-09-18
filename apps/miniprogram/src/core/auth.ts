import type { AuthMe, AuthSession, BearerCredential, Credential } from '@weknora/api-client';
import { ScopeGuard, type ScopeStamp } from './scope.ts';
import type { ValueStore } from './intent.ts';
export interface AuthPort {
  login(input:{email:string;password:string}):Promise<AuthSession>;
  me():Promise<AuthMe>;
  refresh(token:string):Promise<{access_token:string;refresh_token:string}>;
  switchTenant(id:number,refreshToken?:string):Promise<AuthSession>;
  logout():Promise<void>;
}
export interface SessionView {
  phase:'anonymous'|'loading'|'ready'|'switching'|'error';
  userId:string|null; userName:string; tenantId:string|null; tenantName:string;
  memberships:readonly unknown[]; error?:string;
}
const anonymous=():SessionView=>({phase:'anonymous',userId:null,userName:'',tenantId:null,tenantName:'',memberships:[]});
function isBearer(value:unknown):value is BearerCredential {
  if(value===null||typeof value!=='object')return false;
  const v=value as Record<string,unknown>;
  return v.kind==='bearer'&&typeof v.accessToken==='string'&&v.accessToken.length>0&&
    (v.refreshToken===undefined||typeof v.refreshToken==='string');
}
/** URL hosts are case-insensitive, so storage keys must not embed raw host case:
 *  builds pointed at https://WeKnora-App.example and https://weknora-app.example
 *  talk to the same server and must share one session identity. */
export function normalizeApiOrigin(origin:string):string{
  const trimmed=origin.replace(/\/+$/,'');
  const match=/^(https:\/\/)([^/?#]+)([/?#].*)?$/i.exec(trimmed);
  return match?match[1]+match[2].toLowerCase()+(match[3]??''):trimmed;
}
/** Credentials are private implementation state, never included in observable UI state. */
export class AuthCoordinator {
  readonly scope:ScopeGuard;
  private origin:string; private store:ValueStore; private api:AuthPort; private key:string;
  private value:SessionView=anonymous(); private secret:Credential={kind:'anonymous'};
  private transition=0;
  private listeners=new Set<()=>void>(); private refreshing:Promise<void>|null=null;
  constructor(origin:string,store:ValueStore,api:AuthPort){
    this.origin=normalizeApiOrigin(origin);this.store=store;this.api=api;this.key=`wk:auth:${this.origin}`;
    this.migrateCaseVariants();
    this.scope=new ScopeGuard({origin:this.origin,userId:null,tenantId:null});
  }
  /** Older builds stored credentials under host-case variants of the same origin.
   *  Adopt the first valid bearer into the canonical key and drop every variant,
   *  so logins survive an origin-case change and no token is left behind on logout. */
  private migrateCaseVariants():void{
    const keys=typeof this.store.keys==='function'?this.store.keys():[];
    for(const legacy of keys){
      if(!legacy.startsWith('wk:auth:')||legacy===this.key)continue;
      if(normalizeApiOrigin(legacy.slice('wk:auth:'.length))!==this.origin)continue;
      const value=this.store.read(legacy);
      if(isBearer(value)&&this.store.read(this.key)===undefined)this.store.write(this.key,value);
      this.store.remove(legacy);
    }
  }
  snapshot=():SessionView=>this.value;
  subscribe=(listener:()=>void):(()=>void)=>{this.listeners.add(listener);return()=>this.listeners.delete(listener)};
  credential():Credential{return {...this.secret}}
  private publish(next:SessionView):void{this.value=Object.freeze(next);for(const fn of this.listeners)fn()}
  private save(value:Credential):void{this.store.write(this.key,value);this.secret=value}
  private setIdentity(me:AuthMe):void{
    const tenantId=me.tenant?.id==null?null:String(me.tenant.id);
    this.scope.switchTo({origin:this.origin,userId:me.user.id,tenantId});
    this.publish({phase:'ready',userId:me.user.id,userName:String(me.user.username??me.user.name??me.user.email??'用户'),tenantId,
      tenantName:String(me.tenant?.name??''),memberships:me.memberships??[]});
  }
  async bootstrap():Promise<void>{
    const operation=++this.transition;
    const stored=this.store.read(this.key);
    if(!isBearer(stored)){this.secret={kind:'anonymous'};this.publish(anonymous());return}
    this.secret=stored;this.publish({...anonymous(),phase:'loading'});
    try{
      const me=await this.api.me();
      if(operation===this.transition)this.setIdentity(me);
    }catch(error){
      if(operation!==this.transition)return;
      const status=typeof error==='object'&&error!==null?(error as {status?:number}).status:undefined;
      if(status===401&&stored.refreshToken){
        try{
          await this.refresh(this.scope.capture());
          if(operation!==this.transition)return;
          const me=await this.api.me();
          if(operation===this.transition)this.setIdentity(me);
          return;
        }catch{if(operation!==this.transition)return}
      }
      if(status===401){this.clear();return}
      this.publish({...anonymous(),phase:'error',error:'无法验证登录状态，请检查网络后重试'});
    }
  }
  async login(email:string,password:string):Promise<void>{
    if(!email.trim()||!password)throw new Error('请输入邮箱和密码');
    if(this.value.phase==='loading'||this.value.phase==='switching')throw new Error('正在处理登录状态');
    const operation=++this.transition;
    this.publish({...anonymous(),phase:'loading'});
    try{
      const session=await this.api.login({email:email.trim(),password});
      if(operation!==this.transition)throw new Error('Stale auth transition');
      await this.accept(session,operation);
    }catch(error){if(operation===this.transition)this.clear();throw error}
  }
  async accept(session:AuthSession,operation=++this.transition):Promise<void>{
    if(operation!==this.transition)throw new Error('Stale auth transition');
    if(!session.token||!session.refreshToken)throw new Error('登录响应缺少凭证');
    this.scope.invalidate();this.save({kind:'bearer',accessToken:session.token,refreshToken:session.refreshToken});
    // Identity always comes from the authenticated server, not decoded JWT or local user input.
    const me=await this.api.me();
    if(operation!==this.transition)throw new Error('Stale auth transition');
    this.setIdentity(me);
  }
  async refresh(stamp:ScopeStamp):Promise<void>{
    if(!this.scope.isCurrent(stamp))throw new Error('Stale auth scope');
    if(this.refreshing)return this.refreshing;
    const secret=this.secret;
    if(secret.kind!=='bearer'||!secret.refreshToken)throw new Error('需要重新登录');
    const work=(async()=>{
      let next;
      try{
        next=await this.api.refresh(secret.refreshToken!);
      }catch(error){
        // A definitive 401 from the refresh endpoint means the refresh token is
        // dead: staying "ready" would turn every later action into an endless
        // 401 loop. Network-class failures are NOT credential verdicts; the
        // session stays for a later retry.
        const status=typeof error==='object'&&error!==null?(error as {status?:number}).status:undefined;
        if(status===401)this.clear();
        throw error;
      }
      if(!this.scope.isCurrent(stamp)||this.secret.kind!=='bearer'||this.secret.accessToken!==secret.accessToken)throw new Error('Stale auth scope');
      this.save({kind:'bearer',accessToken:next.access_token,refreshToken:next.refresh_token});
    })();
    this.refreshing=work;
    try{await work}finally{if(this.refreshing===work)this.refreshing=null}
  }
  async switchTenant(id:number):Promise<void>{
    if(this.value.phase!=='ready'||!Number.isSafeInteger(id)||id<=0)throw new Error('工作空间不可切换');
    const previous=this.value,secret=this.secret;if(secret.kind!=='bearer')throw new Error('请先登录');
    const operation=++this.transition;
    this.scope.invalidate();this.publish({...previous,phase:'switching'});
    let received=false;
    try{
      const next=await this.api.switchTenant(id,secret.refreshToken);
      if(operation!==this.transition)throw new Error('Stale auth transition');
      received=true;
      if(!next.token||!next.refreshToken)throw new Error('空间切换响应缺少凭证');
      if(String(next.tenant?.id)!==String(id))throw new Error('空间切换响应不匹配');
      if(next.user?.id!==undefined&&String(next.user.id)!==previous.userId)throw new Error('空间切换返回了其他用户');
      this.save({kind:'bearer',accessToken:next.token,refreshToken:next.refreshToken});
      this.scope.switchTo({origin:this.origin,userId:previous.userId,tenantId:String(id)});
      this.publish({...previous,phase:'ready',tenantId:String(id),tenantName:String(next.tenant?.name??''),memberships:next.memberships??previous.memberships});
    }catch(error){
      // A response may already have rotated credentials: never pretend the old login remains valid.
      if(operation===this.transition){if(received)this.clear();else this.publish({...previous,phase:'ready'});}
      throw error;
    }
  }
  clear():void{this.transition++;this.secret={kind:'anonymous'};this.store.remove(this.key);this.scope.switchTo({origin:this.origin,userId:null,tenantId:null});this.publish(anonymous())}
  async logout():Promise<void>{
    const work=this.api.logout();this.clear();
    try{await work}catch{/* Local logout succeeds; remote revocation may require network. */}
  }
}
