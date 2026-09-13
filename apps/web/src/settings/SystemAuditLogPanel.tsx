import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

type Row = Record<string, unknown>;
function rowsOf(payload: unknown): Row[] { if (!payload || typeof payload !== 'object') return []; const value = (payload as Row).items; return Array.isArray(value) ? value.filter((row): row is Row => Boolean(row) && typeof row === 'object' && !Array.isArray(row)) : []; }
function text(row: Row, key: string): string { const value = row[key]; return typeof value === 'string' || typeof value === 'number' ? String(value) : '—'; }
export function SystemAuditLogPanel({ client, payload }: { client: WeKnoraClient; payload: unknown }) {
  void client;
  const locale = useSettingsLocale();
  const t = settingsT(locale);
  const [selected, setSelected] = useState<Row | null>(null);
  const rows = rowsOf(payload);
  const title = t('system.globalSettings.audit.tabLabel') === 'system.globalSettings.audit.tabLabel' ? '审计日志' : t('system.globalSettings.audit.tabLabel');
  return <section className="wk-system-audit"><header className="wk-settings-panel-heading"><h2>{title}</h2><p>{t('system.globalSettings.audit.description') === 'system.globalSettings.audit.description' ? '查看平台级管理操作和结果。' : t('system.globalSettings.audit.description')}</p></header><Card>{rows.length === 0 ? <Status>暂无审计记录</Status> : <div className="wk-audit-table-wrap"><table className="wk-audit-table"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td>{text(row, 'created_at')}</td><td>{text(row, 'actor_user_id') === '—' ? '系统' : text(row, 'actor_user_id')}</td><td><span className="wk-audit-tag">{text(row, 'action')}</span></td><td>{text(row, 'target_id') === '—' ? text(row, 'request_path') : text(row, 'target_id')}</td><td>{text(row, 'outcome')}</td></tr>)}</tbody></table></div>}</Card>{selected ? <div className="wk-audit-detail" role="dialog" aria-modal="true" aria-label="审计记录详情"><div className="wk-audit-detail-head"><h3>审计记录详情</h3><button type="button" aria-label="关闭" onClick={() => setSelected(null)}>×</button></div><dl>{Object.entries(selected).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : JSON.stringify(value)}</dd></div>)}</dl></div> : null}</section>;
}
