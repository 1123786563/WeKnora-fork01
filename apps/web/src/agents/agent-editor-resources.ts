/**
 * R491 — runtime fetch of the agent editor catalogs, aligning the React editor
 * with the Vue baseline:
 *
 *   Vue (frontend/src/stores/editorResources.ts)          here
 *   ----------------------------------------------------  ---------------------------
 *   ensureAgentTypePresets  GET /api/v1/agents/type-presets
 *                                       -> client.configuration.agents.typePresets()
 *   ensurePromptTemplates   GET /api/v1/tenants/kv/prompt-templates
 *                                       -> client.settings.promptTemplates.get()
 *     (agent_system_prompt section backs the 使用模板 popup and the preset
 *      system_prompt_id body lookup)
 *   ensurePlaceholders      GET /api/v1/agents/placeholders
 *                                       -> client.configuration.agents.placeholders()
 *   CACHE_TTL_MS 60s + inflight de-dup (runOnce)          same, module-scope
 *
 * Divergence decision (documented in the R491 report): Vue keeps empty locals
 * when a fetch fails — the dropdown/hints simply render nothing. The React
 * client instead keeps the vendored static catalogs (agent-type-presets.ts /
 * agent-editor.ts PLACEHOLDER_DEFINITIONS) as the fallback so a failed or
 * empty fetch degrades to the previous R485/R486 behaviour instead of an
 * empty editor.
 */
