export type ChatAppState = 'active' | 'background' | 'inactive' | 'unknown' | 'extension';
export type ChatAppStateAction = 'abort' | 'resume' | 'ignore';

export function chatAppStateAction(
  state: ChatAppState,
  hasSession: boolean,
  hasActiveStream: boolean,
): ChatAppStateAction {
  if (state !== 'active') return hasActiveStream ? 'abort' : 'ignore';
  return hasSession && !hasActiveStream ? 'resume' : 'ignore';
}
