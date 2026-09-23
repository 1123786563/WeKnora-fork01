import { useState } from 'react';
import type { ConfigurationRecord, WeKnoraClient } from '@weknora/api-client';
import { Button, Checkbox, Input, Select, Textarea } from 'tdesign-react';
import { Card, Status } from './ui.tsx';
import { configurationPayload, type ConfigurationDraft, type ConfigurationSectionKey } from './surface.ts';
import { configurationDraftFromRecord, credentialInput, credentialStatusAfterClear, newConfigurationDraft, savedConfigurationId } from './editor.ts';
import './config-u.css';

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
    // S5 tdesign 迁移：Input 不再把 required 透传到内层 input，原生表单校验
    // 改为等价 JS 守卫（空必填项直接提示，不打 API）。
    if (!draft.name.trim() || (section === 'models' && (!draft.type?.trim() || !draft.source?.trim()))) {
      setError('Fill in the required fields.'); setBusy(false); return;
    }
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
  /* T15：原 w-full box-border 旧栈 utility 语义化。 */
  const fieldClass = 'wk-cfg-field';
  return <Card className="wk-configuration-editor"><div className="wk-settings-panel-heading wk-cfg-ed-1"><div><p className="wk-eyebrow wk-cfg-ed-2">{record ? 'Edit' : 'Create'} {section}</p><h3>{record ? draft.name : `New ${section}`}</h3><p className="wk-muted wk-cfg-ed-3">Secrets are write-only and use dedicated credential endpoints.</p></div><Button type="button" theme="default" variant="outline" disabled={busy} onClick={onCancel}>Close</Button></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor wk-cfg-ed-4" onSubmit={(event) => void save(event)}><label>Name<Input className={fieldClass} value={draft.name} onChange={(value) => setField('name', String(value))} /></label><label>Description<Textarea className={fieldClass} autosize={{ minRows: 2, maxRows: 2 }} value={draft.description ?? ''} onChange={(value) => setField('description', String(value))} /></label>{section === 'agents' ? <label>Avatar<Input className={fieldClass} value={draft.avatar ?? ''} placeholder="https://…" onChange={(value) => setField('avatar', String(value))} /></label> : null}{section === 'models' ? <><label>Type<Input className={fieldClass} value={draft.type ?? ''} onChange={(value) => setField('type', String(value))} /></label><label>Source<Input className={fieldClass} value={draft.source ?? ''} onChange={(value) => setField('source', String(value))} /></label></> : null}{section === 'mcp' ? <><label>URL<Input className={fieldClass} type="url" value={draft.url ?? ''} onChange={(value) => setField('url', String(value))} /></label><label>Transport<Select className={fieldClass} value={draft.transportType ?? 'sse'} onChange={(value) => setField('transportType', String(value))}><Select.Option value="sse" label="SSE">SSE</Select.Option><Select.Option value="http-streamable" label="HTTP streamable">HTTP streamable</Select.Option><Select.Option value="stdio" label="stdio">stdio</Select.Option></Select></label><Checkbox className="wk-cfg-ed-5" checked={draft.enabled !== false} onChange={(checked) => setField('enabled', checked)} label="Enabled" /></> : null}{section === 'agents' ? <><label>System prompt<Textarea className={fieldClass} autosize={{ minRows: 6, maxRows: 6 }} value={draft.systemPrompt ?? ''} onChange={(value) => setField('systemPrompt', String(value))} /></label><Checkbox className="wk-cfg-ed-5" checked={draft.memoryEnabled === true} onChange={(checked) => setField('memoryEnabled', checked)} label="Memory enabled" /></> : null}{section !== 'agents' ? <label>Safe configuration JSON<Textarea className={fieldClass} autosize={{ minRows: 6, maxRows: 6 }} value={draft.details} onChange={(value) => setField('details', String(value))} /></label> : null}{section === 'models' || section === 'mcp' ? <div className="wk-credential-fields"><label>API key<Input className={fieldClass} type="password" autocomplete="new-password" value={draft.apiKey ?? ''} placeholder={configured('api_key') ? 'Configured; leave blank to keep' : 'Enter a new key'} onChange={(value) => setField('apiKey', String(value))} /></label>{section === 'models' ? <label>App secret<Input className={fieldClass} type="password" autocomplete="new-password" value={draft.appSecret ?? ''} placeholder={configured('app_secret') ? 'Configured; leave blank to keep' : 'Enter a new secret'} onChange={(value) => setField('appSecret', String(value))} /></label> : <label>Token<Input className={fieldClass} type="password" autocomplete="new-password" value={draft.token ?? ''} placeholder={configured('token') ? 'Configured; leave blank to keep' : 'Enter a token'} onChange={(value) => setField('token', String(value))} /></label>}<div className="wk-list-actions wk-cfg-ed-6">{draft.id && configured('api_key') ? <Button type="button" theme="default" variant="outline" disabled={busy} onClick={() => void clearCredential('api_key')}>Clear API key</Button> : null}{section === 'models' && draft.id && configured('app_secret') ? <Button type="button" theme="default" variant="outline" disabled={busy} onClick={() => void clearCredential('app_secret')}>Clear app secret</Button> : null}{section === 'mcp' && draft.id && configured('token') ? <Button type="button" theme="default" variant="outline" disabled={busy} onClick={() => void clearCredential('token')}>Clear token</Button> : null}</div></div> : null}<div className="wk-list-actions wk-cfg-ed-6"><Button type="submit" theme="default" variant="outline" loading={busy}>{record ? 'Save changes' : 'Create configuration'}</Button><Button type="button" theme="default" variant="outline" disabled={busy} onClick={onCancel}>Cancel</Button>{section === 'mcp' && draft.id ? <><Button type="button" theme="default" variant="outline" disabled={busy} onClick={() => void testMcp()}>Test connection</Button><Button type="button" theme="default" variant="outline" disabled={busy} onClick={() => void loadTools()}>Load tools</Button><Button type="button" theme="default" variant="outline" disabled={busy} onClick={() => void authorizeMcp()}>Authorize OAuth</Button></> : null}</div></form>{section === 'mcp' && mcpTools.length > 0 ? <div><h4>Server tools</h4><ul className="wk-list wk-cfg-ed-7">{mcpTools.map((tool) => <li key={tool.name} className="wk-cfg-ed-8"><div className="wk-list-item-copy wk-cfg-ed-9"><strong>{tool.name}</strong><small>{tool.description ?? 'No description returned.'}</small></div></li>)}</ul></div> : null}</Card>;
}
