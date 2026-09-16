import { createJsonTransport, createWeKnoraClient, createExecutionsApi, type BearerCredential, type ExecutionCommandInput, type StartExecutionInput } from '@weknora/api-client';
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

export interface ConversationMessage {
  id: string;
  role: ConversationMessageRole;
  text: string;
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
    updateExecution({ runID: ack.run_id, requestID: ack.request_id, status: ack.status });
    // A start acknowledgement is admission only. Re-read the durable request
    // so an unknown/pending response survives a remount and scope transition.
    if (ack.status === 'unknown' || ack.status === 'pending' || ack.status === 'dispatching') {
      await refreshRequest(requestID, captured.signal);
    }
  });
  const refreshRequest = async (requestID: string, signal?: AbortSignal): Promise<void> => {
    const lookup = await input.executions.lookup(requestID, signal);
    if (lookup.state === 'unknown' || lookup.state === 'pending' || lookup.state === 'dispatching') {
      updateExecution({ runID: lookup.run_id ?? model.execution?.runID ?? '', requestID, status: lookup.state, reason: lookup.reason });
    } else if (lookup.run_id) {
      updateExecution({ runID: lookup.run_id, requestID, status: lookup.state, reason: lookup.reason });
    }
  };
  model = createConversationViewModel({
    scope: { ...identity, spaceId: input.spaceId },
    messages: input.messages,
    pendingInteractions: input.pendingInteractions,
    capabilities: { canCancel: true, canSteer: true },
    commands: {
      cancel: async (runID, expectedRevision = 0) => { await input.executions.command(runID, { action: 'cancel', expected_revision: expectedRevision }); },
      steer: async (runID, text, expectedRevision = 0) => { await input.executions.command(runID, { action: 'steer', text, expected_revision: expectedRevision }); },
      refreshPending: async (interactionID) => { await refreshRequest(latestRequestID ?? interactionID); },
      approve: async (interactionID, expectedRevision = 0) => {
        if (!input.executions.decide) throw new Error('APPROVAL_UNSUPPORTED');
        await input.executions.decide(interactionID, { action: 'approve', expected_revision: expectedRevision });
        const item = model.pendingInteractions.find((candidate) => candidate.id === interactionID);
        if (item) item.status = 'approved';
        notify();
      },
      reject: async (interactionID, expectedRevision = 0) => {
        if (!input.executions.decide) throw new Error('APPROVAL_UNSUPPORTED');
        await input.executions.decide(interactionID, { action: 'reject', expected_revision: expectedRevision });
        const item = model.pendingInteractions.find((candidate) => candidate.id === interactionID);
        if (item) item.status = 'rejected';
        notify();
      },
    },
    send,
  });
  model.subscribe = (listener) => { listeners.add(listener); return () => listeners.delete(listener); };
  model.getSnapshot = () => model;
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
}): ExecutionApi {
  const transport = createJsonTransport(fetch);
  const client = createWeKnoraClient({
    baseURL: input.origin,
    transport: {
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
  return {
    start: (request, signal) => executions.start(request, signal),
    lookup: (requestID, signal) => executions.lookup(requestID, signal),
    command: (runID, command, signal) => executions.command(runID, command, signal),
    decide: async (interactionID, decision, signal) => {
      await client.chat.approvals.resolveTool(interactionID, { decision: decision.action }, signal);
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
