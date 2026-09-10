export interface ChatDraftScope {
  origin: string;
  userId: string | null;
  tenantId: string | null;
  sessionId: string;
}

export type ChatDraftKey = readonly ['weknora', 'chat-draft', string, string | null, string | null, string];

export function chatDraftKey(scope: ChatDraftScope): ChatDraftKey {
  const sessionId = scope.sessionId.trim();
  if (!sessionId) throw new Error('sessionId must not be empty');
  return ['weknora', 'chat-draft', scope.origin.replace(/\/+$/, ''), scope.userId, scope.tenantId, sessionId];
}
