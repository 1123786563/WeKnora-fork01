import { useEffect, useState } from 'react';
import { INTEGRATION_SECTIONS, type IntegrationKey } from './registry.ts';
import { buildEmbedUpdatePayload } from './form.ts';

export interface APIPrincipalConfig { mode: 'tenant' | 'direct_header' | 'signed_token'; direct_header_name: string; signed_token_header_name: string; require_direct_header: boolean; has_hmac_secret: boolean }

export interface IntegrationResource {
  id: string;
  name?: string;
  platform?: string;
  agent_id?: string;
  enabled?: boolean;
  [key: string]: unknown;
}

export interface IntegrationPrincipalToken { token: string; headerName: string; expiresInSeconds: number; externalUserId: string }
export interface IntegrationActions {
  onCreateEmbed?: (input: Record<string, unknown>) => Promise<void>;
  onUpdateEmbed?: (id: string, input: Record<string, unknown>) => Promise<void>;
  onDeleteEmbed?: (id: string) => Promise<void>;
  onRotateEmbed?: (id: string) => Promise<void>;
  onCreateIm?: (input: { agentId: string; platform: string; name: string; credentials: Record<string, unknown> }) => Promise<void>;
  onUpdateIm?: (id: string, input: Record<string, unknown>) => Promise<void>;
  onToggleIm?: (id: string) => Promise<void>;
  onDeleteIm?: (id: string) => Promise<void>;
  principal?: APIPrincipalConfig | null;
  onSavePrincipal?: (input: { mode: APIPrincipalConfig['mode']; requireDirectHeader: boolean; hmacSecret?: string }) => Promise<void>;
  onCreatePrincipalTestToken?: (externalUserId: string) => Promise<IntegrationPrincipalToken>;
}

export interface IntegrationsPageProps {
  embedChannels: readonly IntegrationResource[];
  imChannels: readonly IntegrationResource[];
  apiBaseUrl: string;
  initialTab?: IntegrationKey;
  loading?: boolean;
  error?: string;
  onReload?: () => void;
  onOpenEmbed?: (channel: IntegrationResource) => void;
  actions?: IntegrationActions;
}

function labelFor(key: IntegrationKey): string {
  return ({ im: 'IM channels', embed: 'Embed channels', api: 'API access', cli: 'CLI', chrome: 'Browser extension', claw: 'Claw skill' })[key];
}

