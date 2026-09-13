import type { RuntimeQueues } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';

type Row = Record<string, unknown>;
const runtimeFallbacks: Record<string, string> = {
  'system.globalSettings.runtime.title': '运行时队列',
  'system.globalSettings.runtime.description': '查看任务队列、工作池和模型限流状态。',
  'system.globalSettings.runtime.summary.title': '队列概览',
  'system.globalSettings.runtime.summary.active': '活跃任务',
  'system.globalSettings.runtime.summary.pending': '等待任务',
  'system.globalSettings.runtime.summary.retry': '重试',
  'system.globalSettings.runtime.summary.archived': '失败归档',
  'system.globalSettings.runtime.poolsTitle': '工作池',
  'system.globalSettings.runtime.poolsDescription': '按工作池查看并发和排队情况。',
  'system.globalSettings.runtime.detailsTitle': '队列详情',
  'system.globalSettings.runtime.detailsDescription': '查看各队列任务状态和延迟。',
  'system.globalSettings.runtime.unavailableTitle': '运行时不可用',
  'system.globalSettings.runtime.unavailable': '当前部署未启用运行时队列。',
  'system.globalSettings.runtime.empty': '暂无队列数据。',
  'system.globalSettings.runtime.models.title': '模型限流',
  'system.globalSettings.runtime.models.description': '查看模型并发使用和等待状态。',
  'system.globalSettings.runtime.models.disabled': '模型限流未启用。',
  'system.globalSettings.runtime.models.empty': '暂无模型限流数据。',
  'system.globalSettings.runtime.models.columns.waiting': '等待',
  'system.globalSettings.runtime.status.paused': '已暂停',
  'system.globalSettings.runtime.status.actionRequired': '需处理',
  'system.globalSettings.runtime.status.retrying': '重试中',
  'system.globalSettings.runtime.status.working': '运行中',
  'system.globalSettings.runtime.status.waiting': '等待中',
  'system.globalSettings.runtime.status.idle': '空闲',
};
function fallbackRuntimeText(key: string, values?: Record<string, string | number>): string {
  let value = runtimeFallbacks[key] ?? key.split('.').pop() ?? key;
  for (const [name, replacement] of Object.entries(values ?? {})) value = value.replace(`{${name}}`, String(replacement));
  return value;
}
const numberOf = (row: Row, key: string) => typeof row[key] === 'number' ? row[key] as number : 0;
const stringOf = (row: Row, key: string) => typeof row[key] === 'string' ? row[key] as string : '';
const percent = (active: number, limit: number) => limit > 0 ? Math.min(100, Math.round(active / limit * 100)) : 0;
const runtimeText = (t: ReturnType<typeof settingsT>, key: string, fallback: string, values?: Record<string, string | number>) => {
  const value = t(key, values);
  return value === key ? fallbackRuntimeText(key, values) : value;
};

function queueStatus(row: Row, t: ReturnType<typeof settingsT>): string {
  if (row.paused === true) return t('system.globalSettings.runtime.status.paused');
  if (numberOf(row, 'archived') > 0) return t('system.globalSettings.runtime.status.actionRequired');
  if (numberOf(row, 'retry') > 0) return t('system.globalSettings.runtime.status.retrying');
  if (numberOf(row, 'active') > 0) return t('system.globalSettings.runtime.status.working');
  if (numberOf(row, 'pending') > 0 || numberOf(row, 'scheduled') > 0) return t('system.globalSettings.runtime.status.waiting');
  return t('system.globalSettings.runtime.status.idle');
}

