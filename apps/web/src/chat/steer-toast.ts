import type { ChatCopyTable } from '@weknora/views';

/*
 * R476-A2 — Vue chat/index.vue differentiates the steer notices per scenario
 * (MessagePlugin calls) instead of one operationFailed bucket (R475 A3
 * leftover):
 *   enqueue failure                → error input.messages.steerFailed
 *   promote failure                → error input.messages.steerPromoteFailed
 *   remove failure / refused       → error input.messages.steerRemoveFailed
 *   already_injected answers       → info  input.messages.steerAlreadyInjected
 * Failure toasts follow the Vue `e?.message || t(key)` contract: the
 * server-provided error message wins, the scenario copy is the fallback.
 */

export type SteerNoticeScenario =
  | 'enqueueFailed'
  | 'promoteFailed'
  | 'removeFailed'
  | 'alreadyInjected';

export function steerNoticeCopy(copy: ChatCopyTable, scenario: SteerNoticeScenario): string {
  switch (scenario) {
    case 'enqueueFailed': return copy.steerFailed;
    case 'promoteFailed': return copy.steerPromoteFailed;
    case 'removeFailed': return copy.steerRemoveFailed;
    case 'alreadyInjected': return copy.steerAlreadyInjected;
  }
}

/** Vue `e?.message || t(key)`: keep the server message when there is one. */
export function steerFailureCopy(copy: ChatCopyTable, scenario: SteerNoticeScenario, cause: unknown): string {
  if (scenario === 'alreadyInjected') return steerNoticeCopy(copy, scenario);
  return cause instanceof Error && cause.message ? cause.message : steerNoticeCopy(copy, scenario);
}
