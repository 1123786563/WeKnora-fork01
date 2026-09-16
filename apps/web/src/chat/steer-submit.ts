import type { ChatSubmission } from '@weknora/views';

export interface SteerMentionItem {
  id: string;
  name: string;
  type: 'kb' | 'file' | 'tag' | 'mcp' | 'skill';
  kbType?: 'document' | 'faq';
  kbId?: string;
  kbName?: string;
  skillName?: string;
}

export const STEER_CONFLICT_STATUS = 409;

export type SteerAction = 
  | { kind: 'send'; submission: ChatSubmission }
  | {
    kind: 'enqueue';
    input: {
      query: string;
      delivery: 'after';
      channel: string;
      expectedAssistantMessageId?: string;
      steerId: string;
      mentionedItems?: SteerMentionItem[];
    };
  };

/**
 * A follow-up while a turn is running is queued as a steer carrying the
 * assistant message id the client believes is current plus a client-generated
 * steer id; when the page is idle there is no run to steer, so the message
 * routes through the normal send path instead.
 */
export function buildSteerAction(options: {
  streaming: boolean;
  content: string;
  assistantMessageId?: string;
  mentionedItems?: readonly SteerMentionItem[];
  newSteerId: () => string;
}): SteerAction {
  if (!options.streaming) {
    return { kind: 'send', submission: { content: options.content, status: 'pending' } };
  }
  return {
    kind: 'enqueue',
    input: {
      query: options.content,
      delivery: 'after',
      channel: 'web',
      ...(options.assistantMessageId ? { expectedAssistantMessageId: options.assistantMessageId } : {}),
      steerId: options.newSteerId(),
      ...(options.mentionedItems && options.mentionedItems.length > 0 ? { mentionedItems: [...options.mentionedItems] } : {}),
    },
  };
}

/** A 409 means the run moved on; the caller re-bases onto the fresh id and retries once. */
export function isSteerConflict(error: unknown): boolean {
  if (typeof error !== 'object' || error === null) return false;
  const candidate = error as {
    status?: unknown;
    response?: { status?: unknown };
    cause?: { status?: unknown };
  };
  return candidate.status === STEER_CONFLICT_STATUS
    || candidate.response?.status === STEER_CONFLICT_STATUS
    || candidate.cause?.status === STEER_CONFLICT_STATUS;
}
