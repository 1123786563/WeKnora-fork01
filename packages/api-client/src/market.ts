import type { ClientRequest } from './client.ts';
import type { SkillCatalogInstallResult } from './configuration.ts';
import type { ExpertCreatedAgent, InstantiateExpertInput, InstantiateResult } from './experts.ts';

/**
 * Skill market API (internal/handler/skill_market.go,
 * tenant_skill_market.go, tenant_expert_market.go; internal/router/
 * routes_skill_market.go, routes_tenant_skill_market.go,
 * routes_tenant_expert_market.go). Three surfaces:
 *
 * - the remote SkillHub market: skill search/rankings (Viewer+; a cached
 *   answer served after a failed refresh answers 200 with stale=true), the
 *   remote skill install (Admin+), the skillset (expert) index/detail
 *   (Viewer+) and the one-action skillset-to-agent install (Contributor+);
 * - the tenant-internal skill market: admins publish/unpublish workspace
 *   catalog skills, every member lists them (Viewer+) and installs them
 *   onto sandbox configs (Admin+, the catalog-install guard mirrored);
 * - the tenant-internal expert market: a member exports one of their agents
 *   as an immutable expert snapshot (OwnedAgentOrAdmin), every member lists
 *   (Viewer+) and instantiates it as their own agent (Contributor+).
 *
 * Field names mirror the backend DTOs exactly (SkillMarketResult /
 * SkillMarketListing / MarketSkillInstallResult / MarketSkillset /
 * MarketSkillsetDetail in internal/types/interfaces/skill_market.go,
 * PublishedSkillView / PublishedSkillEntry / TenantSkillInstallResult in
 * tenant_skill_market.go, PublishedExpertView / PublishedExpertEntry in
 * tenant_expert_market.go). Registry-sourced text (name/description/version,
 * the raw passthrough map) carries no non-empty guarantee, so only
 * string-ness is enforced; slug/id identity fields are enforced non-empty.
 * The two 202 install surfaces answer success=false with per-config errors
 * on partial failure — the catalog-install contract configuration.ts already
 * types — so their envelopes are parsed like configuration.ts install, not
 * like the strict successfulData readers.
 */
export interface MarketSkillSummary {
  slug: string;
  name: string;
  description: string;
  version: string;
  raw?: Record<string, string>;
}

export interface MarketListResult<T> {
  results: T[];
  stale: boolean;
}

export interface MarketSkillInstallResult {
  catalog_id: string;
  install_ids: string[];
  errors?: Record<string, string>;
}

export interface MarketSkillset {
  slug: string;
  name: string;
  description: string;
  skill_slugs: string[];
  installed: boolean;
}

export interface MarketSkillsetIndex {
  skillsets: MarketSkillset[];
  stale: boolean;
}

export interface MarketSkillsetDetail {
  slug: string;
  name: string;
  description: string;
  name_en?: string;
  description_en?: string;
  skill_slugs: string[];
  installed: boolean;
  stale: boolean;
}

/**
 * The skillset-install outcome is the expert instantiate result (experts.ts)
 * plus the materialized expert's manifest ID and content digest — the two
 * keys POST /experts/market/:slug/install adds on top (skill_market.go
 * InstallSkillset). The service always fills both on success.
 */
export interface MarketSkillsetInstallResult extends InstantiateResult {
  expert_id: string;
  snapshot_sha256: string;
}

export interface TenantPublishedSkill {
  catalog_id: string;
  name: string;
  description?: string;
  version?: string;
  publisher_name: string;
  installed: boolean;
}

export interface TenantPublishedSkillIndex {
  skills: TenantPublishedSkill[];
}

/** The publish row POST /skills/catalog/:id/publish answers (PublishedSkillView). */
export interface PublishedSkillView {
  catalog_id: string;
  published_by: string;
  published_at: string;
  updated_at: string;
}

export interface TenantPublishedExpert {
  id: string;
  name: string;
  description?: string;
  publisher_name: string;
  installed: boolean;
  created_at: string;
}

export interface TenantPublishedExpertIndex {
  experts: TenantPublishedExpert[];
}

/** The publish row POST /agents/:id/publish-expert answers (PublishedExpertView). */
export interface PublishedExpertView {
  id: string;
  agent_id: string;
  name: string;
  description?: string;
  snapshot_sha256: string;
  published_by: string;
  published_at: string;
  updated_at: string;
}

