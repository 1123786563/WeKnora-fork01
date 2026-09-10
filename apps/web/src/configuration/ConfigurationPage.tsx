import { useEffect, useState } from 'react';
import type { AgentConfiguration, McpConfiguration, ModelConfiguration, SkillConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { configurationSections, configurationStatus, type ConfigurationSectionKey } from './surface.ts';

type Records = { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] };

function message(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function values(record: Record<string, unknown>): string { return Object.entries(record).filter(([key]) => !['id', 'name', 'config', 'parameters', 'auth_config'].includes(key)).map(([key, value]) => `${key}: ${typeof value === 'string' ? value : JSON.stringify(value)}`).join(' · '); }

export function ConfigurationPage({ client }: { client: WeKnoraClient }) {
  const [records, setRecords] = useState<Records>({ agents: [], models: [], mcp: [], skills: [] });
  const [available, setAvailable] = useState<boolean | null>(null);
  const [errors, setErrors] = useState<Partial<Record<ConfigurationSectionKey, string>>>({});
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true); setErrors({});
    const results = await Promise.allSettled([client.configuration.agents.listWithState(), client.configuration.models.list(), client.configuration.mcp.list(), client.configuration.skills.listWithAvailability()]);
    const next: Records = { agents: [], models: [], mcp: [], skills: [] };
    const nextErrors: Partial<Record<ConfigurationSectionKey, string>> = {};
    const agent = results[0]; if (agent.status === 'fulfilled') next.agents = agent.value.items.map((item) => ({ ...item, disabled_by_server: agent.value.disabledOwnAgentIds.includes(item.id) })); else nextErrors.agents = message(agent.reason, 'Unable to load agents');
    const model = results[1]; if (model.status === 'fulfilled') next.models = model.value; else nextErrors.models = message(model.reason, 'Unable to load models');
    const mcp = results[2]; if (mcp.status === 'fulfilled') next.mcp = mcp.value; else nextErrors.mcp = message(mcp.reason, 'Unable to load MCP services');
    const skills = results[3]; if (skills.status === 'fulfilled') { next.skills = skills.value.items; setAvailable(skills.value.skillsAvailable); } else nextErrors.skills = message(skills.reason, 'Unable to load skills');
    setRecords(next); setErrors(nextErrors); setLoading(false);
  }

  useEffect(() => { void load(); }, [client]);

  return <main className="wk-page wk-configuration-page"><header className="wk-header"><div><p className="wk-eyebrow">Platform configuration</p><h1>Agents, models, MCP and skills</h1><p className="wk-muted">Read-only inventory backed by the shared configuration API. Secrets are removed at the API boundary.</p></div><button type="button" className="wk-settings-tab" onClick={() => void load()} disabled={loading}>Reload</button></header><div className="wk-configuration-grid">{configurationSections.map((section) => { const items = records[section.key]; return <Card key={section.key} className="wk-configuration-card"><div className="wk-configuration-card-heading"><div><h2>{section.title}</h2><p className="wk-muted">{section.description}</p></div><span className="wk-role-badge">{section.writeSupport}</span></div>{errors[section.key] ? <Status tone="error">{errors[section.key]}</Status> : section.key === 'skills' && available === false ? <Status tone="warning">Skill catalog is unavailable in the current deployment.</Status> : loading ? <Status>Loading…</Status> : items.length === 0 ? <Status>No configured entries.</Status> : <ul className="wk-list">{items.map((item, index) => { const row = item as Record<string, unknown>; const status = configurationStatus(row); const disabled = section.key === 'agents' && row.disabled_by_server === true; return <li key={String(row.id ?? row.name ?? index)}><div className="wk-list-item-copy"><strong>{String(row.name ?? row.id ?? 'Unnamed')}</strong><span>{status}{disabled ? ' · disabled by server' : ''}</span><small>{values(row) || (section.key === 'skills' ? String(row.description ?? 'Catalog entry') : 'Configuration is present; health is verified by a separate operation.')}</small></div></li>; })}</ul>}</Card>; })}</div></main>;
}
