export interface OrderView {
 id: string; payment: 'pending'|'paid'|'closed';
 fulfillment: 'pending'|'processing'|'fulfilled'|'attention';
 amount_fen: string; currency: 'CNY';
}
// The commercial summary wire shape mirrors the handler projection
// (internal/handler/commercial.go Summary): the purchased subscription or
// the base tier. It is NOT a ledger — available/held/refund_locked are
// served by no endpoint; resource usage comes from
// parseCommercialUsageList (GET /api/v1/commercial/usage) instead.
export interface CommercialSubscription {
 id:string; plan_key:string; plan_version:number;
 paid_until:string|null; version:number; downgrade_reason?:string;
}
export interface CommercialSummary {
 tenant_id:number; subscription:CommercialSubscription|null;
 base_tier:boolean; base_tier_key?:string; can_manage_billing:boolean;
}
export interface CommercialUsageRow { resource:string; used:number; limit:number|null }
export interface QuoteView {
 id:string;amount_fen:string;credit_delta:string;expires_at:string;
 // #81 AC1 additive freeze fields (legacy quotes answer without them):
 currency?:string; features?:Record<string,boolean>; line_items?:PurchaseLineItemView[];
}
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

function integer(value: unknown, field: string, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new Error('invalid ' + label + ' (' + field + ')');
  return value;
}

export function parseCommercialSummary(value:unknown):CommercialSummary {
 if(typeof value!=='object'||value===null||Array.isArray(value)) throw new Error('invalid commercial summary');
 const v=value as Record<string,unknown>;
 integer(v.tenant_id,'tenant_id','commercial summary');
 if(typeof v.base_tier!=='boolean') throw new Error('invalid commercial summary (base_tier)');
 if(typeof v.can_manage_billing!=='boolean') throw new Error('invalid commercial summary (can_manage_billing)');
 if(v.base_tier_key!==undefined&&typeof v.base_tier_key!=='string') throw new Error('invalid commercial summary (base_tier_key)');
 // A null subscription is the legal base-tier branch (B05): nothing else to
 // validate inside it. A purchased subscription validates field by field.
 if(v.subscription===null) return v as unknown as CommercialSummary;
 if(typeof v.subscription!=='object'||v.subscription===null||Array.isArray(v.subscription)) throw new Error('invalid commercial summary (subscription)');
 const sub=v.subscription as Record<string,unknown>;
 nonEmptyString(sub.id,'subscription.id','commercial summary');
 nonEmptyString(sub.plan_key,'subscription.plan_key','commercial summary');
 integer(sub.plan_version,'subscription.plan_version','commercial summary');
 if(sub.paid_until!==null&&typeof sub.paid_until!=='string') throw new Error('invalid commercial summary (subscription.paid_until)');
 integer(sub.version,'subscription.version','commercial summary');
 if(sub.downgrade_reason!==undefined&&typeof sub.downgrade_reason!=='string') throw new Error('invalid commercial summary (subscription.downgrade_reason)');
 // Unknown fields pass through verbatim (contract convention).
 return v as unknown as CommercialSummary;
}

export function parseCommercialUsageList(value:unknown):CommercialUsageRow[] {
 if(!Array.isArray(value)) throw new Error('invalid commercial usage list');
 return value.map((row):CommercialUsageRow=>{
  if(typeof row!=='object'||row===null) throw new Error('invalid commercial usage row');
  const v=row as Record<string,unknown>;
  nonEmptyString(v.resource,'resource','commercial usage row');
  integer(v.used,'used','commercial usage row');
  if(v.limit!==null) integer(v.limit,'limit','commercial usage row');
  return v as unknown as CommercialUsageRow;
 });
}

