import type { RuntimeQueues, WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { settingsT, useSettingsLocale } from './PortedSectionsPanel.tsx';
import { useRef, useState } from 'react';

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
const runtimeLocaleFallbacks: Record<string, Record<string, string>> = {
  'en-US': {
    'system.globalSettings.runtime.title': 'Runtime queues', 'system.globalSettings.runtime.description': 'View task queues, worker pools and model limiter status.', 'system.globalSettings.runtime.summary.title': 'Queue overview', 'system.globalSettings.runtime.summary.active': 'Active tasks', 'system.globalSettings.runtime.summary.pending': 'Pending tasks', 'system.globalSettings.runtime.summary.retry': 'Retries', 'system.globalSettings.runtime.summary.archived': 'Failed archive', 'system.globalSettings.runtime.poolsTitle': 'Worker pools', 'system.globalSettings.runtime.poolsDescription': 'View concurrency and queueing by worker pool.', 'system.globalSettings.runtime.detailsTitle': 'Queue details', 'system.globalSettings.runtime.detailsDescription': 'View task status and latency for each queue.', 'system.globalSettings.runtime.unavailableTitle': 'Runtime unavailable', 'system.globalSettings.runtime.unavailable': 'Runtime queues are not enabled in this deployment.', 'system.globalSettings.runtime.empty': 'No queue data.', 'system.globalSettings.runtime.models.title': 'Model limiter', 'system.globalSettings.runtime.models.description': 'View model concurrency and waiting status.', 'system.globalSettings.runtime.models.disabled': 'Model limiter is not enabled.', 'system.globalSettings.runtime.models.empty': 'No model limiter data.', 'system.globalSettings.runtime.models.columns.waiting': 'Waiting', 'system.globalSettings.runtime.status.paused': 'Paused', 'system.globalSettings.runtime.status.actionRequired': 'Action required', 'system.globalSettings.runtime.status.retrying': 'Retrying', 'system.globalSettings.runtime.status.working': 'Running', 'system.globalSettings.runtime.status.waiting': 'Waiting', 'system.globalSettings.runtime.status.idle': 'Idle',
  },
  'ja-JP': {
    'system.globalSettings.runtime.title': 'ランタイムキュー', 'system.globalSettings.runtime.description': 'タスクキュー、ワーカープール、モデル制限の状態を確認します。', 'system.globalSettings.runtime.summary.title': 'キュー概要', 'system.globalSettings.runtime.summary.active': '実行中のタスク', 'system.globalSettings.runtime.summary.pending': '待機中のタスク', 'system.globalSettings.runtime.summary.retry': '再試行', 'system.globalSettings.runtime.summary.archived': '失敗アーカイブ', 'system.globalSettings.runtime.poolsTitle': 'ワーカープール', 'system.globalSettings.runtime.poolsDescription': 'プールごとの同時実行数と待機状況を確認します。', 'system.globalSettings.runtime.detailsTitle': 'キュー詳細', 'system.globalSettings.runtime.detailsDescription': '各キューのタスク状態と遅延を確認します。', 'system.globalSettings.runtime.unavailableTitle': 'ランタイムを利用できません', 'system.globalSettings.runtime.unavailable': 'この環境ではランタイムキューが有効になっていません。', 'system.globalSettings.runtime.empty': 'キューデータはありません。', 'system.globalSettings.runtime.models.title': 'モデル制限', 'system.globalSettings.runtime.models.description': 'モデルの同時実行数と待機状態を確認します。', 'system.globalSettings.runtime.models.disabled': 'モデル制限は有効になっていません。', 'system.globalSettings.runtime.models.empty': 'モデル制限データはありません。', 'system.globalSettings.runtime.models.columns.waiting': '待機', 'system.globalSettings.runtime.status.paused': '一時停止', 'system.globalSettings.runtime.status.actionRequired': '要対応', 'system.globalSettings.runtime.status.retrying': '再試行中', 'system.globalSettings.runtime.status.working': '実行中', 'system.globalSettings.runtime.status.waiting': '待機中', 'system.globalSettings.runtime.status.idle': 'アイドル',
  },
  'ko-KR': {
    'system.globalSettings.runtime.title': '런타임 큐', 'system.globalSettings.runtime.description': '작업 큐, 워커 풀 및 모델 제한 상태를 확인합니다.', 'system.globalSettings.runtime.summary.title': '큐 개요', 'system.globalSettings.runtime.summary.active': '활성 작업', 'system.globalSettings.runtime.summary.pending': '대기 작업', 'system.globalSettings.runtime.summary.retry': '재시도', 'system.globalSettings.runtime.summary.archived': '실패 보관', 'system.globalSettings.runtime.poolsTitle': '워커 풀', 'system.globalSettings.runtime.poolsDescription': '워커 풀별 동시성과 대기 상태를 확인합니다.', 'system.globalSettings.runtime.detailsTitle': '큐 세부 정보', 'system.globalSettings.runtime.detailsDescription': '큐별 작업 상태와 지연 시간을 확인합니다.', 'system.globalSettings.runtime.unavailableTitle': '런타임을 사용할 수 없음', 'system.globalSettings.runtime.unavailable': '이 배포에서는 런타임 큐가 활성화되지 않았습니다.', 'system.globalSettings.runtime.empty': '큐 데이터가 없습니다.', 'system.globalSettings.runtime.models.title': '모델 제한', 'system.globalSettings.runtime.models.description': '모델 동시 사용량과 대기 상태를 확인합니다.', 'system.globalSettings.runtime.models.disabled': '모델 제한이 활성화되지 않았습니다.', 'system.globalSettings.runtime.models.empty': '모델 제한 데이터가 없습니다.', 'system.globalSettings.runtime.models.columns.waiting': '대기', 'system.globalSettings.runtime.status.paused': '일시 중지', 'system.globalSettings.runtime.status.actionRequired': '처리 필요', 'system.globalSettings.runtime.status.retrying': '재시도 중', 'system.globalSettings.runtime.status.working': '실행 중', 'system.globalSettings.runtime.status.waiting': '대기 중', 'system.globalSettings.runtime.status.idle': '유휴',
  },
  'ru-RU': {
    'system.globalSettings.runtime.title': 'Очереди среды выполнения', 'system.globalSettings.runtime.description': 'Просмотр очередей задач, пулов работников и ограничений моделей.', 'system.globalSettings.runtime.summary.title': 'Обзор очередей', 'system.globalSettings.runtime.summary.active': 'Активные задачи', 'system.globalSettings.runtime.summary.pending': 'Ожидающие задачи', 'system.globalSettings.runtime.summary.retry': 'Повторы', 'system.globalSettings.runtime.summary.archived': 'Архив ошибок', 'system.globalSettings.runtime.poolsTitle': 'Пулы работников', 'system.globalSettings.runtime.poolsDescription': 'Просмотр параллелизма и очереди по пулу.', 'system.globalSettings.runtime.detailsTitle': 'Подробности очередей', 'system.globalSettings.runtime.detailsDescription': 'Просмотр состояния задач и задержки каждой очереди.', 'system.globalSettings.runtime.unavailableTitle': 'Среда выполнения недоступна', 'system.globalSettings.runtime.unavailable': 'В этом развертывании очереди среды выполнения не включены.', 'system.globalSettings.runtime.empty': 'Данных очередей нет.', 'system.globalSettings.runtime.models.title': 'Ограничение моделей', 'system.globalSettings.runtime.models.description': 'Просмотр параллельного использования моделей и ожидания.', 'system.globalSettings.runtime.models.disabled': 'Ограничение моделей не включено.', 'system.globalSettings.runtime.models.empty': 'Данных ограничений моделей нет.', 'system.globalSettings.runtime.models.columns.waiting': 'Ожидание', 'system.globalSettings.runtime.status.paused': 'Приостановлено', 'system.globalSettings.runtime.status.actionRequired': 'Требуется действие', 'system.globalSettings.runtime.status.retrying': 'Повтор', 'system.globalSettings.runtime.status.working': 'Выполняется', 'system.globalSettings.runtime.status.waiting': 'Ожидание', 'system.globalSettings.runtime.status.idle': 'Простой',
  },
};
const drawerCopy: Record<string, { detail: string; close: string; loading: string; empty: string; unavailable: string; failed: string }> = {
  'zh-CN': { detail: '任务详情', close: '关闭', loading: '加载中...', empty: '暂无任务', unavailable: '运行时队列不可用。', failed: '任务加载失败。' },
  'en-US': { detail: 'Task details', close: 'Close', loading: 'Loading...', empty: 'No tasks', unavailable: 'Runtime queue unavailable.', failed: 'Failed to load tasks.' },
  'ja-JP': { detail: 'タスク詳細', close: '閉じる', loading: '読み込み中...', empty: 'タスクはありません', unavailable: 'ランタイムキューを利用できません。', failed: 'タスクの読み込みに失敗しました。' },
  'ko-KR': { detail: '작업 세부 정보', close: '닫기', loading: '로드 중...', empty: '작업이 없습니다', unavailable: '런타임 큐를 사용할 수 없습니다.', failed: '작업을 불러오지 못했습니다.' },
  'ru-RU': { detail: 'Подробности задачи', close: 'Закрыть', loading: 'Загрузка...', empty: 'Задач нет', unavailable: 'Очередь среды выполнения недоступна.', failed: 'Не удалось загрузить задачи.' },
};
function fallbackRuntimeText(key: string, values?: Record<string, string | number>, locale = 'zh-CN'): string {
  let value = runtimeLocaleFallbacks[locale]?.[key] ?? runtimeFallbacks[key] ?? key.split('.').pop() ?? key;
  for (const [name, replacement] of Object.entries(values ?? {})) value = value.replace(`{${name}}`, String(replacement));
  return value;
}
const numberOf = (row: Row, key: string) => typeof row[key] === 'number' ? row[key] as number : 0;
const stringOf = (row: Row, key: string) => typeof row[key] === 'string' ? row[key] as string : '';
const percent = (active: number, limit: number) => limit > 0 ? Math.min(100, Math.round(active / limit * 100)) : 0;
const runtimeText = (t: ReturnType<typeof settingsT>, key: string, fallback: string, values?: Record<string, string | number>, locale = 'zh-CN') => {
  const value = t(key, values);
  return value === key ? fallbackRuntimeText(key, values, locale) : value;
};

function queueStatus(row: Row, t: ReturnType<typeof settingsT>): string {
  if (row.paused === true) return t('system.globalSettings.runtime.status.paused');
  if (numberOf(row, 'archived') > 0) return t('system.globalSettings.runtime.status.actionRequired');
  if (numberOf(row, 'retry') > 0) return t('system.globalSettings.runtime.status.retrying');
  if (numberOf(row, 'active') > 0) return t('system.globalSettings.runtime.status.working');
  if (numberOf(row, 'pending') > 0 || numberOf(row, 'scheduled') > 0) return t('system.globalSettings.runtime.status.waiting');
  return t('system.globalSettings.runtime.status.idle');
}

export function RuntimeQueuesPanel({ client, payload, loading = false, error = null }: { client: WeKnoraClient; payload: RuntimeQueues | null; loading?: boolean; error?: string | null }) {
  const locale = useSettingsLocale();
  const baseT = settingsT(locale);
  const drawer = drawerCopy[locale] ?? drawerCopy['zh-CN'];
  const t: ReturnType<typeof settingsT> = (key, values) => {
    const value = baseT(key, values);
    return value === key && key.startsWith('system.globalSettings.runtime.') ? fallbackRuntimeText(key, values, locale) : value;
  };
  const [taskDrawer, setTaskDrawer] = useState<{ queue: string; state: string; tasks: Row[]; loading: boolean; error: string | null } | null>(null);
  const taskRequestGeneration = useRef(0);
  if (loading && !payload) return <section className="wk-runtime-queues grid gap-4" aria-live="polite"><div className="wk-rq-skeleton h-[84px] rounded-lg bg-[linear-gradient(90deg,rgba(120,135,155,0.08),rgba(120,135,155,0.18),rgba(120,135,155,0.08))]" /><div className="wk-rq-skeleton h-[84px] rounded-lg bg-[linear-gradient(90deg,rgba(120,135,155,0.08),rgba(120,135,155,0.18),rgba(120,135,155,0.08))]" /><div className="wk-rq-skeleton h-[84px] rounded-lg bg-[linear-gradient(90deg,rgba(120,135,155,0.08),rgba(120,135,155,0.18),rgba(120,135,155,0.08))]" /></section>;
  if (error) return <section className="wk-runtime-queues" role="alert"><Card><Status tone="error">{t('system.globalSettings.runtime.errors.generic')}</Status><p className="wk-muted text-muted">{error}</p></Card></section>;
  if (!payload || (!payload.available && !payload.model_limiter_available)) return <section className="wk-runtime-queues"><Card><Status>{t('system.globalSettings.runtime.unavailableTitle')}</Status><p className="wk-muted text-muted">{t('system.globalSettings.runtime.unavailable')}</p></Card></section>;
  const queues = payload.queues as Row[];
  const pools = payload.pools as Row[];
  const models = payload.models as Row[];
  async function openTasks(row: Row, state: string) {
    const queue = stringOf(row, 'name');
    if (!queue || numberOf(row, state) <= 0) return;
    const generation = ++taskRequestGeneration.current;
    setTaskDrawer({ queue, state, tasks: [], loading: true, error: null });
    try {
      const result = await client.administration.runtime.tasks.list(queue, state, { pageSize: 20 });
      if (generation !== taskRequestGeneration.current) return;
      setTaskDrawer({ queue, state, tasks: result.tasks as Row[], loading: false, error: result.available ? null : drawer.unavailable });
    } catch (reason) {
      if (generation !== taskRequestGeneration.current) return;
      setTaskDrawer({ queue, state, tasks: [], loading: false, error: reason instanceof Error ? reason.message : drawer.failed });
    }
  }
  const taskButton = (row: Row, state: string) => numberOf(row, state) > 0
    ? <button type="button" className="wk-rq-count-button cursor-pointer border-0 bg-transparent p-1.5 font-[inherit] text-[#0a8f4c] underline hover:text-[#067a3f] focus-visible:text-[#067a3f]" onClick={() => void openTasks(row, state)}>{numberOf(row, state)}</button>
    : <span>0</span>;
  const active = queues.reduce((sum, row) => sum + numberOf(row, 'active'), 0);
  const pending = queues.reduce((sum, row) => sum + numberOf(row, 'pending'), 0);
  const retry = queues.reduce((sum, row) => sum + numberOf(row, 'retry'), 0);
  const archived = queues.reduce((sum, row) => sum + numberOf(row, 'archived'), 0);
  const title = runtimeText(t, 'system.globalSettings.runtime.title', '运行时队列');
  return <section className="wk-runtime-queues grid gap-4 [&_h3]:m-0 [&_h3]:mb-2 [&_h3]:text-base" aria-label={title}>
    <header className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><h2>{title}</h2><p>{runtimeText(t, 'system.globalSettings.runtime.description', '查看任务队列、工作池和模型限流状态。')}</p></header>
    {payload.available ? <>
      <Card className="wk-rq-overview bg-[rgba(7,192,95,0.04)]"><h3>{runtimeText(t, 'system.globalSettings.runtime.summary.title', '队列概览')}</h3><div className="wk-rq-metrics grid grid-cols-4 gap-3 max-[720px]:grid-cols-2"><strong className="text-2xl text-[#118053]">{active}<span className="mt-[3px] block text-xs font-normal text-[#5c6b83]">{runtimeText(t, 'system.globalSettings.runtime.summary.active', '活跃任务')}</span></strong><strong className="text-2xl text-[#118053]">{pending}<span className="mt-[3px] block text-xs font-normal text-[#5c6b83]">{runtimeText(t, 'system.globalSettings.runtime.summary.pending', '等待任务')}</span></strong><strong className={`text-2xl ${retry > 0 ? 'text-[#b26a08]' : 'text-[#118053]'}`}>{retry}<span className="mt-[3px] block text-xs font-normal text-[#5c6b83]">{runtimeText(t, 'system.globalSettings.runtime.summary.retry', '重试')}</span></strong><strong className={`text-2xl ${archived > 0 ? 'text-[#c23434]' : 'text-[#118053]'}`}>{archived}<span className="mt-[3px] block text-xs font-normal text-[#5c6b83]">{runtimeText(t, 'system.globalSettings.runtime.summary.archived', '失败归档')}</span></strong></div></Card>
      <Card><h3>{t('system.globalSettings.runtime.poolsTitle')}</h3><p className="wk-muted text-muted">{t('system.globalSettings.runtime.poolsDescription')}</p><div className="wk-rq-pools grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2.5">{pools.map((row, index) => <div className="wk-rq-pool grid grid-cols-[1fr_auto] gap-1.5 rounded-lg border border-[rgba(120,135,155,0.24)] p-3" key={stringOf(row, 'name') || String(index)}><strong>{stringOf(row, 'name') || '—'}</strong><b>{numberOf(row, 'instances') > 0 ? `${numberOf(row, 'active')}/${numberOf(row, 'cluster_capacity')}` : numberOf(row, 'concurrency')}</b><span>{t('system.globalSettings.runtime.queueCount', { value: numberOf(row, 'queue_count') })}</span></div>)}</div></Card>
      <Card><h3>{t('system.globalSettings.runtime.detailsTitle')}</h3><p className="wk-muted text-muted">{t('system.globalSettings.runtime.detailsDescription')}</p>{queues.length === 0 ? <Status>{t('system.globalSettings.runtime.empty')}</Status> : <div className="wk-rq-table-wrap overflow-x-auto"><table className="wk-rq-table w-full min-w-[700px] border-collapse text-[13px] [&_th]:border-b [&_th]:border-[rgba(120,135,155,0.18)] [&_th]:px-2.5 [&_th]:py-3 [&_th]:text-left [&_th]:whitespace-nowrap [&_td]:border-b [&_td]:border-[rgba(120,135,155,0.18)] [&_td]:px-2.5 [&_td]:py-3 [&_td]:text-left [&_td]:whitespace-nowrap [&_thead_th]:text-xs [&_thead_th]:font-medium [&_thead_th]:text-[#5c6b83] [&_tbody_th]:font-semibold"><thead><tr>{['queue','active','pending','retry','archived','completed','latency','status'].map((key) => <th key={key}>{runtimeText(t, `system.globalSettings.runtime.columns.${key}`, key)}</th>)}</tr></thead><tbody>{queues.map((row, index) => <tr key={stringOf(row, 'name') || String(index)}><th>{stringOf(row, 'name') || '—'}</th><td>{taskButton(row, 'active')}</td><td>{taskButton(row, 'pending')}</td><td>{taskButton(row, 'retry')}</td><td>{taskButton(row, 'archived')}</td><td>{taskButton(row, 'completed')}</td><td>{numberOf(row, 'latency_ms') || '—'}</td><td>{queueStatus(row, t)}</td></tr>)}</tbody></table></div>}</Card>
    </> : null}
    <Card><h3>{t('system.globalSettings.runtime.models.title')}</h3><p className="wk-muted text-muted">{t('system.globalSettings.runtime.models.description')}</p>{!payload.model_limiter_available ? <Status>{t('system.globalSettings.runtime.models.disabled')}</Status> : models.length === 0 ? <Status>{t('system.globalSettings.runtime.models.empty')}</Status> : <div className="wk-rq-models grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2.5">{models.map((row, index) => { const current = numberOf(row, 'active'); const limit = numberOf(row, 'limit'); return <div className="wk-rq-model grid grid-cols-[1fr_auto] gap-1.5 rounded-lg border border-[rgba(120,135,155,0.24)] p-3" key={stringOf(row, 'model_id') || String(index)}><div><strong>{stringOf(row, 'name') || stringOf(row, 'model_id') || '—'}</strong><span>{current} / {limit}</span></div><progress max={100} value={percent(current, limit)} /><small>{numberOf(row, 'waiting')} {t('system.globalSettings.runtime.models.columns.waiting')}</small></div>; })}</div>}</Card>
    {taskDrawer ? <div className="wk-rq-task-drawer fixed bottom-0 right-0 top-0 z-[3200] w-[min(720px,100vw)] max-w-[720px] overflow-y-auto border-l border-[rgba(120,135,155,0.25)] bg-white p-6 shadow-[-8px_0_24px_rgba(23,32,51,0.14)]" role="dialog" aria-modal="true" aria-label={`${taskDrawer.queue} ${taskDrawer.state}`}><div className="wk-rq-task-drawer__head mb-5 flex items-start justify-between"><div><h3>{taskDrawer.queue}</h3><p className="wk-muted text-muted">{taskDrawer.state} · {drawer.detail}</p></div><button type="button" aria-label={drawer.close} className="cursor-pointer border-0 bg-transparent text-2xl leading-none" onClick={() => { taskRequestGeneration.current += 1; setTaskDrawer(null); }}>×</button></div>{taskDrawer.loading ? <Status>{drawer.loading}</Status> : taskDrawer.error ? <Status tone="error">{taskDrawer.error}</Status> : taskDrawer.tasks.length === 0 ? <Status>{drawer.empty}</Status> : <ul className="wk-list m-0 list-none p-0">{taskDrawer.tasks.map((task, index) => <li key={stringOf(task, 'id') || String(index)} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{stringOf(task, 'type') || '—'}</strong><span className="font-mono text-[0.8rem] text-muted">{stringOf(task, 'state') || taskDrawer.state}</span>{stringOf(task, 'last_error') ? <small>{stringOf(task, 'last_error')}</small> : null}</div></li>)}</ul>}</div> : null}
  </section>;
}
