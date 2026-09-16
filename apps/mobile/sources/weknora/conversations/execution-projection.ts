import type { ExecutionEvent, ExecutionSnapshot } from '@weknora/contracts';
import type { ConversationMessage, PendingInteraction, ConversationExecution } from './view-model';

export interface ProductConversationProjection {
  messages: ConversationMessage[];
  pendingInteractions: PendingInteraction[];
  execution: ConversationExecution | null;
  /** Last durable event included by this projection. Used as the SSE resume cursor. */
  watermark: number;
  /** Sequence identities already applied; retained to absorb stream replays. */
  eventSeqs?: readonly number[];
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

interface ProjectedMessage extends ConversationMessage {
  delta?: boolean;
}

function eventMessage(event: ExecutionEvent): ProjectedMessage | undefined {
  const payload = event.payload;
  const row = (payload.message && typeof payload.message === 'object' ? payload.message : payload) as Record<string, unknown>;
  const kind = event.type.toLowerCase();
  const role = row.role === 'user' || row.role === 'tool' || row.role === 'system' ? row.role : 'assistant';
  const messageID = text(row.id) ?? text(row.message_id) ?? text(payload.message_id)
    // Tool events have a stable tool/call identity even when they are not
    // nested under `message`. Do not manufacture a fresh entity per delta.
    ?? (kind.includes('tool') ? text(row.tool_id) ?? text(row.call_id) ?? text(payload.tool_id) : undefined)
    // Legacy complete events have no identity. They remain visible, while
    // identity-less deltas are ignored rather than rendered as duplicates.
    ?? (!kind.includes('delta') ? `${event.run_id}:message:${event.seq}` : undefined);
  if (!messageID) return undefined;
  const content = text(row.text) ?? text(row.content) ?? text(payload.delta) ?? text(payload.output);
  const messageBlocks = blocks(row.blocks ?? payload.blocks, content);
  if (messageBlocks === undefined || (content === undefined && messageBlocks.length === 0)) return undefined;
  return {
    id: messageID,
    role,
    text: content ?? messageBlocks.map((block) => block.text).join(''),
    blocks: messageBlocks,
    ...(text(row.agent_id) ? { agentID: row.agent_id as string } : {}),
    createdAt: event.occurred_at,
    delta: kind.includes('delta'),
  };
}

function mergeMessage(previous: ConversationMessage | undefined, incoming: ProjectedMessage): ConversationMessage {
  if (!previous) {
    const { delta: _delta, ...message } = incoming;
    return message;
  }
  const incomingBlocks = incoming.blocks ?? [];
  const previousBlocks = previous.blocks ?? [];
  const blockMap = new Map(previousBlocks.map((block) => [block.id ?? `${block.kind}:0`, block]));
  for (const [index, block] of incomingBlocks.entries()) {
    const key = block.id ?? `${block.kind}:${index}`;
    const old = blockMap.get(key);
    if (!old) blockMap.set(key, block);
    else if (incoming.delta) blockMap.set(key, { ...old, text: old.text + block.text });
    else blockMap.set(key, { ...old, ...block });
  }
  const mergedBlocks = [...blockMap.values()];
  const delta = incoming.delta === true;
  return {
    ...previous,
    ...incoming,
    text: delta ? previous.text + incoming.text : incoming.text || previous.text,
    blocks: mergedBlocks.length > 0 ? mergedBlocks : previous.blocks,
  };
}

function mergeEventMessages(messages: ConversationMessage[], events: readonly ExecutionEvent[]): ConversationMessage[] {
  const byID = new Map(messages.map((message) => [message.id, message]));
  const order = messages.map((message) => message.id);
  for (const event of events) {
    const incoming = eventMessage(event);
    if (!incoming) continue;
    if (!byID.has(incoming.id)) order.push(incoming.id);
    byID.set(incoming.id, mergeMessage(byID.get(incoming.id), incoming));
  }
  return order.map((id) => byID.get(id)).filter((value): value is ConversationMessage => value !== undefined);
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
  const messages = mergeEventMessages([], events);
  const pendingByID = new Map<string, PendingInteraction>();
  for (const item of events.map(eventPending).filter((value): value is PendingInteraction => value !== undefined)) pendingByID.set(item.id, item);
  const execution: ConversationExecution = { runID: snapshot.execution.run_id, requestID: '', status: snapshot.execution.execution_status, revision: snapshot.execution.revision };
  return { messages, pendingInteractions: [...pendingByID.values()], execution, watermark: snapshot.watermark, eventSeqs: events.map((event) => event.seq) };
}

/** Apply one W09 online event to the current product projection. */
export function projectExecutionEvent(projection: ProductConversationProjection, event: ExecutionEvent): ProductConversationProjection {
  if (projection.eventSeqs?.includes(event.seq)) return projection;
  const pending = eventPending(event);
  const pendingByID = new Map(projection.pendingInteractions.map((item) => [item.id, item]));
  if (pending) pendingByID.set(pending.id, pending);
  return {
    ...projection,
    messages: mergeEventMessages(projection.messages, [event]),
    pendingInteractions: [...pendingByID.values()],
    watermark: Math.max(projection.watermark, event.seq),
    eventSeqs: [...(projection.eventSeqs ?? []), event.seq],
  };
}
