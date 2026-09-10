import { useEffect, useState } from 'react';
import type { DataSource, DataSourceConnectorType, DataSourceSyncLog, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { buildDataSourceInput, dataSourceFormFrom, type DataSourceFormValues } from './form.ts';

const newForm: DataSourceFormValues = { name: '', type: '', schedule: '0 0 */6 * * *', mode: 'incremental', conflict: 'overwrite', deletions: true, credentialsText: '', settingsText: '' };

function connectionError(value: unknown): string | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (row.success === false) return typeof row.message === 'string' ? row.message : typeof row.error === 'string' ? row.error : 'Connection test failed';
  return null;
}

export function DataSourcesPage({ client, knowledgeBaseId }: { client: WeKnoraClient; knowledgeBaseId: string }) {
  const [sources, setSources] = useState<DataSource[]>([]);
  const [types, setTypes] = useState<DataSourceConnectorType[]>([]);
  const [logs, setLogs] = useState<DataSourceSyncLog[]>([]);
  const [logsSource, setLogsSource] = useState<DataSource | null>(null);
  const [editing, setEditing] = useState<DataSource | null | undefined>(undefined);
  const [form, setForm] = useState<DataSourceFormValues>(newForm);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [action, setAction] = useState<string | null>(null);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const dataSources = client.dataSources;

  async function load() {
    setLoading(true);
    try { setSources(await dataSources.list(knowledgeBaseId)); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to load data sources' }); }
    finally { setLoading(false); }
  }
  useEffect(() => {
    void load();
    void dataSources.types().then((items) => setTypes(items.sort((left, right) => left.priority - right.priority))).catch((error) => setMessage({ tone: 'warning', text: error instanceof Error ? `Connector types unavailable: ${error.message}` : 'Connector types unavailable' }));
  }, [client, knowledgeBaseId]);

  function openCreate() { setEditing(null); setForm({ ...newForm, type: types[0]?.type ?? '' }); setMessage(null); }
  function openEdit(source: DataSource) { setEditing(source); setForm(dataSourceFormFrom(source)); setMessage(null); }
  function updateForm<K extends keyof DataSourceFormValues>(key: K, value: DataSourceFormValues[K]) { setForm((current) => ({ ...current, [key]: value })); }

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setMessage(null);
    try {
      const input = buildDataSourceInput(form);
      const config = input.config as Record<string, unknown>;
      const credentials = config.credentials as Record<string, unknown>;
      if (form.credentialsText.trim()) {
        const result = await dataSources.validateCredentials(form.type, credentials);
        const failure = connectionError(result); if (failure) throw new Error(failure);
      }
      if (editing && !form.credentialsText.trim()) input.config = { ...config, credentials: undefined };
      const saved = editing ? await dataSources.update(editing.id, input) : await dataSources.create({ ...input, knowledge_base_id: knowledgeBaseId });
      if (form.credentialsText.trim()) {
        if (editing) await dataSources.putCredentials(saved.id, credentials);
      }
      setEditing(undefined); setMessage({ tone: 'success', text: 'Data source saved.' }); await load();
    } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to save data source' }); }
    finally { setSaving(false); }
  }
  async function remove(source: DataSource) { if (!window.confirm(`Delete data source ${source.name}?`)) return; setAction(source.id); try { await dataSources.remove(source.id); await load(); setMessage({ tone: 'success', text: 'Data source deleted.' }); } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to delete data source' }); } finally { setAction(null); } }
  async function run(source: DataSource, operation: 'sync' | 'pause' | 'resume') { setAction(`${operation}:${source.id}`); try { const result = operation === 'sync' ? await dataSources.sync(source.id) : operation === 'pause' ? await dataSources.pause(source.id) : await dataSources.resume(source.id); if (connectionError(result)) throw new Error(connectionError(result)!); await load(); setMessage({ tone: 'success', text: operation === 'sync' ? 'Sync requested; completion is tracked in logs.' : `Data source ${operation}d.` }); } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : `Unable to ${operation} data source` }); } finally { setAction(null); } }
  async function showLogs(source: DataSource) { setLogsSource(source); setLogs([]); try { setLogs(await dataSources.logs(source.id)); } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to load sync logs' }); } }

  return <main className="wk-page wk-data-sources-page"><header className="wk-header"><div><p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p><h1>Data sources</h1><p className="wk-muted">Configure connectors, test credentials, and track synchronization separately.</p></div><Button type="button" onClick={openCreate}>Add data source</Button></header>
    <Card>{message ? <Status tone={message.tone}>{message.text}</Status> : null}{loading ? <Status>Loading data sources…</Status> : sources.length === 0 ? <Status>No data sources configured.</Status> : <ul className="wk-list">{sources.map((source) => <li key={source.id} className="wk-data-source-item"><div className="wk-list-item-copy"><strong>{source.name}</strong><span>{source.type} · {source.sync_mode ?? 'incremental'} · {source.status ?? 'unknown'}</span><small>{source.last_sync_at ? `Last sync ${new Date(source.last_sync_at).toLocaleString()}` : 'Never synchronized'}{source.error_message ? ` · ${source.error_message}` : ''}</small></div><div className="wk-list-actions"><Button type="button" onClick={() => openEdit(source)}>Edit</Button><Button type="button" disabled={action !== null} onClick={() => void run(source, 'sync')}>Sync now</Button>{source.status === 'paused' ? <Button type="button" disabled={action !== null} onClick={() => void run(source, 'resume')}>Resume</Button> : <Button type="button" disabled={action !== null} onClick={() => void run(source, 'pause')}>Pause</Button>}<Button type="button" onClick={() => void showLogs(source)}>Logs</Button><Button type="button" disabled={action !== null} onClick={() => void remove(source)}>Delete</Button></div></li>)}</ul>}</Card>
    {logsSource ? <Card className="wk-data-source-logs"><div className="wk-header"><div><h2>Sync logs · {logsSource.name}</h2><p className="wk-muted">A successful connection test does not imply synchronization completed.</p></div><Button type="button" onClick={() => setLogsSource(null)}>Close</Button></div>{logs.length === 0 ? <Status>No sync logs returned.</Status> : <ul className="wk-list">{logs.map((log) => <li key={log.id}><span>{log.status}</span><small>{typeof log.started_at === 'string' ? log.started_at : ''}{typeof log.error_message === 'string' ? ` · ${log.error_message}` : ''}</small></li>)}</ul>}</Card> : null}
    {editing !== undefined ? <Card className="wk-data-source-editor"><div className="wk-header"><div><h2>{editing ? 'Edit data source' : 'Add data source'}</h2><p className="wk-muted">Credentials are only sent when you explicitly enter them.</p></div><Button type="button" onClick={() => setEditing(undefined)}>Close</Button></div><form className="wk-wiki-editor" onSubmit={save}><label>Name <input required value={form.name} onChange={(event) => updateForm('name', event.target.value)} /></label><label>Connector type <select required value={form.type} onChange={(event) => updateForm('type', event.target.value)}>{types.map((type) => <option key={type.type} value={type.type}>{type.name} ({type.type})</option>)}</select></label><label>Credentials <textarea rows={4} value={form.credentialsText} onChange={(event) => updateForm('credentialsText', event.target.value)} placeholder="token = secret\nOne key=value per line" /></label><label>Connector settings <textarea rows={4} value={form.settingsText} onChange={(event) => updateForm('settingsText', event.target.value)} placeholder="workspace_id = example\nOne key=value per line" /></label><label>Schedule <input required value={form.schedule} onChange={(event) => updateForm('schedule', event.target.value)} /></label><label>Sync mode <select value={form.mode} onChange={(event) => updateForm('mode', event.target.value as DataSourceFormValues['mode'])}><option value="incremental">Incremental</option><option value="full">Full</option></select></label><label>Conflict strategy <select value={form.conflict} onChange={(event) => updateForm('conflict', event.target.value as DataSourceFormValues['conflict'])}><option value="overwrite">Overwrite</option><option value="skip">Skip</option></select></label><label><input type="checkbox" checked={form.deletions} onChange={(event) => updateForm('deletions', event.target.checked)} /> Synchronize deletions</label><Button type="submit" loading={saving}>Save data source</Button></form></Card> : null}
  </main>;
}