export function RuntimeQueuesPanel({ payload, loading = false, error = null }: { payload: RuntimeQueues | null; loading?: boolean; error?: string | null }) {
  const locale = useSettingsLocale();
  const baseT = settingsT(locale);
  const t: ReturnType<typeof settingsT> = (key, values) => {
    const value = baseT(key, values);
    return value === key && key.startsWith('system.globalSettings.runtime.') ? fallbackRuntimeText(key, values) : value;
  };
  if (loading && !payload) return <section className="wk-runtime-queues" aria-live="polite"><div className="wk-rq-skeleton" /><div className="wk-rq-skeleton" /><div className="wk-rq-skeleton" /></section>;
  if (error) return <section className="wk-runtime-queues" role="alert"><Card><Status tone="error">{t('system.globalSettings.runtime.errors.generic')}</Status><p className="wk-muted">{error}</p></Card></section>;
  if (!payload || (!payload.available && !payload.model_limiter_available)) return <section className="wk-runtime-queues"><Card><Status>{t('system.globalSettings.runtime.unavailableTitle')}</Status><p className="wk-muted">{t('system.globalSettings.runtime.unavailable')}</p></Card></section>;
  const queues = payload.queues as Row[];
  const pools = payload.pools as Row[];
  const models = payload.models as Row[];
  const active = queues.reduce((sum, row) => sum + numberOf(row, 'active'), 0);
  const pending = queues.reduce((sum, row) => sum + numberOf(row, 'pending'), 0);
  const retry = queues.reduce((sum, row) => sum + numberOf(row, 'retry'), 0);
  const archived = queues.reduce((sum, row) => sum + numberOf(row, 'archived'), 0);
  const title = runtimeText(t, 'system.globalSettings.runtime.title', '运行时队列');
  return <section className="wk-runtime-queues" aria-label={title}>
    <header className="wk-settings-panel-heading"><h2>{title}</h2><p>{runtimeText(t, 'system.globalSettings.runtime.description', '查看任务队列、工作池和模型限流状态。')}</p></header>
    {payload.available ? <>
      <Card className="wk-rq-overview"><h3>{runtimeText(t, 'system.globalSettings.runtime.summary.title', '队列概览')}</h3><div className="wk-rq-metrics"><strong>{active}<span>{runtimeText(t, 'system.globalSettings.runtime.summary.active', '活跃任务')}</span></strong><strong>{pending}<span>{runtimeText(t, 'system.globalSettings.runtime.summary.pending', '等待任务')}</span></strong><strong className={retry > 0 ? 'is-warning' : ''}>{retry}<span>{runtimeText(t, 'system.globalSettings.runtime.summary.retry', '重试')}</span></strong><strong className={archived > 0 ? 'is-danger' : ''}>{archived}<span>{runtimeText(t, 'system.globalSettings.runtime.summary.archived', '失败归档')}</span></strong></div></Card>
      <Card><h3>{t('system.globalSettings.runtime.poolsTitle')}</h3><p className="wk-muted">{t('system.globalSettings.runtime.poolsDescription')}</p><div className="wk-rq-pools">{pools.map((row, index) => <div className="wk-rq-pool" key={stringOf(row, 'name') || String(index)}><strong>{stringOf(row, 'name') || '—'}</strong><b>{numberOf(row, 'instances') > 0 ? `${numberOf(row, 'active')}/${numberOf(row, 'cluster_capacity')}` : numberOf(row, 'concurrency')}</b><span>{t('system.globalSettings.runtime.queueCount', { value: numberOf(row, 'queue_count') })}</span></div>)}</div></Card>
      <Card><h3>{t('system.globalSettings.runtime.detailsTitle')}</h3><p className="wk-muted">{t('system.globalSettings.runtime.detailsDescription')}</p>{queues.length === 0 ? <Status>{t('system.globalSettings.runtime.empty')}</Status> : <div className="wk-rq-table-wrap"><table className="wk-rq-table"><thead><tr>{['queue','active','pending','retry','archived','completed','latency','status'].map((key) => <th key={key}>{t(`system.globalSettings.runtime.columns.${key}`)}</th>)}</tr></thead><tbody>{queues.map((row, index) => <tr key={stringOf(row, 'name') || String(index)}><th>{stringOf(row, 'name') || '—'}</th><td>{numberOf(row, 'active')}</td><td>{numberOf(row, 'pending')}</td><td>{numberOf(row, 'retry')}</td><td>{numberOf(row, 'archived')}</td><td>{numberOf(row, 'completed')}</td><td>{numberOf(row, 'latency_ms') || '—'}</td><td>{queueStatus(row, t)}</td></tr>)}</tbody></table></div>}</Card>
    </> : null}
    <Card><h3>{t('system.globalSettings.runtime.models.title')}</h3><p className="wk-muted">{t('system.globalSettings.runtime.models.description')}</p>{!payload.model_limiter_available ? <Status>{t('system.globalSettings.runtime.models.disabled')}</Status> : models.length === 0 ? <Status>{t('system.globalSettings.runtime.models.empty')}</Status> : <div className="wk-rq-models">{models.map((row, index) => { const current = numberOf(row, 'active'); const limit = numberOf(row, 'limit'); return <div className="wk-rq-model" key={stringOf(row, 'model_id') || String(index)}><div><strong>{stringOf(row, 'name') || stringOf(row, 'model_id') || '—'}</strong><span>{current} / {limit}</span></div><progress max={100} value={percent(current, limit)} /><small>{numberOf(row, 'waiting')} {t('system.globalSettings.runtime.models.columns.waiting')}</small></div>; })}</div>}</Card>
  </section>;
}
