import type { WeKnoraClient } from '@weknora/api-client';
// S1 评审回收（真迁）：扫描可见域换 tdesign 组件——t-alert（error 分支，
// operation 重试钮）+ t-empty（空态）；header/空态样式平移 settings.td.css
// §18（Vue SystemAuditLog.vue scoped 块，根类 .system-audit-log）。
import { Alert, Button, Empty } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { createPortal } from 'react-dom';
import { useEffect, useRef, useState } from 'react';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

// audit tag tones —— S6 Tailwind 收编后回到 settings-wrapper.css 的
// .wk-audit-tag--* 语义变体（色值 = 原 utility 串字面量）。
const AUDIT_TAG_TONES: Record<string, string> = {
  success: 'wk-audit-tag--success',
  danger: 'wk-audit-tag--danger',
  warning: 'wk-audit-tag--warning',
  neutral: 'wk-audit-tag--neutral',
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
  return <div className="system-audit-log">{/* Vue SystemAuditLog.vue:3-21 — 面板自持 section-header（audit-page-header：
        titlewrap 内 h2 + section-description，margin-bottom 20px、无分割线），
        刷新按钮 20×20 悬于右上；样式平移 settings.td.css §18。 */}
  <header className="section-header audit-page-header"><div className="audit-page-header__title"><h2>{title}</h2><p className="section-description">{t('system.globalSettings.audit.description') === 'system.globalSettings.audit.description' ? copy.description : t('system.globalSettings.audit.description')}</p></div><button type="button" className={`rq-refresh ${loading ? 'rq-refresh-spin' : ''}`} title={copy.refresh} aria-label={copy.refresh} onClick={() => void loadMore(true)} disabled={loading}>{/* Vue SystemAuditLog.vue:14-18 — t-icon refresh/loading（rq-refresh-spin 旋转），本地 sprite 与 Vue 端同 0.4.5 glyph（台账 #10）；几何随 wrapper 共用 .rq-refresh 规则。 */}<TIcon name={loading ? 'loading' : 'refresh'} /></button></header>
  <div className="audit-page-body">
  {/* Vue SystemAuditLog.vue:23-29 — error 分支：audit-page-branch--error 内
      t-alert(theme=error) + operation 重试 t-button(size small)。 */}
  {loadError ? <div className="audit-page-branch audit-page-branch--error"><Alert theme="error" message={loadError} operation={<Button size="small" onClick={() => void loadMore(true)}>{copy.retry}</Button>} /></div> : null}{/* Vue SystemAuditLog.vue:31-39 — 空态为 min-height 280px 居中的
      t-empty（默认「暂无数据」标题 + 描述行），无外框卡片。 */}
  {rows.length === 0 ? <div className="audit-page-branch audit-page-branch--empty" data-testid="audit-empty"><Empty description={copy.empty} /></div> : <div className="audit-page-branch">{/* 表格 + 明细抽屉为 React 保留实现（扫描环境无审计数据不可见，
      测试锚点 wk-audit-table/wk-audit-detail；见 task-12c 报告偏离项）。 */}
  <div><div className="wk-audit-table-wrap"><table className="wk-audit-table"><thead><tr><th>时间</th><th>操作者</th><th>操作</th><th>目标</th><th>结果</th></tr></thead><tbody>{rows.map((row, index) => { const date = auditDateParts(text(row, 'created_at'), locale); const target = auditTargetSummary(row); return <tr key={text(row, 'id') === '—' ? String(index) : text(row, 'id')} tabIndex={0} onClick={() => setSelected(row)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(row); } }}><td><div className="wk-audit-time"><span>{date.date}</span><span>{date.time}</span></div></td><td><div className="wk-audit-actor"><span>{text(row, 'actor_user_id') === '—' ? copy.system : text(row, 'actor_user_id').slice(0, 8)}</span>{text(row, 'actor_role') !== '—' ? <small>{text(row, 'actor_role')}</small> : null}</div></td><td><span className={'wk-audit-tag ' + (AUDIT_TAG_TONES[auditOutcomeTone(row.action)] ?? AUDIT_TAG_TONES.neutral)}>{text(row, 'action')}</span></td><td><div className="wk-audit-target"><strong>{target.key}</strong>{target.diff ? <small>{target.diff}</small> : null}</div></td><td><span className={'wk-audit-tag ' + (AUDIT_TAG_TONES[auditOutcomeTone(row.outcome)] ?? AUDIT_TAG_TONES.neutral)}>{text(row, 'outcome')}</span></td></tr>; })}</tbody></table></div>{cursor > 0 ? <button type="button" className="wk-audit-load-more" onClick={() => void loadMore()} disabled={loading}>{loading ? copy.loading : copy.more}</button> : null}</div></div>}
  </div>{selected ? createPortal(<div className="wk-audit-detail" role="dialog" aria-modal="true" aria-label={copy.detail}><div className="wk-audit-detail-head"><div><h3>{text(selected, 'action')}</h3><p>{auditDateParts(text(selected, 'created_at'), locale).date} {auditDateParts(text(selected, 'created_at'), locale).time}</p></div><button type="button" aria-label={copy.close} onClick={() => setSelected(null)}>×</button></div><section><h4>{copy.summary}</h4><dl>{Object.entries({ [copy.time]: `${auditDateParts(text(selected, 'created_at'), locale).date} ${auditDateParts(text(selected, 'created_at'), locale).time}`, [copy.actor]: text(selected, 'actor_user_id') === '—' ? copy.system : text(selected, 'actor_user_id'), [copy.action]: text(selected, 'action'), [copy.outcome]: text(selected, 'outcome'), [copy.target]: auditTargetSummary(selected).key }).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></section><section><h4>{copy.request}</h4><dl>{[['method', text(selected, 'request_method')], ['path', text(selected, 'request_path')]].filter(([, value]) => value !== '—').map(([key, value]) => <div key={key}><dt>{key}</dt><dd className="mono">{value}</dd></div>)}</dl></section><section><h4>{copy.details}</h4><pre className="mono">{selected.details === undefined || selected.details === null ? '{}' : typeof selected.details === 'string' ? selected.details : JSON.stringify(selected.details, null, 2)}</pre></section></div>, document.body) : null}</div>;
}
