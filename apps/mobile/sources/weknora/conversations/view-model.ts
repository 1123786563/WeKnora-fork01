import type { ExecutionDTO } from '@weknora/contracts';
import type { ProductScope } from '../platform/product-session';

export interface ExecutionApi {
  start(input: { request_id: string; session_id: string; agent_id: string; target_id: string; workspace_ref: string; text: string; budget_upper: number }): Promise<unknown>;
  lookup(requestID: string): Promise<unknown>;
  command(runID: string, input: { action: 'cancel'; expected_revision: number } | { action: 'steer'; text: string; expected_revision: number }): Promise<unknown>;
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
}): ConversationViewModel {
  const identity = input.scope.identity();
  let latestRequestID: string | undefined;
  const send = createSendController(async (text, requestID) => {
    latestRequestID = requestID;
    await input.executions.start({
      request_id: requestID,
      session_id: input.sessionId,
      agent_id: input.agent.id,
      target_id: input.targetId,
      workspace_ref: input.workspaceRef,
      text,
      budget_upper: input.budgetUpper,
    });
  });
  return createConversationViewModel({
    scope: { ...identity, spaceId: input.spaceId },
    capabilities: { canCancel: true, canSteer: true },
    commands: {
      cancel: async (runID, expectedRevision = 0) => { await input.executions.command(runID, { action: 'cancel', expected_revision: expectedRevision }); },
      steer: async (runID, text, expectedRevision = 0) => { await input.executions.command(runID, { action: 'steer', text, expected_revision: expectedRevision }); },
      refreshPending: async (interactionID) => { await input.executions.lookup(latestRequestID ?? interactionID); },
    },
    send,
  });
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