export interface PublishExpertInput {
  name?: string;
  description?: string;
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
/**
 * The 202 install envelopes answer success=false when some sandbox configs
 * never started (per-config failures ride data.errors) — the catalog-install
 * contract — so success is checked for boolean-ness, not trueness.
 */
function acceptedData(value: unknown, path: string): { success: boolean; data: unknown } {
  const envelope = record(value, path);
  if (typeof envelope.success !== 'boolean') throw new Error(`${path}.success must be a boolean`);
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new Error(`${path}.data is required`);
  return { success: envelope.success, data: envelope.data };
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}
function stringArray(value: unknown, path: string): string[] {
  if (!Array.isArray(value)) throw new Error(`${path} must be an array`);
  return value.map((item, index) => text(item, `${path}[${index}]`));
}
function optionalString(row: RecordValue, field: string, path: string): string | undefined {
  if (row[field] === undefined) return undefined;
  return text(row[field], `${path}.${field}`);
}
function stringMap(value: unknown, path: string): Record<string, string> {
  if (value === null) return {};
  const row = record(value, path);
  return Object.fromEntries(Object.entries(row).map(([key, item]) => {
    if (typeof item !== 'string') throw new Error(`${path}.${key} must be a string`);
    return [key, item];
  }));
}
function parseActionEnvelope(value: unknown, path: string): void {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
}

// The registry's showcase kinds (skillhub/client.go rankingEndpoints); the
// backend answers anything else with a 400, so the kind is checked here
// first (the mbti dimension check precedent).
const rankingKinds = ['hot', 'featured', 'newest', 'recommended', 'trending', 'paid'] as const;

function parseSkillSummary(value: unknown, path: string): MarketSkillSummary {
  const row = record(value, path);
  return {
    slug: required(row.slug, `${path}.slug`),
    name: text(row.name, `${path}.name`),
    description: text(row.description, `${path}.description`),
    version: text(row.version, `${path}.version`),
    ...parseRawSummary(row, path),
  };
}

function parseRawSummary(row: RecordValue, path: string): { raw?: Record<string, string> } {
  // raw is the stringified registry entry (map[string]string, omitted when
  // empty server-side); present means every value is a string.
  if (row.raw === undefined) return {};
  return { raw: stringMap(row.raw, `${path}.raw`) };
}

function parseListing(value: unknown, path: string): MarketListResult<MarketSkillSummary> {
  const data = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(data.results)) throw new Error(`${path}.data.results must be an array`);
  return {
    results: data.results.map((item, index) => parseSkillSummary(item, `${path}.data.results[${index}]`)),
    stale: booleanValue(data.stale, `${path}.data.stale`),
  };
}

function parseSkillInstallResult(value: unknown, path: string): MarketSkillInstallResult {
  const envelope = acceptedData(value, path);
  const data = record(envelope.data, `${path}.data`);
  // catalog_id is "" only on the handler's degenerate nil-result defense;
  // install_ids is always an array (normalized server-side).
  const result: MarketSkillInstallResult = {
    catalog_id: text(data.catalog_id, `${path}.data.catalog_id`),
    install_ids: stringArray(data.install_ids, `${path}.data.install_ids`),
  };
  if (data.errors !== undefined) result.errors = stringMap(data.errors, `${path}.data.errors`);
  if (!envelope.success && (!result.errors || Object.keys(result.errors).length === 0)) {
    throw new Error(`${path}.data.errors is required for a partial result`);
  }
  return result;
}

function parseSkillset(value: unknown, path: string): MarketSkillset {
  const row = record(value, path);
  return {
    slug: required(row.slug, `${path}.slug`),
    name: text(row.name, `${path}.name`),
    description: text(row.description, `${path}.description`),
    skill_slugs: stringArray(row.skill_slugs, `${path}.skill_slugs`),
    installed: booleanValue(row.installed, `${path}.installed`),
  };
}

function parseSkillsetIndex(value: unknown, path: string): MarketSkillsetIndex {
  const data = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(data.skillsets)) throw new Error(`${path}.data.skillsets must be an array`);
  return {
    skillsets: data.skillsets.map((item, index) => parseSkillset(item, `${path}.data.skillsets[${index}]`)),
    stale: booleanValue(data.stale, `${path}.data.stale`),
  };
}

