import { useEffect, useState } from 'react';
import type { AgentConfiguration, InstalledSkill, ModelConfiguration, SkillCatalog, SkillFile, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { installProgress, isSafeSkillFilePath, normalizeSandboxConfigIds, redactDebugValue, shouldPollInstalledSkill } from './management.ts';

function errorMessage(value: unknown): string { return value instanceof Error ? value.message : 'Configuration operation failed'; }

export function AgentOperations({ client, agents, disabledIds }: { client: WeKnoraClient; agents: AgentConfiguration[]; disabledIds: string[] }) {
  const [selected, setSelected] = useState(agents[0]?.id ?? '');
  const [organizationId, setOrganizationId] = useState('');
  const [permission, setPermission] = useState<'viewer' | 'editor' | 'admin'>('viewer');
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!agents.some((agent) => agent.id === selected)) setSelected(agents[0]?.id ?? '');
  }, [agents, selected]);

  async function share() {
    if (!selected || !organizationId.trim()) { setError('Select an agent and enter an organization ID.'); return; }
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.identity.organizations.agentShares.create(selected, { organization_id: organizationId.trim(), permission });
      setNotice('Agent share saved by the server.');
    } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  async function setDisabled(disabled: boolean) {
    if (!selected) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.identity.organizations.agentShares.setDisabledByMe(selected, disabled);
      setNotice(disabled ? 'Agent hidden for this workspace.' : 'Agent restored for this workspace.');
    } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  const selectedDisabled = disabledIds.includes(selected);
  return <Card className="wk-configuration-operations"><h2>Agent selection and sharing</h2><p className="wk-muted">Selection is local to the current chat entry. Sharing and hide/show preferences remain server-authorized.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="wk-form-grid wk-form-grid--two"><label>Selected agent<select value={selected} onChange={(event) => setSelected(event.target.value)}><option value="">No agent</option>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}{disabledIds.includes(agent.id) ? ' · disabled' : ''}</option>)}</select></label><label>Organization ID<input value={organizationId} onChange={(event) => setOrganizationId(event.target.value)} placeholder="org_…" /></label><label>Share permission<select value={permission} onChange={(event) => setPermission(event.target.value as typeof permission)}><option value="viewer">Viewer</option><option value="editor">Editor</option><option value="admin">Admin</option></select></label></div><div className="wk-list-actions"><Button type="button" disabled={busy || !selected} onClick={() => void setDisabled(!selectedDisabled)}>{selectedDisabled ? 'Show selected agent' : 'Hide selected agent'}</Button><Button type="button" loading={busy} disabled={!selected || !organizationId.trim()} onClick={() => void share()}>Share agent</Button></div></Card>;
}

export function ModelDebugPanel({ client, models }: { client: WeKnoraClient; models: ModelConfiguration[] }) {
  const [modelId, setModelId] = useState(models[0]?.id ?? '');
  const [input, setInput] = useState('');
  const [documents, setDocuments] = useState('');
  const [result, setResult] = useState<unknown>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => { if (!models.some((model) => model.id === modelId)) setModelId(models[0]?.id ?? ''); }, [modelId, models]);

  async function debug(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!modelId || !input.trim()) { setError('Select a model and enter an input.'); return; }
    setBusy(true); setError(null); setResult(null);
    try {
      const parsedDocuments = documents.trim() ? documents.split('\n').map((item) => item.trim()).filter(Boolean) : undefined;
      setResult(await client.configuration.models.debug(modelId, { input, documents: parsedDocuments, options: { thinking: false } }));
    } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  return <Card className="wk-configuration-operations"><h2>Model debug</h2><p className="wk-muted">The provider call runs on the server; previews and observations are redacted before rendering. No usage/occupancy endpoint is available in the current backend contract.</p>{error ? <Status tone="error">{error}</Status> : null}<form className="wk-settings-editor" onSubmit={(event) => void debug(event)}><label>Model<select value={modelId} onChange={(event) => setModelId(event.target.value)}><option value="">No model</option>{models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}</select></label><label>Input<textarea required rows={3} value={input} onChange={(event) => setInput(event.target.value)} /></label><label>Documents, one per line<textarea rows={3} value={documents} onChange={(event) => setDocuments(event.target.value)} /></label><Button type="submit" loading={busy} disabled={!modelId}>Run debug</Button></form>{result ? <pre className="wk-debug">{JSON.stringify(redactDebugValue(result), null, 2)}</pre> : null}</Card>;
}

function catalogStatuses(catalog: SkillCatalog): InstalledSkill[] {
  return (catalog.installations ?? []).map((installation) => ({ id: installation.skillId, name: catalog.name, enabled: installation.enabled, status: installation.status === 'removed' ? 'failed' : installation.status, error: installation.error }));
}