import type { WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import {
  AGENT_TYPE_PRESETS,
  builtinAgentSystemPromptTemplates,
  type AgentPromptTemplateOption,
  type AgentTypePreset,
} from './agent-type-presets.ts';
import type { PromptPlaceholderDef } from './agent-editor.ts';

export type { AgentPromptTemplateOption } from './agent-type-presets.ts';

/** Placeholder catalog keyed by prompt field, mirroring the backend grouping. */
export type AgentPlaceholderCatalog = Record<string, PromptPlaceholderDef[]>;

/** Raw fetch outcome — null per catalog means "not available, use the fallback". */
export interface AgentEditorRuntimeData {
  typePresets: AgentTypePreset[] | null;
  promptTemplates: AgentPromptTemplateOption[] | null;
  placeholders: AgentPlaceholderCatalog | null;
}

/** Resolved editor resources: runtime data where fetched, static catalogs otherwise. */
export interface AgentEditorResources {
  typePresets: AgentTypePreset[];
  promptTemplates: AgentPromptTemplateOption[];
  placeholders: AgentPlaceholderCatalog | null;
}

const CACHE_TTL_MS = 60_000;

function toTypePresets(value: unknown): AgentTypePreset[] | null {
  const rows = (value as { data?: unknown } | null)?.data ?? value;
  if (!Array.isArray(rows) || rows.length === 0) return null;
  const presets: AgentTypePreset[] = [];
  for (const row of rows) {
    if (row === null || typeof row !== 'object') continue;
    const record = row as Record<string, unknown>;
    if (typeof record.id !== 'string' || record.id === '') continue;
    if (record.i18n === null || typeof record.i18n !== 'object') continue;
    presets.push(row as unknown as AgentTypePreset);
  }
  return presets.length > 0 ? presets : null;
}

function toPromptTemplates(value: unknown): AgentPromptTemplateOption[] | null {
  // client.settings.promptTemplates.get() returns the unwrapped KV record
  // ({agent_system_prompt: [...]}) while raw stubs may hand back the envelope.
  const root = (value as { data?: unknown } | null)?.data ?? value;
  if (root === null || typeof root !== 'object') return null;
  const rows = (root as Record<string, unknown>).agent_system_prompt;
  if (!Array.isArray(rows) || rows.length === 0) return null;
  const templates: AgentPromptTemplateOption[] = [];
  for (const row of rows) {
    if (row === null || typeof row !== 'object') continue;
    const record = row as Record<string, unknown>;
    if (typeof record.id !== 'string' || typeof record.content !== 'string') continue;
    templates.push({
      id: record.id,
      name: typeof record.name === 'string' ? record.name : record.id,
      description: typeof record.description === 'string' ? record.description : '',
      content: record.content,
      ...(record.default === true ? { default: true } : {}),
      ...(typeof record.mode === 'string' ? { mode: record.mode } : {}),
    });
  }
  return templates.length > 0 ? templates : null;
}

function toPlaceholders(value: unknown): AgentPlaceholderCatalog | null {
  const root = (value as { data?: unknown } | null)?.data ?? value;
  if (root === null || typeof root !== 'object' || Array.isArray(root)) return null;
  const catalog: AgentPlaceholderCatalog = {};
  let count = 0;
  for (const [field, rows] of Object.entries(root as Record<string, unknown>)) {
    if (!Array.isArray(rows) || rows.length === 0) continue;
    const defs: PromptPlaceholderDef[] = [];
    for (const row of rows) {
      if (row === null || typeof row !== 'object' || typeof (row as Record<string, unknown>).name !== 'string') continue;
      const record = row as Record<string, unknown>;
      defs.push({
        name: record.name as string,
        label: typeof record.label === 'string' ? record.label : (record.name as string),
        description: typeof record.description === 'string' ? record.description : '',
      });
    }
    if (defs.length > 0) {
      catalog[field] = defs;
      count += defs.length;
    }
  }
  return count > 0 ? catalog : null;
}

/** Static-catalog fallback: the vendored preset table plus the builtin agent_system_prompt list. */
export function fallbackAgentEditorResources(locale: Locale): AgentEditorResources {
  return {
    typePresets: AGENT_TYPE_PRESETS,
    promptTemplates: builtinAgentSystemPromptTemplates(locale),
    placeholders: null,
  };
}

/** Merge a raw fetch outcome over the static fallback (locale-aware for the fallback strings). */
export function resolveAgentEditorResources(runtime: AgentEditorRuntimeData | null, locale: Locale): AgentEditorResources {
  const fallback = fallbackAgentEditorResources(locale);
  if (!runtime) return fallback;
  return {
    typePresets: runtime.typePresets ?? fallback.typePresets,
    promptTemplates: runtime.promptTemplates ?? fallback.promptTemplates,
    placeholders: runtime.placeholders,
  };
}

let cachedAt = 0;
let cachedData: AgentEditorRuntimeData | null = null;
let inflight: Promise<AgentEditorRuntimeData> | null = null;

/** Test seam: drop the TTL cache and any in-flight join (also usable as an invalidation hook). */
export function resetAgentEditorResourcesCache(): void {
  cachedAt = 0;
  cachedData = null;
  inflight = null;
}

async function fetchRuntimeData(client: WeKnoraClient, signal?: AbortSignal): Promise<AgentEditorRuntimeData> {
  // The Promise.resolve wrapper also catches a synchronously missing client
  // method (older stubs) so one absent endpoint cannot take the load down —
  // the same guard the parser-engines dep uses.
  const [presets, templates, placeholders] = await Promise.all([
    Promise.resolve().then(() => client.configuration.agents.typePresets?.(signal)).then(toTypePresets).catch(() => null),
    Promise.resolve().then(() => client.settings.promptTemplates?.get?.(signal)).then(toPromptTemplates).catch(() => null),
    Promise.resolve().then(() => client.configuration.agents.placeholders?.(signal)).then(toPlaceholders).catch(() => null),
  ]);
  return { typePresets: presets, promptTemplates: templates, placeholders };
}

/**
 * Fetch the three editor catalogs with the Vue editorResources caching
 * semantics: a 60s TTL cache plus in-flight de-duplication, `force` bypasses
 * both. Never rejects — unavailable catalogs come back as null fields and the
 * caller resolves them over the static fallback.
 */
export async function loadAgentEditorResources(
  client: WeKnoraClient,
  options: { force?: boolean; signal?: AbortSignal } = {},
): Promise<AgentEditorRuntimeData> {
  if (!options.force && cachedData !== null && Date.now() - cachedAt < CACHE_TTL_MS) return cachedData;
  if (inflight) return inflight;
  const pending = fetchRuntimeData(client, options.signal)
    .then((data) => {
      cachedData = data;
      cachedAt = Date.now();
      return data;
    })
    .finally(() => { inflight = null; });
  inflight = pending;
  return pending;
}
