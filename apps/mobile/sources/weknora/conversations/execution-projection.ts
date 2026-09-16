import type { ExecutionEvent, ExecutionSnapshot } from '@weknora/contracts';
import type { ConversationMessage, PendingInteraction, ConversationExecution } from './view-model';

export interface ProductConversationProjection {
  messages: ConversationMessage[];
  pendingInteractions: PendingInteraction[];
  execution: ConversationExecution | null;
}

function text(value: unknown): string | undefined {
  return typeof value === 'string' && value.length > 0 ? value : undefined;
}

function blocks(value: unknown, fallback?: string) {
  if (!Array.isArray(value)) return fallback === undefined ? undefined : [{ kind: 'text' as const, text: fallback }];
  const result = value.flatMap((item, index): Array<{ id: string; kind: 'text' | 'tool' | 'thinking'; text: string }> => {
    if (!item || typeof item !== 'object') return [];
    const row = item as Record<string, unknown>;
    const kind = row.kind === 'tool' || row.kind === 'thinking' ? row.kind : 'text';
    const content = text(row.text) ?? text(row.content);
    return content === undefined ? [] : [{ id: text(row.id) ?? `${kind}-${index}`, kind, text: content }];
  });
  return result.length > 0 ? result : undefined;
}

function eventMessage(event: ExecutionEvent): ConversationMessage | undefined {
  const payload = event.payload;
  const row = (payload.message && typeof payload.message === 'object' ? payload.message : payload) as Record<string, unknown>;
  const kind = event.type.toLowerCase();
  const role = row.role === 'user' || row.role === 'tool' || row.role === 'system' ? row.role : 'assistant';
  const messageID = text(row.id) ?? text(row.message_id) ?? `${event.run_id}:message:${event.seq}`;
  const content = text(row.text) ?? text(row.content) ?? text(payload.delta) ?? text(payload.output);
  const messageBlocks = blocks(row.blocks ?? payload.blocks, content);
  if (messageBlocks === undefined || (content === undefined && messageBlocks.length === 0)) return undefined;
  return { id: messageID, role, text: content ?? messageBlocks.map((block) => block.text).join(''), blocks: messageBlocks, ...(text(row.agent_id) ? { agentID: row.agent_id as string } : {}), createdAt: event.occurred_at };
}

function eventPending(event: ExecutionEvent): PendingInteraction | undefined {
  const payload = event.payload;
  const value = payload.pending_interaction ?? payload.interaction ?? (event.type.includes('approval') ? payload : undefined);
  if (!value || typeof value !== 'object') return undefined;
  const row = value as Record<string, unknown>;
  const id = text(row.id) ?? text(row.interaction_id);
  if (!id) return undefined;
  const status = row.status === 'approved' || row.status === 'rejected' || row.status === 'expired' ? row.status : 'pending';
  return { id, kind: row.kind === 'question' || row.kind === 'permission' ? row.kind : 'approval', status, label: text(row.label) ?? text(row.name) ?? id, ...(text(row.reason) ? { reason: row.reason as string } : {}), ...(typeof row.revision === 'number' ? { revision: row.revision } : {}) };
}

export function projectExecutionSnapshot(snapshot: ExecutionSnapshot, storedEvents: readonly ExecutionEvent[] = []): ProductConversationProjection {
  const events = [...storedEvents, ...snapshot.events].sort((a, b) => a.seq - b.seq).filter((event, index, all) => index === all.findIndex((candidate) => candidate.seq === event.seq));
  const messages = events.map(eventMessage).filter((value): value is ConversationMessage => value !== undefined);
  const pendingByID = new Map<string, PendingInteraction>();
  for (const item of events.map(eventPending).filter((value): value is PendingInteraction => value !== undefined)) pendingByID.set(item.id, item);
  const execution: ConversationExecution = { runID: snapshot.execution.run_id, requestID: '', status: snapshot.execution.execution_status, revision: snapshot.execution.revision };
  return { messages, pendingInteractions: [...pendingByID.values()], execution };
}
