import { useEffect, useState } from 'react';
import type { AgentConfiguration, InstalledSkill, ModelConfiguration, ModelDebugInput, SkillCatalog, SkillCatalogInstallResult, SkillFile, SkillFileContent, SkillInstallStatus, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Select, Status, Textarea } from '@weknora/ui';
import { installProgress, isSafeSkillFilePath, normalizeSandboxConfigIds, redactDebugValue, shouldPollInstalledSkill, skillFileTree } from './management.ts';

function errorMessage(value: unknown): string { return value instanceof Error ? value.message : 'Configuration operation failed'; }

export function agentShareInput(organizationId: string): { organization_id: string; permission: 'viewer' } {
  return { organization_id: organizationId.trim(), permission: 'viewer' };
}

export function canStopSkillInstallation(status: SkillInstallStatus): boolean { return status === 'installing'; }

export function skillInstallFeedback(result: SkillCatalogInstallResult): { tone: 'success' | 'warning'; message: string } {
  const accepted = Object.keys(result.installs).length;
  const errors = Object.entries(result.errors ?? {});
  if (errors.length === 0) return { tone: 'success', message: `Install accepted for ${accepted} sandbox configuration(s).` };
  return {
    tone: 'warning',
    message: `Install accepted for ${accepted} sandbox configuration(s); ${errors.length} rejected: ${errors.map(([id, message]) => `${id}: ${message}`).join('; ')}.`,
  };
}

export function modelDebugRequest(modelType: string | undefined, input: string, documents: string, file?: Blob): ModelDebugInput {
  if (modelType === 'VLLM' && file === undefined) throw new Error('An image file is required for VLLM debug.');
  if (modelType === 'ASR' && file === undefined) throw new Error('An audio file is required for ASR debug.');
  if (modelType !== 'VLLM' && modelType !== 'ASR' && !input.trim()) throw new Error('Enter an input.');
  const parsedDocuments = documents.trim() ? documents.split('\n').map((item) => item.trim()).filter(Boolean) : undefined;
  return { input, documents: parsedDocuments, options: { thinking: false }, ...(file === undefined ? {} : { file }) };
}

export function AgentOperations({ client, agents, disabledIds }: { client: WeKnoraClient; agents: AgentConfiguration[]; disabledIds: string[] }) {
  const [selected, setSelected] = useState(agents[0]?.id ?? '');
  const [organizationId, setOrganizationId] = useState('');
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
      await client.identity.organizations.agentShares.create(selected, agentShareInput(organizationId));
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
  return <Card className="wk-configuration-operations"><h2>Agent sharing and visibility</h2><p className="wk-muted text-muted">Select an agent here for sharing or hide/show operations. To choose an agent for a conversation, use the Agent selector in the chat entry. Shared agents are read-only in the receiving workspace, so every share uses Viewer permission. Sharing and hide/show preferences remain server-authorized.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="wk-form-grid wk-form-grid--two grid grid-cols-2 gap-4 max-[720px]:grid-cols-1 mt-3"><label>Selected agent<Select value={selected} onChange={(event) => setSelected(event.target.value)}><option value="">No agent</option>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}{disabledIds.includes(agent.id) ? ' · disabled' : ''}</option>)}</Select></label><label>Organization ID<Input value={organizationId} onChange={(event) => setOrganizationId(event.target.value)} placeholder="org_…" /></label><label>Share permission<Input value="Viewer (read-only)" readOnly /></label></div><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="button" disabled={busy || !selected} onClick={() => void setDisabled(!selectedDisabled)}>{selectedDisabled ? 'Show selected agent' : 'Hide selected agent'}</Button><Button type="button" loading={busy} disabled={!selected || !organizationId.trim()} onClick={() => void share()}>Share agent</Button></div></Card>;
}

