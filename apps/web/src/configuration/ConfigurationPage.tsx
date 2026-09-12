import { useEffect, useState } from 'react';
import type { AgentConfiguration, ConfigurationRecord, McpConfiguration, ModelConfiguration, SkillConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { configurationSections, configurationStatus, type ConfigurationSectionKey } from './surface.ts';
import { ConfigurationEditor } from './ConfigurationEditor.tsx';
import { AgentOperations, ModelDebugPanel, SkillOperations } from './ConfigurationOperations.tsx';
import { modelInUseDetails, modelUsageBindingLabel, type ModelUsageDetails } from './model-usage.ts';
import { filterAgentsByQuery, groupAgents, type AgentGroupKey } from './agent-groups.ts';

type Records = { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] };
type EditableSection = Exclude<ConfigurationSectionKey, 'skills'>;

function message(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function values(record: Record<string, unknown>): string {
  return Object.entries(record).filter(([key]) => !['id', 'name', 'config', 'parameters', 'auth_config', 'credentials'].includes(key))
    .map(([key, value]) => `${key}: ${typeof value === 'string' ? value : JSON.stringify(value)}`).join(' · ');
}

function ModelUsageNotice({ modelName, details, onClose }: { modelName: string; details: ModelUsageDetails; onClose: () => void }) {
  const knowledgeBaseTotal = Math.max(details.knowledge_base_total, details.knowledge_bases.length);
  const agentTotal = Math.max(details.agent_total, details.agents.length);
  return <Card className="wk-configuration-usage" role="alert"><div className="wk-settings-panel-heading"><div><h2>Model is still in use</h2><p className="wk-muted">{modelName} cannot be deleted until its active bindings are removed.</p></div><Button type="button" onClick={onClose}>Close</Button></div>{knowledgeBaseTotal > 0 ? <section><h3>Knowledge bases ({knowledgeBaseTotal})</h3><ul className="wk-list">{details.knowledge_bases.map((item) => <li key={item.id}><strong>{item.name || item.id}</strong><small>{item.bindings.map(modelUsageBindingLabel).join(' · ')}</small></li>)}</ul>{knowledgeBaseTotal > details.knowledge_bases.length ? <p className="wk-muted">Showing {details.knowledge_bases.length} of {knowledgeBaseTotal} knowledge-base bindings.</p> : null}</section> : null}{agentTotal > 0 ? <section><h3>Agents ({agentTotal})</h3><ul className="wk-list">{details.agents.map((item) => <li key={item.id}><strong>{item.name || item.id}</strong><small>{item.bindings.map(modelUsageBindingLabel).join(' · ')}</small></li>)}</ul>{agentTotal > details.agents.length ? <p className="wk-muted">Showing {details.agents.length} of {agentTotal} agent bindings.</p> : null}</section> : null}{details.long_term_memory.bindings.length > 0 ? <section><h3>Long-term memory</h3><p>{details.long_term_memory.bindings.map(modelUsageBindingLabel).join(' · ')}</p></section> : null}</Card>;
}

export function ConfigurationPage({ client }: { client: WeKnoraClient }) {
  const [records, setRecords] = useState<Records>({ agents: [], models: [], mcp: [], skills: [] });
  const [available, setAvailable] = useState<boolean | null>(null);
  const [errors, setErrors] = useState<Partial<Record<ConfigurationSectionKey, string>>>({});
  const [loading, setLoading] = useState(true);
  const [editor, setEditor] = useState<{ section: EditableSection; record?: ConfigurationRecord } | null>(null);
  const [creator, setCreator] = useState<'all' | 'mine' | 'others'>('all');
  const [usageConflict, setUsageConflict] = useState<{ modelName: string; details: ModelUsageDetails } | null>(null);
  const [currentUserId, setCurrentUserId] = useState('');

  function renderConfigurationRow(sectionKey: ConfigurationSectionKey, item: ConfigurationRecord, index: number) {
    const row = item as Record<string, unknown>;
    const status = configurationStatus(row);
    const disabled = sectionKey === 'agents' && row.disabled_by_server === true;
    return <li key={String(row.id ?? row.name ?? index)}><div className="wk-list-item-copy"><strong>{String(row.name ?? row.id ?? 'Unnamed')}</strong><span>{status}{disabled ? ' · disabled by server' : ''}{row.is_builtin === true ? ' · builtin' : ''}</span><small>{values(row) || (sectionKey === 'skills' ? String(row.description ?? 'Catalog entry') : 'Configuration is present; use Test/health operations for provider state.')}</small></div>{configurationSections.find((section) => section.key === sectionKey)?.writeSupport === 'supported' ? <div className="wk-list-actions"><Button type="button" onClick={() => openEditor(sectionKey, item)}>Edit</Button>{sectionKey === 'skills' ? null : <Button type="button" onClick={() => void removeConfiguration(sectionKey, String(row.id ?? ''))}>Remove</Button>}</div> : null}</li>;
  }

  function renderAgentGroups() {
    const groups = groupAgents(records.agents, currentUserId);
    const groupLabels: Record<AgentGroupKey, string> = { builtin: 'Built-in', mine: 'Created by me', shared: 'Shared with me' };
    return <div className="wk-agent-groups"><label className="wk-agent-search">Search agents<input value={agentQuery} onChange={(event) => setAgentQuery(event.target.value)} placeholder="Filter by name or description" /></label>{(['builtin', 'mine', 'shared'] as AgentGroupKey[]).map((groupKey) => {
      const group = filterAgentsByQuery(groups[groupKey], agentQuery);
      if (group.length === 0) return null;
      const isCollapsed = collapsedGroups[groupKey];
      return <section key={groupKey} className="wk-agent-section"><button type="button" className="wk-agent-section-header" onClick={() => setCollapsedGroups((current) => ({ ...current, [groupKey]: !current[groupKey] }))}><strong>{groupLabels[groupKey]}</strong><span className="wk-agent-section-count">{group.length}</span><span aria-hidden="true">{isCollapsed ? '▸' : '▾'}</span></button>{isCollapsed ? null : <ul className="wk-list">{group.map((item, index) => renderConfigurationRow('agents', item, index))}</ul>}</section>;
    })}</div>;
  }
  const [agentQuery, setAgentQuery] = useState('');
  const [collapsedGroups, setCollapsedGroups] = useState<Record<AgentGroupKey, boolean>>({ builtin: false, mine: false, shared: false });

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
  useEffect(() => { void client.auth.me().then((me) => setCurrentUserId(String(me.user?.id ?? ''))).catch(() => setCurrentUserId('')); }, [client]);

  function openEditor(section: ConfigurationSectionKey, record?: ConfigurationRecord) {
    if (section !== 'skills') setEditor({ section, ...(record ? { record } : {}) });
  }

  async function removeConfiguration(section: ConfigurationSectionKey, itemId: string) {
    if (section === 'skills') return;
    if (!window.confirm(`Remove this ${section} configuration?`)) return;
    setErrors({}); setUsageConflict(null);
    try { await client.configuration[section].remove(itemId); await load(); }
    catch (cause) {
      const details = section === 'models' ? modelInUseDetails(cause) : null;
      if (details) {
        const model = records.models.find((item) => item.id === itemId);
        setUsageConflict({ modelName: model?.name ?? itemId, details });
        return;
      }
      setErrors({ [section]: message(cause, `Unable to remove ${section}`) });
    }
  }

  return <main className="wk-page wk-configuration-page">
    <header className="wk-header"><div><p className="wk-eyebrow">Platform configuration</p><h1>Agents, models, MCP and skills</h1><p className="wk-muted">Manage supported configuration through typed APIs. Secrets are write-only, and configuration presence never proves provider health.</p></div><Button type="button" onClick={() => void load()} disabled={loading}>Reload</Button></header>
    {editor ? <ConfigurationEditor client={client} section={editor.section} record={editor.record} onSaved={() => { setEditor(null); void load(); }} onCancel={() => setEditor(null)} /> : null}
    {usageConflict ? <ModelUsageNotice modelName={usageConflict.modelName} details={usageConflict.details} onClose={() => setUsageConflict(null)} /> : null}
    <div className="wk-configuration-operations"><label>Agent source<select value={creator} onChange={(event) => setCreator(event.target.value as typeof creator)}><option value="all">All agents</option><option value="mine">My agents</option><option value="others">Shared agents</option></select></label></div>
    <AgentOperations client={client} agents={records.agents} disabledIds={records.agents.filter((item) => (item as Record<string, unknown>).disabled_by_server === true).map((item) => item.id)} />
    <ModelDebugPanel client={client} models={records.models} />
    <SkillOperations client={client} />
    <div className="wk-configuration-grid">{configurationSections.map((section) => { const items = records[section.key]; return <Card key={section.key} className="wk-configuration-card">
      <div className="wk-configuration-card-heading"><div><h2>{section.title}</h2><p className="wk-muted">{section.description}</p></div><div className="wk-list-actions"><span className="wk-role-badge">{section.writeSupport}</span>{section.writeSupport === 'supported' ? <Button type="button" onClick={() => openEditor(section.key)}>Add</Button> : null}</div></div>
      {section.key === 'skills' && !errors.skills && available === false ? <Status tone="warning">Sandbox-installed skills are unavailable for the current sandbox selection. The skill catalog remains available.</Status> : null}
      {errors[section.key] ? <Status tone="error">{errors[section.key]}</Status> : loading ? <Status>Loading…</Status> : section.key === 'agents' && items.length > 0 ? renderAgentGroups() : items.length === 0 ? <Status>No configured entries.</Status> : <ul className="wk-list">{items.map((item, index) => renderConfigurationRow(section.key, item, index))}</ul>}
    </Card>; })}</div>
  </main>;
}
