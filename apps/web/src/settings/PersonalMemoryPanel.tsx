import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { memoryItemPatch, memoryWorkspacePatch } from './surface.ts';

type MemoryRow = Record<string, unknown>;

function rowId(row: MemoryRow): string { return typeof row.id === 'string' || typeof row.id === 'number' ? String(row.id) : ''; }
function rowContent(row: MemoryRow): string { return typeof row.content === 'string' ? row.content : ''; }

export function MemoryWorkspacePanel({ client, initialConfig }: { client: WeKnoraClient; initialConfig: unknown }) {
  const row = initialConfig !== null && typeof initialConfig === 'object' && !Array.isArray(initialConfig) ? initialConfig as MemoryRow : {};
  const [enabled, setEnabled] = useState(row.enabled === true);
  const [writeMode, setWriteMode] = useState(row.write_mode === 'auto' ? 'auto' : 'explicit_only');
  const [maxItems, setMaxItems] = useState(typeof row.max_items === 'number' ? row.max_items : 200);
  const [vectorRecall, setVectorRecall] = useState(row.vector_recall !== false);
  const [retrievalConditioning, setRetrievalConditioning] = useState(row.retrieval_conditioning !== false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    setEnabled(row.enabled === true); setWriteMode(row.write_mode === 'auto' ? 'auto' : 'explicit_only'); setMaxItems(typeof row.max_items === 'number' ? row.max_items : 200); setVectorRecall(row.vector_recall !== false); setRetrievalConditioning(row.retrieval_conditioning !== false);
  }, [initialConfig]);

  async function save(next: { enabled?: boolean; writeMode?: string; maxItems?: number; vectorRecall?: boolean; retrievalConditioning?: boolean }) {
    const values = { enabled: next.enabled ?? enabled, writeMode: next.writeMode ?? writeMode, maxItems: next.maxItems ?? maxItems, vectorRecall: next.vectorRecall ?? vectorRecall, retrievalConditioning: next.retrievalConditioning ?? retrievalConditioning };
    setBusy(true); setError(null); setNotice(null);
    try { await client.settings.memory.workspace.update(memoryWorkspacePatch(values.enabled, values.writeMode, values.maxItems, values.vectorRecall, values.retrievalConditioning)); setEnabled(values.enabled); setWriteMode(values.writeMode); setMaxItems(values.maxItems); setVectorRecall(values.vectorRecall); setRetrievalConditioning(values.retrievalConditioning); setNotice('Workspace memory settings saved.'); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to save workspace memory settings.'); }
    finally { setBusy(false); }
  }

  return <Card><h3>Workspace memory</h3><p className="wk-muted">Tenant administrators control whether memory can be used. Changes are sent as the supported configuration fields only.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<label><input type="checkbox" checked={enabled} disabled={busy} onChange={(event) => void save({ enabled: event.target.checked })} /> Enable workspace memory</label><label>Write mode<select value={writeMode} disabled={busy || !enabled} onChange={(event) => void save({ writeMode: event.target.value })}><option value="explicit_only">Explicit only</option><option value="auto">Automatic extraction</option></select></label><label>Maximum items<input type="number" min={10} max={2000} step={10} value={maxItems} disabled={busy || !enabled} onChange={(event) => setMaxItems(Number(event.target.value))} onBlur={() => void save({})} /></label><label><input type="checkbox" checked={vectorRecall} disabled={busy || !enabled} onChange={(event) => void save({ vectorRecall: event.target.checked })} /> Vector recall</label><label><input type="checkbox" checked={retrievalConditioning} disabled={busy || !enabled} onChange={(event) => void save({ retrievalConditioning: event.target.checked })} /> Retrieval conditioning</label></Card>;
}