function parseSkillsetDetail(value: unknown, path: string): MarketSkillsetDetail {
  const data = record(successfulData(value, path), `${path}.data`);
  const detail: MarketSkillsetDetail = {
    slug: required(data.slug, `${path}.data.slug`),
    name: text(data.name, `${path}.data.name`),
    description: text(data.description, `${path}.data.description`),
    ...(data.name_en === undefined ? {} : { name_en: text(data.name_en, `${path}.data.name_en`) }),
    ...(data.description_en === undefined ? {} : { description_en: text(data.description_en, `${path}.data.description_en`) }),
    skill_slugs: stringArray(data.skill_slugs, `${path}.data.skill_slugs`),
    installed: booleanValue(data.installed, `${path}.data.installed`),
    stale: booleanValue(data.stale, `${path}.data.stale`),
  };
  return detail;
}

function parseCreatedAgent(value: unknown, path: string): ExpertCreatedAgent {
  // The lenient CustomAgent narrowing experts.ts applies to the instantiate
  // result's agent field (see experts.ts ExpertCreatedAgent).
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

function parseInstantiateData(data: RecordValue, path: string): InstantiateResult {
  return {
    agent: parseCreatedAgent(data.agent, `${path}.agent`),
    pending_skills: stringArray(data.pending_skills, `${path}.pending_skills`),
    skill_install_ids: stringArray(data.skill_install_ids, `${path}.skill_install_ids`),
  };
}

function parseInstantiateResult(value: unknown, path: string): InstantiateResult {
  const data = record(successfulData(value, path), `${path}.data`);
  return parseInstantiateData(data, `${path}.data`);
}

function parseSkillsetInstallResult(value: unknown, path: string): MarketSkillsetInstallResult {
  const data = record(successfulData(value, path), `${path}.data`);
  return {
    ...parseInstantiateData(data, `${path}.data`),
    // The service always fills both (skill_market_service.go
    // InstallMarketSkillset); the handler only adds them when non-empty.
    expert_id: required(data.expert_id, `${path}.data.expert_id`),
    snapshot_sha256: required(data.snapshot_sha256, `${path}.data.snapshot_sha256`),
  };
}

function parsePublishedSkillView(value: unknown, path: string): PublishedSkillView {
  const data = record(successfulData(value, path), `${path}.data`);
  return {
    catalog_id: required(data.catalog_id, `${path}.data.catalog_id`),
    // published_by is "" for a machine-principal (system) publish.
    published_by: text(data.published_by, `${path}.data.published_by`),
    published_at: text(data.published_at, `${path}.data.published_at`),
    updated_at: text(data.updated_at, `${path}.data.updated_at`),
  };
}

function parseTenantPublishedSkill(value: unknown, path: string): TenantPublishedSkill {
  const row = record(value, path);
  return {
    catalog_id: required(row.catalog_id, `${path}.catalog_id`),
    name: text(row.name, `${path}.name`),
    ...(row.description === undefined ? {} : { description: text(row.description, `${path}.description`) }),
    ...(row.version === undefined ? {} : { version: text(row.version, `${path}.version`) }),
    publisher_name: text(row.publisher_name, `${path}.publisher_name`),
    installed: booleanValue(row.installed, `${path}.installed`),
  };
}

function parseTenantSkills(value: unknown, path: string): TenantPublishedSkillIndex {
  const data = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(data.skills)) throw new Error(`${path}.data.skills must be an array`);
  return { skills: data.skills.map((item, index) => parseTenantPublishedSkill(item, `${path}.data.skills[${index}]`)) };
}

function parseTenantSkillInstallResult(value: unknown, path: string): SkillCatalogInstallResult {
  // The handler answers {installs, errors?} — the catalog-install shape
  // configuration.ts already types (TenantSkillInstallResult re-declares it
  // server-side for the same reason).
  const envelope = acceptedData(value, path);
  const data = record(envelope.data, `${path}.data`);
  const installs = stringMap(data.installs, `${path}.data.installs`);
  const errors = data.errors === undefined ? undefined : stringMap(data.errors, `${path}.data.errors`);
  if (!envelope.success && (!errors || Object.keys(errors).length === 0)) {
    throw new Error(`${path}.data.errors is required for a partial result`);
  }
  return { installs, ...(errors === undefined ? {} : { errors }) };
}

