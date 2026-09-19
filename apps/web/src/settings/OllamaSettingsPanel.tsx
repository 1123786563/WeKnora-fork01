import { useEffect, useState } from 'react';
import type { OllamaModel, OllamaStatus, SettingsPayload, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Input, Status } from '@weknora/ui';
import { ollamaModelInput } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

type OllamaPayload = { status?: OllamaStatus; models?: OllamaModel[] };
function object(value: unknown): Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function payload(value: unknown): OllamaPayload { const row = object(value); return { status: object(row.status) as OllamaStatus, models: Array.isArray(row.models) ? row.models.filter((item): item is OllamaModel => item !== null && typeof item === 'object' && typeof (item as Record<string, unknown>).name === 'string') : [] }; }
function taskId(value: SettingsPayload): string { const id = value.task_id ?? value.taskId ?? value.id; return typeof id === 'string' || typeof id === 'number' ? String(id) : ''; }

const OLLAMA_EXTRA_COPY: Record<Locale, { progressUpdated: string; task: string; accepted: string; progress: string; reported: string; sizeUnavailable: string; bytes: string }> = {
  'zh-CN': { progressUpdated: '进度已刷新', task: '任务', accepted: '已接受', progress: '进度', reported: '已返回', sizeUnavailable: '大小未知', bytes: '字节' },
  'en-US': { progressUpdated: 'Progress refreshed', task: 'Task', accepted: 'Accepted', progress: 'Progress', reported: 'Reported', sizeUnavailable: 'Size unavailable', bytes: 'bytes' },
  'ja-JP': { progressUpdated: '進捗を更新しました', task: 'タスク', accepted: '受付済み', progress: '進捗', reported: '取得済み', sizeUnavailable: 'サイズ不明', bytes: 'バイト' },
  'ko-KR': { progressUpdated: '진행률을 새로 고쳤습니다', task: '작업', accepted: '접수됨', progress: '진행률', reported: '보고됨', sizeUnavailable: '크기 없음', bytes: '바이트' },
  'ru-RU': { progressUpdated: 'Прогресс обновлён', task: 'Задача', accepted: 'Принято', progress: 'Прогресс', reported: 'Получено', sizeUnavailable: 'Размер неизвестен', bytes: 'байт' },
};

export function OllamaSettingsPanel({ client, initialValue }: { client: WeKnoraClient; initialValue: unknown }) {
  const initial = payload(initialValue);
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const copy = OLLAMA_EXTRA_COPY[locale];
  const [status, setStatus] = useState<OllamaStatus | null>(initial.status ?? null);
  const [models, setModels] = useState<OllamaModel[]>(initial.models ?? []);
  const [modelName, setModelName] = useState('');
  const [activeTask, setActiveTask] = useState('');
  const [progress, setProgress] = useState<SettingsPayload | null>(null);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [loadFailed, setLoadFailed] = useState(initialValue == null);

  useEffect(() => { const next = payload(initialValue); setStatus(next.status ?? { available: false }); setModels(next.models ?? []); setLoadFailed(initialValue == null); }, [initialValue]);

  async function refresh() {
    setBusy(true); setTesting(true); setStatus(null); setError(null); setNotice(null);
    try { const [nextStatus, nextModels] = await Promise.all([client.settings.ollama.status(), client.settings.ollama.models()]); setStatus(nextStatus); setModels(nextModels); setLoadFailed(false); }
    catch (reason) { setStatus({ available: false }); setError(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.connectFailed')); }
    finally { setTesting(false); setBusy(false); }
  }

  async function download() {
    if (!modelName.trim()) return;
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.settings.ollama.download(ollamaModelInput(modelName)); const id = taskId(result); setActiveTask(id); setProgress(result); setModelName(''); setNotice(t('ollamaSettings.toasts.downloadStarted', { name: id || '' })); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.downloadFailed')); }
    finally { setBusy(false); }
  }

  async function checkProgress() {
    if (!activeTask) return;
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.settings.ollama.progress(activeTask); setProgress(result); setNotice(copy.progressUpdated); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.progressFailed')); }
    finally { setBusy(false); }
  }

  return <div className="wk-settings-ollama">
    <Card>
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
        <div><h3>{t('ollamaSettings.title')}</h3><p className="wk-muted text-muted m-0">{t('ollamaSettings.description')}</p></div>
        <Button type="button" disabled={busy} onClick={() => void refresh()}>{t('ollamaSettings.status.retest')}</Button>
      </div>
      <Status tone={testing ? 'neutral' : status?.available ? 'success' : 'warning'}>{testing ? t('ollamaSettings.status.testing') : status?.available ? t('ollamaSettings.status.available') + (status.version ? ' · ' + status.version : '') : status ? t('ollamaSettings.status.unavailable') + (status.error ? ': ' + status.error : '') : t('ollamaSettings.status.untested')}</Status>
      <dl className="wk-settings-values mb-0 mt-4 grid gap-[.65rem]"><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{t('ollamaSettings.address.label')}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{status?.baseUrl || '—'}</dd></div></dl>
      {!testing && (loadFailed || status?.available === false) ? <div className="mt-4"><Status tone="warning">{t('ollamaSettings.address.failed')}</Status></div> : null}
    </Card>
    {status?.available && !testing ? <>
      <Card>
        <h3>{t('ollamaSettings.download.title')}</h3>
        <p className="wk-muted text-muted">{t('ollamaSettings.download.descPrefix')} <a href="https://ollama.com/search" target="_blank" rel="noopener noreferrer">{t('ollamaSettings.download.browse')}</a></p>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Input aria-label={t('ollamaSettings.download.placeholder')} className="w-full min-w-0" value={modelName} placeholder={t('ollamaSettings.download.placeholder')} onChange={(event) => setModelName(event.target.value)} /><Button type="button" disabled={busy || !modelName.trim()} onClick={() => void download()}>{t('ollamaSettings.download.download')}</Button>{activeTask ? <Button type="button" disabled={busy} onClick={() => void checkProgress()}>{t('common.refresh')}</Button> : null}</div>
        {progress ? <dl className="wk-settings-values mb-0 mt-4 grid gap-[.65rem]"><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{copy.task}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{activeTask || copy.accepted}</dd></div><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{copy.progress}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{String(progress.progress ?? progress.status ?? copy.reported)}</dd></div></dl> : null}
      </Card>
      <Card>
        <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><h3>{t('ollamaSettings.installed.title')}</h3><p className="wk-muted text-muted m-0">{t('ollamaSettings.installed.desc')}</p></div><Button type="button" disabled={busy} onClick={() => void refresh()}>{t('common.refresh')}</Button></div>
        {models.length === 0 ? <Status>{t('ollamaSettings.installed.empty')}</Status> : <ul className="wk-list m-0 list-none p-0">{models.map((model) => <li key={model.name} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{model.name}</strong><span className="font-mono text-[0.8rem] text-muted">{model.size ? `${model.size} ${copy.bytes}` : copy.sizeUnavailable}{model.modified_at ? ` · ${model.modified_at}` : ''}</span></div></li>)}</ul>}
      </Card>
    </> : null}
  </div>;
}