export function PersonalMemorySettingsPanel({ client, initialSettings }: { client: WeKnoraClient; initialSettings: unknown }) {
  const row = initialSettings !== null && typeof initialSettings === 'object' && !Array.isArray(initialSettings) ? initialSettings as MemoryRow : {};
  const [enabled, setEnabled] = useState(row.enabled === true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => { setEnabled(row.enabled === true); }, [initialSettings]);

  async function updateEnabled(next: boolean) {
    setBusy(true); setError(null); setNotice(null);
    try { const updated = await client.settings.memory.personal.updateEnabled(next); setEnabled(updated.enabled === true); setNotice(next ? 'Personal memory enabled.' : 'Personal memory disabled.'); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to update personal memory state.'); }
    finally { setBusy(false); }
  }

  return <Card><h3>Memory availability</h3><p className="wk-muted">This toggle is persisted by the server for the current user. It does not silently enable workspace memory.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<label><input type="checkbox" checked={enabled} disabled={busy} onChange={(event) => void updateEnabled(event.target.checked)} /> Enable personal memory</label><dl className="wk-settings-values"><div><dt>enabled</dt><dd>{String(enabled)}</dd></div></dl></Card>;
}

export function PersonalMemoryPanel({ client, initialItems }: { client: WeKnoraClient; initialItems: unknown }) {
  const [items, setItems] = useState<MemoryRow[]>(Array.isArray(initialItems) ? initialItems.filter((item): item is MemoryRow => item !== null && typeof item === 'object' && !Array.isArray(item)) : []);
  const [draft, setDraft] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingContent, setEditingContent] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    setItems(Array.isArray(initialItems) ? initialItems.filter((item): item is MemoryRow => item !== null && typeof item === 'object' && !Array.isArray(item)) : []);
  }, [initialItems]);

  async function createItem(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(null); setNotice(null);
    try {
      const created = await client.settings.memory.personal.items.create(memoryItemPatch(draft));
      setItems((current) => [...current, created]); setDraft(''); setNotice('Personal memory saved.');
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to save personal memory.'); }
    finally { setBusy(false); }
  }

  async function saveEdit(id: string) {
    setBusy(true); setError(null); setNotice(null);
    try {
      const updated = await client.settings.memory.personal.items.update(id, memoryItemPatch(editingContent));
      setItems((current) => current.map((item) => rowId(item) === id ? updated : item)); setEditingId(null); setEditingContent(''); setNotice('Personal memory updated.');
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to update personal memory.'); }
    finally { setBusy(false); }
  }

  async function removeItem(id: string) {
    if (!id || !window.confirm('Delete this personal memory?')) return;
    setBusy(true); setError(null); setNotice(null);
    try { await client.settings.memory.personal.items.remove(id); setItems((current) => current.filter((item) => rowId(item) !== id)); setNotice('Personal memory deleted.'); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to delete personal memory.'); }
    finally { setBusy(false); }
  }

  return <div className="wk-settings-memory"><Card><h3>Personal memory</h3><p className="wk-muted">Create, edit, or delete only the memories owned by the current user. Server responses remain authoritative.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<form className="wk-settings-editor" onSubmit={(event) => void createItem(event)}><label>New memory<textarea required rows={3} value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="A fact you want WeKnora to remember" /></label><Button type="submit" loading={busy}>Save memory</Button></form></Card><Card><h3>Saved items</h3>{items.length === 0 ? <Status>No personal memories returned.</Status> : <ul className="wk-list">{items.map((item, index) => { const id = rowId(item); return <li key={id || index}><div className="wk-list-item-copy">{editingId === id ? <textarea rows={3} value={editingContent} onChange={(event) => setEditingContent(event.target.value)} /> : <strong>{rowContent(item) || 'Untitled memory'}</strong>}<span>{String(item.status ?? 'active')} · {String(item.kind ?? 'personal')}</span></div><div className="wk-list-actions">{editingId === id ? <><Button type="button" loading={busy} onClick={() => void saveEdit(id)}>Save</Button><Button type="button" disabled={busy} onClick={() => { setEditingId(null); setEditingContent(''); }}>Cancel</Button></> : <><Button type="button" disabled={!id || busy} onClick={() => { setEditingId(id); setEditingContent(rowContent(item)); }}>Edit</Button><Button type="button" disabled={!id || busy} onClick={() => void removeItem(id)}>Delete</Button></>}</div></li>; })}</ul>}</Card></div>;
}
