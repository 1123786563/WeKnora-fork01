import { useEffect, useRef, useState } from 'react';
import type { AgentConfiguration, ConfigurationRecord, McpConfiguration, ModelConfiguration, SkillConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Button, Input, Select } from 'tdesign-react';
import { Card, Status } from './ui.tsx';
import { configurationSections, configurationStatus, type ConfigurationSectionKey } from './surface.ts';
import { ConfigurationEditor } from './ConfigurationEditor.tsx';
import { AgentOperations, ModelDebugPanel, SkillOperations } from './ConfigurationOperations.tsx';
import { modelInUseDetails, type ModelUsageDetails } from './model-usage.ts';
import { ModelUsageNotice } from './ModelUsageNotice.tsx';
import { canManageAgent, filterAgentsByQuery, groupAgents, type AgentGroupKey } from './agent-groups.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import './config-u.css';

type Records = { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] };
type EditableSection = Exclude<ConfigurationSectionKey, 'skills'>;

function message(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function values(record: Record<string, unknown>): string {
  return Object.entries(record).filter(([key]) => !['id', 'name', 'config', 'parameters', 'auth_config', 'credentials'].includes(key))
    .map(([key, value]) => `${key}: ${typeof value === 'string' ? value : JSON.stringify(value)}`).join(' · ');
}

export function ConfigurationPage({ client }: { client: WeKnoraClient }) {
  const t = createTranslator(useAppLocale());
  const [records, setRecords] = useState<Records>({ agents: [], models: [], mcp: [], skills: [] });
  const [available, setAvailable] = useState<boolean | null>(null);
  const [errors, setErrors] = useState<Partial<Record<ConfigurationSectionKey, string>>>({});
  const [loading, setLoading] = useState(true);
  const [editor, setEditor] = useState<{ section: EditableSection; record?: ConfigurationRecord } | null>(null);
  const [creator, setCreator] = useState<'all' | 'mine' | 'others'>('all');
  const [usageConflict, setUsageConflict] = useState<{ modelName: string; details: ModelUsageDetails } | null>(null);
  const [currentUserId, setCurrentUserId] = useState('');
  const loadGeneration = useRef(0);

  function renderConfigurationRow(sectionKey: ConfigurationSectionKey, item: ConfigurationRecord, index: number) {
    const row = item as Record<string, unknown>;
    const status = configurationStatus(row);
    const disabled = sectionKey === 'agents' && row.disabled_by_server === true;
    const writable = sectionKey !== 'agents' || canManageAgent(row);
    return <li key={String(row.id ?? row.name ?? index)} className="wk-cfg-page-1"><div className="wk-list-item-copy wk-cfg-page-2"><strong>{String(row.name ?? row.id ?? 'Unnamed')}</strong><span className="wk-cfg-page-3">{status}{disabled ? ' · disabled by server' : ''}{row.is_builtin === true ? ' · builtin' : ''}</span><small>{values(row) || (sectionKey === 'skills' ? String(row.description ?? 'Catalog entry') : 'Configuration is present; use Test/health operations for provider state.')}</small></div>{configurationSections.find((section) => section.key === sectionKey)?.writeSupport === 'supported' && writable ? <div className="wk-list-actions wk-cfg-page-4"><Button type="button" theme="default" variant="outline" onClick={() => openEditor(sectionKey, item)}>Edit</Button>{sectionKey === 'skills' ? null : <Button type="button" theme="default" variant="outline" onClick={() => void removeConfiguration(sectionKey, String(row.id ?? ''))}>Remove</Button>}</div> : null}</li>;
  }

  function renderAgentGroups() {
    const groups = groupAgents(records.agents, currentUserId);
    const groupLabels: Record<AgentGroupKey, string> = { builtin: 'Built-in', mine: 'Created by me', shared: 'Shared with me' };
    return <div className="wk-agent-groups"><label className="wk-agent-search">Search agents<Input value={agentQuery} onChange={(value) => setAgentQuery(String(value))} placeholder="Filter by name or description" /></label>{(['builtin', 'mine', 'shared'] as AgentGroupKey[]).map((groupKey) => {
      const group = filterAgentsByQuery(groups[groupKey], agentQuery);
      if (group.length === 0) return null;
      const isCollapsed = collapsedGroups[groupKey];
      return <section key={groupKey} className="wk-agent-section"><button type="button" className="wk-agent-section-header" onClick={() => setCollapsedGroups((current) => ({ ...current, [groupKey]: !current[groupKey] }))}><strong>{groupLabels[groupKey]}</strong><span className="wk-agent-section-count">{group.length}</span><span aria-hidden="true">{isCollapsed ? '▸' : '▾'}</span></button>{isCollapsed ? null : <ul className="wk-list wk-cfg-page-5">{group.map((item, index) => renderConfigurationRow('agents', item, index))}</ul>}</section>;
    })}</div>;
  }
  const [agentQuery, setAgentQuery] = useState('');
  const [collapsedGroups, setCollapsedGroups] = useState<Record<AgentGroupKey, boolean>>({ builtin: false, mine: false, shared: false });

  async function load() {
    const generation = ++loadGeneration.current;
    setLoading(true); setErrors({}); setAvailable(null);
    const results = await Promise.allSettled([
      client.configuration.agents.listWithState({ creator }), client.configuration.models.list(),
      client.configuration.mcp.list(), client.configuration.skills.listWithAvailability(),
    ]);
    if (generation !== loadGeneration.current) return;
    const next: Records = { agents: [], models: [], mcp: [], skills: [] };
    const nextErrors: Partial<Record<ConfigurationSectionKey, string>> = {};
    const agent = results[0];
    if (agent.status === 'fulfilled') next.agents = agent.value.items.map((item) => ({ ...item, disabled_by_server: agent.value.disabledOwnAgentIds.includes(item.id) }));
    else nextErrors.agents = message(agent.reason, t('common.error'));
    const model = results[1]; if (model.status === 'fulfilled') next.models = model.value; else nextErrors.models = message(model.reason, t('common.error'));
    const mcp = results[2]; if (mcp.status === 'fulfilled') next.mcp = mcp.value; else nextErrors.mcp = message(mcp.reason, t('common.error'));
    const skills = results[3];
    if (skills.status === 'fulfilled') { next.skills = skills.value.items; setAvailable(skills.value.skillsAvailable); }
    else nextErrors.skills = message(skills.reason, t('common.error'));
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
      setErrors({ [section]: message(cause, t('common.error')) });
    }
  }

  return <main className="wk-page wk-cfg-page-6">
    <header className="wk-header wk-cfg-page-7"><div><p className="wk-eyebrow wk-cfg-page-8">Platform configuration</p><h1 className="wk-cfg-page-9">Agents, models, MCP and skills</h1><p className="wk-muted wk-cfg-page-10">Manage supported configuration through typed APIs. Secrets are write-only, and configuration presence never proves provider health.</p></div><Button type="button" theme="default" variant="outline" onClick={() => void load()} disabled={loading}>Reload</Button></header>
    {editor ? <ConfigurationEditor client={client} section={editor.section} record={editor.record} onSaved={() => { setEditor(null); void load(); }} onCancel={() => setEditor(null)} /> : null}
    {usageConflict ? <ModelUsageNotice modelName={usageConflict.modelName} details={usageConflict.details} onClose={() => setUsageConflict(null)} /> : null}
    <div className="wk-configuration-operations"><label>Agent source<Select value={creator} onChange={(value) => setCreator(value as typeof creator)}><Select.Option value="all" label="All agents">All agents</Select.Option><Select.Option value="mine" label="My agents">My agents</Select.Option><Select.Option value="others" label="Shared agents">Shared agents</Select.Option></Select></label></div>
    <AgentOperations client={client} agents={records.agents} disabledIds={records.agents.filter((item) => (item as Record<string, unknown>).disabled_by_server === true).map((item) => item.id)} />
    <ModelDebugPanel client={client} models={records.models} />
    <SkillOperations client={client} />
    <div className="wk-cfg-page-11">{configurationSections.map((section) => { const items = records[section.key]; return <Card key={section.key} className="wk-cfg-page-12">
      <div className="wk-cfg-page-13"><div><h2 className="wk-cfg-page-14">{section.title}</h2><p className="wk-muted wk-cfg-page-15">{section.description}</p></div><div className="wk-list-actions wk-cfg-page-4"><span className="wk-cfg-page-16">{section.writeSupport}</span>{section.writeSupport === 'supported' ? <Button type="button" theme="default" variant="outline" onClick={() => openEditor(section.key)}>Add</Button> : null}</div></div>
      {section.key === 'skills' && !errors.skills && available === false ? <Status tone="warning">Sandbox-installed skills are unavailable for the current sandbox selection. The skill catalog remains available.</Status> : null}
      {errors[section.key] ? <Status tone="error">{errors[section.key]}</Status> : loading ? <Status>Loading…</Status> : section.key === 'agents' && items.length > 0 ? renderAgentGroups() : items.length === 0 ? <Status>No configured entries.</Status> : <ul className="wk-list wk-cfg-page-5">{items.map((item, index) => renderConfigurationRow(section.key, item, index))}</ul>}
    </Card>; })}</div>
  </main>;
}
