export interface ValueStore { read(key: string): unknown; write(key: string, value: unknown): void; remove(key: string): void }
export interface IntentRecord { requestId: string; state: 'submitting'|'pending'|'dispatching'|'admitted'|'rejected'|'unknown'; runId?: string }
export class PendingIntent {
  private store: ValueStore; private key: string; private id: () => string;
  constructor(store: ValueStore, scope: string, id: () => string) { this.store=store; this.key=`wk:mini:intent:${scope}`; this.id=id; }
  current(): IntentRecord | null {
    const v=this.store.read(this.key);
    if (!v || typeof v!=='object' || typeof (v as IntentRecord).requestId!=='string') return null;
    return v as IntentRecord;
  }
  begin(): IntentRecord {
    const existing=this.current(); if (existing) return existing;
    const next: IntentRecord={requestId:this.id(),state:'submitting'}; this.store.write(this.key,next); return next;
  }
  reconcile(result: {state: IntentRecord['state']; run_id?: string}): IntentRecord {
    const existing=this.current(); if (!existing) throw new Error('No pending intent');
    if (result.state==='admitted' && !result.run_id) throw new Error('Admitted request requires run id');
    const next={...existing,state:result.state,...(result.run_id?{runId:result.run_id}:{})}; this.store.write(this.key,next); return next;
  }
  reset(): void { if(this.current() && this.current()?.state!=='rejected') throw new Error('Cannot discard an active request'); this.store.remove(this.key); }
  /** Call only after the run navigation/recovery reference was saved. */
  acknowledge(): void { if(this.current()?.state!=='admitted') throw new Error('Request is not admitted'); this.store.remove(this.key); }
}
let sequence=0;
/** Unique intent correlation, NOT a password, credential, signature or auth token. */
export function requestId(): string { sequence+=1; return `mini-${Date.now().toString(36)}-${sequence.toString(36)}-${Math.random().toString(36).slice(2,14)}`; }
