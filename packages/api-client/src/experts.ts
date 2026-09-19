import type { ClientRequest } from './client.ts';

/**
 * Expert-template API (internal/handler/expert.go, internal/router/routes_expert.go).
 * List/detail are Viewer+ reads over the shipped config/experts catalog;
 * instantiate is Contributor+ and maps a template onto a tenant-owned custom
 * agent via ExpertService. Responses ride the standard success envelope and
 * are unwrapped/validated here like every other module.
 *
 * Field names mirror the backend DTOs exactly (expertSummaryDTO /
 * expertDetailDTO in expert.go): label/description and quick-prompt
 * title/description/prompt are already locale-resolved plain strings
 * server-side (service.ResolveExpertLocaleText at the request locale), so
 * they may legitimately be empty and only their string-ness is enforced.
 */
export interface ExpertSummary {
  id: string;
  label: string;
  description: string;
  icon_name: string;
  color: string;
  persona_mbti: string;
  quick_prompt_count: number;
  skills: string[];
}

export interface ExpertQuickPrompt {
  title: string;
  description: string;
  prompt: string;
  color: string;
  icon_name: string;
}

export interface ExpertDetail extends ExpertSummary {
  persona_markdown: string;
  quick_prompts: ExpertQuickPrompt[];
}

/**
 * The agent field of an instantiate response is the full CustomAgent JSON
 * (internal/types/custom_agent.go). The agents API (configuration.ts) types
 * agents as loosely-typed AgentConfiguration records (index signature), which
 * would force Task 7 to cast every field, so this module narrows to the minimal
 * shape the instantiate flow needs. Only id/name are validated (non-empty,
 * like every agents parse); description/avatar/config are read leniently —
 * the backend always serializes them, but this stays resilient to shape drift
 * instead of deep-validating the whole CustomAgent.
 */
export interface ExpertCreatedAgent {
  id: string;
  name: string;
  description: string;
  avatar: string;
  config: Record<string, unknown>;
}

export interface InstantiateResult {
  agent: ExpertCreatedAgent;
  pending_skills: string[];
  skill_install_ids: string[];
}

export interface InstantiateExpertInput {
  agentName?: string;
  sandboxConfigId?: string;
}

type RecordValue = Record<string, unknown>;
function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}
function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
function text(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new Error(`${path} must be a string`);
  return value;
}
function numberValue(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error(`${path} must be a finite number`);
  return value;
}
function successfulData(value: unknown, path: string): unknown {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new Error(`${path}.data is required`);
  return envelope.data;
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}
function stringArray(value: unknown, path: string): string[] {
  if (!Array.isArray(value)) throw new Error(`${path} must be an array`);
  return value.map((item, index) => text(item, `${path}[${index}]`));
}

function parseSummary(value: unknown, path: string): ExpertSummary {
  const row = record(value, path);
  return {
    id: required(row.id, `${path}.id`),
    label: text(row.label, `${path}.label`),
    description: text(row.description, `${path}.description`),
    icon_name: text(row.icon_name, `${path}.icon_name`),
    color: text(row.color, `${path}.color`),
    persona_mbti: text(row.persona_mbti, `${path}.persona_mbti`),
    quick_prompt_count: numberValue(row.quick_prompt_count, `${path}.quick_prompt_count`),
    skills: stringArray(row.skills, `${path}.skills`),
  };
}

function parseSummaries(value: unknown, path: string): ExpertSummary[] {
  const data = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(data.experts)) throw new Error(`${path}.data.experts must be an array`);
  return data.experts.map((item, index) => parseSummary(item, `${path}.data.experts[${index}]`));
}

function parseQuickPrompt(value: unknown, path: string): ExpertQuickPrompt {
  const row = record(value, path);
  return {
    title: text(row.title, `${path}.title`),
    description: text(row.description, `${path}.description`),
    prompt: text(row.prompt, `${path}.prompt`),
    color: text(row.color, `${path}.color`),
    icon_name: text(row.icon_name, `${path}.icon_name`),
  };
}

function parseDetail(value: unknown, path: string): ExpertDetail {
  const row = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(row.quick_prompts)) throw new Error(`${path}.data.quick_prompts must be an array`);
  return {
    ...parseSummary(row, `${path}.data`),
    persona_markdown: text(row.persona_markdown, `${path}.data.persona_markdown`),
    quick_prompts: row.quick_prompts.map((item, index) => parseQuickPrompt(item, `${path}.data.quick_prompts[${index}]`)),
  };
}

function parseCreatedAgent(value: unknown, path: string): ExpertCreatedAgent {
  const row = record(value, path);
  const config = row.config;
  return {
    id: required(row.id, `${path}.id`),
    name: required(row.name, `${path}.name`),
    description: typeof row.description === 'string' ? row.description : '',
    avatar: typeof row.avatar === 'string' ? row.avatar : '',
    config: config !== null && typeof config === 'object' && !Array.isArray(config) ? config as RecordValue : {},
  };
}

function parseInstantiateResult(value: unknown, path: string): InstantiateResult {
  const row = record(successfulData(value, path), `${path}.data`);
  return {
    agent: parseCreatedAgent(row.agent, `${path}.data.agent`),
    pending_skills: stringArray(row.pending_skills, `${path}.data.pending_skills`),
    skill_install_ids: stringArray(row.skill_install_ids, `${path}.data.skill_install_ids`),
  };
}

export function createExpertsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async list(signal?: AbortSignal): Promise<ExpertSummary[]> {
      return parseSummaries(await request({ method: 'GET', path: '/api/v1/experts', ...(signal === undefined ? {} : { signal }) }), '/experts');
    },
    async get(expertId: string, signal?: AbortSignal): Promise<ExpertDetail> {
      return parseDetail(await request({ method: 'GET', path: `/api/v1/experts/${id(expertId, 'expertId')}`, ...(signal === undefined ? {} : { signal }) }), '/experts/:id');
    },
    async instantiate(expertId: string, input?: InstantiateExpertInput, signal?: AbortSignal): Promise<InstantiateResult> {
      if (input !== undefined) {
        if (input.agentName !== undefined && typeof input.agentName !== 'string') throw new Error('agentName must be a string');
        if (input.sandboxConfigId !== undefined && typeof input.sandboxConfigId !== 'string') throw new Error('sandboxConfigId must be a string');
      }
      return parseInstantiateResult(await request({
        method: 'POST',
        path: `/api/v1/experts/${id(expertId, 'expertId')}/instantiate`,
        body: {
          ...(input?.agentName === undefined ? {} : { agent_name: input.agentName }),
          ...(input?.sandboxConfigId === undefined ? {} : { sandbox_config_id: input.sandboxConfigId }),
        },
        ...(signal === undefined ? {} : { signal }),
      }), '/experts/:id/instantiate');
    },
  };
}

export type ExpertsApi = ReturnType<typeof createExpertsApi>;
