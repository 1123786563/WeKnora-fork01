// O04: Craft usage display vocabulary. The server aggregates the O01 usage
// ledger (physical model-call facts) into UsageView; unknown consumption is
// never folded into zero token counts — it stays its own visible line so a
// space administrator can reconcile instead of being shown a fabricated
// "free"/complete number. Amounts are intentionally NOT part of this module:
// money comes only from the commercial view.

export interface UsageView {knownCalls:number;unknownCalls:number;inputTokens:number;outputTokens:number;funding:'platform'|'byok'|'mixed'}
export function usageLabel(v:UsageView):string {
  return `已记录 ${v.knownCalls} 次调用，${v.unknownCalls} 次用量待核对`;
}

// fundingNote explains who bears the model cost. BYOK calls run on the
// space's own credentials: the platform charges nothing for them and the UI
// must never render them as "free" — the space itself pays its provider.
export function fundingNote(v:UsageView):string {
  if (v.funding === 'byok') return 'BYOK：本次模型调用由空间自有凭据承担，平台不计金额';
  if (v.funding === 'mixed') return '含 BYOK 调用：部分模型调用由空间自有凭据承担，平台不计该部分金额';
  return '';
}

// asOfLabel renders the explicit data-cut stamp. A late usage correction
// arrives as a new revision; the panel offers a refresh that re-reads the
// ledger and moves as_of forward — the displayed numbers are always tied to
// the moment they were read.
export function asOfLabel(asOf:string):string {
  const m = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})/.exec(asOf);
  const stamp = m ? `${m[1]} ${m[2]} UTC` : asOf;
  return `数据截至 ${stamp}（晚到用量修正后可刷新）`;
}

// UsageCallRow is one physical model attempt of the session's runs, as the
// usage endpoint reports it. status 'unknown' means the attempt's usage could
// not be observed (stream break, cancellation) — the row renders as
// "用量待核对", never as zeros.
export interface UsageCallRow {
  callId:string;attemptId:string;runtime:'main'|'oc';delegationId:string;
  modelId:string;funding:string;status:string;
  inputTokens:number;outputTokens:number;cachedTokens:number;observedAt:string;
}

// usageRows renders the main/child call breakdown lines. Main-runtime calls
// and OpenCode child delegations are listed side by side exactly as recorded:
// a parent rollup is never billed as if the child's tokens were the parent's.
export function usageRows(calls:UsageCallRow[]):string[] {
  return calls.map(c => {
    const head = c.runtime === 'oc'
      ? `oc（子执行 ${c.delegationId}）· ${c.modelId} · ${c.funding}`
      : `main · ${c.modelId} · ${c.funding}`;
    if (c.status === 'unknown') return `${head} · 用量待核对`;
    return `${head} · ${c.status} · in ${c.inputTokens} / out ${c.outputTokens}`;
  });
}
