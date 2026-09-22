import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { Icon as TIcon } from 'tdesign-icons-react';
import { createPortal } from 'react-dom';
import { useEffect, useRef, useState } from 'react';
import { EmptyState } from './EmptyState.tsx';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

// audit tag tones (was settings-wrapper.css .wk-audit-tag--* variants + neutral)
const AUDIT_TAG_TONES: Record<string, string> = {
  success: 'bg-[rgba(7,192,95,0.1)] border-[rgba(7,192,95,0.24)] text-[#078a45]',
  danger: 'bg-[rgba(220,60,60,0.08)] border-[rgba(220,60,60,0.2)] text-[#c03939]',
  warning: 'bg-[rgba(234,167,49,0.1)] border-[rgba(234,167,49,0.22)] text-[#9a6a0b]',
  neutral: 'bg-[rgba(120,135,155,0.1)] border-[rgba(120,135,155,0.2)] text-[#5c6b83]',
};
const AUDIT_COPY: Record<string, Record<string, string>> = {
  'zh-CN': { title: '审计日志', description: '记录平台级操作：系统设置变更、系统管理员授予/回收、配额批量同步等。按时间倒序展示。', refresh: '刷新', retry: '重试', empty: '暂无平台级审计事件。', loading: '加载中...', more: '加载更多', detail: '审计记录详情', close: '关闭', summary: '摘要', request: '请求', details: '详情', time: '时间', actor: '操作者', action: '操作', target: '目标', outcome: '结果', system: '系统', loadFailed: '审计日志加载失败' },
  'en-US': { title: 'Audit log', description: 'View platform administration actions and outcomes.', refresh: 'Refresh', retry: 'Retry', empty: 'No audit records', loading: 'Loading...', more: 'Load more', detail: 'Audit record details', close: 'Close', summary: 'Summary', request: 'Request', details: 'Details', time: 'Time', actor: 'Actor', action: 'Action', target: 'Target', outcome: 'Outcome', system: 'System', loadFailed: 'Failed to load audit log' },
  'ja-JP': { title: '監査ログ', description: 'プラットフォーム管理操作と結果を確認します。', refresh: '更新', retry: '再試行', empty: '監査記録はありません', loading: '読み込み中...', more: 'さらに読み込む', detail: '監査記録の詳細', close: '閉じる', summary: '概要', request: 'リクエスト', details: '詳細', time: '時刻', actor: '操作者', action: '操作', target: '対象', outcome: '結果', system: 'システム', loadFailed: '監査ログの読み込みに失敗しました' },
  'ko-KR': { title: '감사 로그', description: '플랫폼 관리 작업과 결과를 확인합니다.', refresh: '새로 고침', retry: '재시도', empty: '감사 기록이 없습니다', loading: '로드 중...', more: '더 불러오기', detail: '감사 기록 세부 정보', close: '닫기', summary: '요약', request: '요청', details: '세부 정보', time: '시간', actor: '작업자', action: '작업', target: '대상', outcome: '결과', system: '시스템', loadFailed: '감사 로그를 불러오지 못했습니다' },
  'ru-RU': { title: 'Журнал аудита', description: 'Просмотр административных действий платформы и их результатов.', refresh: 'Обновить', retry: 'Повторить', empty: 'Записей аудита нет', loading: 'Загрузка...', more: 'Загрузить ещё', detail: 'Подробности записи аудита', close: 'Закрыть', summary: 'Сводка', request: 'Запрос', details: 'Подробности', time: 'Время', actor: 'Оператор', action: 'Действие', target: 'Цель', outcome: 'Результат', system: 'Система', loadFailed: 'Не удалось загрузить журнал аудита' },
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
  const copy = AUDIT_COPY[locale] ?? AUDIT_COPY['zh-CN'];
  const root = payload && typeof payload === 'object' ? payload as Row : {};
  const [selected, setSelected] = useState<Row | null>(null);
  const [rows, setRows] = useState<Row[]>(rowsOf(payload));
  const [cursor, setCursor] = useState(typeof root.nextCursor === 'number' ? root.nextCursor : 0);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const loadGeneration = useRef(0);
  async function loadMore(reset = false) {
    if (loading || (!reset && !cursor)) return;
    const generation = reset ? ++loadGeneration.current : loadGeneration.current;
    setLoading(true); setLoadError(null);
    try { const result = await client.administration.auditLog.list({ limit: 50, ...(reset ? {} : { afterId: cursor }) }); if (generation !== loadGeneration.current) return; setRows((current) => reset ? result.items as Row[] : [...current, ...result.items as Row[]]); setCursor(result.nextCursor || 0); }
    catch (reason) { if (generation === loadGeneration.current) setLoadError(reason instanceof Error ? reason.message : copy.loadFailed); }
    finally { if (generation === loadGeneration.current) setLoading(false); }
  }
  useEffect(() => {
    if (!selected) return;
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') setSelected(null); };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [selected]);
  const title = t('system.globalSettings.audit.tabLabel') === 'system.globalSettings.audit.tabLabel' ? copy.title : t('system.globalSettings.audit.tabLabel');
  return <section className="wk-system-audit grid">{/* Vue SystemAuditLog.vue:3-21 — 面板自持 section-header（20px/600 h2 +
        14px 描述，margin-bottom 20px、无分割线），刷新按钮 20×20 悬于右上。 */}
  <header className="section-header flex items-start justify-between gap-4 max-[720px]:flex-col"><div><h2>{title}</h2><p className="section-description">{t('system.globalSettings.audit.description') === 'system.globalSettings.audit.description' ? copy.description : t('system.globalSettings.audit.description')}</p></div><button type="button" className={`wk-audit-refresh rq-refresh inline-flex h-5 w-5 shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-[12px] transition-colors hover:bg-[rgba(0,0,0,0.06)] hover:text-[#07c05f] disabled:cursor-default disabled:opacity-70 ${loading ? 'animate-[wk-audit-spin_0.8s_linear_infinite]' : ''}`} aria-label={copy.refresh} onClick={() => void loadMore(true)} disabled={loading}>{/* Vue SystemAuditLog.vue:14-18 — t-icon refresh/loading（rq-refresh-spin 旋转），本地 sprite 与 Vue 端同 0.4.5 glyph（台账 #10）。 */}<TIcon name={loading ? 'loading' : 'refresh'} /></button></header>{loadError ? <Card role="alert"><Status tone="error">{loadError}</Status><button type="button" onClick={() => void loadMore(true)}>{copy.retry}</button></Card> : null}{/* Vue SystemAuditLog.vue:34-39 — 空态为 min-height 280px 居中的
      t-empty（默认「暂无数据」标题 + 描述行），无外框卡片。 */}
  {rows.length === 0 ? <div className="wk-audit-empty flex items-center justify-center" data-testid="audit-empty"><EmptyState description={copy.empty} /></div> : <Card><div className="wk-audit-table-wrap overflow-x-auto"><table className="wk-audit-table w-full min-w-[680px] border-collapse text-xs [&_th]:border-b [&_th]:border-[rgba(120,135,155,0.18)] [&_th]:px-2 [&_th]:py-3 [&_th]:text-left [&_td]:border-b [&_td]:border-[rgba(120,135,155,0.18)] [&_td]:px-2 [&_td]:py-3 [&_td]:text-left [&_tbody_tr]:cursor-pointer [&_tbody_tr]:outline-none [&_tbody_tr:hover]:bg-[rgba(7,192,95,0.05)] [&_tbody_tr:focus]:bg-[rgba(7,192,95,0.05)]"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => { const date = auditDateParts(text(row, 'created_at'), locale); const target = auditTargetSummary(row); return <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td><div className="wk-audit-time flex flex-col gap-0.5 [&>span:last-child]:text-[11px] [&>span:last-child]:text-[#8a96a8]"><span>{date.date}</span><span>{date.time}</span></div></td><td><div className="wk-audit-actor flex flex-col gap-0.5 [&>small]:text-[11px] [&>small]:text-[#8a96a8]"><span>{text(row, 'actor_user_id') === '—' ? copy.system : text(row, 'actor_user_id').slice(0, 8)}</span>{text(row, 'actor_role') !== '—' ? <small>{text(row, 'actor_role')}</small> : null}</div></td><td><span className={`wk-audit-tag inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${AUDIT_TAG_TONES[auditOutcomeTone(row.action)] ?? AUDIT_TAG_TONES.neutral}`}>{text(row, 'action')}</span></td><td><div className="wk-audit-target flex flex-col gap-0.5 [&>strong]:break-all [&>strong]:font-mono [&>strong]:text-xs [&>strong]:font-medium [&>small]:text-[11px] [&>small]:text-[#8a96a8]"><strong>{target.key}</strong>{target.diff ? <small>{target.diff}</small> : null}</div></td><td><span className={`wk-audit-tag inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${AUDIT_TAG_TONES[auditOutcomeTone(row.outcome)] ?? AUDIT_TAG_TONES.neutral}`}>{text(row, 'outcome')}</span></td></tr>; })}</tbody></table></div>{cursor > 0 ? <button type="button" className="wk-audit-load-more mx-auto mt-3.5 block cursor-pointer rounded-[4px] border border-[rgba(120,135,155,0.35)] bg-transparent px-3 py-[5px]" onClick={() => void loadMore()} disabled={loading}>{loading ? copy.loading : copy.more}</button> : null}</Card>}{selected ? createPortal(<div className="wk-audit-detail fixed bottom-0 right-0 top-0 z-[3200] w-[min(640px,100vw)] max-w-[640px] overflow-y-auto border-l border-[rgba(120,135,155,0.25)] bg-white p-6 shadow-[-8px_0_24px_rgba(23,32,51,0.14)] [&_section]:border-t [&_section]:border-[rgba(120,135,155,0.16)] [&_section]:py-[18px] [&_h4]:m-0 [&_h4]:mb-3 [&_h4]:text-xs [&_h4]:font-semibold [&_h4]:tracking-[0.04em] [&_h4]:text-[#8a96a8] [&_dl]:m-0 [&_dl]:grid [&_dl]:gap-2.5 [&_dl>div]:border-b [&_dl>div]:border-[rgba(120,135,155,0.18)] [&_dl>div]:pb-2 [&_dt]:text-xs [&_dt]:text-[#5c6b83] [&_dd]:m-0 [&_dd]:mt-1 [&_dd]:break-all [&_pre]:m-0 [&_pre]:whitespace-pre-wrap [&_pre]:rounded-lg [&_pre]:border [&_pre]:border-[rgba(120,135,155,0.18)] [&_pre]:bg-[#f7f9fb] [&_pre]:p-3 [&_pre]:text-xs [&_pre]:leading-[1.55]" role="dialog" aria-modal="true" aria-label={copy.detail}><div className="wk-audit-detail-head mb-6 flex items-start justify-between [&_h3]:m-0 [&_h3]:mb-1.5 [&_h3]:text-lg [&_h3]:font-semibold [&_p]:m-0 [&_p]:text-xs [&_p]:text-[#8a96a8] [&_button]:cursor-pointer [&_button]:border-0 [&_button]:bg-transparent [&_button]:text-2xl"><div><h3>{text(selected, 'action')}</h3><p>{auditDateParts(text(selected, 'created_at'), locale).date} {auditDateParts(text(selected, 'created_at'), locale).time}</p></div><button type="button" aria-label={copy.close} onClick={() => setSelected(null)}>×</button></div><section><h4>{copy.summary}</h4><dl>{Object.entries({ [copy.time]: `${auditDateParts(text(selected, 'created_at'), locale).date} ${auditDateParts(text(selected, 'created_at'), locale).time}`, [copy.actor]: text(selected, 'actor_user_id') === '—' ? copy.system : text(selected, 'actor_user_id'), [copy.action]: text(selected, 'action'), [copy.outcome]: text(selected, 'outcome'), [copy.target]: auditTargetSummary(selected).key }).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></section><section><h4>{copy.request}</h4><dl>{[['method', text(selected, 'request_method')], ['path', text(selected, 'request_path')]].filter(([, value]) => value !== '—').map(([key, value]) => <div key={key}><dt>{key}</dt><dd className="mono">{value}</dd></div>)}</dl></section><section><h4>{copy.details}</h4><pre className="mono">{selected.details === undefined || selected.details === null ? '{}' : typeof selected.details === 'string' ? selected.details : JSON.stringify(selected.details, null, 2)}</pre></section></div>, document.body) : null}</section>;
}