export function parseQuoteView(value:unknown):QuoteView {
 if(typeof value!=='object'||value===null||Array.isArray(value)) throw new Error('invalid quote');
 const v=value as Record<string,unknown>;
 const id=nonEmptyString(v.id,'id','quote');
 const amount_fen=digitString(v.amount_fen,'amount_fen','quote');
 if(typeof v.credit_delta!=='string'||!/^-?\d+$/.test(v.credit_delta)) throw new Error('invalid quote (credit_delta)');
 const expires_at=nonEmptyString(v.expires_at,'expires_at','quote');
 const out:QuoteView = {id,amount_fen,credit_delta:v.credit_delta,expires_at};
 // #81 AC1 freeze pass-through: currency, frozen entitlements and the
 // subscription-fee line items ride along when present (never invented).
 if(v.currency!==undefined) {
  nonEmptyString(v.currency,'currency','quote');
  out.currency = v.currency as string;
 }
 if(v.features!==undefined) {
  if(typeof v.features!=='object'||v.features===null||Array.isArray(v.features)) throw new Error('invalid quote (features)');
  const features:Record<string,boolean> = {};
  for(const [k,val] of Object.entries(v.features as Record<string,unknown>)) {
   if(typeof val!=='boolean') throw new Error('invalid quote (features)');
   features[k]=val;
  }
  out.features = features;
 }
 if(v.line_items!==undefined) {
  if(!Array.isArray(v.line_items)) throw new Error('invalid quote (line_items)');
  out.line_items = v.line_items.map((row):PurchaseLineItemView=>{
   if(typeof row!=='object'||row===null) throw new Error('invalid quote (line_items)');
   const li=row as Record<string,unknown>;
   return { kind: nonEmptyString(li.kind,'kind','quote line item'),
    name: typeof li.name==='string'?li.name:'',
    amount_fen: digitString(li.amount_fen,'amount_fen','quote line item') };
  });
 }
 return out;
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
  // represented by a lower value.
  const locked_credits=digitString(v.locked_credits,'locked_credits','refund');
  return {id,state:v.state,amount_fen,locked_credits};
}

// #81 purchase vocabulary: PurchaseLineItemView is one frozen invoice line
// (kind is the closed subscription_fee token in the first slice);
// PurchaseView.state is the CLOSED product token set — the provider's raw
// states (e.g. incomplete) NEVER cross this contract (spec L210). Reason is
// the closed platform-failure token, present only on a non-confirmable
// answer.
export interface PurchaseLineItemView { kind:string; name:string; amount_fen:string }
export interface PurchaseView {
  state:'awaiting_payment'|'active'|'absent'|'canceled';
  order?:OrderView; plan_key?:string; plan_version?:number;
  amount_fen?:string; currency?:string; reason?:string;
}

const PURCHASE_STATES = new Set<string>(['awaiting_payment','active','absent','canceled']);
const PURCHASE_REASONS = new Set<string>(['unconfigured','unreachable','invalid_response','unsupported']);

export function parsePurchaseView(value:unknown):PurchaseView {
  if(typeof value!=='object'||value===null||Array.isArray(value)) throw new Error('invalid purchase');
  const v=value as Record<string,unknown>;
  if(typeof v.state!=='string'||!PURCHASE_STATES.has(v.state)) throw new Error('invalid purchase (state)');
  const out:PurchaseView = { state: v.state as PurchaseView['state'] };
  if(v.order!==undefined&&v.order!==null) out.order = parseOrderView(v.order);
  if(v.plan_key!==undefined) out.plan_key = nonEmptyString(v.plan_key,'plan_key','purchase');
  if(v.plan_version!==undefined) {
    const n = integer(v.plan_version,'plan_version','purchase');
    out.plan_version = n;
  }
  if(v.amount_fen!==undefined&&v.amount_fen!==null&&v.amount_fen!=='') {
    out.amount_fen = digitString(v.amount_fen,'amount_fen','purchase');
  }
  if(v.currency!==undefined&&v.currency!=='') out.currency = nonEmptyString(v.currency,'currency','purchase');
  if(v.reason!==undefined&&v.reason!=='') {
    if(typeof v.reason!=='string'||!PURCHASE_REASONS.has(v.reason)) throw new Error('invalid purchase (reason)');
    out.reason = v.reason;
  }
  return out;
}