export function ModelDebugPanel({ client, models }: { client: WeKnoraClient; models: ModelConfiguration[] }) {
  const [modelId, setModelId] = useState(models[0]?.id ?? '');
  const [input, setInput] = useState('');
  const [documents, setDocuments] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<unknown>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => { if (!models.some((model) => model.id === modelId)) setModelId(models[0]?.id ?? ''); }, [modelId, models]);
  const modelType = models.find((model) => model.id === modelId)?.type;
  const fileKind = modelType === 'VLLM' ? 'image' : modelType === 'ASR' ? 'audio' : null;

  async function debug(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!modelId) { setError('Select a model.'); return; }
    let request: ModelDebugInput;
    try { request = modelDebugRequest(modelType, input, documents, file ?? undefined); } catch (cause) { setError(errorMessage(cause)); return; }
    setBusy(true); setError(null); setResult(null);
    try {
      setResult(await client.configuration.models.debug(modelId, request));
    } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  return <Card className="wk-configuration-operations"><h2>Model debug</h2><p className="wk-muted text-muted">The provider call runs on the server; previews and observations are redacted before rendering. VLLM requires an image and ASR requires an audio file. No usage/occupancy endpoint is available in the current backend contract.</p>{error ? <Status tone="error">{error}</Status> : null}<form className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem]" onSubmit={(event) => void debug(event)}><label>Model<Select value={modelId} onChange={(event) => { setModelId(event.target.value); setFile(null); }}><option value="">No model</option>{models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}</Select></label><label>Input<Textarea required={fileKind === null} rows={3} value={input} onChange={(event) => setInput(event.target.value)} /></label><label>Documents, one per line<Textarea rows={3} value={documents} onChange={(event) => setDocuments(event.target.value)} /></label>{fileKind ? <label>{fileKind === 'image' ? 'Image file' : 'Audio file'}<input type="file" accept={`${fileKind}/*`} required onChange={(event) => setFile(event.currentTarget.files?.[0] ?? null)} /></label> : null}<Button type="submit" loading={busy} disabled={!modelId}>Run debug</Button></form>{result ? <pre className="wk-debug border-b border-line-soft pb-[0.8rem] font-mono text-[0.75rem] text-muted [overflow-wrap:anywhere]">{JSON.stringify(redactDebugValue(result), null, 2)}</pre> : null}</Card>;
}

function catalogStatuses(catalog: SkillCatalog): InstalledSkill[] {
  return (catalog.installations ?? []).map((installation) => ({ id: installation.skillId, name: catalog.name, enabled: installation.enabled, status: installation.status === 'removed' ? 'failed' : installation.status, error: installation.error }));
}