function parsePublishedExpertView(value: unknown, path: string): PublishedExpertView {
  const data = record(successfulData(value, path), `${path}.data`);
  return {
    id: required(data.id, `${path}.data.id`),
    agent_id: required(data.agent_id, `${path}.data.agent_id`),
    name: text(data.name, `${path}.data.name`),
    ...(data.description === undefined ? {} : { description: text(data.description, `${path}.data.description`) }),
    snapshot_sha256: text(data.snapshot_sha256, `${path}.data.snapshot_sha256`),
    published_by: text(data.published_by, `${path}.data.published_by`),
    published_at: text(data.published_at, `${path}.data.published_at`),
    updated_at: text(data.updated_at, `${path}.data.updated_at`),
  };
}

function parseTenantPublishedExpert(value: unknown, path: string): TenantPublishedExpert {
  const row = record(value, path);
  return {
    id: required(row.id, `${path}.id`),
    name: text(row.name, `${path}.name`),
    ...(row.description === undefined ? {} : { description: text(row.description, `${path}.description`) }),
    publisher_name: text(row.publisher_name, `${path}.publisher_name`),
    installed: booleanValue(row.installed, `${path}.installed`),
    created_at: text(row.created_at, `${path}.created_at`),
  };
}

function parseTenantExperts(value: unknown, path: string): TenantPublishedExpertIndex {
  const data = record(successfulData(value, path), `${path}.data`);
  if (!Array.isArray(data.experts)) throw new Error(`${path}.data.experts must be an array`);
  return { experts: data.experts.map((item, index) => parseTenantPublishedExpert(item, `${path}.data.experts[${index}]`)) };
}

function instantiateBody(input?: InstantiateExpertInput): Record<string, unknown> {
  if (input !== undefined) {
    if (input.agentName !== undefined && typeof input.agentName !== 'string') throw new Error('agentName must be a string');
    if (input.sandboxConfigId !== undefined && typeof input.sandboxConfigId !== 'string') throw new Error('sandboxConfigId must be a string');
  }
  return {
    ...(input?.agentName === undefined ? {} : { agent_name: input.agentName }),
    ...(input?.sandboxConfigId === undefined ? {} : { sandbox_config_id: input.sandboxConfigId }),
  };
}

function requireSandboxConfigIds(value: string[]): string[] {
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string' || item.trim() === '')) {
    throw new Error('sandboxConfigIds must be a string array');
  }
  return value;
}

