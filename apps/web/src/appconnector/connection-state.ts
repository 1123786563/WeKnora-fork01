// W04 app-connector display state helpers.
//
// Pure functions only — no React, so node:test covers them directly.

import type { ConnectionView, InstallationView, SyncStatusView } from '@weknora/contracts';

export interface ConnectionViewWire {
 id:string;kind:'personal'|'space';state:string;owner_id:string|null;
}
export function connectionLabel(c:ConnectionViewWire):string {
 if(c.state==='revoked') return '连接已撤销';
 return c.kind==='personal'?'个人连接':'空间连接';
}

// Pause-reason display mirrors internal/appconnector/sync.go IsValidPauseReason:
// only the server vocabulary gets a human label, anything else stays verbatim
// instead of being silently coerced into a known reason.
const pauseReasonLabels: Record<string,string> = {
 plan: '套餐到期，暂停新同步',
 permission: '授权失效，需重新授权后恢复',
 conflict: '同步冲突待处理',
 rate_limit: '触发限流，稍后自动恢复',
};
export function pauseReasonLabel(reason:string|null):string|null {
 if(reason===null) return null;
 return pauseReasonLabels[reason] ?? reason;
}

// An installation whose upgrade expanded its scope parks in
// reauthorization_required: the UI must prompt for re-authorization, never
// pretend the upgrade finished cleanly.
export function requiresReauthorization(installation:Pick<InstallationView,'scopes'> & {state?:string}):boolean {
 return installation.state==='reauthorization_required';
}

export function syncStatusText(status:Pick<SyncStatusView,'state'|'pause_reason'|'requires_reauthorization'>):string {
 if(status.requires_reauthorization) return '需重新授权';
 if(status.pause_reason!==null) {
  const label=pauseReasonLabel(status.pause_reason);
  return label===null?status.state:`${status.state}（${label}）`;
 }
 return status.state;
}
