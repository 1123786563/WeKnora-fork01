export type AgentEditorSection = 'basic' | 'model' | 'prompts' | 'tools' | 'knowledge' | 'suggestions' | 'personalization' | 'integration-im' | 'integration-embed';
const aliases: Record<string, AgentEditorSection> = { models: 'model', settings: 'basic', system: 'prompts', integrations: 'integration-im', im: 'integration-im', embed: 'integration-embed' };

export interface AgentRoute { editId: string | null; section: AgentEditorSection; highlight: string | null; sourceTenantId: string | null }
export function parseAgentRoute(path: string): AgentRoute | null {
  const url = new URL(path, 'http://localhost');
  if (url.pathname !== '/platform/agents') return null;
  const raw = url.searchParams.get('section') || 'basic';
  return { editId: url.searchParams.get('edit'), section: aliases[raw] ?? (['basic', 'model', 'prompts', 'tools', 'knowledge', 'suggestions', 'personalization', 'integration-im', 'integration-embed'].includes(raw) ? raw as AgentEditorSection : 'basic'), highlight: url.searchParams.get('highlight'), sourceTenantId: url.searchParams.get('sourceTenantId') };
}
export function buildAgentPath(id: string, section: AgentEditorSection = 'basic', sourceTenantId?: string): string {
  const params = new URLSearchParams({ edit: id, section });
  if (sourceTenantId) params.set('sourceTenantId', sourceTenantId);
  return `/platform/agents?${params}`;
}
export function sectionForField(field: string): AgentEditorSection { return field === 'summary_model' || field === 'rerank_model' ? 'model' : field === 'allowed_tools' ? 'tools' : 'basic'; }