export function createMarketApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async searchSkills(query: string, limit?: number, signal?: AbortSignal): Promise<MarketListResult<MarketSkillSummary>> {
      if (typeof query !== 'string') throw new Error('query must be a string');
      if (limit !== undefined && (!Number.isInteger(limit) || limit < 1)) throw new Error('limit must be a positive integer');
      const params = new URLSearchParams();
      if (query) params.set('q', query);
      if (limit !== undefined) params.set('limit', String(limit));
      const suffix = params.toString();
      return parseListing(await request({
        method: 'GET',
        path: `/api/v1/skills/market/search${suffix ? `?${suffix}` : ''}`,
        ...(signal === undefined ? {} : { signal }),
      }), '/skills/market/search');
    },
    async rankings(kind: string, signal?: AbortSignal): Promise<MarketListResult<MarketSkillSummary>> {
      if (!rankingKinds.includes(kind as (typeof rankingKinds)[number])) throw new Error('kind must be one of hot, featured, newest, recommended, trending, paid');
      return parseListing(await request({
        method: 'GET',
        path: `/api/v1/skills/market/rankings/${id(kind, 'kind')}`,
        ...(signal === undefined ? {} : { signal }),
      }), '/skills/market/rankings/:kind');
    },
    async installSkill(slug: string, sandboxConfigIds: string[], signal?: AbortSignal): Promise<MarketSkillInstallResult> {
      return parseSkillInstallResult(await request({
        method: 'POST',
        path: '/api/v1/skills/market/install',
        body: { slug: required(slug, 'slug'), sandbox_config_ids: requireSandboxConfigIds(sandboxConfigIds) },
        ...(signal === undefined ? {} : { signal }),
      }), '/skills/market/install');
    },
    async skillsets(signal?: AbortSignal): Promise<MarketSkillsetIndex> {
      return parseSkillsetIndex(await request({
        method: 'GET',
        path: '/api/v1/experts/market',
        ...(signal === undefined ? {} : { signal }),
      }), '/experts/market');
    },
    async skillset(slug: string, signal?: AbortSignal): Promise<MarketSkillsetDetail> {
      return parseSkillsetDetail(await request({
        method: 'GET',
        path: `/api/v1/experts/market/${id(slug, 'slug')}`,
        ...(signal === undefined ? {} : { signal }),
      }), '/experts/market/:slug');
    },
    async installSkillset(slug: string, input?: InstantiateExpertInput, signal?: AbortSignal): Promise<MarketSkillsetInstallResult> {
      return parseSkillsetInstallResult(await request({
        method: 'POST',
        path: `/api/v1/experts/market/${id(slug, 'slug')}/install`,
        body: instantiateBody(input),
        ...(signal === undefined ? {} : { signal }),
      }), '/experts/market/:slug/install');
    },
    async tenantSkills(signal?: AbortSignal): Promise<TenantPublishedSkillIndex> {
      return parseTenantSkills(await request({
        method: 'GET',
        path: '/api/v1/market/tenant/skills',
        ...(signal === undefined ? {} : { signal }),
      }), '/market/tenant/skills');
    },
    async publishSkill(catalogId: string, signal?: AbortSignal): Promise<PublishedSkillView> {
      return parsePublishedSkillView(await request({
        method: 'POST',
        path: `/api/v1/skills/catalog/${id(catalogId, 'catalogId')}/publish`,
        // The publish body must be empty or {} — send the explicit object.
        body: {},
        ...(signal === undefined ? {} : { signal }),
      }), '/skills/catalog/:id/publish');
    },
    async unpublishSkill(catalogId: string, signal?: AbortSignal): Promise<void> {
      parseActionEnvelope(await request({
        method: 'DELETE',
        path: `/api/v1/skills/catalog/${id(catalogId, 'catalogId')}/publish`,
        ...(signal === undefined ? {} : { signal }),
      }), '/skills/catalog/:id/publish DELETE');
    },
    async installTenantSkill(catalogId: string, sandboxConfigIds: string[], signal?: AbortSignal): Promise<SkillCatalogInstallResult> {
      return parseTenantSkillInstallResult(await request({
        method: 'POST',
        path: `/api/v1/market/tenant/skills/${id(catalogId, 'catalogId')}/install`,
        body: { sandbox_config_ids: requireSandboxConfigIds(sandboxConfigIds) },
        ...(signal === undefined ? {} : { signal }),
      }), '/market/tenant/skills/:catalogId/install');
    },
    async tenantExperts(signal?: AbortSignal): Promise<TenantPublishedExpertIndex> {
      return parseTenantExperts(await request({
        method: 'GET',
        path: '/api/v1/market/tenant/experts',
        ...(signal === undefined ? {} : { signal }),
      }), '/market/tenant/experts');
    },
    async publishAgentAsExpert(agentId: string, input?: PublishExpertInput, signal?: AbortSignal): Promise<PublishedExpertView> {
      if (input !== undefined) {
        if (input.name !== undefined && typeof input.name !== 'string') throw new Error('name must be a string');
        if (input.description !== undefined && typeof input.description !== 'string') throw new Error('description must be a string');
      }
      return parsePublishedExpertView(await request({
        method: 'POST',
        path: `/api/v1/agents/${id(agentId, 'agentId')}/publish-expert`,
        // The body is optional overrides only; {} decodes as "no overrides".
        body: {
          ...(input?.name === undefined ? {} : { name: input.name }),
          ...(input?.description === undefined ? {} : { description: input.description }),
        },
        ...(signal === undefined ? {} : { signal }),
      }), '/agents/:id/publish-expert');
    },
    async unpublishExpert(expertId: string, signal?: AbortSignal): Promise<void> {
      parseActionEnvelope(await request({
        method: 'DELETE',
        path: `/api/v1/market/tenant/experts/${id(expertId, 'expertId')}`,
        ...(signal === undefined ? {} : { signal }),
      }), '/market/tenant/experts/:id DELETE');
    },
    async installTenantExpert(expertId: string, input?: InstantiateExpertInput, signal?: AbortSignal): Promise<InstantiateResult> {
      return parseInstantiateResult(await request({
        method: 'POST',
        path: `/api/v1/market/tenant/experts/${id(expertId, 'expertId')}/install`,
        body: instantiateBody(input),
        ...(signal === undefined ? {} : { signal }),
      }), '/market/tenant/experts/:id/install');
    },
  };
}

export type MarketApi = ReturnType<typeof createMarketApi>;
