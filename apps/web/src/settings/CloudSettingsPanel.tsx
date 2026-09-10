import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { cloudCredentialPatch } from './surface.ts';

function row(value: unknown): Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}; }

export function CloudSettingsPanel({ client, initialValue }: { client: WeKnoraClient; initialValue: unknown }) {
  const [status, setStatus] = useState(() => row(initialValue));
  const [appId, setAppId] = useState('');
  const [appSecret, setAppSecret] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setStatus(row(initialValue)); }, [initialValue]);

  async function reload() {
    setBusy(true); setError(null); setNotice(null);
    try { setStatus(await client.settings.weknoraCloud.status()); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to query WeKnora Cloud status.'); }
    finally { setBusy(false); }
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try { await client.settings.weknoraCloud.saveCredentials(cloudCredentialPatch(appId, appSecret)); setAppId(''); setAppSecret(''); setNotice('WeKnora Cloud credentials saved.'); await reload(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to save WeKnora Cloud credentials; the current form values were kept.'); setBusy(false); }
  }

  const needsReinit = status.needs_reinit === true;
  const configured = status.has_models === true && !needsReinit;
  return <div className="wk-settings-cloud"><Card><div className="wk-settings-panel-heading"><div><h3>WeKnora Cloud connection</h3><p className="wk-muted">Credentials are submitted only to the server. They are never read back or stored in the React form.</p></div><Button type="button" disabled={busy} onClick={() => void reload()}>Refresh status</Button></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<Status tone={configured ? 'success' : needsReinit ? 'warning' : 'neutral'}>{configured ? 'Configured' : needsReinit ? `Reinitialization required${typeof status.reason === 'string' ? `: ${status.reason}` : ''}` : 'Not configured'}</Status><dl className="wk-settings-values"><div><dt>models</dt><dd>{status.has_models === true ? 'available' : 'not available'}</dd></div><div><dt>status</dt><dd>{typeof status.status === 'string' ? status.status : 'server reported'}</dd></div></dl></Card><Card><h3>Configure credentials</h3><form className="wk-settings-editor" onSubmit={(event) => void save(event)}><label>App ID<input required autoComplete="off" value={appId} onChange={(event) => setAppId(event.target.value)} /></label><label>App secret<input required type="password" autoComplete="new-password" value={appSecret} onChange={(event) => setAppSecret(event.target.value)} /></label><Button type="submit" loading={busy}>Save credentials</Button></form></Card></div>;
}
