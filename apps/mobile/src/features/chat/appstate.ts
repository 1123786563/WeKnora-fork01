export type ChatAppState = 'active' | 'background' | 'inactive' | 'unknown' | 'extension';
export type ChatAppStateAction = 'abort' | 'resume' | 'ignore';

import { canAutoResumeRun, initialRunLifecycle, type RunLifecycle } from './run-lifecycle.ts';

export function chatAppStateAction(
  state: ChatAppState,
  hasSession: boolean,
  hasActiveStream: boolean,
  lifecycle: RunLifecycle = initialRunLifecycle(),
): ChatAppStateAction {
  if (state !== 'active') return hasActiveStream ? 'abort' : 'ignore';
  return hasSession && !hasActiveStream && canAutoResumeRun(lifecycle) ? 'resume' : 'ignore';
}
