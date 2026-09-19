import type { ClientRequest } from './client.ts';

/**
 * Builtin sub-agent role catalog API (internal/handler/subagent.go,
 * internal/router/routes_subagent.go). Catalog list/detail are Viewer+ reads
 * over config/subagents/library plus the tenant's installed flags; the
 * agent-scoped list/install/remove ride the agentsRead/agentsWrite matrices
 * (list is owner-guarded like agent update). Responses ride the standard
 * success envelope and are unwrapped/validated here like every other module.
 *
 * Field names mirror the backend DTOs exactly (SubagentCatalogEntry /
 * SubagentCatalogDivision / SubagentCatalog / SubagentCatalogDetail in
 * internal/types/interfaces/subagent.go). An absent locale is an empty
 * string, never omitted (name/body pairs), and the scalar frontmatter fields
 * (emoji/color/vibe/tools_raw) carry no loader-level non-empty guarantee, so
 * only their string-ness is enforced; slug/division are identity fields the
 * scanner derives from the filename stem and division directory, enforced
 * non-empty.
 */
export interface SubagentCatalogEntry {
  slug: string;
  division: string;
  name_zh: string;
  name_en: string;
  emoji: string;
  color: string;
  installed: boolean;
}

export interface SubagentDivision {
  slug: string;
  label: string;
  icon: string;
  color: string;
  count: number;
}

export interface SubagentCatalog {
  divisions: SubagentDivision[];
  total: number;
  entries: SubagentCatalogEntry[];
}

export interface SubagentDetail {
  slug: string;
  division: string;
  name_zh: string;
  name_en: string;
  emoji: string;
  color: string;
  vibe: string;
  tools_raw: string;
  body_zh: string;
  body_en: string;
  installed: boolean;
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
function booleanValue(value: unknown, path: string): boolean {
  if (typeof value !== 'boolean') throw new Error(`${path} must be a boolean`);
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

function parseDivision(value: unknown, path: string): SubagentDivision {
  const row = record(value, path);
  return {
    slug: required(row.slug, `${path}.slug`),
    label: text(row.label, `${path}.label`),
    icon: text(row.icon, `${path}.icon`),
    color: text(row.color, `${path}.color`),
    count: numberValue(row.count, `${path}.count`),
  };
}

function parseCatalogEntry(value: unknown, path: string): SubagentCatalogEntry {
  const row = record(value, path);
  return {
    slug: required(row.slug, `${path}.slug`),
    division: required(row.division, `${path}.division`),
    name_zh: text(row.name_zh, `${path}.name_zh`),
    name_en: text(row.name_en, `${path}.name_en`),
    emoji: text(row.emoji, `${path}.emoji`),
    color: text(row.color, `${path}.color`),
    installed: booleanValue(row.installed, `${path}.installed`),
  };
}

function parseCatalog(value: unknown, path: string): SubagentCatalog {
  const data = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(data.divisions)) throw new Error(`${path}.data.divisions must be an array`);
  if (!Array.isArray(data.entries)) throw new Error(`${path}.data.entries must be an array`);
  return {
    divisions: data.divisions.map((item, index) => parseDivision(item, `${path}.data.divisions[${index}]`)),
    total: numberValue(data.total, `${path}.data.total`),
    entries: data.entries.map((item, index) => parseCatalogEntry(item, `${path}.data.entries[${index}]`)),
  };
}

function parseDetail(value: unknown, path: string): SubagentDetail {
  const row = record(successfulData(value, path), `${path}.data`);
  return {
    slug: required(row.slug, `${path}.data.slug`),
    division: required(row.division, `${path}.data.division`),
    name_zh: text(row.name_zh, `${path}.data.name_zh`),
    name_en: text(row.name_en, `${path}.data.name_en`),
    emoji: text(row.emoji, `${path}.data.emoji`),
    color: text(row.color, `${path}.data.color`),
    vibe: text(row.vibe, `${path}.data.vibe`),
    tools_raw: text(row.tools_raw, `${path}.data.tools_raw`),
    body_zh: text(row.body_zh, `${path}.data.body_zh`),
    body_en: text(row.body_en, `${path}.data.body_en`),
    installed: booleanValue(row.installed, `${path}.data.installed`),
  };
}

function parseSubagentList(value: unknown, path: string): string[] {
  const data = record(successfulData(value, path), `${path}.data`);
  return stringArray(data.subagents, `${path}.data.subagents`);
}

export function createSubagentsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async catalog(signal?: AbortSignal): Promise<SubagentCatalog> {
      return parseCatalog(await request({ method: 'GET', path: '/api/v1/subagent-catalog', ...(signal === undefined ? {} : { signal }) }), '/subagent-catalog');
    },
    async get(slug: string, signal?: AbortSignal): Promise<SubagentDetail> {
      return parseDetail(await request({ method: 'GET', path: `/api/v1/subagent-catalog/${id(slug, 'slug')}`, ...(signal === undefined ? {} : { signal }) }), '/subagent-catalog/:slug');
    },
    async listAgent(agentId: string, signal?: AbortSignal): Promise<string[]> {
      return parseSubagentList(await request({ method: 'GET', path: `/api/v1/agents/${id(agentId, 'agentId')}/subagents`, ...(signal === undefined ? {} : { signal }) }), '/agents/:id/subagents');
    },
    async install(agentId: string, slug: string, locale?: string, signal?: AbortSignal): Promise<string[]> {
      if (locale !== undefined && typeof locale !== 'string') throw new Error('locale must be a string');
      return parseSubagentList(await request({
        method: 'POST',
        path: `/api/v1/agents/${id(agentId, 'agentId')}/subagents`,
        body: { slug: required(slug, 'slug'), ...(locale === undefined ? {} : { locale }) },
        ...(signal === undefined ? {} : { signal }),
      }), '/agents/:id/subagents POST');
    },
    async remove(agentId: string, slug: string, signal?: AbortSignal): Promise<string[]> {
      return parseSubagentList(await request({ method: 'DELETE', path: `/api/v1/agents/${id(agentId, 'agentId')}/subagents/${id(slug, 'slug')}`, ...(signal === undefined ? {} : { signal }) }), '/agents/:id/subagents/:slug DELETE');
    },
  };
}

export type SubagentsApi = ReturnType<typeof createSubagentsApi>;
