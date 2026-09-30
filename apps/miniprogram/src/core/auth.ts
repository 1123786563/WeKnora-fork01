import type { AuthMe, AuthSession } from '@weknora/api-client';

/** 会话的呈现层视图（components/ui.tsx useSession 消费；凭据永不进入本视图）。 */
export interface SessionView {
  phase:'anonymous'|'loading'|'ready'|'switching'|'error';
  userId:string|null; userName:string; tenantId:string|null; tenantName:string;
  memberships:readonly unknown[]; error?:string;
}

export const anonymous=():SessionView=>({phase:'anonymous',userId:null,userName:'',tenantId:null,tenantName:'',memberships:[]});

/** URL hosts are case-insensitive, so storage keys must not embed raw host case:
 *  builds pointed at https://WeKnora-App.example and https://weknora-app.example
 *  talk to the same server and must share one session identity. */
export function normalizeApiOrigin(origin:string):string{
  const trimmed=origin.replace(/\/+$/,'');
  const match=trimmed.match(/^(https:\/\/)([^/?#]+?)([\/?#].*)?$/i);
  return match?match[1]+match[2].toLowerCase()+(match[3]??''):trimmed;
}

export function isBearer(value:unknown):value is {kind:'bearer';accessToken:string;refreshToken?:string}{
  if(value===null||typeof value!=='object')return false;
  const v=value as Record<string,unknown>;
  return v.kind==='bearer'&&typeof v.accessToken==='string'&&v.accessToken.length>0&&
    (v.refreshToken===undefined||typeof v.refreshToken==='string');
}

/** 仅供类型参考的 wire 形态（session 门面富集时使用；不再有本地 AuthPort 编排）。 */
export type MeWire = AuthMe;
export type SessionWire = AuthSession;
