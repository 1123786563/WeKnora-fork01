import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Status } from '@weknora/ui';
import type { Locale } from '@weknora/i18n';
import { cloudCredentialPatch } from './surface.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

function row(value: unknown): Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }

const CLOUD_STATUS_COPY: Record<Locale, { models: string; available: string; unavailable: string; status: string; serverReported: string; loadFailed: string }> = {
  'zh-CN': { models: '模型', available: '可用', unavailable: '不可用', status: '状态', serverReported: '服务端已返回', loadFailed: '云服务状态加载失败' },
  'en-US': { models: 'Models', available: 'Available', unavailable: 'Unavailable', status: 'Status', serverReported: 'Server reported', loadFailed: 'Failed to load cloud status' },
  'ja-JP': { models: 'モデル', available: '利用可能', unavailable: '利用不可', status: '状態', serverReported: 'サーバー報告', loadFailed: 'クラウド状態の読み込みに失敗しました' },
  'ko-KR': { models: '모델', available: '사용 가능', unavailable: '사용 불가', status: '상태', serverReported: '서버 보고', loadFailed: '클라우드 상태를 불러오지 못했습니다' },
  'ru-RU': { models: 'Модели', available: 'Доступно', unavailable: 'Недоступно', status: 'Статус', serverReported: 'Сообщено сервером', loadFailed: 'Не удалось загрузить состояние облака' },
};

export function CloudSettingsPanel({ client, initialValue }: { client: WeKnoraClient; initialValue: unknown }) {
  const t = settingsT(readInitialLocale());
  const locale = readInitialLocale();
  const statusCopy = CLOUD_STATUS_COPY[locale];
  const [status, setStatus] = useState(() => row(initialValue));
  const [appId, setAppId] = useState('');
  const [appSecret, setAppSecret] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [formExpanded, setFormExpanded] = useState(() => !(status.has_models === true && status.needs_reinit !== true));

  useEffect(() => {
    const next = row(initialValue);
    setStatus(next);
    setFormExpanded(!(next.has_models === true && next.needs_reinit !== true));
  }, [initialValue]);

  async function reload() {
    setBusy(true); setError(null); setNotice(null);
    try { setStatus(await client.settings.weknoraCloud.status()); }
    catch (reason) { setError(reason instanceof Error ? reason.message : statusCopy.loadFailed); }
    finally { setBusy(false); }
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try { await client.settings.weknoraCloud.saveCredentials(cloudCredentialPatch(appId, appSecret)); setAppId(''); setAppSecret(''); setNotice(t('settings.weknoraCloud.saveSuccess')); await reload(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('settings.weknoraCloud.saveFailed')); setBusy(false); }
  }

  const needsReinit = status.needs_reinit === true;
  const configured = status.has_models === true && !needsReinit;
  return <div className="wk-settings-cloud"><Card><div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><h3>{t('settings.weknoraCloud.title')}</h3><p className="wk-muted text-muted m-0">{t('settings.weknoraCloud.description')}</p></div><Button type="button" disabled={busy} onClick={() => void reload()}>{t('common.refresh')}</Button></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="flex items-center justify-between gap-3"><Status tone={configured ? 'success' : needsReinit ? 'warning' : 'neutral'}>{configured ? t('settings.weknoraCloud.configured') : needsReinit ? t('settings.weknoraCloud.credentialExpired') + (typeof status.reason === 'string' ? ': ' + status.reason : '') : t('settings.weknoraCloud.unconfigured')}</Status>{configured && !formExpanded ? <Button type="button" disabled={busy} onClick={() => setFormExpanded(true)}>{t('settings.weknoraCloud.reconfigure')}</Button> : null}</div><dl className="wk-settings-values mb-0 mt-4 grid gap-[.65rem]"><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{statusCopy.models}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{status.has_models === true ? statusCopy.available : statusCopy.unavailable}</dd></div><div className="grid grid-cols-[minmax(8rem,14rem)_minmax(0,1fr)] gap-[.8rem] border-b border-line-soft py-[.55rem] max-[720px]:grid-cols-1 max-[720px]:gap-1"><dt className="text-muted-strong font-[650] [overflow-wrap:anywhere]">{statusCopy.status}</dt><dd className="m-0 font-mono text-[.85rem] [overflow-wrap:anywhere] whitespace-pre-wrap">{typeof status.status === 'string' ? status.status : statusCopy.serverReported}</dd></div></dl></Card>{formExpanded ? <Card><h3>{t('settings.weknoraCloud.usageTitle')}</h3><form className="wk-settings-editor my-4 grid max-w-[620px] gap-[.8rem] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold" onSubmit={(event) => void save(event)}><label>{t('settings.weknoraCloud.appIdLabel')}<Input required autoComplete="off" value={appId} onChange={(event) => setAppId(event.target.value)} /></label><label>{t('settings.weknoraCloud.appSecretLabel')}<Input required type="password" autoComplete="new-password" value={appSecret} onChange={(event) => setAppSecret(event.target.value)} /></label><p className="wk-muted text-muted">{t('settings.weknoraCloud.saveHint')}</p><Button type="submit" loading={busy}>{t('settings.weknoraCloud.saveBtn')}</Button></form></Card> : null}</div>;
}