export function IntegrationsPage({ embedChannels, imChannels, apiBaseUrl, initialTab = 'embed', loading = false, error, onReload, onOpenEmbed, actions = {} }: IntegrationsPageProps) {
  const [tab, setTab] = useState<IntegrationKey>(initialTab);
  const [localError, setLocalError] = useState('');
  const [embedAgentId, setEmbedAgentId] = useState('');
  const [embedName, setEmbedName] = useState('');
  const [embedOrigins, setEmbedOrigins] = useState('https://example.com');
  const [imAgentId, setImAgentId] = useState('');
  const [imPlatform, setImPlatform] = useState('feishu');
  const [imName, setImName] = useState('');
  const [imCredentials, setImCredentials] = useState('{}');
  const [editedNames, setEditedNames] = useState<Record<string, string>>({});
  const [principalMode, setPrincipalMode] = useState<APIPrincipalConfig['mode']>(actions.principal?.mode ?? 'tenant');
  const [requireDirectHeader, setRequireDirectHeader] = useState(actions.principal?.require_direct_header ?? false);
  const [hmacSecret, setHmacSecret] = useState('');
  const [externalUserId, setExternalUserId] = useState('playground-user');
  const [principalToken, setPrincipalToken] = useState<IntegrationPrincipalToken | null>(null);
  const [apiKey, setApiKey] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [playgroundPath, setPlaygroundPath] = useState('/api/v1/knowledge-chat/');
  const [playgroundBody, setPlaygroundBody] = useState('{\n  "query": "Hello from the API playground"\n}');
  const [playgroundOutput, setPlaygroundOutput] = useState('');
  const [busy, setBusy] = useState(false);
  const section = INTEGRATION_SECTIONS.find((item) => item.key === tab)!;
  useEffect(() => { if (!actions.principal) return; setPrincipalMode(actions.principal.mode); setRequireDirectHeader(actions.principal.require_direct_header); }, [actions.principal]);
  const run = async (operation: () => Promise<void>) => { setBusy(true); setLocalError(''); try { await operation(); } catch (cause) { setLocalError(cause instanceof Error ? cause.message : 'Integration operation failed.'); } finally { setBusy(false); } };
  const createEmbed = () => run(async () => { if (!actions.onCreateEmbed) return; await actions.onCreateEmbed({ agent_id: embedAgentId.trim(), name: embedName.trim(), allowed_origins: embedOrigins.split(/[\n,]/).map((value) => value.trim()).filter(Boolean), enabled: true }); setEmbedName(''); onReload?.(); });
  const createIm = () => run(async () => { if (!actions.onCreateIm) return; let credentials: Record<string, unknown>; try { const parsed: unknown = JSON.parse(imCredentials); if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Credentials must be a JSON object.'); credentials = parsed as Record<string, unknown>; } catch (cause) { throw cause instanceof Error ? cause : new Error('Credentials must be valid JSON.'); } await actions.onCreateIm({ agentId: imAgentId.trim(), platform: imPlatform, name: imName.trim(), credentials }); setImName(''); onReload?.(); });
  const savePrincipal = () => run(async () => { await actions.onSavePrincipal?.({ mode: principalMode, requireDirectHeader, ...(hmacSecret.trim() ? { hmacSecret: hmacSecret.trim() } : {}) }); setHmacSecret(''); });
  const createPrincipalToken = () => run(async () => { if (!actions.onCreatePrincipalTestToken) return; setPrincipalToken(await actions.onCreatePrincipalTestToken(externalUserId.trim())); });
  const runPlayground = () => run(async () => { const path = playgroundPath.trim().replace('{session_id}', encodeURIComponent(sessionId.trim())); if (!apiKey.trim()) throw new Error('Enter an API key for this request.'); let body: unknown; try { body = JSON.parse(playgroundBody); } catch { throw new Error('Request body must be valid JSON.'); } const headers: Record<string, string> = { 'Content-Type': 'application/json', Accept: 'text/event-stream', 'X-API-Key': apiKey.trim() }; if (principalToken) headers[principalToken.headerName] = principalToken.token; const response = await fetch(`${apiBaseUrl.replace(/\/$/, '')}${path}`, { method: 'POST', headers, body: JSON.stringify(body) }); const text = await response.text(); if (!response.ok) throw new Error(`HTTP ${response.status}: ${text.slice(0, 500)}`); setPlaygroundOutput(text); });
  return (
    <main className="wk-integrations-page">
      <header className="wk-integrations-header">
        <div><p className="wk-eyebrow">React migration seam</p><h1>Integrations</h1><p className="wk-muted">Manage visitor channels and connect external clients.</p></div>
        {onReload ? <button className="wk-button" type="button" onClick={onReload}>Reload</button> : null}
      </header>
      <nav className="wk-integrations-tabs" aria-label="Integrations">
        {INTEGRATION_SECTIONS.map((item) => <button type="button" key={item.key} className={item.key === tab ? 'is-active' : ''} onClick={() => setTab(item.key)}>{labelFor(item.key)}</button>)}
      </nav>
      <section className="wk-integrations-panel">
        <div className="wk-integrations-panel-heading"><div><h2>{labelFor(section.key)}</h2><p>{section.external ? 'Open the maintained integration guide or marketplace listing.' : `Owned by the ${section.apiDomain} API domain.`}</p></div>{section.minRole === 'owner' ? <span className="wk-role-badge">Owner</span> : null}</div>
        {loading ? <p className="wk-status">Loading integrations…</p> : null}
        {error || localError ? <p className="wk-status wk-status-error" role="alert">{error || localError}</p> : null}
        {!loading && !error && tab === 'embed' ? <><form className="wk-integration-form" onSubmit={(event) => { event.preventDefault(); void createEmbed(); }}><h3>Create Embed channel</h3><label>Agent ID<input required value={embedAgentId} onChange={(event) => setEmbedAgentId(event.target.value)} /></label><label>Name<input required value={embedName} onChange={(event) => setEmbedName(event.target.value)} /></label><label>Allowed origins<input required value={embedOrigins} onChange={(event) => setEmbedOrigins(event.target.value)} /></label><button className="wk-button" type="submit" disabled={busy || !actions.onCreateEmbed}>Create</button></form><ResourceList items={embedChannels} empty="No Embed channels configured." actionLabel="Open" onAction={onOpenEmbed} editedNames={editedNames} onNameChange={(id, name) => setEditedNames((current) => ({ ...current, [id]: name }))} onSaveName={actions.onUpdateEmbed ? (item) => run(async () => { await actions.onUpdateEmbed?.(item.id, buildEmbedUpdatePayload(item, editedNames[item.id] ?? item.name ?? '')); onReload?.(); }) : undefined} onDelete={actions.onDeleteEmbed ? (id) => run(async () => { await actions.onDeleteEmbed?.(id); onReload?.(); }) : undefined} onRotate={actions.onRotateEmbed ? (id) => run(async () => { await actions.onRotateEmbed?.(id); }) : undefined} /> </> : null}
        {!loading && !error && tab === 'im' ? <><form className="wk-integration-form" onSubmit={(event) => { event.preventDefault(); void createIm(); }}><h3>Create IM channel</h3><label>Agent ID<input required value={imAgentId} onChange={(event) => setImAgentId(event.target.value)} /></label><label>Platform<select value={imPlatform} onChange={(event) => setImPlatform(event.target.value)}>{['feishu', 'lark', 'wecom', 'slack', 'telegram', 'dingtalk', 'mattermost', 'yunzhijia', 'wechat', 'qqbot'].map((platform) => <option key={platform}>{platform}</option>)}</select></label><label>Name<input required value={imName} onChange={(event) => setImName(event.target.value)} /></label><label>Credentials JSON<textarea rows={3} value={imCredentials} onChange={(event) => setImCredentials(event.target.value)} /></label><button className="wk-button" type="submit" disabled={busy || !actions.onCreateIm}>Create</button></form><ResourceList items={imChannels} empty="No IM channels configured." editedNames={editedNames} onNameChange={(id, name) => setEditedNames((current) => ({ ...current, [id]: name }))} onSaveName={actions.onUpdateIm ? (item) => run(async () => { await actions.onUpdateIm?.(item.id, { name: editedNames[item.id] ?? '' }); onReload?.(); }) : undefined} onToggle={actions.onToggleIm ? (id) => run(async () => { await actions.onToggleIm?.(id); onReload?.(); }) : undefined} onDelete={actions.onDeleteIm ? (id) => run(async () => { await actions.onDeleteIm?.(id); onReload?.(); }) : undefined} /> </> : null}
        {!loading && !error && tab === 'api' ? <ApiIntegrationPanel apiBaseUrl={apiBaseUrl} actions={actions} principalMode={principalMode} setPrincipalMode={setPrincipalMode} requireDirectHeader={requireDirectHeader} setRequireDirectHeader={setRequireDirectHeader} hmacSecret={hmacSecret} setHmacSecret={setHmacSecret} externalUserId={externalUserId} setExternalUserId={setExternalUserId} principalToken={principalToken} onSavePrincipal={savePrincipal} onCreatePrincipalToken={createPrincipalToken} apiKey={apiKey} setApiKey={setApiKey} sessionId={sessionId} setSessionId={setSessionId} playgroundPath={playgroundPath} setPlaygroundPath={setPlaygroundPath} playgroundBody={playgroundBody} setPlaygroundBody={setPlaygroundBody} playgroundOutput={playgroundOutput} onRunPlayground={runPlayground} busy={busy} /> : null}
        {!loading && !error && section.external ? <div className="wk-integration-copy"><p>This entry is intentionally an external guide so the main app does not embed third-party credentials or runtimes.</p><a className="wk-button" href={section.externalUrl} target="_blank" rel="noreferrer">Open {labelFor(section.key)} guide</a></div> : null}
      </section>
    </main>
  );
}

function ResourceList({ items, empty, actionLabel, onAction, editedNames = {}, onNameChange, onSaveName, onDelete, onRotate, onToggle }: { items: readonly IntegrationResource[]; empty: string; actionLabel?: string; onAction?: (item: IntegrationResource) => void; editedNames?: Record<string, string>; onNameChange?: (id: string, name: string) => void; onSaveName?: (item: IntegrationResource) => void; onDelete?: (id: string) => void; onRotate?: (id: string) => void; onToggle?: (id: string) => void }) {
  if (items.length === 0) return <p className="wk-status">{empty}</p>;
  return <div className="wk-integration-list">{items.map((item) => <article className="wk-integration-card" key={item.id}><div className="wk-list-item-copy"><input aria-label={`Name for ${item.id}`} value={editedNames[item.id] ?? item.name ?? item.platform ?? item.id} onChange={(event) => onNameChange?.(item.id, event.target.value)} /><span>{item.id}{item.platform ? ` · ${item.platform}` : ''}{item.credentials_configured === true ? ' · credentials configured' : ''}</span></div><div className="wk-integration-card-actions"><span className={item.enabled === false ? 'wk-disabled' : 'wk-enabled'}>{item.enabled === false ? 'Disabled' : 'Enabled'}</span>{onSaveName ? <button className="wk-button" type="button" onClick={() => onSaveName(item)}>Save</button> : null}{onToggle ? <button className="wk-button" type="button" onClick={() => onToggle(item.id)}>{item.enabled === false ? 'Enable' : 'Disable'}</button> : null}{onRotate ? <button className="wk-button" type="button" onClick={() => onRotate(item.id)}>Rotate</button> : null}{onAction && actionLabel ? <button className="wk-button" type="button" onClick={() => onAction(item)}>{actionLabel}</button> : null}{onDelete ? <button className="wk-button" type="button" onClick={() => onDelete(item.id)}>Delete</button> : null}</div></article>)}</div>;
}

function ApiIntegrationPanel(props: { apiBaseUrl: string; actions: IntegrationActions; principalMode: APIPrincipalConfig['mode']; setPrincipalMode: (value: APIPrincipalConfig['mode']) => void; requireDirectHeader: boolean; setRequireDirectHeader: (value: boolean) => void; hmacSecret: string; setHmacSecret: (value: string) => void; externalUserId: string; setExternalUserId: (value: string) => void; principalToken: IntegrationPrincipalToken | null; onSavePrincipal: () => void; onCreatePrincipalToken: () => void; apiKey: string; setApiKey: (value: string) => void; sessionId: string; setSessionId: (value: string) => void; playgroundPath: string; setPlaygroundPath: (value: string) => void; playgroundBody: string; setPlaygroundBody: (value: string) => void; playgroundOutput: string; onRunPlayground: () => void; busy: boolean }) {
  const { apiBaseUrl, actions, principalMode, setPrincipalMode, requireDirectHeader, setRequireDirectHeader, hmacSecret, setHmacSecret, externalUserId, setExternalUserId, principalToken, onSavePrincipal, onCreatePrincipalToken, apiKey, setApiKey, sessionId, setSessionId, playgroundPath, setPlaygroundPath, playgroundBody, setPlaygroundBody, playgroundOutput, onRunPlayground, busy } = props;
  return <div className="wk-api-integration"><div className="wk-integration-copy"><p>API base URL</p><code>{apiBaseUrl}</code><a href={`${apiBaseUrl.replace(/\/$/, '')}/docs`} target="_blank" rel="noreferrer">Open API documentation</a></div><form className="wk-integration-form" onSubmit={(event) => { event.preventDefault(); onSavePrincipal(); }}><h3>External user Principal</h3><p className="wk-muted">Owner-only server configuration. The HMAC secret is write-only and is cleared after saving.</p><label>Mode<select value={principalMode} onChange={(event) => setPrincipalMode(event.target.value as APIPrincipalConfig['mode'])}><option value="tenant">Tenant (shared session)</option><option value="direct_header">Direct header (trusted server only)</option><option value="signed_token">Signed token (recommended)</option></select></label><label><input type="checkbox" checked={requireDirectHeader} onChange={(event) => setRequireDirectHeader(event.target.checked)} /> Require direct external-user header</label>{principalMode === 'signed_token' ? <label>HMAC secret<input type="password" autoComplete="new-password" value={hmacSecret} onChange={(event) => setHmacSecret(event.target.value)} placeholder={actions.principal?.has_hmac_secret ? 'Configured; leave blank to keep' : 'Required for first save'} /></label> : null}<button className="wk-button" type="submit" disabled={busy || !actions.onSavePrincipal}>Save Principal config</button></form><div className="wk-integration-form"><h3>Generate short-lived test Principal</h3><label>External user ID<input value={externalUserId} onChange={(event) => setExternalUserId(event.target.value)} /></label><button className="wk-button" type="button" disabled={busy || !actions.onCreatePrincipalTestToken || !externalUserId.trim()} onClick={onCreatePrincipalToken}>Generate token</button>{principalToken ? <div className="wk-secret-output"><p>Use header <code>{principalToken.headerName}</code> for {principalToken.externalUserId}; expires in {principalToken.expiresInSeconds}s.</p><textarea readOnly rows={3} value={principalToken.token} /></div> : null}</div><form className="wk-integration-form" onSubmit={(event) => { event.preventDefault(); onRunPlayground(); }}><h3>API playground (SSE)</h3><p className="wk-muted">Nothing is persisted in this form. The API key stays in memory and is sent only to the request below.</p><label>API key<input type="password" autoComplete="off" value={apiKey} onChange={(event) => setApiKey(event.target.value)} /></label><label>Session ID<input value={sessionId} onChange={(event) => setSessionId(event.target.value)} placeholder="Existing session ID" /></label><label>POST path<input required value={playgroundPath} onChange={(event) => setPlaygroundPath(event.target.value)} /></label><label>JSON body<textarea rows={6} value={playgroundBody} onChange={(event) => setPlaygroundBody(event.target.value)} /></label><button className="wk-button" type="submit" disabled={busy || !apiKey.trim()}>Run SSE request</button>{playgroundOutput ? <pre className="wk-api-output">{playgroundOutput}</pre> : null}</form></div>;
}
