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

// --- R439 A3: Vue ConnectionsView/AppsView badge contracts ----------------
//
// Labels mirror the Vue locale vocabulary (frontend/src/i18n/locales, apps.*
// keys, present in all five locales); tones map TDesign themes onto the
// React Status vocabulary (danger→error, default→neutral).

// ConnectionsView stateLabel: active/revoked get their apps.common label,
// anything else stays verbatim as 状态：{state} instead of being coerced.
export function connectionStateLabel(state:string):string {
 if(state==='active') return '活跃';
 if(state==='revoked') return '已断开';
 return `状态：${state}`;
}

// ConnectionsView stateTheme: active→success, revoked→danger, else default.
export function connectionStateTone(state:string):'success'|'error'|'neutral' {
 if(state==='active') return 'success';
 if(state==='revoked') return 'error';
 return 'neutral';
}

// AppsView installationStateLabel: active/disabled vocabulary; an absent
// state (older backend) renders as an em dash like the other Vue fallbacks.
export function installationStateLabel(state:string|undefined):string {
 if(state==='active') return '活跃';
 if(state==='disabled') return '已停用';
 if(state===undefined) return '—';
 return `状态：${state}`;
}

// ConnectionsView revoke catch: a 409 status or VERSION_CONFLICT code is the
// server's CAS refusing a stale auth_version — a conflict that must prompt a
// re-read, never a fabricated success or a generic failure.
export function revokeFailureKind(e:unknown):'conflict'|'generic' {
 const err=e as { status?:unknown; code?:unknown } | null;
 if(err!==null&&typeof err==='object') {
  if(err.status===409) return 'conflict';
  if(err.code==='VERSION_CONFLICT') return 'conflict';
 }
 return 'generic';
}

// AppsView riskLabel: apps.risk vocabulary, unknown risks stay verbatim.
export function catalogRiskLabel(risk:string):string {
 if(risk==='read') return '只读';
 if(risk==='write') return '写入';
 if(risk==='send') return '发送';
 if(risk==='delete') return '删除';
 return risk;
}

// AppsView riskTheme: read→success, write→warning, send/delete→danger.
export function catalogRiskTone(risk:string):'success'|'warning'|'error'|'neutral' {
 if(risk==='read') return 'success';
 if(risk==='write') return 'warning';
 if(risk==='send'||risk==='delete') return 'error';
 return 'neutral';
}

// AppsView published column tag: 已发布(success) / 未发布(default).
export function catalogPublishedLabel(published:boolean):string {
 return published?'已发布':'未发布';
}
