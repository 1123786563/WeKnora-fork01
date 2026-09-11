export interface OrderView {
 id: string; payment: 'pending'|'paid'|'closed';
 fulfillment: 'pending'|'processing'|'fulfilled'|'attention';
 amount_fen: string; currency: 'CNY';
}
export interface CommercialSummary {
 plan_name:string; paid_until:string|null; available:string;
 held:string; refund_locked:string; as_of:string; stale:boolean;
}
export interface QuoteView { id:string;amount_fen:string;credit_delta:string;expires_at:string; }
export interface QuoteInput { plan_key:string; plan_version:number; subscription_version:number; }
export interface CreateOrderInput { quote_id:string; provider:'wechat'|'alipay'; idempotency_key:string; }
export interface RefundInput { order_id:string; amount_fen:string; reason:string; idempotency_key:string; }
export function parseOrderView(value:unknown):OrderView {
 if(typeof value!=='object'||value===null) throw new Error('invalid order');
 const v=value as Record<string,unknown>;
 if(typeof v.id!=='string'||typeof v.amount_fen!=='string'||!/^\d+$/.test(v.amount_fen)||
    v.currency!=='CNY'||!['pending','paid','closed'].includes(String(v.payment))||
    !['pending','processing','fulfilled','attention'].includes(String(v.fulfillment))) throw new Error('invalid order');
 return v as unknown as OrderView;
}

function digitString(value: unknown, field: string, label: string): string {
  if (typeof value !== 'string' || !/^\d+$/.test(value)) throw new Error('invalid ' + label + ' (' + field + ')');
  return value;
}

function nonEmptyString(value: unknown, field: string, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error('invalid ' + label + ' (' + field + ')');
  return value;
}

export function parseCommercialSummary(value:unknown):CommercialSummary {
 if(typeof value!=='object'||value===null||Array.isArray(value)) throw new Error('invalid commercial summary');
 const v=value as Record<string,unknown>;
 const plan_name=nonEmptyString(v.plan_name,'plan_name','commercial summary');
 if(v.paid_until!==null&&typeof v.paid_until!=='string') throw new Error('invalid commercial summary (paid_until)');
 const available=digitString(v.available,'available','commercial summary');
 const held=digitString(v.held,'held','commercial summary');
 const refund_locked=digitString(v.refund_locked,'refund_locked','commercial summary');
 const as_of=nonEmptyString(v.as_of,'as_of','commercial summary');
 if(typeof v.stale!=='boolean') throw new Error('invalid commercial summary (stale)');
 return {plan_name,paid_until:v.paid_until,available,held,refund_locked,as_of,stale:v.stale};
}

export function parseQuoteView(value:unknown):QuoteView {
 if(typeof value!=='object'||value===null||Array.isArray(value)) throw new Error('invalid quote');
 const v=value as Record<string,unknown>;
 const id=nonEmptyString(v.id,'id','quote');
 const amount_fen=digitString(v.amount_fen,'amount_fen','quote');
 if(typeof v.credit_delta!=='string'||!/^-?\d+$/.test(v.credit_delta)) throw new Error('invalid quote (credit_delta)');
 const expires_at=nonEmptyString(v.expires_at,'expires_at','quote');
 return {id,amount_fen,credit_delta:v.credit_delta,expires_at};
}

export interface RefundView { id:string; state:string; amount_fen:string; locked_credits:string; }

// C05 refund lifecycle vocabulary (internal/commercial/refund.go). The wire
// contract accepts exactly these states; 'refund_unknown' and 'rejected' exist
// only as web-layer display fallbacks (refundMessage), never as server
// projection states.
const REFUND_VIEW_STATES = new Set<string>([
  'requested',
  'reviewing',
  'pending',
  'revocation_pending',
  'completed',
  'failed_confirmed',
  'not_created_confirmed',
]);

export function parseRefundView(value:unknown):RefundView {
  if(typeof value!=='object'||value===null||Array.isArray(value)) throw new Error('invalid refund');
  const v=value as Record<string,unknown>;
  const id=nonEmptyString(v.id,'id','refund');
  if(typeof v.state!=='string'||!REFUND_VIEW_STATES.has(v.state)) throw new Error('invalid refund (state)');
  const amount_fen=digitString(v.amount_fen,'amount_fen','refund');
  // C05: locked credits are held credits and are never negative; a release is
  // represented by a lower value (mirrors CommercialSummary.refund_locked).
  const locked_credits=digitString(v.locked_credits,'locked_credits','refund');
  return {id,state:v.state,amount_fen,locked_credits};
}
