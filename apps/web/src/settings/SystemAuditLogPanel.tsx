import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { createPortal } from 'react-dom';
import { useEffect, useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

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
  return <section className="wk-system-audit"><header className="wk-settings-panel-heading"><div><h2>{title}</h2><p>{t('system.globalSettings.audit.description') === 'system.globalSettings.audit.description' ? '查看平台级管理操作和结果。' : t('system.globalSettings.audit.description')}</p></div><button type="button" className="wk-audit-refresh" aria-label="刷新" onClick={() => void loadMore(true)} disabled={loading}>{loading ? '↻' : '⟳'}</button></header>{loadError ? <Card role="alert"><Status tone="error">{loadError}</Status><button type="button" onClick={() => void loadMore(true)}>重试</button></Card> : null}<Card>{rows.length === 0 ? <Status>暂无审计记录</Status> : <div className="wk-audit-table-wrap"><table className="wk-audit-table"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => { const date = auditDateParts(text(row, 'created_at'), locale); const target = auditTargetSummary(row); return <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td><div className="wk-audit-time"><span>{date.date}</span><span>{date.time}</span></div></td><td><div className="wk-audit-actor"><span>{text(row, 'actor_user_id') === '—' ? '系统' : text(row, 'actor_user_id').slice(0, 8)}</span>{text(row, 'actor_role') !== '—' ? <small>{text(row, 'actor_role')}</small> : null}</div></td><td><span className={`wk-audit-tag wk-audit-tag--${auditOutcomeTone(row.action)}`}>{text(row, 'action')}</span></td><td><div className="wk-audit-target"><strong>{target.key}</strong>{target.diff ? <small>{target.diff}</small> : null}</div></td><td><span className={`wk-audit-tag wk-audit-tag--${auditOutcomeTone(row.outcome)}`}>{text(row, 'outcome')}</span></td></tr>; })}</tbody></table></div>}{cursor > 0 ? <button type="button" className="wk-audit-load-more" onClick={() => void loadMore()} disabled={loading}>{loading ? '加载中...' : '加载更多'}</button> : null}</Card>{selected ? createPortal(<div className="wk-audit-detail" role="dialog" aria-modal="true" aria-label="审计记录详情"><div className="wk-audit-detail-head"><div><h3>{text(selected, 'action')}</h3><p>{auditDateParts(text(selected, 'created_at'), locale).date} {auditDateParts(text(selected, 'created_at'), locale).time}</p></div><button type="button" aria-label="关闭" onClick={() => setSelected(null)}>×</button></div><section><h4>摘要</h4><dl>{Object.entries({ 时间: `${auditDateParts(text(selected, 'created_at'), locale).date} ${auditDateParts(text(selected, 'created_at'), locale).time}`, 操作者: text(selected, 'actor_user_id') === '—' ? '系统' : text(selected, 'actor_user_id'), 操作: text(selected, 'action'), 结果: text(selected, 'outcome'), 目标: auditTargetSummary(selected).key }).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></section><section><h4>请求</h4><dl>{[['method', text(selected, 'request_method')], ['path', text(selected, 'request_path')]].filter(([, value]) => value !== '—').map(([key, value]) => <div key={key}><dt>{key}</dt><dd className="mono">{value}</dd></div>)}</dl></section><section><h4>详情</h4><pre className="mono">{selected.details === undefined || selected.details === null ? '{}' : typeof selected.details === 'string' ? selected.details : JSON.stringify(selected.details, null, 2)}</pre></section></div>, document.body) : null}</section>;
}
