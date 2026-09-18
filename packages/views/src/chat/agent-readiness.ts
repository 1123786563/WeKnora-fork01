/**
 * Agent chat-readiness gates, ported from upstream
 * frontend/src/utils/agent-readiness.ts. An agent is selectable only when it
 * explicitly references a usable chat model; smart-reasoning agents
 * additionally need a rerank model whenever knowledge_search can run. The
 * rules stay aligned with backend agentRequiresRerankModel.
 */

export type AgentNotReadyReasonKey = 'summary_model' | 'rerank_model';

export interface AgentReadinessConfig {
  model_id?: unknown;
  rerank_model_id?: unknown;
  kb_selection_mode?: unknown;
  allowed_tools?: unknown;
  agent_mode?: unknown;
}

export interface AgentReadinessModel {
  id: string;
  type?: string;
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

/**
 * An agent is chat-ready only when it explicitly references a usable chat
 * model. A tenant-wide default model must not hide an incomplete agent
 * configuration.
 */
export function agentHasConfiguredChatModel(config: AgentReadinessConfig | undefined, models: readonly AgentReadinessModel[]): boolean {
  const modelId = asString(config?.model_id);
  if (!modelId) return false;
  return models.some((model) => model.type === 'KnowledgeQA' && model.id === modelId);
}

/**
 * Rerank is needed only when knowledge_search can actually run. Explicitly
 * disabling the knowledge-base scope makes KB tools ineffective even if an
 * older configuration still lists knowledge_search in allowed_tools.
 */
export function agentRequiresRerankModel(config: AgentReadinessConfig | undefined): boolean {
  if (!config || config.kb_selection_mode === 'none') return false;
  const allowedTools = Array.isArray(config.allowed_tools) ? config.allowed_tools : [];
  // The backend falls back to DefaultAllowedTools when the list is empty,
  // and that default includes knowledge_search.
  if (allowedTools.length === 0) return true;
  return allowedTools.includes('knowledge_search');
}

export function getAgentNotReadyReasonKeys(
  config: AgentReadinessConfig | undefined,
  models: readonly AgentReadinessModel[],
  options: { isAgentMode: boolean },
): AgentNotReadyReasonKey[] {
  const reasons: AgentNotReadyReasonKey[] = [];
  if (!agentHasConfiguredChatModel(config, models)) {
    reasons.push('summary_model');
  }
  if (options.isAgentMode && agentRequiresRerankModel(config)) {
    const rerankModelId = asString(config?.rerank_model_id);
    const rerankExists = !!rerankModelId && models.some((model) => model.type === 'Rerank' && model.id === rerankModelId);
    if (!rerankExists) {
      reasons.push('rerank_model');
    }
  }
  return reasons;
}

/** Map missing-config reasons to the agent editor section that fixes them. */
export function resolveAgentNotReadySection(reasons: readonly AgentNotReadyReasonKey[]): string {
  if (reasons.includes('summary_model') || reasons.includes('rerank_model')) return 'model';
  return 'model';
}

/** First missing item — highlights the corresponding editor field after navigation. */
export function resolveAgentNotReadyHighlight(reasons: readonly AgentNotReadyReasonKey[]): AgentNotReadyReasonKey | undefined {
  return reasons[0];
}
