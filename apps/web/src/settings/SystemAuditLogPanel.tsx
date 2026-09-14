import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { createPortal } from 'react-dom';
import { useEffect, useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

// audit tag tones (was settings-wrapper.css .wk-audit-tag--* variants + neutral)
const AUDIT_TAG_TONES: Record<string, string> = {
  success: 'bg-[rgba(7,192,95,0.1)] border-[rgba(7,192,95,0.24)] text-[#078a45]',
  danger: 'bg-[rgba(220,60,60,0.08)] border-[rgba(220,60,60,0.2)] text-[#c03939]',
  warning: 'bg-[rgba(234,167,49,0.1)] border-[rgba(234,167,49,0.22)] text-[#9a6a0b]',
  neutral: 'bg-[rgba(120,135,155,0.1)] border-[rgba(120,135,155,0.2)] text-[#5c6b83]',
};

type Row = Record<string, unknown>;
function rowsOf(payload: unknown): Row[] { if (!payload || typeof payload !== 'object') return []; const value = (payload as Row).items; return Array.isArray(value) ? value.filter((row): row is Row => Boolean(row) && typeof row === 'object' && !Array.isArray(row)) : []; }
function text(row: Row, key: string): string { const value = row[key]; return typeof value === 'string' || typeof value === 'number' ? String(value) : '—'; }
export function auditDateParts(value: string | undefined, locale: string): { date: string; time: string } {
  if (!value) return { date: '-', time: '' };
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return { date: value, time: '' };
  return {
    date: new Intl.DateTimeFormat(locale, { year: 'numeric', month: '2-digit', day: '2-digit' }).format(date),
    time: new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date),
  };
}
export function auditOutcomeTone(outcome: unknown): 'success' | 'danger' | 'default' { return outcome === 'denied' ? 'danger' : outcome === 'success' ? 'success' : 'default'; }
export function auditTargetSummary(row: Row): { key: string; diff: string } {
  const details = row.details && typeof row.details === 'object' && !Array.isArray(row.details) ? row.details as Row : {};
  if (row.action === 'system.setting_changed') {
    const key = text(details, 'key') !== '—' ? text(details, 'key') : text(row, 'target_id') !== '—' ? text(row, 'target_id') : text(row, 'target_type');
    const before = details.before;
    const after = details.after;
    return { key, diff: before !== undefined && after !== undefined ? `${String(before)} → ${String(after)}` : '' };
  }
  if (row.action === 'system.queue_task_retried' || row.action === 'system.queue_task_run_now' || row.action === 'system.queue_task_cancelled' || row.action === 'system.queue_task_deleted') {
    const queue = text(details, 'queue');
    const task = text(details, 'task_id') !== '—' ? text(details, 'task_id') : text(row, 'target_id');
    return { key: queue !== '—' && task !== '—' ? `${queue}:${task}` : task !== '—' ? task : queue, diff: '' };
  }
  const target = text(row, 'target_id') !== '—' ? text(row, 'target_id') : text(row, 'target_type');
  return { key: target, diff: '' };
}
export function SystemAuditLogPanel({ client, payload }: { client: WeKnoraClient; payload: unknown }) {
  const locale = useSettingsLocale();
  const t = settingsT(locale);
  const root = payload && typeof payload === 'object' ? payload as Row : {};
  const [selected, setSelected] = useState<Row | null>(null);
  const [rows, setRows] = useState<Row[]>(rowsOf(payload));
  const [cursor, setCursor] = useState(typeof root.nextCursor === 'number' ? root.nextCursor : 0);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  async function loadMore(reset = false) {
    if (loading || (!reset && !cursor)) return;
    setLoading(true); setLoadError(null);
    try { const result = await client.administration.auditLog.list({ limit: 50, ...(reset ? {} : { afterId: cursor }) }); setRows((current) => reset ? result.items as Row[] : [...current, ...result.items as Row[]]); setCursor(result.nextCursor || 0); }
    catch (reason) { setLoadError(reason instanceof Error ? reason.message : '审计日志加载失败'); }
    finally { setLoading(false); }
  }
  useEffect(() => {
    if (!selected) return;
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') setSelected(null); };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [selected]);
  const title = t('system.globalSettings.audit.tabLabel') === 'system.globalSettings.audit.tabLabel' ? '审计日志' : t('system.globalSettings.audit.tabLabel');
  return <section className="wk-system-audit grid gap-4"><header className="wk-settings-panel-heading"><div><h2>{title}</h2><p>{t('system.globalSettings.audit.description') === 'system.globalSettings.audit.description' ? '查看平台级管理操作和结果。' : t('system.globalSettings.audit.description')}</p></div><button type="button" className={`wk-audit-refresh inline-flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded-md border-0 p-0 text-sm text-[#8a96a8] transition-colors hover:bg-[rgba(120,135,155,0.12)] hover:text-[#07c05f] disabled:cursor-default disabled:opacity-70 ${loading ? 'animate-[wk-audit-spin_0.8s_linear_infinite]' : ''}`} aria-label="刷新" onClick={() => void loadMore(true)} disabled={loading}>{loading ? '↻' : '⟳'}</button></header>{loadError ? <Card role="alert"><Status tone="error">{loadError}</Status><button type="button" onClick={() => void loadMore(true)}>重试</button></Card> : null}<Card>{rows.length === 0 ? <Status>暂无审计记录</Status> : <div className="wk-audit-table-wrap overflow-x-auto"><table className="wk-audit-table w-full min-w-[680px] border-collapse text-xs [&_th]:border-b [&_th]:border-[rgba(120,135,155,0.18)] [&_th]:px-2 [&_th]:py-3 [&_th]:text-left [&_td]:border-b [&_td]:border-[rgba(120,135,155,0.18)] [&_td]:px-2 [&_td]:py-3 [&_td]:text-left [&_tbody_tr]:cursor-pointer [&_tbody_tr]:outline-none [&_tbody_tr:hover]:bg-[rgba(7,192,95,0.05)] [&_tbody_tr:focus]:bg-[rgba(7,192,95,0.05)]"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => { const date = auditDateParts(text(row, 'created_at'), locale); const target = auditTargetSummary(row); return <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td><div className="wk-audit-time flex flex-col gap-0.5 [&>span:last-child]:text-[11px] [&>span:last-child]:text-[#8a96a8]"><span>{date.date}</span><span>{date.time}</span></div></td><td><div className="wk-audit-actor flex flex-col gap-0.5 [&>small]:text-[11px] [&>small]:text-[#8a96a8]"><span>{text(row, 'actor_user_id') === '—' ? '系统' : text(row, 'actor_user_id').slice(0, 8)}</span>{text(row, 'actor_role') !== '—' ? <small>{text(row, 'actor_role')}</small> : null}</div></td><td><span className={`wk-audit-tag inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${AUDIT_TAG_TONES[auditOutcomeTone(row.action)] ?? AUDIT_TAG_TONES.neutral}`}>{text(row, 'action')}</span></td><td><div className="wk-audit-target flex flex-col gap-0.5 [&>strong]:break-all [&>strong]:font-mono [&>strong]:text-xs [&>strong]:font-medium [&>small]:text-[11px] [&>small]:text-[#8a96a8]"><strong>{target.key}</strong>{target.diff ? <small>{target.diff}</small> : null}</div></td><td><span className={`wk-audit-tag inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${AUDIT_TAG_TONES[auditOutcomeTone(row.outcome)] ?? AUDIT_TAG_TONES.neutral}`}>{text(row, 'outcome')}</span></td></tr>; })}</tbody></table></div>}{cursor > 0 ? <button type="button" className="wk-audit-load-more mx-auto mt-3.5 block cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={() => void loadMore()} disabled={loading}>{loading ? '加载中...' : '加载更多'}</button> : null}</Card>{selected ? createPortal(<div className="wk-audit-detail fixed bottom-0 right-0 top-0 z-[3200] w-[min(640px,100vw)] max-w-[640px] overflow-y-auto border-l border-[rgba(120,135,155,0.25)] bg-white p-6 shadow-[-8px_0_24px_rgba(23,32,51,0.14)] [&_section]:border-t [&_section]:border-[rgba(120,135,155,0.16)] [&_section]:py-[18px] [&_h4]:m-0 [&_h4]:mb-3 [&_h4]:text-xs [&_h4]:font-semibold [&_h4]:tracking-[0.04em] [&_h4]:text-[#8a96a8] [&_dl]:m-0 [&_dl]:grid [&_dl]:gap-2.5 [&_dl>div]:border-b [&_dl>div]:border-[rgba(120,135,155,0.18)] [&_dl>div]:pb-2 [&_dt]:text-xs [&_dt]:text-[#5c6b83] [&_dd]:m-0 [&_dd]:mt-1 [&_dd]:break-all [&_pre]:m-0 [&_pre]:whitespace-pre-wrap [&_pre]:rounded-lg [&_pre]:border [&_pre]:border-[rgba(120,135,155,0.18)] [&_pre]:bg-[#f7f9fb] [&_pre]:p-3 [&_pre]:text-xs [&_pre]:leading-[1.55]" role="dialog" aria-modal="true" aria-label="审计记录详情"><div className="wk-audit-detail-head mb-6 flex items-start justify-between [&_h3]:m-0 [&_h3]:mb-1.5 [&_h3]:text-lg [&_h3]:font-semibold [&_p]:m-0 [&_p]:text-xs [&_p]:text-[#8a96a8] [&_button]:cursor-pointer [&_button]:border-0 [&_button]:bg-transparent [&_button]:text-2xl"><div><h3>{text(selected, 'action')}</h3><p>{auditDateParts(text(selected, 'created_at'), locale).date} {auditDateParts(text(selected, 'created_at'), locale).time}</p></div><button type="button" aria-label="关闭" onClick={() => setSelected(null)}>×</button></div><section><h4>摘要</h4><dl>{Object.entries({ 时间: `${auditDateParts(text(selected, 'created_at'), locale).date} ${auditDateParts(text(selected, 'created_at'), locale).time}`, 操作者: text(selected, 'actor_user_id') === '—' ? '系统' : text(selected, 'actor_user_id'), 操作: text(selected, 'action'), 结果: text(selected, 'outcome'), 目标: auditTargetSummary(selected).key }).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></section><section><h4>请求</h4><dl>{[['method', text(selected, 'request_method')], ['path', text(selected, 'request_path')]].filter(([, value]) => value !== '—').map(([key, value]) => <div key={key}><dt>{key}</dt><dd className="mono">{value}</dd></div>)}</dl></section><section><h4>详情</h4><pre className="mono">{selected.details === undefined || selected.details === null ? '{}' : typeof selected.details === 'string' ? selected.details : JSON.stringify(selected.details, null, 2)}</pre></section></div>, document.body) : null}</section>;
}
