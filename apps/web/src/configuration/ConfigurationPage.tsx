import { useEffect, useState } from 'react';
import type { AgentConfiguration, ConfigurationRecord, McpConfiguration, ModelConfiguration, SkillConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { configurationSections, configurationStatus, type ConfigurationSectionKey } from './surface.ts';
import { ConfigurationEditor } from './ConfigurationEditor.tsx';
import { AgentOperations, ModelDebugPanel, SkillOperations } from './ConfigurationOperations.tsx';

type Records = { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] };
type EditableSection = Exclude<ConfigurationSectionKey, 'skills'>;

function message(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function values(record: Record<string, unknown>): string {
  return Object.entries(record).filter(([key]) => !['id', 'name', 'config', 'parameters', 'auth_config', 'credentials'].includes(key))
    .map(([key, value]) => `${key}: ${typeof value === 'string' ? value : JSON.stringify(value)}`).join(' · ');
}

export function ConfigurationPage({ client }: { client: WeKnoraClient }) {
  const [records, setRecords] = useState<Records>({ agents: [], models: [], mcp: [], skills: [] });
  const [available, setAvailable] = useState<boolean | null>(null);
  const [errors, setErrors] = useState<Partial<Record<ConfigurationSectionKey, string>>>({});
  const [loading, setLoading] = useState(true);
  const [editor, setEditor] = useState<{ section: EditableSection; record?: ConfigurationRecord } | null>(null);
  const [creator, setCreator] = useState<'all' | 'mine' | 'others'>('all');

  async function load() {
    setLoading(true); setErrors({}); setAvailable(null);
    const results = await Promise.allSettled([
      client.configuration.agents.listWithState({ creator }), client.configuration.models.list(),
      client.configuration.mcp.list(), client.configuration.skills.listWithAvailability(),
    ]);
    const next: Records = { agents: [], models: [], mcp: [], skills: [] };
    const nextErrors: Partial<Record<ConfigurationSectionKey, string>> = {};
    const agent = results[0];
    if (agent.status === 'fulfilled') next.agents = agent.value.items.map((item) => ({ ...item, disabled_by_server: agent.value.disabledOwnAgentIds.includes(item.id) }));
    else nextErrors.agents = message(agent.reason, 'Unable to load agents');
    const model = results[1]; if (model.status === 'fulfilled') next.models = model.value; else nextErrors.models = message(model.reason, 'Unable to load models');
    const mcp = results[2]; if (mcp.status === 'fulfilled') next.mcp = mcp.value; else nextErrors.mcp = message(mcp.reason, 'Unable to load MCP services');
    const skills = results[3];
    if (skills.status === 'fulfilled') { next.skills = skills.value.items; setAvailable(skills.value.skillsAvailable); }
    else nextErrors.skills = message(skills.reason, 'Unable to load skills');
    setRecords(next); setErrors(nextErrors); setLoading(false);
  }

  useEffect(() => { void load(); }, [client, creator]);

  function openEditor(section: ConfigurationSectionKey, record?: ConfigurationRecord) {
    if (section !== 'skills') setEditor({ section, ...(record ? { record } : {}) });
  }

  async function removeConfiguration(section: ConfigurationSectionKey, itemId: string) {
    if (section === 'skills') return;
    if (!window.confirm(`Remove this ${section} configuration?`)) return;
    setErrors({});
    try { await client.configuration[section].remove(itemId); await load(); }
    catch (cause) { setErrors({ [section]: message(cause, `Unable to remove ${section}`) }); }
  }

  return <main className="wk-page wk-configuration-page">
    <header className="wk-header"><div><p className="wk-eyebrow">Platform configuration</p><h1>Agents, models, MCP and skills</h1><p className="wk-muted">Manage supported configuration through typed APIs. Secrets are write-only, and configuration presence never proves provider health.</p></div><Button type="button" onClick={() => void load()} disabled={loading}>Reload</Button></header>
    {editor ? <ConfigurationEditor client={client} section={editor.section} record={editor.record} onSaved={() => { setEditor(null); void load(); }} onCancel={() => setEditor(null)} /> : null}
    <div className="wk-configuration-operations"><label>Agent source<select value={creator} onChange={(event) => setCreator(event.target.value as typeof creator)}><option value="all">All agents</option><option value="mine">My agents</option><option value="others">Shared agents</option></select></label></div>
    <AgentOperations client={client} agents={records.agents} disabledIds={records.agents.filter((item) => (item as Record<string, unknown>).disabled_by_server === true).map((item) => item.id)} />
    <ModelDebugPanel client={client} models={records.models} />
    <SkillOperations client={client} />
    <div className="wk-configuration-grid">{configurationSections.map((section) => { const items = records[section.key]; return <Card key={section.key} className="wk-configuration-card">
      <div className="wk-configuration-card-heading"><div><h2>{section.title}</h2><p className="wk-muted">{section.description}</p></div><div className="wk-list-actions"><span className="wk-role-badge">{section.writeSupport}</span>{section.writeSupport === 'supported' ? <Button type="button" onClick={() => openEditor(section.key)}>Add</Button> : null}</div></div>
      {section.key === 'skills' && !errors.skills && available === false ? <Status tone="warning">Sandbox-installed skills are unavailable for the current sandbox selection. The skill catalog remains available.</Status> : null}
      {errors[section.key] ? <Status tone="error">{errors[section.key]}</Status> : loading ? <Status>Loading…</Status> : items.length === 0 ? <Status>No configured entries.</Status> : <ul className="wk-list">{items.map((item, index) => { const row = item as Record<string, unknown>; const status = configurationStatus(row); const disabled = section.key === 'agents' && row.disabled_by_server === true; return <li key={String(row.id ?? row.name ?? index)}><div className="wk-list-item-copy"><strong>{String(row.name ?? row.id ?? 'Unnamed')}</strong><span>{status}{disabled ? ' · disabled by server' : ''}</span><small>{values(row) || (section.key === 'skills' ? String(row.description ?? 'Catalog entry') : 'Configuration is present; use Test/health operations for provider state.')}</small></div>{section.writeSupport === 'supported' ? <div className="wk-list-actions"><Button type="button" onClick={() => openEditor(section.key, item)}>Edit</Button>{section.key === 'skills' ? null : <Button type="button" onClick={() => void removeConfiguration(section.key, String(row.id ?? ''))}>Remove</Button>}</div> : null}</li>; })}</ul>}
    </Card>; })}</div>
  </main>;
}
