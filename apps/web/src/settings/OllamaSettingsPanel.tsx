import { useEffect, useState } from 'react';
import type { OllamaModel, OllamaStatus, SettingsPayload, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { ollamaModelInput } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

type OllamaPayload = { status?: OllamaStatus; models?: OllamaModel[] };
function object(value: unknown): Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function payload(value: unknown): OllamaPayload { const row = object(value); return { status: object(row.status) as OllamaStatus, models: Array.isArray(row.models) ? row.models.filter((item): item is OllamaModel => item !== null && typeof item === 'object' && typeof (item as Record<string, unknown>).name === 'string') : [] }; }
function taskId(value: SettingsPayload): string { const id = value.task_id ?? value.taskId ?? value.id; return typeof id === 'string' || typeof id === 'number' ? String(id) : ''; }

export function OllamaSettingsPanel({ client, initialValue }: { client: WeKnoraClient; initialValue: unknown }) {
  const initial = payload(initialValue);
  const t = settingsT(readInitialLocale());
  const [status, setStatus] = useState<OllamaStatus>(initial.status ?? { available: false });
  const [models, setModels] = useState<OllamaModel[]>(initial.models ?? []);
  const [modelName, setModelName] = useState('');
  const [activeTask, setActiveTask] = useState('');
  const [progress, setProgress] = useState<SettingsPayload | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { const next = payload(initialValue); setStatus(next.status ?? { available: false }); setModels(next.models ?? []); }, [initialValue]);

  async function refresh() {
    setBusy(true); setError(null); setNotice(null);
    try { const [nextStatus, nextModels] = await Promise.all([client.settings.ollama.status(), client.settings.ollama.models()]); setStatus(nextStatus); setModels(nextModels); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.connectFailed')); }
    finally { setBusy(false); }
  }

  async function download() {
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.settings.ollama.download(ollamaModelInput(modelName)); const id = taskId(result); setActiveTask(id); setProgress(result); setModelName(''); setNotice(t('ollamaSettings.toasts.downloadStarted', { name: id || '' })); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.downloadFailed')); }
    finally { setBusy(false); }
  }

  async function checkProgress() {
    if (!activeTask) return;
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.settings.ollama.progress(activeTask); setProgress(result); setNotice(t('ollamaSettings.toasts.progressFailed')); {/* TODO(migration): progress-refreshed success toast has no key */} }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('ollamaSettings.toasts.progressFailed')); }
    finally { setBusy(false); }
  }

  return <div className="wk-settings-ollama"><Card><div className="wk-settings-panel-heading"><div><h3>{t('ollamaSettings.title')}</h3><p className="wk-muted">The service address is deployment-owned. React reports availability from the server and never infers health from a model list.</p></div><Button type="button" disabled={busy} onClick={() => void refresh()}>{t('ollamaSettings.status.retest')}</Button></div><Status tone={status.available ? 'success' : 'warning'}>{status.available ? t('ollamaSettings.status.available') + (status.version ? ' · ' + status.version : '') : t('ollamaSettings.status.unavailable') + (status.error ? ': ' + status.error : '')}</Status><dl className="wk-settings-values"><div><dt>baseUrl</dt><dd>{status.baseUrl || '—'}</dd></div><div><dt>models</dt><dd>{models.length}</dd></div></dl></Card><Card><h3>{t('ollamaSettings.download.title')}</h3><p className="wk-muted">Downloads are initiated by the authenticated server. A task identifier is retained only for progress lookup.</p><div className="wk-list-actions"><input aria-label={t('ollamaSettings.download.placeholder')} value={modelName} placeholder={t('ollamaSettings.download.placeholder')} onChange={(event) => setModelName(event.target.value)} /><Button type="button" disabled={busy || !status.available} onClick={() => void download()}>{t('ollamaSettings.download.download')}</Button>{activeTask ? <Button type="button" disabled={busy} onClick={() => void checkProgress()}>Refresh progress{/* TODO(migration): no settings.* key */}</Button> : null}</div>{progress ? <dl className="wk-settings-values"><div><dt>task</dt><dd>{activeTask || 'accepted'}</dd></div><div><dt>progress</dt><dd>{String(progress.progress ?? progress.status ?? 'reported')}</dd></div></dl> : null}</Card><Card><div className="wk-settings-panel-heading"><div><h3>{t('ollamaSettings.installed.title')}</h3><p className="wk-muted">Only the server-returned model metadata is displayed.</p></div><Button type="button" disabled={busy} onClick={() => void refresh()}>{t('common.refresh')}</Button></div>{models.length === 0 ? <Status>{t('ollamaSettings.installed.empty')}</Status> : <ul className="wk-list">{models.map((model) => <li key={model.name}><div className="wk-list-item-copy"><strong>{model.name}</strong><span>{model.size ? `${model.size} bytes` : 'size unavailable'}{model.modified_at ? ` · ${model.modified_at}` : ''}</span></div></li>)}</ul>}</Card></div>;
}
