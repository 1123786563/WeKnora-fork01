export interface ModelUsageResource {
  id: string;
  name: string;
  bindings: string[];
}

export interface ModelUsageDetails {
  knowledge_bases: ModelUsageResource[];
  agents: ModelUsageResource[];
  long_term_memory: { bindings: string[] };
  knowledge_base_total: number;
  agent_total: number;
}

type RecordValue = Record<string, unknown>;

function record(value: unknown): RecordValue | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as RecordValue : null;
}

function stringList(value: unknown): string[] | null {
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string' || item.trim() === '')) return null;
  return value as string[];
}

function resources(value: unknown): ModelUsageResource[] | null {
  if (!Array.isArray(value)) return null;
  const parsed: ModelUsageResource[] = [];
  for (const item of value) {
    const row = record(item);
    const bindings = row ? stringList(row.bindings) : null;
    if (!row || typeof row.id !== 'string' || row.id.trim() === '' || typeof row.name !== 'string' || !bindings || bindings.length === 0) return null;
    parsed.push({ id: row.id, name: row.name, bindings });
  }
  return parsed;
}

function total(value: unknown, listed: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= listed ? Math.floor(value) : listed;
}

export function parseModelUsageDetails(value: unknown): ModelUsageDetails | null {
  const row = record(value);
  const memory = row ? record(row.long_term_memory) : null;
  const knowledgeBases = row ? resources(row.knowledge_bases) : null;
  const agents = row ? resources(row.agents) : null;
  const memoryBindings = memory ? stringList(memory.bindings) : null;
  if (!knowledgeBases || !agents || !memoryBindings) return null;
  const details: ModelUsageDetails = {
    knowledge_bases: knowledgeBases,
    agents,
    long_term_memory: { bindings: memoryBindings },
    knowledge_base_total: total(row?.knowledge_base_total, knowledgeBases.length),
    agent_total: total(row?.agent_total, agents.length),
  };
  return details.knowledge_base_total > 0 || details.agent_total > 0 || memoryBindings.length > 0 ? details : null;
}

export function modelInUseDetails(error: unknown): ModelUsageDetails | null {
  const row = record(error);
  const code = row?.code;
  if (code !== 2300 && code !== '2300' && code !== 'MODEL_IN_USE') return null;
  return parseModelUsageDetails(row?.details);
}

export function modelUsageBindingLabel(binding: string): string {
  return binding.replace(/_/g, ' ');
}
