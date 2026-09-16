import { createJsonTransport, createWeKnoraClient, createExecutionsApi, type BearerCredential, type ExecutionCommandInput, type StartExecutionInput, type ProductAuthSession } from '@weknora/api-client';
import type { ExecutionDTO } from '@weknora/contracts';
import type { ProductScope } from '../platform/product-session';

export interface ExecutionApi {
  start(input: StartExecutionInput, signal?: AbortSignal): Promise<{ run_id: string; request_id: string; status: string }>;
  lookup(requestID: string, signal?: AbortSignal): Promise<{ state: 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown'; run_id?: string; reason?: string }>;
  command(runID: string, input: ExecutionCommandInput, signal?: AbortSignal): Promise<unknown>;
  decide?(interactionID: string, input: { action: 'approve' | 'reject'; expected_revision: number }, signal?: AbortSignal): Promise<void>;
}

export interface ConversationScope {
  origin: string;
  userId: string | null;
  tenantId: string | null;
  spaceId: string | null;
}

export type ConversationMessageRole = 'user' | 'assistant' | 'tool' | 'system';
export type ConversationContentBlock = { id?: string; kind: 'text' | 'tool' | 'thinking'; text: string };

export interface ConversationMessage {
  id: string;
  role: ConversationMessageRole;
  text: string;
  blocks?: ConversationContentBlock[];
  agentID?: string;
  createdAt?: string;
}

export type PendingInteractionStatus = 'pending' | 'approved' | 'rejected' | 'expired';

export interface PendingInteraction {
  id: string;
  kind: 'approval' | 'question' | 'permission' | 'unknown';
  status: PendingInteractionStatus;
  label: string;
  reason?: string;
  revision?: number;
  error?: string;
}

export interface ConversationCapabilities {
  canCancel: boolean;
  canSteer: boolean;
  canAttach: boolean;
  canVoice: boolean;
}

export interface ConversationCommands {
  cancel(runID: string, expectedRevision?: number): Promise<void>;
  steer(runID: string, text: string, expectedRevision?: number): Promise<void>;
  refreshPending?(interactionID: string): Promise<void>;
  approve?(interactionID: string, expectedRevision?: number): Promise<void>;
  reject?(interactionID: string, expectedRevision?: number): Promise<void>;
}

export interface ConversationExecution {
  runID: string;
  requestID: string;
  status: string;
  revision?: number;
  reason?: string;
}

export interface ConversationRequestRecord {
  requestID: string;
  runID?: string;
  status: string;
  reason?: string;
}

export interface ConversationRequestStorage {
  getLatest(): Promise<ConversationRequestRecord | undefined>;
  set(record: ConversationRequestRecord): Promise<void>;
}

export interface ConversationViewModel {
  scope: ConversationScope;
  messages: ConversationMessage[];
  pendingInteractions: PendingInteraction[];
  capabilities: ConversationCapabilities;
  execution: ConversationExecution | null;
  commands: ConversationCommands;
  send?: SendController;
  subscribe?: (listener: () => void) => () => void;
  getSnapshot?: () => ConversationViewModel;
}


export interface ConversationViewModelInput {
  scope: ConversationScope;
  messages?: ConversationMessage[];
  pendingInteractions?: PendingInteraction[];
  capabilities?: Partial<ConversationCapabilities>;
  execution?: ExecutionDTO | { run_id: string; request_id: string; status: string; revision?: number; reason?: string } | null;
  commands: ConversationCommands;
  send?: SendController;
}

export interface ProductAgentOption { id: string; name: string; }

/** Builds the product conversation from W07 scope and the W06 execution SDK. */
export function createProductConversationViewModel(input: {
  scope: ProductScope;
  spaceId: string | null;
  sessionId: string;
  agent: ProductAgentOption;
  targetId: string;
  workspaceRef: string;
  budgetUpper: number;
  executions: ExecutionApi;
  messages?: ConversationMessage[];
  pendingInteractions?: PendingInteraction[];
  requestStorage?: ConversationRequestStorage;
}): ConversationViewModel {
  const identity = input.scope.identity();
  let latestRequestID: string | undefined;
  const listeners = new Set<() => void>();
  let model!: ConversationViewModel;
  const notify = () => listeners.forEach((listener) => listener());
  const updateExecution = (execution: ConversationExecution | null) => {
    model.execution = execution;
    notify();
  };
  const persistExecution = async (execution: ConversationExecution) => {
    if (!input.requestStorage) return;
    await input.requestStorage.set({ requestID: execution.requestID, runID: execution.runID || undefined, status: execution.status, ...(execution.reason ? { reason: execution.reason } : {}) });
  };
  const send = createSendController(async (text, requestID) => {
    latestRequestID = requestID;
    const captured = input.scope.capture();
    const ack = await input.executions.start({
      request_id: requestID,
      session_id: input.sessionId,
      agent_id: input.agent.id,
      target_id: input.targetId,
      workspace_ref: input.workspaceRef,
      text,
      budget_upper: input.budgetUpper,
    }, captured.signal);
    if (!input.scope.accept(captured.generation)) return;
    const execution = { runID: ack.run_id, requestID: ack.request_id, status: ack.status };
    updateExecution(execution);
    await persistExecution(execution);
    // A start acknowledgement is admission only. Re-read the durable request
    // so an unknown/pending response survives a remount and scope transition.
    if (ack.status === 'unknown' || ack.status === 'pending' || ack.status === 'dispatching') {
      await refreshRequest(requestID, captured.signal);
    }
  });
  const refreshRequest = async (requestID: string, signal?: AbortSignal): Promise<void> => {
    const captured = input.scope.capture();
    const lookup = await input.executions.lookup(requestID, signal ?? captured.signal);
    if (signal?.aborted || captured.signal.aborted) return;
    if (!input.scope.accept(captured.generation)) return;
    let execution: ConversationExecution;
    if (lookup.state === 'unknown' || lookup.state === 'pending' || lookup.state === 'dispatching') {
      execution = { runID: lookup.run_id ?? model.execution?.runID ?? '', requestID, status: lookup.state, reason: lookup.reason };
    } else if (lookup.run_id) {
      execution = { runID: lookup.run_id, requestID, status: lookup.state, reason: lookup.reason };
    } else {
      execution = { runID: model.execution?.runID ?? '', requestID, status: lookup.state, reason: lookup.reason };
    }
    updateExecution(execution);
    await persistExecution(execution);
  };
  model = createConversationViewModel({
    scope: { ...identity, spaceId: input.spaceId },
    messages: input.messages,
    pendingInteractions: input.pendingInteractions,
    capabilities: { canCancel: true, canSteer: true },
    commands: {
      cancel: async (runID, expectedRevision = 0) => {
        const captured = input.scope.capture();
        await input.executions.command(runID, { action: 'cancel', expected_revision: expectedRevision }, captured.signal);
        if (!input.scope.accept(captured.generation)) throw new Error('SCOPE_CHANGED');
      },
      steer: async (runID, text, expectedRevision = 0) => {
        const captured = input.scope.capture();
        await input.executions.command(runID, { action: 'steer', text, expected_revision: expectedRevision }, captured.signal);
        if (!input.scope.accept(captured.generation)) throw new Error('SCOPE_CHANGED');
      },
      refreshPending: async (interactionID) => { await refreshRequest(latestRequestID ?? interactionID); },
      approve: async (interactionID, expectedRevision = 0) => {
        if (!input.executions.decide) throw new Error('APPROVAL_UNSUPPORTED');
        const captured = input.scope.capture();
        try {
          await input.executions.decide(interactionID, { action: 'approve', expected_revision: expectedRevision }, captured.signal);
        } catch (error) {
          const item = model.pendingInteractions.find((candidate) => candidate.id === interactionID);
          if (item) { item.error = error instanceof Error ? error.message : 'APPROVAL_FAILED'; notify(); }
          throw error;
        }
        if (!input.scope.accept(captured.generation)) return;
        const item = model.pendingInteractions.find((candidate) => candidate.id === interactionID);
        if (item) item.status = 'approved';
        notify();
      },
      reject: async (interactionID, expectedRevision = 0) => {
        if (!input.executions.decide) throw new Error('APPROVAL_UNSUPPORTED');
        const captured = input.scope.capture();
        try {
          await input.executions.decide(interactionID, { action: 'reject', expected_revision: expectedRevision }, captured.signal);
        } catch (error) {
          const item = model.pendingInteractions.find((candidate) => candidate.id === interactionID);
          if (item) { item.error = error instanceof Error ? error.message : 'APPROVAL_FAILED'; notify(); }
          throw error;
        }
        if (!input.scope.accept(captured.generation)) return;
        const item = model.pendingInteractions.find((candidate) => candidate.id === interactionID);
        if (item) item.status = 'rejected';
        notify();
      },
    },
    send,
  });
  model.subscribe = (listener) => { listeners.add(listener); return () => listeners.delete(listener); };
  model.getSnapshot = () => model;
  if (input.requestStorage) {
    void input.requestStorage.getLatest().then((record) => {
      if (!record) return;
      latestRequestID = record.requestID;
      updateExecution({ runID: record.runID ?? '', requestID: record.requestID, status: record.status, reason: record.reason });
      if (record.status === 'unknown' || record.status === 'pending' || record.status === 'dispatching') {
        void refreshRequest(record.requestID).catch(() => undefined);
      }
    }).catch(() => undefined);
  }
  return model;
}

/** Generates a fresh command identity for every user submission. */
export function createRequestID(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') return `mobile:${globalThis.crypto.randomUUID()}`;
  return `mobile:${Date.now().toString(36)}:${Math.random().toString(36).slice(2)}`;
}

/** Product-owned authenticated W06 SDK adapter. Happy credentials never enter this client. */
export function createProductExecutionApi(input: {
  origin: string;
  credential: BearerCredential;
  scope: ProductScope;
  authSession?: ProductAuthSession | null;
}): ExecutionApi {
  const transport = createJsonTransport(fetch);
  const client = createWeKnoraClient({
    baseURL: input.origin,
    transport: input.authSession?.transport ?? {
      send: (request) => transport.send({
        ...request,
        headers: { ...request.headers, authorization: `Bearer ${input.credential.accessToken}` },
      }),
    },
  });
  const executions = createExecutionsApi(async (request) => {
    const result = await client.request(request);
    return result;
  });
  const scoped = async <T,>(operation: (signal: AbortSignal) => Promise<T>, signal?: AbortSignal): Promise<T> => {
    const captured = input.scope.capture();
    const result = await operation(signal ?? captured.signal);
    if (!input.scope.accept(captured.generation)) throw new Error('SCOPE_CHANGED');
    return result;
  };
  return {
    start: (request, signal) => scoped((activeSignal) => executions.start(request, activeSignal), signal),
    lookup: (requestID, signal) => scoped((activeSignal) => executions.lookup(requestID, activeSignal), signal),
    command: (runID, command, signal) => scoped((activeSignal) => executions.command(runID, command, activeSignal), signal),
    decide: async (interactionID, decision, signal) => {
      await scoped((activeSignal) => client.chat.approvals.resolveTool(interactionID, { decision: decision.action, expected_revision: decision.expected_revision }, activeSignal), signal);
    },
  };
}

/**
 * The only boundary where Happy-shaped session data is allowed to become a
 * product conversation model. The view never receives Happy credentials or
 * raw protocol messages, and product scope remains explicit in every model.
 */
export function createConversationViewModel(input: ConversationViewModelInput): ConversationViewModel {
  // ExecutionDTO carries run_status/execution_status instead of the legacy
  // wire fields, so normalize per branch: the DTO path keeps its statuses and
  // an empty request id (the DTO does not carry one).
  const raw = input.execution;
  const execution = raw == null ? null : {
    runID: raw.run_id,
    requestID: 'request_id' in raw ? raw.request_id : '',
    status: 'status' in raw ? raw.status : raw.execution_status,
    ...('revision' in raw && raw.revision !== undefined ? { revision: raw.revision } : {}),
    ...('reason' in raw && raw.reason !== undefined ? { reason: raw.reason } : {}),
  };
  return {
    scope: { ...input.scope },
    messages: [...(input.messages ?? [])],
    pendingInteractions: [...(input.pendingInteractions ?? [])],
    capabilities: {
      canCancel: false,
      canSteer: false,
      canAttach: false,
      canVoice: false,
      ...input.capabilities,
    },
    execution,
    commands: input.commands,
    send: input.send,
  };
}

export interface SendController {
  submit(text: string, requestID: string): Promise<void>;
  draft(): string;
  busy(): boolean;
}

/** A single-flight sender. A failed request deliberately leaves its draft. */
export function createSendController(send: (text: string, requestID: string) => Promise<void>): SendController {
  let text = '';
  let pending = false;
  return {
    draft: () => text,
    busy: () => pending,
    async submit(value: string, requestID: string): Promise<void> {
      if (pending) throw new Error('SEND_IN_PROGRESS');
      if (value.trim() === '') return;
      if (requestID.trim() === '') throw new Error('REQUEST_ID_REQUIRED');
      text = value;
      pending = true;
      try {
        await send(value, requestID);
        text = '';
      } finally {
        pending = false;
      }
    },
  };
}
