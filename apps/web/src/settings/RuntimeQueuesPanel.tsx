import type { RuntimeQueues, WeKnoraClient } from '@weknora/api-client';
// S6：Status 无 TDesign 对应（playbook §1 附行），走 shared/wk-legacy。
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
// T12c：t-switch（size small）与 t-icon sprite glyph 对齐 Vue 端
// （RuntimeQueues.vue:12-16 / :26-33 / :77 / :138 / :145）。
import { Switch } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { useSettingsLocale } from './PortedSectionsPanel.tsx';
import { useEffect, useRef, useState } from 'react';

type Row = Record<string, unknown>;
const numberOf = (row: Row, key: string) => typeof row[key] === 'number' ? row[key] as number : 0;
const stringOf = (row: Row, key: string) => typeof row[key] === 'string' ? row[key] as string : '';

// zh-CN copy mirrors frontend/src/views/system/RuntimeQueues.vue (system.globalSettings.runtime.*).
const runtimeCopy: Record<string, string> = {
  'title': '任务队列运行时',
  'description': '后台任务队列的实时负载，以及各独立 worker 池的每实例并发配置。支持查看任务明细和安全管理，每 5 秒自动刷新。',
  'refresh': '刷新',
  'autoRefresh': '自动刷新（每 5 秒）',
  'loading': '加载中...',
  'retry': '重试',
  'unavailableTitle': '任务队列不可用',
  'unavailable': '当前部署未启用 Redis / asynq 队列（Lite 模式），无队列可展示。',
  'empty': '暂无队列数据',
  'detailsTitle': '队列明细',
  'detailsDescription': '各处理通道的实时负载与等待情况。“最终失败”表示任务超过重试上限，已停止自动执行。',
  'poolsTitle': 'Worker 池',
  'poolsDescription': '各阶段拥有保底容量，核心解析与内容富化还可借用共享弹性池。',
  'perInstance': '卡片主值为集群运行中/容量',
  'poolConfigured': '单实例配置 {value}',
  'poolInstances': '{value} 个实例',
  'poolUtilization': '利用率 {value}%',
  'queueCount': '{value} 个队列',
  'weightShort': '权重 {value}',
  'footnote': '卡片主值为集群运行中/实时容量；单实例配置修改后需重启。共享弹性池只消费核心解析和内容富化队列。',
  'updatedAt': '更新于 {value}',
  'models.title': '模型并发占用',
  'models.description': '观察后台任务实际进入模型服务时的并发占用；上方是任务调度，这里是模型服务限流，两者处于不同处理阶段。',
  'models.scope': '占用为集群全局 · 等待为当前实例',
  'models.disabled': '模型后台并发治理未启用。可在全局设置中配置模型默认并发上限。',
  'models.empty': '暂无模型调用数据；模型首次执行后台任务后会出现在这里。',
  'models.backgroundOnly': '仅统计后台任务，不包含交互式对话',
  'models.status.queued': '限流中',
  'models.status.full': '已满载',
  'models.columns.model': '模型 ID',
  'models.columns.active': '调用中',
  'models.columns.waiting': '限流等待',
  'models.columns.usage': '并发用量',
  'failedNotice.title': '{count} 个任务待处理',
  'failedNotice.description': '点击下方表中红色「最终失败」数字查看原因，修复后可手动重试。',
  'status.working': '处理中',
  'status.waiting': '等待中',
  'status.idle': '空闲',
  'status.actionRequired': '需处理',
  'status.retrying': '重试中',
  'status.paused': '已暂停',
  'columns.queue': '队列',
  'columns.active': '运行中',
  'columns.pending': '排队',
  'columns.scheduled': '定时',
  'columns.retry': '重试',
  'columns.archived': '最终失败',
  'columns.completed': '已完成',
  'columns.latency': '最早等待',
  'columns.status': '状态',
  'summary.title': '运行概览',
  'summary.active': '运行中',
  'summary.pending': '排队中',
  'summary.retry': '重试中',
  'summary.archived': '最终失败',
  'pools.core': '核心解析',
  'pools.postprocess': '后处理编排',
  'pools.enrichment': '内容富化',
  'pools.maintenance': '维护与同步',
  'pools.shared': '共享弹性',
  'pools.wiki': 'Wiki 池',
  'poolDescriptions.core': '文档解析与手工重解析的保底容量',
  'poolDescriptions.postprocess': '解析完成后的收尾与富化扇出',
  'poolDescriptions.enrichment': '摘要、图片、图谱与问题生成',
  'poolDescriptions.maintenance': '数据源同步、批处理与删除清理',
  'poolDescriptions.shared': '由核心解析与内容富化按积压借用',
  'poolDescriptions.wiki': 'Wiki 内容生成与全局收尾',
  'queueNames.default': '文档解析',
  'queueNames.chat_attachment': '对话附件解析',
  'queueNames.postprocess': '后处理编排',
  'queueNames.summary': '摘要生成',
  'queueNames.sync': '数据源同步',
  'queueNames.low': '维护与批处理',
  'queueNames.multimodal': '多模态处理',
  'queueNames.graph': '图谱抽取',
  'queueNames.question': '问题生成',
  'queueNames.wiki': 'Wiki 处理',
  'queueDescriptions.default': '文档解析、手工重解析',
  'queueDescriptions.chat_attachment': '会话内上传附件解析',
  'queueDescriptions.postprocess': '解析收尾、富化扇出',
  'queueDescriptions.summary': '文档摘要、表格摘要',
  'queueDescriptions.sync': '手动与定时同步',
  'queueDescriptions.low': 'FAQ 导入、批量重解析、删除清理',
  'queueDescriptions.multimodal': '图片 OCR、视觉描述',
  'queueDescriptions.graph': '分块图谱抽取',
  'queueDescriptions.question': '分块问题生成',
  'queueDescriptions.wiki': '内容生成、索引收尾',
  'errors.generic': '获取队列状态失败',
  'tasks.title': '任务明细 · {queue}',
  'tasks.description': '查看各状态任务及其安全管理动作。定时、重试任务按最近执行时间优先，其余状态按时间倒序。',
  'tasks.listTitle': '{state}任务',
  'tasks.unavailable': '当前部署不支持查看任务明细',
  'tasks.empty': '这个队列当前没有{state}任务',
  'tasks.loadError': '获取任务明细失败',
  'tasks.loadMore': '加载更多',
  'tasks.loadedSummary': '已加载 {count} 条，继续下滑或点击加载',
  'tasks.loadedAll': '已全部加载，共 {count} 条',
  'tasks.loadingMore': '正在加载更多…',
  'tasks.attempts': '执行 {current}/{max}',
  'tasks.unknownTarget': '未识别到关联对象',
  'tasks.knowledgeBaseLabel': '知识库 ID',
  'tasks.knowledgeLabel': '文档 ID',
  'tasks.taskIDLabel': '业务任务 ID',
  'tasks.tenantLabel': '空间 ID',
  'tasks.sourceLabel': '来源 ID',
  'tasks.targetLabel': '目标 ID',
  'tasks.sourceKBLabel': '来源知识库',
  'tasks.targetKBLabel': '目标知识库',
  'tasks.dataSourceLabel': '数据源 ID',
  'tasks.syncLogLabel': '同步记录 ID',
  'tasks.knowledgeCountLabel': '文档数量',
  'tasks.enqueuedAt': '入队时间',
  'tasks.startedAt': '开始时间',
  'tasks.nextProcessAt': '下次执行',
  'tasks.lastFailedAt': '最后失败',
  'tasks.completedAt': '完成时间',
  'tasks.deadline': '执行截止',
  'tasks.worker': '执行实例',
  'tasks.health': '运行健康',
  'tasks.orphaned': '执行实例已失联，等待恢复',
  'tasks.cancel': '终止任务',
  'tasks.runNow': '立即执行',
  'tasks.deleteRecord': '清除记录',
  'tasks.purgeArchived': '清除全部失败任务',
  'tasks.stateFilter': '按任务状态筛选',
  'tasks.states.active': '运行中',
  'tasks.states.pending': '排队中',
  'tasks.states.scheduled': '定时执行',
  'tasks.states.retry': '重试中',
  'tasks.states.archived': '最终失败',
  'tasks.states.completed': '已完成',
  'tasks.guides.active': '运行中任务可查看执行实例、开始时间和截止时间。只有具备完整业务取消语义的任务才允许终止。',
  'tasks.guides.pending': '排队任务尚未被 worker 领取。终止操作会同步更新业务状态，而不是只删除 Redis 记录。',
  'tasks.guides.scheduled': '定时任务可提前立即执行；支持业务取消的文档任务也可以安全终止。',
  'tasks.guides.retry': '请结合最后错误、重试次数和下次执行时间判断是否立即执行或终止。',
  'tasks.guides.archived': '请先修复失败原因再立即执行。清除记录不会完成原业务任务。',
  'tasks.guides.completed': '这里只展示设置了结果保留时间的近期完成任务，不提供管理动作。',
  'tasks.taskTypes.documentProcess': '文档解析',
  'tasks.taskTypes.manualProcess': '手工重新处理',
  'tasks.taskTypes.temporaryDocumentProcess': '聊天附件解析',
  'tasks.taskTypes.postProcess': '文档后处理',
  'tasks.taskTypes.summary': '摘要生成',
  'tasks.taskTypes.tableSummary': '表格摘要生成',
  'tasks.taskTypes.question': '问题生成',
  'tasks.taskTypes.multimodal': '图片多模态处理',
  'tasks.taskTypes.graph': '知识图谱抽取',
  'tasks.taskTypes.sync': '数据源同步',
  'tasks.taskTypes.faqImport': 'FAQ 导入',
  'tasks.taskTypes.batchReparse': '批量重新解析',
  'tasks.taskTypes.batchDelete': '批量删除',
  'tasks.taskTypes.move': '文档移动',
  'tasks.taskTypes.indexDelete': '索引删除',
  'tasks.taskTypes.kbClone': '知识库复制',
  'tasks.taskTypes.kbDelete': '知识库删除',
  'tasks.taskTypes.wikiIngest': 'Wiki 内容生成',
  'tasks.taskTypes.wikiFinalize': 'Wiki 收尾处理',
};
const TASK_STATES = ['active', 'pending', 'scheduled', 'retry', 'archived', 'completed'] as const;
type TaskState = (typeof TASK_STATES)[number];
const taskTypeKeys: Record<string, string> = {
  'document:process': 'documentProcess',
  'manual:process': 'manualProcess',
  'temporary_document:process': 'temporaryDocumentProcess',
  'knowledge:post_process': 'postProcess',
  'summary:generation': 'summary',
  'datatable:summary': 'tableSummary',
  'question:generation': 'question',
  'image:multimodal': 'multimodal',
  'chunk:extract': 'graph',
  'datasource:sync': 'sync',
  'faq:import': 'faqImport',
  'knowledge:list_reparse': 'batchReparse',
  'knowledge:list_delete': 'batchDelete',
  'knowledge:move': 'move',
  'index:delete': 'indexDelete',
  'kb:clone': 'kbClone',
  'kb:delete': 'kbDelete',
  'wiki:ingest': 'wikiIngest',
  'wiki:finalize': 'wikiFinalize',
};

