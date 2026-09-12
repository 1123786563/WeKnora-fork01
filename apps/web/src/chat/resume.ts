import type { ChatMessage } from '@weknora/contracts';

/**
 * Picks the persisted assistant message a continue-stream GET should attach
 * to: the newest assistant row that is not marked complete. IM-originated
 * replies are skipped — their answer is generated on the IM side and never
 * streams through this server, so continue-stream would always fail.
 */
export function findResumeTargetMessage(messages: readonly ChatMessage[]): string | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role !== 'assistant') continue;
    if (message.is_completed) return undefined;
    if ((message as Record<string, unknown>).channel === 'im') return undefined;
    return message.id;
  }
  return undefined;
}
