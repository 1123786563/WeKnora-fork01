import { useEffect, useState } from 'react';
import type { SandboxBackendType, SandboxConfigRecord, SandboxConfigUpsert, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';

type Role = 'viewer' | 'admin' | 'owner' | 'system-admin';
type Props = { client: WeKnoraClient; role: Role; initialData?: { items: SandboxConfigRecord[]; workspaceScriptsDisabled: boolean } };

const backends: SandboxBackendType[] = ['cube', 'e2b', 'docker'];

export function SandboxSettingsPanel({ client, role, initialData }: Props) {
  const canEdit = role === 'admin' || role === 'owner';
  const [data, setData] = useState(initialData);
  const [loading, setLoading] = useState(initialData === undefined);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [filter, setFilter] = useState<'all' | SandboxBackendType>('all');
  const [draft, setDraft] = useState<SandboxConfigUpsert | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    setLoading(true); setError(null);
    try { setData(await client.sandboxConfigurations.list()); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load sandbox configurations'); }
    finally { setLoading(false); }
  }
  useEffect(() => { if (initialData === undefined) void load(); }, [client, initialData]);
  const items = data?.items ?? [];
  const filtered = filter === 'all' ? items : items.filter((item) => item.sandbox_type === filter);
  function startCreate(type: SandboxBackendType = 'docker') { setEditingId(null); setDraft({ name: '', description: '', config: { sandbox_type: type, [type]: {} } }); }
  function startEdit(item: SandboxConfigRecord) { setEditingId(item.id); setDraft({ name: item.name, description: item.description, config: item.config }); }
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!draft || busy || !draft.name.trim()) return;
    setBusy(true); setError(null); setNotice(null);
    try { if (editingId) await client.sandboxConfigurations.update(editingId, { ...draft, name: draft.name.trim() }); else await client.sandboxConfigurations.create({ ...draft, name: draft.name.trim() }); setDraft(null); setNotice(editingId ? 'Sandbox configuration updated.' : 'Sandbox configuration created.'); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to save sandbox configuration'); }
    finally { setBusy(false); }
  }
  async function remove(item: SandboxConfigRecord) {
    if (!canEdit || busy || !window.confirm(`Delete sandbox configuration “${item.name}”?`)) return;
    setBusy(true); setError(null); setNotice(null);
    try { await client.sandboxConfigurations.remove(item.id); setNotice('Sandbox configuration deleted.'); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to delete sandbox configuration'); }
    finally { setBusy(false); }
  }
  async function setScriptsDisabled(disabled: boolean) {
    if (!canEdit || busy) return;
    setBusy(true); setError(null); setNotice(null);
    try { const result = await client.sandboxConfigurations.setWorkspacePolicy(disabled); setData((current) => current ? { ...current, workspaceScriptsDisabled: result.workspaceScriptsDisabled } : current); setNotice(disabled ? 'Script execution disabled.' : 'Script execution enabled.'); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to update script policy'); }
    finally { setBusy(false); }
  }
  if (loading) return <Card data-testid="sandbox-settings"><Status>Loading sandbox configurations…</Status></Card>;
  return <section className="wk-sandbox-settings" data-testid="sandbox-settings">
    <div className="wk-settings-panel-heading"><div><h3>Sandbox settings</h3><p className="wk-muted">Configure isolated execution environments for this workspace.</p></div>{canEdit ? <Button type="button" onClick={() => startCreate()}>Add sandbox configuration</Button> : null}</div>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}
    {canEdit ? <div className="wk-sandbox-policy"> <div><strong>Script execution</strong><p className="wk-muted">Disable new script execution for the workspace.</p></div>{data?.workspaceScriptsDisabled ? <Button type="button" disabled={busy} onClick={() => void setScriptsDisabled(false)}>Enable script execution</Button> : <div data-confirm="disable-scripts"><details><summary>Disable script execution</summary><div><p>Existing sandboxes may continue until they exit.</p><Button type="button" disabled={busy} onClick={() => void setScriptsDisabled(true)}>Confirm disable</Button><Button type="button">Cancel</Button></div></details></div>}</div> : null}
    <nav className="wk-model-tabs" aria-label="Sandbox backend"><button type="button" className={filter === 'all' ? 'is-active' : ''} onClick={() => setFilter('all')}>All ({items.length})</button>{backends.map((type) => <button type="button" key={type} className={filter === type ? 'is-active' : ''} onClick={() => setFilter(type)}>{type.toUpperCase()} ({items.filter((item) => item.sandbox_type === type).length})</button>)}</nav>
    {filtered.length === 0 ? <Status>No sandbox configurations configured.</Status> : <div className="wk-sandbox-grid">{filtered.map((item) => <Card key={item.id} className="wk-sandbox-card"><div className="wk-sandbox-card-header"><div><span className="wk-muted">{item.sandbox_type.toUpperCase()}</span><h4>{item.name}</h4></div>{canEdit ? <div className="wk-list-actions"><Button type="button" onClick={() => startEdit(item)}>Edit</Button><Button type="button" disabled={busy} onClick={() => void remove(item)}>Delete</Button></div> : null}</div><p className="wk-muted">{item.description || 'No description.'}</p>{canEdit ? <Button type="button" onClick={() => setNotice('Inventory inspection is not yet ported; deletion remains conflict-protected by the server.')}>Inspect inventory</Button> : null}</Card>)}</div>}
    <p className="wk-muted">Wizard, template catalog, deep checks, and skill installation remain unsupported in this React slice.</p>
    {draft ? <div className="wk-sandbox-editor" role="dialog" aria-modal="true"><form className="wk-settings-editor" onSubmit={(event) => void save(event)}><h3>{editingId ? 'Edit sandbox configuration' : 'Add sandbox configuration'}</h3><label>Name<input required value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label><label>Description<textarea value={draft.description ?? ''} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label><label>Backend<select value={String(draft.config.sandbox_type ?? 'docker')} disabled={Boolean(editingId)} onChange={(event) => { const type = event.target.value as SandboxBackendType; setDraft({ ...draft, config: { ...draft.config, sandbox_type: type, [type]: draft.config[type] ?? {} } }); }}>{backends.map((type) => <option key={type} value={type}>{type.toUpperCase()}</option>)}</select></label><div className="wk-list-actions"><Button type="submit" loading={busy}>Save</Button><Button type="button" disabled={busy} onClick={() => setDraft(null)}>Cancel</Button></div></form></div> : null}
  </section>;
}
