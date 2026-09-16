import { useState } from 'react';
import type { ConfigurationRecord, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Checkbox, Input, Select, Status, Textarea } from '@weknora/ui';
import { configurationPayload, type ConfigurationDraft, type ConfigurationSectionKey } from './surface.ts';
import { configurationDraftFromRecord, credentialInput, credentialStatusAfterClear, newConfigurationDraft, savedConfigurationId } from './editor.ts';

type EditableSection = Exclude<ConfigurationSectionKey, 'skills'>;

function errorMessage(value: unknown): string { return value instanceof Error ? value.message : 'Configuration operation failed'; }

export function ConfigurationEditor({ client, section, record, onSaved, onCancel }: {
  client: WeKnoraClient; section: EditableSection; record?: ConfigurationRecord; onSaved: () => void; onCancel: () => void;
}) {
  const [draft, setDraft] = useState<ConfigurationDraft>(() => record ? configurationDraftFromRecord(section, record) : newConfigurationDraft(section));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [mcpTools, setMcpTools] = useState<Array<{ name: string; description?: string }>>([]);

  function setField<K extends keyof ConfigurationDraft>(key: K, value: ConfigurationDraft[K]) { setDraft((current) => ({ ...current, [key]: value })); }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const payload = configurationPayload(section, draft);
      const api = client.configuration[section];
      const saved = draft.id ? await api.update(draft.id, payload) : await api.create(payload);
      const credentials = credentialInput(section, draft);
      const savedId = savedConfigurationId(draft.id, saved.id);
      if (section === 'models' && Object.keys(credentials).length > 0) await client.configuration.models.credentials.put(savedId, credentials);
      if (section === 'mcp' && Object.keys(credentials).length > 0) await client.configuration.mcp.credentials.put(savedId, credentials);
      onSaved();
    } catch (cause) { setError(errorMessage(cause)); }
    finally { setBusy(false); }
  }

  async function clearCredential(field: 'api_key' | 'app_secret' | 'token') {
    if (!draft.id) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      if (section === 'models' && (field === 'api_key' || field === 'app_secret')) await client.configuration.models.credentials.remove(draft.id, field);
      if (section === 'mcp' && (field === 'api_key' || field === 'token')) await client.configuration.mcp.credentials.remove(draft.id, field);
      setDraft((current) => ({
        ...current,
        ...(field === 'api_key' ? { apiKey: '' } : field === 'app_secret' ? { appSecret: '' } : { token: '' }),
        credentialStatus: credentialStatusAfterClear(current.credentialStatus, field),
      }));
      setNotice(`${field} cleared on the server.`);
    } catch (cause) { setError(errorMessage(cause)); }
    finally { setBusy(false); }
  }

  async function testMcp() {
    if (!draft.id) return; setBusy(true); setError(null); setNotice(null);
    try { const result = await client.configuration.mcp.test(draft.id); setNotice(result.success ? 'MCP connection succeeded.' : `MCP connection failed: ${result.message ?? 'unknown error'}`); }
    catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  async function loadTools() {
    if (!draft.id) return; setBusy(true); setError(null);
    try { setMcpTools(await client.configuration.mcp.tools(draft.id)); }
    catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  async function authorizeMcp() {
    if (!draft.id) return; setBusy(true); setError(null);
    try {
      const result = await client.configuration.mcp.oauth.authorizeUrl(draft.id, { redirectURI: `${window.location.origin}/api/v1/mcp-oauth/callback`, frontendRedirect: window.location.href });
      window.location.assign(result.authorizationUrl);
    } catch (cause) { setError(errorMessage(cause)); setBusy(false); }
  }

  const configured = (field: string) => (draft.credentialStatus?.[field] as { configured?: unknown } | undefined)?.configured === true;
  const fieldClass = 'w-full box-border';
  return <Card className="wk-configuration-editor"><div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{record ? 'Edit' : 'Create'} {section}</p><h3>{record ? draft.name : `New ${section}`}</h3><p className="wk-muted text-muted m-0">Secrets are write-only and use dedicated credential endpoints.</p></div><Button type="button" disabled={busy} onClick={onCancel}>Close</Button></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:font-semibold" onSubmit={(event) => void save(event)}><label>Name<Input className={fieldClass} required value={draft.name} onChange={(event) => setField('name', event.target.value)} /></label><label>Description<Textarea className={fieldClass} rows={2} value={draft.description ?? ''} onChange={(event) => setField('description', event.target.value)} /></label>{section === 'agents' ? <label>Avatar<Input className={fieldClass} value={draft.avatar ?? ''} placeholder="https://…" onChange={(event) => setField('avatar', event.target.value)} /></label> : null}{section === 'models' ? <><label>Type<Input className={fieldClass} required value={draft.type ?? ''} onChange={(event) => setField('type', event.target.value)} /></label><label>Source<Input className={fieldClass} required value={draft.source ?? ''} onChange={(event) => setField('source', event.target.value)} /></label></> : null}{section === 'mcp' ? <><label>URL<Input className={fieldClass} type="url" value={draft.url ?? ''} onChange={(event) => setField('url', event.target.value)} /></label><label>Transport<Select className={fieldClass} value={draft.transportType ?? 'sse'} onChange={(event) => setField('transportType', event.target.value)}><option value="sse">SSE</option><option value="http-streamable">HTTP streamable</option><option value="stdio">stdio</option></Select></label><label className="flex! grid-cols-[auto_1fr] items-center gap-2"><Checkbox checked={draft.enabled !== false} onChange={(event) => setField('enabled', event.target.checked)} /> Enabled</label></> : null}{section === 'agents' ? <><label>System prompt<Textarea className={fieldClass} rows={6} value={draft.systemPrompt ?? ''} onChange={(event) => setField('systemPrompt', event.target.value)} /></label><label className="flex! grid-cols-[auto_1fr] items-center gap-2"><Checkbox checked={draft.memoryEnabled === true} onChange={(event) => setField('memoryEnabled', event.target.checked)} /> Memory enabled</label></> : null}{section !== 'agents' ? <label>Safe configuration JSON<Textarea className={fieldClass} rows={6} value={draft.details} onChange={(event) => setField('details', event.target.value)} /></label> : null}{section === 'models' || section === 'mcp' ? <div className="wk-credential-fields"><label>API key<Input className={fieldClass} type="password" autoComplete="new-password" value={draft.apiKey ?? ''} placeholder={configured('api_key') ? 'Configured; leave blank to keep' : 'Enter a new key'} onChange={(event) => setField('apiKey', event.target.value)} /></label>{section === 'models' ? <label>App secret<Input className={fieldClass} type="password" autoComplete="new-password" value={draft.appSecret ?? ''} placeholder={configured('app_secret') ? 'Configured; leave blank to keep' : 'Enter a new secret'} onChange={(event) => setField('appSecret', event.target.value)} /></label> : <label>Token<Input className={fieldClass} type="password" autoComplete="new-password" value={draft.token ?? ''} placeholder={configured('token') ? 'Configured; leave blank to keep' : 'Enter a token'} onChange={(event) => setField('token', event.target.value)} /></label>}<div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">{draft.id && configured('api_key') ? <Button type="button" disabled={busy} onClick={() => void clearCredential('api_key')}>Clear API key</Button> : null}{section === 'models' && draft.id && configured('app_secret') ? <Button type="button" disabled={busy} onClick={() => void clearCredential('app_secret')}>Clear app secret</Button> : null}{section === 'mcp' && draft.id && configured('token') ? <Button type="button" disabled={busy} onClick={() => void clearCredential('token')}>Clear token</Button> : null}</div></div> : null}<div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="submit" loading={busy}>{record ? 'Save changes' : 'Create configuration'}</Button><Button type="button" disabled={busy} onClick={onCancel}>Cancel</Button>{section === 'mcp' && draft.id ? <><Button type="button" disabled={busy} onClick={() => void testMcp()}>Test connection</Button><Button type="button" disabled={busy} onClick={() => void loadTools()}>Load tools</Button><Button type="button" disabled={busy} onClick={() => void authorizeMcp()}>Authorize OAuth</Button></> : null}</div></form>{section === 'mcp' && mcpTools.length > 0 ? <div><h4>Server tools</h4><ul className="wk-list m-0 list-none p-0">{mcpTools.map((tool) => <li key={tool.name} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{tool.name}</strong><small>{tool.description ?? 'No description returned.'}</small></div></li>)}</ul></div> : null}</Card>;
}
