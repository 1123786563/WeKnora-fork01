import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

type Row = Record<string, unknown>;
function rowsOf(payload: unknown): Row[] { if (!payload || typeof payload !== 'object') return []; const value = (payload as Row).items; return Array.isArray(value) ? value.filter((row): row is Row => Boolean(row) && typeof row === 'object' && !Array.isArray(row)) : []; }
function text(row: Row, key: string): string { const value = row[key]; return typeof value === 'string' || typeof value === 'number' ? String(value) : '—'; }
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
  const title = t('system.globalSettings.audit.tabLabel') === 'system.globalSettings.audit.tabLabel' ? '审计日志' : t('system.globalSettings.audit.tabLabel');
  return <section className="wk-system-audit"><header className="wk-settings-panel-heading"><div><h2>{title}</h2><p>{t('system.globalSettings.audit.description') === 'system.globalSettings.audit.description' ? '查看平台级管理操作和结果。' : t('system.globalSettings.audit.description')}</p></div><button type="button" className="wk-audit-refresh" onClick={() => void loadMore(true)} disabled={loading}>{loading ? '加载中...' : '刷新'}</button></header>{loadError ? <Card role="alert"><Status tone="error">{loadError}</Status><button type="button" onClick={() => void loadMore(false)}>重试</button></Card> : null}<Card>{rows.length === 0 ? <Status>暂无审计记录</Status> : <div className="wk-audit-table-wrap"><table className="wk-audit-table"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td>{text(row, 'created_at')}</td><td>{text(row, 'actor_user_id') === '—' ? '系统' : text(row, 'actor_user_id')}</td><td><span className="wk-audit-tag">{text(row, 'action')}</span></td><td>{text(row, 'target_id') === '—' ? text(row, 'request_path') : text(row, 'target_id')}</td><td>{text(row, 'outcome')}</td></tr>)}</tbody></table></div>}{cursor > 0 ? <button type="button" className="wk-audit-load-more" onClick={() => void loadMore()} disabled={loading}>{loading ? '加载中...' : '加载更多'}</button> : null}</Card>{selected ? <div className="wk-audit-detail" role="dialog" aria-modal="true" aria-label="审计记录详情"><div className="wk-audit-detail-head"><h3>审计记录详情</h3><button type="button" aria-label="关闭" onClick={() => setSelected(null)}>×</button></div><dl>{Object.entries(selected).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : JSON.stringify(value)}</dd></div>)}</dl></div> : null}</section>;
}