export function RuntimeQueuesPanel({ client, payload, loading = false, error = null }: { client: WeKnoraClient; payload: RuntimeQueues | null; loading?: boolean; error?: string | null }) {
  const locale = useSettingsLocale();
  const t = (key: string, values?: Record<string, string | number>): string => {
    const suffix = key.startsWith('system.globalSettings.runtime.') ? key.slice('system.globalSettings.runtime.'.length) : key;
    const template = runtimeCopy[suffix] ?? key;
    return Object.entries(values ?? {}).reduce((acc, [name, replacement]) => acc.replace(new RegExp(`\\{${name}\\}`, 'g'), String(replacement)), template);
  };
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [live, setLive] = useState<RuntimeQueues | null>(payload);
  const [updatedAt, setUpdatedAt] = useState('');
  const liveRef = useRef(payload);
  const [taskDrawer, setTaskDrawer] = useState<{ queue: Row; state: TaskState; tasks: Row[]; loading: boolean; error: string | null } | null>(null);
  const requestGeneration = useRef(0);
  useEffect(() => { liveRef.current = payload; setLive(payload); }, [payload]);
  useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => {
      void client.administration.runtime.queues().then((next) => {
        if (next.available || next.model_limiter_available) { liveRef.current = next; setLive(next); }
        setUpdatedAt(new Date((next.timestamp || Date.now() / 1000) * 1000).toLocaleTimeString('zh-CN', { hour12: false }));
      }).catch(() => undefined);
    }, 5000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, client]);
  // Vue RuntimeQueues.vue load() stamps updatedAt on the FIRST fetch too
  // (L912: resp.timestamp || Date.now()/1000) — not only on poll ticks. The
  // prefetched payload stands in for that first load; without this the
  // 队列明细 header hides its 更新于 line for the first 5s.
  const viewForStamp = live ?? payload;
  useEffect(() => {
    if (!viewForStamp || updatedAt) return;
    const ts = typeof (viewForStamp as unknown as Row).timestamp === 'number' ? (viewForStamp as unknown as Row).timestamp as number : 0;
    setUpdatedAt(new Date((ts || Date.now() / 1000) * 1000).toLocaleTimeString('zh-CN', { hour12: false }));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [viewForStamp]);
  const view = live ?? payload;
  if (loading && !view) return <section className="runtime-queues rq-loading" aria-live="polite">{[1, 2, 3].map((n) => <div key={n} className="rq-skeleton" />)}</section>;
  if (error) return <section className="runtime-queues" role="alert"><div className="rq-state rq-state--error"><div className="rq-state-icon"><ErrorIcon /></div><div className="rq-state-copy"><strong>{t('system.globalSettings.runtime.errors.generic')}</strong><span>{error}</span></div></div></section>;
  if (!view || (!view.available && !view.model_limiter_available)) return <section className="runtime-queues"><div className="rq-state"><div className="rq-state-icon"><InfoIcon /></div><div className="rq-state-copy"><strong>{t('system.globalSettings.runtime.unavailableTitle')}</strong><span>{t('system.globalSettings.runtime.unavailable')}</span></div></div></section>;
  const queues = view.queues as Row[];
  const pools = view.pools as Row[];
  const models = view.models as Row[];
  const totalActive = queues.reduce((sum, row) => sum + numberOf(row, 'active'), 0);
  const totalPending = queues.reduce((sum, row) => sum + numberOf(row, 'pending'), 0);
  const totalRetry = queues.reduce((sum, row) => sum + numberOf(row, 'retry'), 0);
  const totalArchived = queues.reduce((sum, row) => sum + numberOf(row, 'archived'), 0);
  const poolLabel = (name: string) => t(`system.globalSettings.runtime.pools.${name}`) === `system.globalSettings.runtime.pools.${name}` ? name : t(`system.globalSettings.runtime.pools.${name}`);
  const poolDescription = (name: string) => t(`system.globalSettings.runtime.poolDescriptions.${name}`) === `system.globalSettings.runtime.poolDescriptions.${name}` ? name : t(`system.globalSettings.runtime.poolDescriptions.${name}`);
  const queueLabel = (name: string) => { const key = `system.globalSettings.runtime.queueNames.${name}`; return t(key) === key ? name : t(key); };
  const queueDescription = (name: string) => { const key = `system.globalSettings.runtime.queueDescriptions.${name}`; return t(key) === key ? name : t(key); };
  const poolQueueCount = (pool: string) => numberOf(pools.find((item) => item.name === pool) ?? {}, 'queue_count');
  const queueMeta = (row: Row) => {
    const scope = queueDescription(stringOf(row, 'name'));
    const pool = stringOf(row, 'pool');
    return poolQueueCount(pool) > 1 ? `${scope} · ${t('system.globalSettings.runtime.weightShort', { value: numberOf(row, 'weight') })}` : scope;
  };
  const formatLatency = (ms: number) => {
    if (!ms || ms <= 0) return '—';
    if (ms < 1000) return `${ms} ms`;
    const s = ms / 1000;
    if (s < 60) return `${s.toFixed(1)} s`;
    const m = Math.floor(s / 60);
    return `${m}m ${Math.round(s % 60)}s`;
  };
  const queueState = (row: Row): { label: string; tone: string } => {
    if (row.paused === true) return { label: t('system.globalSettings.runtime.status.paused'), tone: 'paused' };
    if (numberOf(row, 'archived') > 0) return { label: t('system.globalSettings.runtime.status.actionRequired'), tone: 'danger' };
    if (numberOf(row, 'retry') > 0) return { label: t('system.globalSettings.runtime.status.retrying'), tone: 'attention' };
    if (numberOf(row, 'active') > 0) return { label: t('system.globalSettings.runtime.status.working'), tone: 'working' };
    if (numberOf(row, 'pending') > 0 || numberOf(row, 'scheduled') > 0) return { label: t('system.globalSettings.runtime.status.waiting'), tone: 'waiting' };
    return { label: t('system.globalSettings.runtime.status.idle'), tone: 'idle' };
  };
  const modelState = (row: Row): { label: string; tone: string } => {
    if (numberOf(row, 'waiting') > 0) return { label: t('system.globalSettings.runtime.models.status.queued'), tone: 'attention' };
    if (numberOf(row, 'active') >= numberOf(row, 'limit')) return { label: t('system.globalSettings.runtime.models.status.full'), tone: 'waiting' };
    if (numberOf(row, 'active') > 0) return { label: t('system.globalSettings.runtime.status.working'), tone: 'working' };
    return { label: t('system.globalSettings.runtime.status.idle'), tone: 'idle' };
  };
  const modelUsage = (row: Row) => numberOf(row, 'limit') > 0 ? Math.min(100, Math.round(numberOf(row, 'active') / numberOf(row, 'limit') * 100)) : 0;
  const taskButton = (row: Row, state: TaskState, extraClass: string) => numberOf(row, state) > 0
    ? <button type="button" className={`rq-task-count ${extraClass}`} onClick={() => void openTasks(row, state)}>{numberOf(row, state)}<ChevronIcon /></button>
    : <span className="rq-number">0</span>;
  async function openTasks(row: Row, state: TaskState) {
    const queue = stringOf(row, 'name');
    if (!queue) return;
    const generation = ++requestGeneration.current;
    setTaskDrawer({ queue: row, state, tasks: [], loading: true, error: null });
    try {
      const result = await client.administration.runtime.tasks.list(queue, state, { pageSize: 20 });
      if (generation !== requestGeneration.current) return;
      setTaskDrawer({ queue: row, state, tasks: (result.tasks ?? []) as Row[], loading: false, error: result.available ? null : t('system.globalSettings.runtime.tasks.unavailable') });
    } catch (reason) {
      if (generation !== requestGeneration.current) return;
      setTaskDrawer({ queue: row, state, tasks: [], loading: false, error: reason instanceof Error ? reason.message : t('system.globalSettings.runtime.tasks.loadError') });
    }
  }
  const taskQueueName = taskDrawer ? stringOf(taskDrawer.queue, 'name') : '';
  const taskStateCount = (state: TaskState) => taskDrawer ? numberOf(taskDrawer.queue, state) : 0;
  const runtimeTaskMeta = (task: Row): Array<{ key: string; label: string; value: string }> => {
    const refs: Array<{ key: string; label: string; value: string }> = [];
    const push = (key: string, label: string, value: unknown) => { if (value !== undefined && value !== null && value !== '' && value !== 0) refs.push({ key, label, value: String(value) }); };
    push('kb', t('system.globalSettings.runtime.tasks.knowledgeBaseLabel'), stringOf(task, 'knowledge_base_id'));
    push('knowledge', t('system.globalSettings.runtime.tasks.knowledgeLabel'), stringOf(task, 'knowledge_id'));
    push('task', t('system.globalSettings.runtime.tasks.taskIDLabel'), stringOf(task, 'task_id'));
    push('source', t('system.globalSettings.runtime.tasks.sourceLabel'), stringOf(task, 'source_id'));
    push('target', t('system.globalSettings.runtime.tasks.targetLabel'), stringOf(task, 'target_id'));
    push('source-kb', t('system.globalSettings.runtime.tasks.sourceKBLabel'), stringOf(task, 'source_kb_id'));
    push('target-kb', t('system.globalSettings.runtime.tasks.targetKBLabel'), stringOf(task, 'target_kb_id'));
    push('datasource', t('system.globalSettings.runtime.tasks.dataSourceLabel'), stringOf(task, 'data_source_id'));
    push('sync-log', t('system.globalSettings.runtime.tasks.syncLogLabel'), stringOf(task, 'sync_log_id'));
    push('knowledge-count', t('system.globalSettings.runtime.tasks.knowledgeCountLabel'), numberOf(task, 'knowledge_count'));
    push('tenant', t('system.globalSettings.runtime.tasks.tenantLabel'), stringOf(task, 'tenant_id'));
    const time = (value: string) => { const date = new Date(value); return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN', { hour12: false }); };
    if (stringOf(task, 'enqueued_at')) refs.push({ key: 'enqueued', label: t('system.globalSettings.runtime.tasks.enqueuedAt'), value: time(stringOf(task, 'enqueued_at')) });
    if (stringOf(task, 'started_at')) refs.push({ key: 'started', label: t('system.globalSettings.runtime.tasks.startedAt'), value: time(stringOf(task, 'started_at')) });
    if (stringOf(task, 'next_process_at')) refs.push({ key: 'next', label: t('system.globalSettings.runtime.tasks.nextProcessAt'), value: time(stringOf(task, 'next_process_at')) });
    if (stringOf(task, 'last_failed_at')) refs.push({ key: 'failed', label: t('system.globalSettings.runtime.tasks.lastFailedAt'), value: time(stringOf(task, 'last_failed_at')) });
    if (stringOf(task, 'completed_at')) refs.push({ key: 'completed', label: t('system.globalSettings.runtime.tasks.completedAt'), value: time(stringOf(task, 'completed_at')) });
    if (stringOf(task, 'deadline')) refs.push({ key: 'deadline', label: t('system.globalSettings.runtime.tasks.deadline'), value: time(stringOf(task, 'deadline')) });
    push('worker', t('system.globalSettings.runtime.tasks.worker'), stringOf(task, 'worker'));
    if (task.is_orphaned) refs.push({ key: 'orphaned', label: t('system.globalSettings.runtime.tasks.health'), value: t('system.globalSettings.runtime.tasks.orphaned') });
    return refs;
  };
  const taskTypeLabel = (type: string) => { const key = taskTypeKeys[type]; return key ? t(`system.globalSettings.runtime.tasks.taskTypes.${key}`) : type; };
  return <section className="runtime-queues" aria-label={t('system.globalSettings.runtime.title')}>
    <header className="rq-header">
      <div className="rq-title-block">
        <h2>{t('system.globalSettings.runtime.title')}</h2>
        <p className="section-description">{t('system.globalSettings.runtime.description')}</p>
      </div>
      <div className="rq-actions">
        <label className="rq-auto-refresh">
          <span className={autoRefresh ? 'rq-live-dot rq-live-dot--active' : 'rq-live-dot'} />
          <span>{t('system.globalSettings.runtime.autoRefresh')}</span>
          <Switch size="small" value={autoRefresh} onChange={(value) => setAutoRefresh(Boolean(value))} aria-label={t('system.globalSettings.runtime.autoRefresh')} />
        </label>
        <button type="button" className="rq-refresh" aria-label={t('system.globalSettings.runtime.refresh')} title={t('system.globalSettings.runtime.refresh')} onClick={() => {
          void client.administration.runtime.queues().then((next) => { liveRef.current = next; setLive(next); setUpdatedAt(new Date((next.timestamp || Date.now() / 1000) * 1000).toLocaleTimeString('zh-CN', { hour12: false })); }).catch(() => undefined);
        }}><TIcon name="refresh" /></button>
      </div>
    </header>
    {view.available ? <>
      <section className="rq-overview" aria-label={t('system.globalSettings.runtime.summary.title')}>
        <div className="rq-overview-title">
          <span className="rq-overview-mark"><TIcon name="chart-line" /></span>
          <span>{t('system.globalSettings.runtime.summary.title')}</span>
        </div>
        <div className="rq-overview-metrics">
          <div className="rq-metric rq-metric--active"><span className="rq-metric-label">{t('system.globalSettings.runtime.summary.active')}</span><strong className="rq-metric-value">{totalActive}</strong></div>
          <div className="rq-metric"><span className="rq-metric-label">{t('system.globalSettings.runtime.summary.pending')}</span><strong className="rq-metric-value">{totalPending}</strong></div>
          <div className={totalRetry > 0 ? 'rq-metric rq-metric--warning' : 'rq-metric'}><span className="rq-metric-label">{t('system.globalSettings.runtime.summary.retry')}</span><strong className="rq-metric-value">{totalRetry}</strong></div>
          <div className={totalArchived > 0 ? 'rq-metric rq-metric--danger' : 'rq-metric'}><span className="rq-metric-label">{t('system.globalSettings.runtime.summary.archived')}</span><strong className="rq-metric-value">{totalArchived}</strong></div>
        </div>
      </section>
      <section className="rq-pools">
        <div className="rq-pools-header">
          <div>
            <h3 className="rq-section-title">{t('system.globalSettings.runtime.poolsTitle')}</h3>
            <p>{t('system.globalSettings.runtime.poolsDescription')}</p>
          </div>
          <span className="rq-pools-note">{t('system.globalSettings.runtime.perInstance')}</span>
        </div>
        <div className="rq-pool-grid">
          {pools.map((pool, index) => <div className="rq-pool-card" key={stringOf(pool, 'name') || String(index)}>
            <div className="rq-pool-topline">
              <span className="rq-pool-name">{poolLabel(stringOf(pool, 'name'))}</span>
              <strong className="rq-pool-value">{numberOf(pool, 'instances') > 0 ? `${numberOf(pool, 'active')}/${numberOf(pool, 'cluster_capacity')}` : numberOf(pool, 'concurrency')}</strong>
            </div>
            <p className="rq-pool-desc">
              {poolDescription(stringOf(pool, 'name'))}
              {/* Vue 模板文本节点结构复刻：poolConfigured 插值后的换行凝结为
                  尾随空格文本节点（"单实例配置 8 "），与下一 " · 1 个实例…" 文本
                  节点拼出双空格分隔（"8␣␣·␣1"）；三个文本节点边界与 Vue 一致
                  （台账 #11 同族：单表达式拼接防跨节点 kerning 漂移）。 */}
              <span className="rq-pool-meta">
                {t('system.globalSettings.runtime.poolConfigured', { value: numberOf(pool, 'concurrency') }) + ' '}
                {numberOf(pool, 'instances') > 0 ? ` · ${t('system.globalSettings.runtime.poolInstances', { value: numberOf(pool, 'instances') })} · ${t('system.globalSettings.runtime.poolUtilization', { value: Math.round(Math.max(0, Math.min(1, typeof pool.utilization === 'number' ? pool.utilization : 0)) * 100) })}` : null}
                {` · ${t('system.globalSettings.runtime.queueCount', { value: numberOf(pool, 'queue_count') })}`}
              </span>
            </p>
          </div>)}
        </div>
      </section>
      <section className="rq-details">
        <div className="rq-details-header">
          <div>
            <h3 className="rq-section-title">{t('system.globalSettings.runtime.detailsTitle')}</h3>
            <p>{t('system.globalSettings.runtime.detailsDescription')}</p>
          </div>
          {updatedAt ? <span className="rq-updated-at"><TIcon name="time" />{t('system.globalSettings.runtime.updatedAt', { value: updatedAt })}</span> : null}
        </div>
        {totalArchived > 0 ? <div className="rq-failed-notice" role="status">
          <span className="rq-failed-notice__icon" aria-hidden="true"><TIcon name="error-circle" /></span>
          <div className="rq-failed-notice__text">
            <p className="rq-failed-notice__title">{t('system.globalSettings.runtime.failedNotice.title', { count: totalArchived })}</p>
            <p className="rq-failed-notice__desc">{t('system.globalSettings.runtime.failedNotice.description')}</p>
          </div>
        </div> : null}
        {queues.length === 0 ? <div className="rq-empty"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M4 6h16M4 10h16M4 14h10" /></svg><span>{t('system.globalSettings.runtime.empty')}</span></div> : <div className="rq-table-shell">
          <table className="rq-table">
            <thead><tr>{['queue', 'active', 'pending', 'retry', 'archived', 'completed', 'latency_ms', 'status'].map((key) => <th key={key} className={['active', 'pending', 'retry', 'archived', 'completed', 'latency_ms'].includes(key) ? 'rq-align-center' : undefined}>{t(`system.globalSettings.runtime.columns.${key === 'latency_ms' ? 'latency' : key}`)}</th>)}</tr></thead>
            <tbody>{queues.map((row, index) => <tr key={stringOf(row, 'name') || String(index)}>
              <td className="rq-queue-cell-td"><div className="rq-queue-cell"><span className="rq-queue-name">{queueLabel(stringOf(row, 'name'))}</span><span className="rq-queue-meta">{queueMeta(row)}</span></div></td>
              <td className="rq-align-center">{taskButton(row, 'active', 'rq-task-count--active')}</td>
              <td className="rq-align-center"><div className="rq-backlog">{taskButton(row, 'pending', '')}{numberOf(row, 'scheduled') > 0 ? <button type="button" className="rq-scheduled-count" onClick={() => void openTasks(row, 'scheduled')}>+{numberOf(row, 'scheduled')} {t('system.globalSettings.runtime.columns.scheduled')}</button> : null}</div></td>
              <td className="rq-align-center">{taskButton(row, 'retry', 'rq-task-count--warning')}</td>
              <td className="rq-align-center">{taskButton(row, 'archived', 'rq-failed-count')}</td>
              <td className="rq-align-center">{taskButton(row, 'completed', 'rq-task-count--completed')}</td>
              <td className="rq-align-center"><span className="rq-latency">{formatLatency(numberOf(row, 'latency_ms'))}</span></td>
              <td><span className={`rq-status rq-status--${queueState(row).tone}`}><i />{queueState(row).label}</span></td>
            </tr>)}</tbody>
          </table>
        </div>}
      </section>
    </> : null}
    <section className="rq-details rq-models">
      <div className="rq-details-header">
        <div>
          <h3 className="rq-section-title">{t('system.globalSettings.runtime.models.title')}</h3>
          <p>{t('system.globalSettings.runtime.models.description')}</p>
        </div>
        <span className="rq-pools-note">{t('system.globalSettings.runtime.models.scope')}</span>
      </div>
      {!view.model_limiter_available ? <div className="rq-empty"><InfoIcon size={28} /><span>{t('system.globalSettings.runtime.models.disabled')}</span></div>
        : models.length === 0 ? <div className="rq-empty"><ServerIcon /><span>{t('system.globalSettings.runtime.models.empty')}</span></div>
          : <div className="rq-table-shell">
            <table className="rq-table">
              <thead><tr><th>{t('system.globalSettings.runtime.models.columns.model')}</th><th className="rq-align-center">{t('system.globalSettings.runtime.models.columns.active')}</th><th className="rq-align-center">{t('system.globalSettings.runtime.models.columns.waiting')}</th><th>{t('system.globalSettings.runtime.models.columns.usage')}</th><th>{t('system.globalSettings.runtime.columns.status')}</th></tr></thead>
              <tbody>{models.map((row, index) => <tr key={stringOf(row, 'model_id') || String(index)}>
                <td><div className="rq-queue-cell"><span className="rq-queue-name">{stringOf(row, 'name') || stringOf(row, 'model_id')}</span><span className="rq-queue-meta">{stringOf(row, 'name') ? stringOf(row, 'model_id') : t('system.globalSettings.runtime.models.backgroundOnly')}</span></div></td>
                <td className="rq-align-center"><span className={numberOf(row, 'active') > 0 ? 'rq-number rq-number--active' : 'rq-number'}>{numberOf(row, 'active')}</span></td>
                <td className="rq-align-center"><span className={numberOf(row, 'waiting') > 0 ? 'rq-number rq-number--warning' : 'rq-number'}>{numberOf(row, 'waiting')}</span></td>
                <td><div className="rq-model-usage"><progress max={100} value={modelUsage(row)} /><span>{numberOf(row, 'active')} / {numberOf(row, 'limit')}</span></div></td>
                <td><span className={`rq-status rq-status--${modelState(row).tone}`}><i />{modelState(row).label}</span></td>
              </tr>)}</tbody>
            </table>
          </div>}
    </section>
    <p className="rq-footnote">{t('system.globalSettings.runtime.footnote')}</p>
    {taskDrawer ? <div className="rq-failed-drawer" role="dialog" aria-modal="true" aria-label={t('system.globalSettings.runtime.tasks.title', { queue: queueLabel(taskQueueName) })}>
      <div className="rq-drawer-backdrop" onClick={() => { requestGeneration.current += 1; setTaskDrawer(null); }} />
      <div className="rq-drawer-panel">
        <header className="rq-drawer-head">
          <div>
            <h3>{t('system.globalSettings.runtime.tasks.title', { queue: queueLabel(taskQueueName) })}</h3>
            <p>{t('system.globalSettings.runtime.tasks.description')}</p>
          </div>
          <button type="button" aria-label="关闭" onClick={() => { requestGeneration.current += 1; setTaskDrawer(null); }}>×</button>
        </header>
        <div className="rq-task-state-filter" role="tablist" aria-label={t('system.globalSettings.runtime.tasks.stateFilter')}>
          {TASK_STATES.map((state) => <button key={state} type="button" role="tab" className={taskDrawer.state === state ? 'rq-task-state-option is-active' : 'rq-task-state-option'} aria-selected={taskDrawer.state === state} onClick={() => { if (taskDrawer.state !== state) void openTasks(taskDrawer.queue, state); }}>
            <span className="rq-task-state-option__label">{t(`system.globalSettings.runtime.tasks.states.${state}`)}</span>
            <span className={taskStateCount(state) > 0 ? 'rq-task-state-option__count has-value' : 'rq-task-state-option__count'}>{taskStateCount(state)}</span>
          </button>)}
        </div>
        <p className="rq-failed-guide-desc">{t(`system.globalSettings.runtime.tasks.guides.${taskDrawer.state}`)}</p>
        <div className="rq-failed-section-head">
          <h4>{t('system.globalSettings.runtime.tasks.listTitle', { state: t(`system.globalSettings.runtime.tasks.states.${taskDrawer.state}`) })}</h4>
        </div>
        {taskDrawer.loading ? <Status>{t('system.globalSettings.runtime.loading')}</Status>
          : taskDrawer.error ? <Status tone="error">{taskDrawer.error}</Status>
            : taskDrawer.tasks.length === 0 ? <Status>{t('system.globalSettings.runtime.tasks.empty', { state: t(`system.globalSettings.runtime.tasks.states.${taskDrawer.state}`) })}</Status>
              : <div className="rq-failed-list-panel">
                {taskDrawer.tasks.map((task, index) => <article key={stringOf(task, 'id') || String(index)} className="rq-failed-row">
                  <div className="rq-failed-row-content">
                    <div className="rq-failed-row-summary">
                      <span className="rq-failed-row-type">{taskTypeLabel(stringOf(task, 'type'))}</span>
                      <span className="rq-failed-row-sep" aria-hidden="true">·</span>
                      <span className={`rq-task-state-pill rq-task-state-pill--${stringOf(task, 'state') || taskDrawer.state}`}>{t(`system.globalSettings.runtime.tasks.states.${stringOf(task, 'state') || taskDrawer.state}`)}</span>
                      <span className="rq-failed-row-sep" aria-hidden="true">·</span>
                      <span className="rq-failed-row-stat">{t('system.globalSettings.runtime.tasks.attempts', { current: numberOf(task, 'retried') + 1, max: numberOf(task, 'max_retry') + 1 })}</span>
                    </div>
                    {runtimeTaskMeta(task).length > 0 ? <dl className="rq-failed-row-refs">{runtimeTaskMeta(task).map((ref) => <div key={ref.key} className="rq-failed-ref"><dt>{ref.label}</dt><dd title={ref.value}>{ref.value}</dd></div>)}</dl>
                      : <p className="rq-failed-row-unknown">{t('system.globalSettings.runtime.tasks.unknownTarget')}</p>}
                    {stringOf(task, 'last_error') ? <p className="rq-failed-row-error">{stringOf(task, 'last_error')}</p> : null}
                  </div>
                </article>)}
              </div>}
      </div>
    </div> : null}
  </section>;
}

function ChevronIcon() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="m9 18 6-6-6-6" /></svg>;
}
function ClockIcon() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>;
}
function InfoIcon({ size = 24 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="9" /><path d="M12 8h.01M12 11v5" /></svg>;
}
function ErrorIcon({ size = 24 }: { size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="9" /><path d="M12 7v6M12 16h.01" /></svg>;
}
function ServerIcon() {
  return <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><rect x="3" y="4" width="18" height="7" rx="1" /><rect x="3" y="13" width="18" height="7" rx="1" /><path d="M7 7.5h.01M7 16.5h.01" /></svg>;
}