export function SkillOperations({ client, initialCatalog }: { client: WeKnoraClient; initialCatalog?: SkillCatalog[] }) {
  const [catalog, setCatalog] = useState<SkillCatalog[]>(initialCatalog ?? []);
  const [source, setSource] = useState('');
  const [sandboxIds, setSandboxIds] = useState('');
  const [selectedId, setSelectedId] = useState('');
  const [files, setFiles] = useState<SkillFile[]>([]);
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function reload() {
    try { setCatalog(await client.configuration.skills.catalog.list()); } catch (cause) { setError(errorMessage(cause)); }
  }
  useEffect(() => { if (!initialCatalog) void reload(); }, [initialCatalog]);
  const active = catalog.flatMap(catalogStatuses).some((skill) => shouldPollInstalledSkill(skill.status));
  useEffect(() => {
    if (!active) return undefined;
    const timer = window.setInterval(() => void reload(), 2500);
    return () => window.clearInterval(timer);
  }, [active]);

  async function register(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!source.trim()) return;
    setBusy(true); setError(null); setNotice(null);
    try { await client.configuration.skills.catalog.register({ source: source.trim() }); setSource(''); setNotice('Skill registered in the workspace catalog.'); await reload(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }
  async function install(id: string) {
    const ids = normalizeSandboxConfigIds(sandboxIds);
    if (ids.length === 0) { setError('Enter at least one sandbox configuration ID.'); return; }
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.configuration.skills.catalog.install(id, ids); setNotice(`Install accepted for ${Object.keys(result.installs).length} sandbox configuration(s).`); await reload(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }
  async function openFiles(id: string) {
    setSelectedId(id); setFileContent(null); setError(null);
    try { setFiles(await client.configuration.skills.catalog.files(id)); } catch (cause) { setError(errorMessage(cause)); }
  }
  async function openFile(path: string) {
    if (!selectedId || !isSafeSkillFilePath(path)) { setError('Unsafe skill file path rejected.'); return; }
    try { const file = await client.configuration.skills.catalog.file(selectedId, path); setFileContent(file.content ?? `[${file.encoding} file; no inline content]`); } catch (cause) { setError(errorMessage(cause)); }
  }
  async function stop(configId: string, skillId: string) {
    setBusy(true); setError(null); setNotice(null);
    try { await client.configuration.skills.installed.stop(configId, skillId); setNotice('Install stop requested and confirmed by the server.'); await reload(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }
  const summary = installProgress(catalog.flatMap(catalogStatuses).map((skill) => skill.status));
  return <Card className="wk-configuration-operations"><div className="wk-settings-panel-heading"><div><h2>Skill catalog and files</h2><p className="wk-muted">Installation is asynchronous and server-owned; status is polled until ready, failed, or removing. File paths are validated before reads.</p></div><Button type="button" disabled={busy} onClick={() => void reload()}>Reload</Button></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor" onSubmit={(event) => void register(event)}><label>Catalog source<input value={source} onChange={(event) => setSource(event.target.value)} placeholder="@owner/skill or https://…" /></label><Button type="submit" loading={busy}>Register skill source</Button></form><label>Sandbox configuration IDs<input value={sandboxIds} onChange={(event) => setSandboxIds(event.target.value)} placeholder="cfg-a, cfg-b" /></label><p className="wk-debug">installations: {summary.complete} complete · {summary.active} active · {summary.failed} failed</p>{catalog.length === 0 ? <Status>No catalog entries returned.</Status> : <ul className="wk-list">{catalog.map((item) => <li key={item.id}><div className="wk-list-item-copy"><strong>{item.name}</strong><span>{item.version ?? 'unversioned'} · {item.installations?.length ?? 0} installation(s)</span><small>{item.description ?? 'No description returned.'}</small>{item.installations?.map((installation) => <small key={installation.sandboxConfigId}>{installation.sandboxConfigName ?? installation.sandboxConfigId}: {installation.status}{installation.error ? ` · ${installation.error}` : ''}{shouldPollInstalledSkill(installation.status === 'removed' ? 'failed' : installation.status) ? <Button type="button" disabled={busy} onClick={() => void stop(installation.sandboxConfigId, installation.skillId)}>Stop</Button> : null}</small>)}</div><div className="wk-list-actions"><Button type="button" disabled={busy} onClick={() => void install(item.id)}>Install</Button><Button type="button" disabled={busy} onClick={() => void openFiles(item.id)}>Files</Button></div></li>)}</ul>}{files.length > 0 ? <div><h3>Files</h3><ul className="wk-list">{files.map((file) => <li key={file.path}><button type="button" onClick={() => void openFile(file.path)}>{file.path} · {file.size} bytes</button></li>)}</ul></div> : null}{fileContent !== null ? <pre className="wk-debug">{fileContent}</pre> : null}</Card>;
}
