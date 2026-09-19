/**
 * Vue-parity resolution for the composer chat-model chip (R016 slice).
 *
 * Data chain ported from the Vue chat input:
 * - frontend/src/utils/contextWindow.ts — context-window formatting
 *   (missing values fall back to the 200K backend default, tokens compact
 *   to 128K / 200K / 1M style suffixes);
 * - frontend/src/stores/chatResources.ts:59 — the chat dropdown lists
 *   GET /api/v1/models rows with type === 'KnowledgeQA' only;
 * - frontend/src/components/Input-field.vue:1067-1105 — the chip shows
 *   display_name || name of the selected model plus the formatted context
 *   window, and renders the localized input.notConfigured fallback when no
 *   model resolves.
 */

export const DEFAULT_MODEL_CONTEXT_WINDOW = 200000;

/** Tokens the backend will actually use: the model's value, or 200K. */
export function effectiveContextWindow(tokens?: unknown): number {
  if (typeof tokens === 'number' && tokens > 0) {
    return tokens;
  }
  return DEFAULT_MODEL_CONTEXT_WINDOW;
}

export function isDefaultContextWindow(tokens?: unknown): boolean {
  return !(typeof tokens === 'number' && tokens > 0);
}

/** Compact label: 128000 -> 128K, 200000 -> 200K, 1048576 -> 1M. */
export function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) {
    return '';
  }
  const n = Math.round(tokens);
  if (n >= 1_000_000 && n % 1_000_000 === 0) {
    return (n / 1_000_000) + 'M';
  }
  if (n >= 1000 && n % 1000 === 0) {
    return (n / 1000) + 'K';
  }
  if (n >= 1024 && n % 1024 === 0) {
    const k = n / 1024;
    if (k >= 1024 && k % 1024 === 0) {
      return (k / 1024) + 'M';
    }
    return k + 'K';
  }
  return String(n);
}

export function formatContextWindow(tokens?: unknown): string {
  return formatTokenCount(effectiveContextWindow(tokens));
}

/** Structural subset of the api-client ModelConfiguration the chip needs. */
export interface ChatModelLike {
  id?: unknown;
  name?: unknown;
  display_name?: unknown;
  type?: unknown;
  parameters?: unknown;
}

/** Vue chatResources.ts:59 — the chat dropdown lists KnowledgeQA models only. */
export function listChatModels<T extends ChatModelLike>(models: readonly T[]): T[] {
  return models.filter((model) => model.type === 'KnowledgeQA');
}

/** input.notConfigured, byte-exact from frontend/src/i18n/locales/*.ts input blocks. */
export const MODEL_CHIP_NOT_CONFIGURED: Record<string, string> = {
  'zh-CN': '未配置',
  'en-US': 'Not configured',
  'ja-JP': '未設定',
  'ko-KR': '구성되지 않음',
  'ru-RU': 'Не настроено',
};

export interface ChatModelChip {
  label: string;
  /** Formatted context suffix ('200K'); empty when no model resolves. */
  context: string;
  /** True when the suffix is the backend default (Vue model-selector-ctx is-default). */
  isDefaultContext: boolean;
}

/**
 * Resolve the chip content the way the Vue new-conversation view does.
 *
 * Selection priority mirrors the Vue chat input chain
 * (frontend/src/components/Input-field.vue):
 * 1. the user's persisted pick (ensureModelSelection's readLastChatModelID
 *    branch) — an explicit pick that differs from the agent binding also wins
 *    over it (agent-model watch keep-pick branch, lines 1001-1024);
 * 2. the selected agent's config.model_id (agent-model watch binding);
 * 3. the first available chat model (ensureModelSelection fallback).
 * A resolved id that is missing from the list renders 未配置 exactly like
 * the Vue find() miss (lines 1067-1078); the shared-agent
 * input.sharedAgentModelLabel variant has no React surface yet.
 */
export function resolveChatModelChip(options: {
  models: readonly ChatModelLike[];
  agentModelId?: unknown;
  /** The persisted in-session model pick (Vue readLastChatModelID). */
  selectedModelId?: unknown;
  notConfiguredLabel?: string;
}): ChatModelChip {
  const notConfiguredLabel = options.notConfiguredLabel ?? MODEL_CHIP_NOT_CONFIGURED['zh-CN'];
  const agentModelId = typeof options.agentModelId === 'string' ? options.agentModelId.trim() : '';
  const selectedModelId = typeof options.selectedModelId === 'string' ? options.selectedModelId.trim() : '';
  const findById = (id: string) => options.models.find((model) => model.id === id);
  let selected: ChatModelLike | undefined;
  if (agentModelId) {
    // Vue agent-model watch: a differing explicit user pick is kept; otherwise
    // the agent's model binds the conversation.
    selected = findById(selectedModelId && selectedModelId !== agentModelId ? selectedModelId : agentModelId);
  } else if (selectedModelId) {
    // The stored last pick seeds the chip only when it still resolves; a
    // missing id stays 未配置 (Vue find() miss).
    selected = findById(selectedModelId);
  } else {
    // Vue ensureModelSelection (Input-field.vue:982-994): with no agent
    // binding and no stored pick the chip falls back to the first available
    // chat model. Verified 2026-09-19 on a fresh origin (both sides had an
    // empty weknora_last_chat_model_id): Vue deterministically shows the
    // first model, so the earlier "prefetch race leaves 未配置" reading no
    // longer holds.
    selected = options.models[0];
  }
  if (!selected) {
    return { label: notConfiguredLabel, context: '', isDefaultContext: false };
  }
  const displayName = typeof selected.display_name === 'string' ? selected.display_name.trim() : '';
  const name = displayName || (typeof selected.name === 'string' ? selected.name.trim() : '') || notConfiguredLabel;
  const parameters = (selected.parameters ?? undefined) as Record<string, unknown> | undefined;
  const tokens = parameters?.context_window;
  return {
    label: name,
    context: formatContextWindow(tokens),
    isDefaultContext: isDefaultContextWindow(tokens),
  };
}
