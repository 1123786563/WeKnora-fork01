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

// Vue OllamaSettings formatSize: tiered fixed-2 units.
function formatSize(bytes: number | undefined): string {
  const value = Number(bytes);
  if (!value || value === 0 || Number.isNaN(value)) return '0 B';
  if (value < 1024) return value + ' B';
  if (value < 1024 * 1024) return (value / 1024).toFixed(2) + ' KB';
  if (value < 1024 * 1024 * 1024) return (value / (1024 * 1024)).toFixed(2) + ' MB';
  return (value / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
}

// Vue OllamaSettings formatDate: 今天/昨天/N 天前, then the locale date.
function formatDate(dateStr: string | undefined, t: (key: string, values?: Record<string, string | number>) => string): string {
  if (!dateStr) return t('ollama.unknown');
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return t('ollama.unknown');
  const days = Math.floor((Date.now() - date.getTime()) / (1000 * 60 * 60 * 24));
  if (days === 0) return t('ollama.today');
  if (days === 1) return t('ollama.yesterday');
  if (days < 7) return t('ollama.daysAgo', { days });
  return date.toLocaleDateString();
}

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

  useEffect(() => { const next = payload(initialValue); setStatus(next.status ?? { available: false }); setModels(next.models ?? []); }, [initialValue]);

  async function refresh() {
    setBusy(true); setTesting(true); setStatus(null); setError(null); setNotice(null);
    try { const [nextStatus, nextModels] = await Promise.all([client.settings.ollama.status(), client.settings.ollama.models()]); setStatus(nextStatus); setModels(nextModels); }
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
    {/* Vue OllamaSettings.vue: the panel owns its section-header (h2 +
        description, no divider) and the rows sit directly on the panel
        background via the shared .settings-group/.setting-row geometry. */}
    <div className="section-header">
      <h2>{t('ollamaSettings.title')}</h2>
      <p className="section-description">{t('ollamaSettings.description')}</p>
    </div>
    <div className="settings-group">
      <div className="setting-row">
        <div className="setting-info"><label>{t('ollamaSettings.status.label')}</label><p className="desc">{t('ollamaSettings.status.desc')}</p></div>
        <div className="setting-control">
          <div className="status-display flex items-center gap-[12px]">
          {testing ? <Status tone="neutral">{t('ollamaSettings.status.testing')}</Status>
            : status?.available ? (
              // Vue t-tag theme=success variant=light: 24px chip, check-circle icon.
              <span className="inline-flex items-center gap-[6px] rounded-[3px] bg-[#e3f9e9] px-[10px] py-[4px] text-[12px] leading-[16px] text-[#0a8f4c]">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false"><path d="M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10zm-1.1-7.4l5.3-5.3-1.4-1.4-3.9 3.9-1.9-1.9-1.4 1.4 3.3 3.3z" /></svg>
                {t('ollamaSettings.status.available')}
              </span>
            ) : status ? (
              // Vue t-tag theme=danger variant=light with close-circle-filled:
              // 24px chip (#fdecee), filled red x-circle icon.
              <span className="inline-flex items-center gap-[6px] rounded-[3px] bg-[#fdecee] px-[10px] py-[4px] text-[12px] leading-[16px] text-[#e34d59]">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false"><path d="M12 22C6.477 22 2 17.523 2 12S6.477 2 12 2s10 4.477 10 10-4.477 10-10 10zm-1.1-7.4 5.3-5.3-1.4-1.4-3.9 3.9-1.9-1.9-1.4 1.4 3.3 3.3z" /></svg>
                {t('ollamaSettings.status.unavailable')}
              </span>
            ) : <Status tone="neutral">{t('ollamaSettings.status.untested')}</Status>}
          <button type="button" className="flex cursor-pointer items-center gap-[8px] border-0 bg-transparent p-0 text-[14px] text-[rgba(0,0,0,0.9)] [font:inherit] hover:text-[#07c05f] disabled:cursor-not-allowed disabled:opacity-60" disabled={busy || testing} onClick={() => void refresh()}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M21 12a9 9 0 11-3-6.7L21 8" /><path d="M21 3v5h-5" /></svg>
            {t('ollamaSettings.status.retest')}
          </button>
          </div>
        </div>
      </div>
      <div className="setting-row">
        <div className="setting-info"><label>{t('ollamaSettings.address.label')}</label><p className="desc">{t('ollamaSettings.address.desc')}</p></div>
        {/* Vue renders the detected address in a disabled input box inside the
            360px control column; the failed-check warning alert stacks under
            it (margin-top 8px), both right-aligned in the same column. */}
        <div className="setting-control setting-control--stacked">
          {/* Vue t-input disabled: 14px left-aligned value, disabled gray. */}
          <Input readOnly disabled value={status?.baseUrl ?? ''} placeholder="—" className="w-full bg-[#eeeeee] text-[14px] text-[rgba(0,0,0,0.26)]" aria-label={t('ollamaSettings.address.label')} />
          {/* Vue t-alert theme=warning (measured): #f9e0c7 wash, 20px filled
              orange icon, 24px horizontal / 16px vertical padding, 14px/22px
              near-black message; the alert fills the same 360px column as the
              disabled input above it. */}
          {status && !status.available ? <p role="alert" className="m-0 mt-0 box-border flex w-full items-start gap-[8px] rounded-[6px] bg-[rgb(249,224,199)] px-[24px] py-[16px] text-[14px] leading-[22px] text-[rgba(0,0,0,0.9)]"><svg className="shrink-0" width="20" height="20" viewBox="0 0 24 24" fill="#ed7b2f" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="10" /><path d="M12 6.6a1.3 1.3 0 0 1 1.3 1.3v5a1.3 1.3 0 1 1-2.6 0v-5A1.3 1.3 0 0 1 12 6.6z" fill="#fff" /><circle cx="12" cy="16.6" r="1.35" fill="#fff" /></svg><span>{t('ollamaSettings.address.failed')}</span></p> : null}
        </div>
      </div>
    </div>
    {status?.available && !testing ? <>
      <Card>
        <h3>{t('ollamaSettings.download.title')}</h3>
        <p className="wk-muted text-muted">{t('ollamaSettings.download.descPrefix')} <a href="https://ollama.com/search" target="_blank" rel="noopener noreferrer">{t('ollamaSettings.download.browse')}</a></p>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Input aria-label={t('ollamaSettings.download.placeholder')} className="w-full min-w-0" value={modelName} placeholder={t('ollamaSettings.download.placeholder')} onChange={(event) => setModelName(event.target.value)} /><Button type="button" className="shrink-0 whitespace-nowrap" disabled={busy || !modelName.trim()} onClick={() => void download()}><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d="M12 3v12" /><path d="M7 10l5 5 5-5" /><path d="M4 21h16" /></svg>{t('ollamaSettings.download.download')}</Button>{activeTask ? <Button type="button" disabled={busy} onClick={() => void checkProgress()}>{t('common.refresh')}</Button> : null}</div>
        {progress ? <dl className="wk-settings-values mb-0 mt-4 grid gap-[.65rem]"><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{copy.task}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{activeTask || copy.accepted}</dd></div><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{copy.progress}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{String(progress.progress ?? progress.status ?? copy.reported)}</dd></div></dl> : null}
      </Card>
      <Card>
        <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><h3>{t('ollamaSettings.installed.title')}</h3><p className="wk-muted text-muted m-0">{t('ollamaSettings.installed.desc')}</p></div><Button type="button" disabled={busy} onClick={() => void refresh()}>{t('common.refresh')}</Button></div>
        {models.length === 0 ? <Status>{t('ollamaSettings.installed.empty')}</Status> : <ul className="wk-list m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3 p-0">{models.map((model) => <li key={model.name} className="rounded-[10px] border border-[#e4e7ec] bg-white px-4 py-3"><div className="min-w-0"><strong className="block truncate text-[14px] text-[#101828]" title={model.name}>{model.name}</strong>{model.size ? <span className="mt-1 block text-[12px] text-muted">{formatSize(Number(model.size))}</span> : null}{model.modified_at ? <span className="mt-[2px] block text-[12px] text-muted">{formatDate(model.modified_at, t)}</span> : null}</div></li>)}</ul>}
      </Card>
    </> : null}
  </div>;
}
