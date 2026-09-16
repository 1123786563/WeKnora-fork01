export const DEFAULT_DIRECT_HEADER_NAME = 'X-External-User-ID';

/** Vue PlaygroundStatus (ApiIntegrationSettings.vue L902). */
export type PlaygroundStepStatus = '' | 'running' | 'success' | 'failed' | 'stopped';
export const DEFAULT_TOKEN_HEADER_NAME = 'X-External-User-Token';

type PrincipalMode = 'tenant' | 'direct_header' | 'signed_token';
type Agent = { id: string; name: string; is_builtin?: boolean };

export function buildPlaygroundHeaders(input: { apiKey: string; mode: PrincipalMode; externalUserId: string; signedToken: string; maskSecrets: boolean }) {
  const common: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'X-API-Key': input.maskSecrets ? '<API_KEY>' : input.apiKey,
  };
  if (input.mode === 'direct_header' && input.externalUserId.trim()) common[DEFAULT_DIRECT_HEADER_NAME] = input.externalUserId.trim();
  if (input.mode === 'signed_token') common[DEFAULT_TOKEN_HEADER_NAME] = input.maskSecrets ? '<JWT>' : input.signedToken.trim();
  return { sessionHeaders: common, chatHeaders: { ...common, Accept: 'text/event-stream' } };
}

export function playgroundRequestPreview(input: { query: string; agentId: string; mode: PrincipalMode; externalUserId: string; signedToken: string; apiKey: string }): string {
  const headers = buildPlaygroundHeaders({ ...input, maskSecrets: true });
  const body = { query: input.query || '<query>', agent_enabled: true, agent_id: input.agentId || '<agent_id>', channel: 'api' };
  return ['POST /api/v1/sessions', JSON.stringify({ headers: headers.sessionHeaders, body: {} }, null, 2), '', 'POST /api/v1/agent-chat/<session_id>', JSON.stringify({ headers: headers.chatHeaders, body }, null, 2)].join('\n');
}

export function playgroundDisabledReason(input: { running: boolean; apiKey: string; agentId: string; query: string; mode: PrincipalMode; externalUserId: string }): string {
  if (input.running) return '';
  if (!input.apiKey.trim()) return 'integrations.api.playgroundNeedApiKey';
  if (!input.agentId) return 'integrations.api.playgroundNeedAgent';
  if (!input.query.trim()) return 'integrations.api.playgroundNeedQuestion';
  if (input.mode === 'signed_token' && !input.externalUserId.trim()) return 'integrations.api.playgroundNeedExternalUser';
  return '';
}

export function interpretSessionResponse(input: { ok: boolean; status: number; payload: unknown }) {
  if (!input.ok) return { ok: false as const, error: responseError(input.payload) || `HTTP ${input.status}` };
  if (!input.payload || typeof input.payload !== 'object') return { ok: false as const, error: `HTTP ${input.status}` };
  const value = input.payload as { success?: unknown; message?: unknown; data?: unknown };
  if (value.success === false) return { ok: false as const, error: typeof value.message === 'string' ? value.message : 'Request failed' };
  const data = value.data && typeof value.data === 'object' ? value.data as { id?: unknown; ID?: unknown } : {};
  const sessionId = typeof data.id === 'string' ? data.id : typeof data.ID === 'string' ? data.ID : '';
  return sessionId ? { ok: true as const, sessionId } : { ok: false as const, error: 'integrations.api.playgroundMissingSessionId' };
}

function responseError(value: unknown): string {
  if (!value || typeof value !== 'object') return '';
  const error = (value as { error?: unknown }).error;
  return error && typeof error === 'object' && typeof (error as { message?: unknown }).message === 'string' ? (error as { message: string }).message : '';
}

export function compactText(text: string, max = 12000): string { return text.length <= max ? text : text.slice(0, max) + '\n...'; }
/** Vue formatJSON (ApiIntegrationSettings.vue L1606-1612). */
export function formatJSON(value: unknown): string { try { return compactText(JSON.stringify(value, null, 2)); } catch { return String(value); } }
export function formatResponseBody(text: string): string {
  if (!text) return '';
  try { return compactText(JSON.stringify(JSON.parse(text), null, 2)); } catch { return compactText(text); }
}
export function hasPlaygroundResult(input: { signedToken: string; sessionResponse: string; streamOutput: string; finalAnswer: string }): boolean { return Boolean(input.signedToken || input.sessionResponse || input.streamOutput || input.finalAnswer); }
export function ensurePlaygroundAgent(current: string, agents: readonly Agent[]): string {
  if (current && agents.some((agent) => agent.id === current)) return current;
  return agents.find((agent) => agent.id === 'builtin-smart-reasoning')?.id || agents[0]?.id || '';
}
export function agentOptionLabel(name: string, builtin: boolean, builtinLabel: string): string { return builtin ? `${name} · ${builtinLabel}` : name; }
export function buildChatRequestBody(input: { query: string; agentId: string }) { return { query: input.query.trim(), agent_enabled: true, agent_id: input.agentId, channel: 'api' }; }
export function externalUserHintKey(mode: PrincipalMode): string { return mode === 'direct_header' ? 'integrations.api.playgroundDirectModeHint' : mode === 'signed_token' ? 'integrations.api.playgroundSignedModeHint' : 'integrations.api.playgroundTenantModeHint'; }
// Vue catch block (ApiIntegrationSettings.vue L1716-1726): on stop both running
// steps land on 'stopped'; on a real failure a still-running step lands on
// 'failed' — never 'success' (L1723 marks the session failed, not done).
export function settlePlaygroundStatuses(input: { sessionStatus: string; chatStatus: string }, stopped: boolean): { sessionStatus: PlaygroundStepStatus; chatStatus: PlaygroundStepStatus } { return { sessionStatus: input.sessionStatus === 'running' ? (stopped ? 'stopped' : 'failed') : input.sessionStatus as PlaygroundStepStatus, chatStatus: input.chatStatus === 'running' ? (stopped ? 'stopped' : 'failed') : input.chatStatus as PlaygroundStepStatus }; }