export function SkillOperations({ client, initialCatalog }: { client: WeKnoraClient; initialCatalog?: SkillCatalog[] }) {
  const [catalog, setCatalog] = useState<SkillCatalog[]>(initialCatalog ?? []);
  const [source, setSource] = useState('');
  const [sandboxIds, setSandboxIds] = useState('');
  const [selectedId, setSelectedId] = useState('');
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [files, setFiles] = useState<SkillFile[]>([]);
  const [selectedFile, setSelectedFile] = useState<SkillFileContent | null>(null);
  const [fileContent, setFileContent] = useState<string | null>(null);
  const [notice, setNotice] = useState<{ tone: 'success' | 'warning'; message: string } | null>(null);
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
    try { await client.configuration.skills.catalog.register({ source: source.trim() }); setSource(''); setNotice({ tone: 'success', message: 'Skill registered in the workspace catalog.' }); await reload(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }
  async function install(id: string) {
    const ids = normalizeSandboxConfigIds(sandboxIds);
    if (ids.length === 0) { setError('Enter at least one sandbox configuration ID.'); return; }
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.configuration.skills.catalog.install(id, ids); setNotice(skillInstallFeedback(result)); await reload(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }
  async function openFiles(id: string) {
    setSelectedId(id); setSelectedPath(null); setSelectedFile(null); setFileContent(null); setError(null);
    try { setFiles(await client.configuration.skills.catalog.files(id)); } catch (cause) { setError(errorMessage(cause)); }
  }
  async function openFile(path: string) {
    if (!selectedId || !isSafeSkillFilePath(path)) { setError('Unsafe skill file path rejected.'); return; }
    try { const file = await client.configuration.skills.catalog.file(selectedId, path); setSelectedPath(path); setSelectedFile(file); setFileContent(file.content ?? `[${file.encoding} file; no inline content]`); } catch (cause) { setError(errorMessage(cause)); }
  }
  async function stop(configId: string, skillId: string) {
    setBusy(true); setError(null); setNotice(null);
    try { await client.configuration.skills.installed.stop(configId, skillId); setNotice({ tone: 'success', message: 'Install stop requested and confirmed by the server.' }); await reload(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }
  const summary = installProgress(catalog.flatMap(catalogStatuses).map((skill) => skill.status));
  const fileRows = skillFileTree(files);
  return <Card className="wk-configuration-operations"><div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col"><div><h2 className="my-1">Skill catalog and files</h2><p className="wk-muted text-muted m-0">Installation is asynchronous and server-owned; status is polled until ready, failed, or removed. File paths are validated before reads.</p></div><Button type="button" disabled={busy} onClick={() => void reload()}>Reload</Button></div>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone={notice.tone}>{notice.message}</Status> : null}<form className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem]" onSubmit={(event) => void register(event)}><label>Catalog source<Input value={source} onChange={(event) => setSource(event.target.value)} placeholder="@owner/skill or https://…" /></label><Button type="submit" loading={busy}>Register skill source</Button></form><label>Sandbox configuration IDs<Input value={sandboxIds} onChange={(event) => setSandboxIds(event.target.value)} placeholder="cfg-a, cfg-b" /></label><p className="wk-debug border-b border-line-soft pb-[0.8rem] font-mono text-[0.75rem] text-muted [overflow-wrap:anywhere]">installations: {summary.complete} complete · {summary.active} active · {summary.failed} failed</p>{catalog.length === 0 ? <Status>No catalog entries returned.</Status> : <ul className="wk-list m-0 list-none p-0">{catalog.map((item) => <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{item.name}</strong><span className="font-mono text-[0.8rem] text-muted">{item.version ?? 'unversioned'} · {item.installations?.length ?? 0} installation(s)</span><small>{item.description ?? 'No description returned.'}</small>{item.installations?.map((installation) => <small key={installation.sandboxConfigId}>{installation.sandboxConfigName ?? installation.sandboxConfigId}: {installation.status}{installation.error ? ` · ${installation.error}` : ''}{canStopSkillInstallation(installation.status) ? <Button type="button" disabled={busy} onClick={() => void stop(installation.sandboxConfigId, installation.skillId)}>Stop</Button> : null}</small>)}</div><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Button type="button" disabled={busy} onClick={() => void install(item.id)}>Install</Button><Button type="button" disabled={busy} onClick={() => void openFiles(item.id)}>Files</Button></div></li>)}</ul>}{fileRows.length > 0 ? <div><h3>Files</h3><ul className="wk-list m-0 list-none p-0">{fileRows.map((file) => <li key={file.path} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><button type="button" disabled={file.isDir} onClick={() => void openFile(file.path)} style={{ paddingLeft: `${file.depth * 16 + 8}px` }}>{file.isDir ? '▸ ' : '· '}{file.name}{file.isDir ? '' : ` · ${file.size ?? 0} bytes`}</button></li>)}</ul></div> : null}{fileContent !== null ? <div><h3>{selectedFile?.path ?? 'File preview'}</h3>{selectedFile?.truncated ? <Status tone="warning">File preview was truncated by the server.</Status> : null}<pre className="wk-debug border-b border-line-soft pb-[0.8rem] font-mono text-[0.75rem] text-muted [overflow-wrap:anywhere]">{fileContent}</pre></div> : null}</Card>;
}
