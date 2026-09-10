import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { settingsResourceInput, settingsResourceRows } from './surface.ts';

type ResourceSection = 'storage' | 'vectorstore' | 'websearch';
type ResourceRow = Record<string, unknown>;
type ResourceApi = {
  list: () => Promise<readonly ResourceRow[]>;
  create: (input: Record<string, unknown>) => Promise<ResourceRow>;
  update: (id: string, input: Record<string, unknown>) => Promise<ResourceRow>;
  remove: (id: string) => Promise<unknown>;
  testById: (id: string) => Promise<{ success: boolean; message?: string; error?: string }>;
  setDefault?: (id: string) => Promise<unknown>;
};

function rowId(row: ResourceRow): string {
  const id = row.id ?? row.uuid ?? row.name;
  return typeof id === 'string' || typeof id === 'number' ? String(id) : '';
}

function rowText(row: ResourceRow, key: string): string { return typeof row[key] === 'string' ? row[key] as string : ''; }

function safeConfig(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).filter(([key]) => {
    const normalized = key.toLowerCase();
    return !normalized.includes('secret') && !normalized.includes('password') && !normalized.includes('token') && !normalized.includes('api_key') && !normalized.includes('access_key');
  }).map(([key, item]) => [key, item && typeof item === 'object' && !Array.isArray(item) ? safeConfig(item) : item]));
}

function apiFor(client: WeKnoraClient, section: ResourceSection): ResourceApi {
  if (section === 'storage') return client.settings.storage.backends;
  if (section === 'vectorstore') return client.settings.vectorStores;
  return client.settings.webSearch.providers;
}

function resourceLabel(section: ResourceSection): string {
  return section === 'storage' ? 'storage backend' : section === 'vectorstore' ? 'vector store' : 'web-search provider';
}

export function ResourceSettingsPanel({ client, section, initialValue }: { client: WeKnoraClient; section: ResourceSection; initialValue: unknown }) {
  const api = apiFor(client, section);
  const [rows, setRows] = useState<ResourceRow[]>(() => settingsResourceRows(initialValue, section));
  const [name, setName] = useState('');
  const [type, setType] = useState('');
  const [configText, setConfigText] = useState('{}');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setRows(settingsResourceRows(initialValue, section)); }, [initialValue, section]);

  function clearForm() {
    setEditingId(null); setName(''); setType(''); setConfigText('{}');
  }

  function edit(row: ResourceRow) {
    setEditingId(rowId(row)); setName(rowText(row, 'name')); setType(rowText(row, 'type'));
    setConfigText(JSON.stringify(safeConfig(row.config), null, 2)); setError(null); setNotice(null);
  }

  async function refresh() {
    setBusy(true); setError(null);
    try { setRows([...await api.list()]); }
    catch (reason) { setError(reason instanceof Error ? reason.message : `Unable to load ${resourceLabel(section)}s.`); }
    finally { setBusy(false); }
  }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const input = settingsResourceInput(name, type, configText);
      const saved = editingId ? await api.update(editingId, input) : await api.create(input);
      setRows((current) => editingId ? current.map((row) => rowId(row) === editingId ? saved : row) : [...current, saved]);
      clearForm(); setNotice(`${resourceLabel(section)} saved.`);
    } catch (reason) { setError(reason instanceof Error ? reason.message : `Unable to save ${resourceLabel(section)}; the server value was kept.`); }
    finally { setBusy(false); }
  }

  async function test(id: string) {
    setBusy(true); setError(null); setNotice(null);
    try { const result = await api.testById(id); if (!result.success) throw new Error(result.error || 'Connection test failed'); setNotice(result.message || 'Connection test succeeded.'); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Connection test failed.'); }
    finally { setBusy(false); }
  }

  async function remove(id: string) {
    if (!id || !window.confirm(`Delete this ${resourceLabel(section)}?`)) return;
    setBusy(true); setError(null); setNotice(null);
    try { await api.remove(id); setRows((current) => current.filter((row) => rowId(row) !== id)); if (editingId === id) clearForm(); setNotice(`${resourceLabel(section)} deleted.`); }
    catch (reason) { setError(reason instanceof Error ? reason.message : `Unable to delete ${resourceLabel(section)}.`); }
    finally { setBusy(false); }
  }

  async function setDefault(id: string) {
    if (!api.setDefault) return;
    setBusy(true); setError(null); setNotice(null);
    try { await api.setDefault(id); setNotice('Default storage backend updated.'); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to set the default storage backend.'); setBusy(false); }
  }

  return <div className="wk-settings-resource"><Card><h3>{editingId ? `Edit ${resourceLabel(section)}` : `Add ${resourceLabel(section)}`}</h3>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor" onSubmit={(event) => void save(event)}><label>Name<input required value={name} onChange={(event) => setName(event.target.value)} /></label><label>Type<input required value={type} onChange={(event) => setType(event.target.value)} placeholder="Provider type" /></label><label>Safe configuration JSON<textarea rows={4} value={configText} onChange={(event) => setConfigText(event.target.value)} /></label><div className="wk-list-actions"><Button type="submit" loading={busy}>{editingId ? 'Save changes' : 'Create'}</Button>{editingId ? <Button type="button" disabled={busy} onClick={clearForm}>Cancel</Button> : null}</div></form></Card><Card><div className="wk-settings-panel-heading"><div><h3>Configured resources</h3><p className="wk-muted">Secrets are never prefilled from server responses. Test and delete operations wait for server confirmation.</p></div><Button type="button" disabled={busy} onClick={() => void refresh()}>Reload</Button></div>{rows.length === 0 ? <Status>No configured {resourceLabel(section)}s returned.</Status> : <ul className="wk-list">{rows.map((row, index) => { const id = rowId(row); return <li key={id || index}><div className="wk-list-item-copy"><strong>{rowText(row, 'name') || id || 'Unnamed resource'}</strong><span>{rowText(row, 'type') || 'type unavailable'}{row.default === true ? ' · default' : ''}</span></div><div className="wk-list-actions"><Button type="button" disabled={!id || busy} onClick={() => edit(row)}>Edit</Button><Button type="button" disabled={!id || busy} loading={busy} onClick={() => void test(id)}>Test</Button>{api.setDefault ? <Button type="button" disabled={!id || busy} onClick={() => void setDefault(id)}>Set default</Button> : null}<Button type="button" disabled={!id || busy} onClick={() => void remove(id)}>Delete</Button></div></li>; })}</ul>}</Card></div>;
}
