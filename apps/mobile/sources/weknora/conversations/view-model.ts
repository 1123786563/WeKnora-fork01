import { createJsonTransport, createWeKnoraClient, createExecutionsApi, type BearerCredential, type ExecutionCommandInput, type StartExecutionInput, type ProductAuthSession } from '@weknora/api-client';
import type { ExecutionDTO, ExecutionEvent, ExecutionSnapshot } from '@weknora/contracts';
import type { ProductScope } from '../platform/product-session';
import type { ClientGateVerdict } from '@weknora/domain/mobile';
import { serverCapabilitiesRequest } from '../platform/protocol-gate';
import { projectExecutionEvent, type ProductConversationProjection } from './execution-projection';
import { executionEventsRequest } from '@weknora/api-client';
import { ExecutionSSEParser } from '../platform/stream-transport';
import { createExecutionRecovery, isTerminalExecutionStatus, subscribeFromLastCommittedCursor, type ExecutionRecovery } from '../executions/recovery';

export interface ExecutionApi {
  start(input: StartExecutionInput, signal?: AbortSignal): Promise<{ run_id: string; request_id: string; status: string }>;
  lookup(requestID: string, signal?: AbortSignal): Promise<{ state: 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown'; run_id?: string; reason?: string }>;
  command(runID: string, input: ExecutionCommandInput, signal?: AbortSignal): Promise<unknown>;
  snapshot?(runID: string, signal?: AbortSignal): Promise<ExecutionSnapshot>;
  stream?(runID: string, lastEventID: string | undefined, onEvent: (event: ExecutionEvent) => void, signal?: AbortSignal): Promise<void>;
  decide?(interactionID: string, input: { action: 'approve' | 'reject'; expected_revision: number }, signal?: AbortSignal): Promise<void>;
  /**
   * W37 protocol handshake: raw /system/capabilities envelope over the same
   * authenticated transport. The protocol gate consumes it; absent on test
   * doubles that do not exercise compatibility.
   */
  capabilities?(signal?: AbortSignal): Promise<unknown>;
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

export interface ConversationProjectionSource {
  load(runID: string, signal?: AbortSignal): Promise<ProductConversationProjection>;
}

export interface ConversationEventStorage {
  read(runID: string, fromSeq?: number): Promise<ExecutionEvent[]>;
  commit(event: ExecutionEvent): Promise<void>;
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
  /** W12 lifecycle recovery controller; mounted by the conversation screen. */
  recovery?: ExecutionRecovery;
  /**
   * W37 protocol-gate notice. Present exactly when the compatibility
   * verdict is not 'full': the safe surface (login, reads, this
   * explanation) stays available while control commands are stopped.
   */
  protocolNotice?: string;
}

/**
 * W37 carry-forward: the protocol compatibility gate the production route
 * mounts from the /system/capabilities handshake. latest() is undefined
 * until the first handshake settles — that state is fail-closed too.
 */
export interface ConversationProtocolGate {
  latest(): ClientGateVerdict | undefined;
  subscribe(listener: () => void): () => void;
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
  projection?: ConversationProjectionSource;
  eventStorage?: ConversationEventStorage;
  /** W37 protocol gate: stops cancel/steer unless the verdict is 'full'. */
  protocolGate?: ConversationProtocolGate;
}): ConversationViewModel {
  const identity = input.scope.identity();
  let latestRequestID: string | undefined;
  const listeners = new Set<() => void>();
  let projectionState: ProductConversationProjection | undefined;
  const streamingRuns = new Set<string>();
  let model!: ConversationViewModel;
  const notify = () => listeners.forEach((listener) => listener());
  const updateExecution = (execution: ConversationExecution | null) => {
    model.execution = execution;
    notify();
  };
  const projectExecutionSnapshotFromStored = (base: ProductConversationProjection, events: readonly ExecutionEvent[]) => (
    [...events].sort((a, b) => a.seq - b.seq).reduce((current, event) => projectExecutionEvent(current, event), base)
  );
  const persistExecution = async (execution: ConversationExecution) => {
    if (!input.requestStorage) return;
    await input.requestStorage.set({ requestID: execution.requestID, runID: execution.runID || undefined, status: execution.status, ...(execution.reason ? { reason: execution.reason } : {}) });
  };
  let lastLoadedProjection: ProductConversationProjection | undefined;
  const applySnapshotProjection = (projection: ProductConversationProjection) => {
    model.messages = projection.messages;
    model.pendingInteractions = projection.pendingInteractions;
    if (projection.execution) {
      model.execution = { ...projection.execution, requestID: model.execution?.requestID ?? projection.execution.requestID };
    }
    notify();
  };
  const applyEventToProjection = (event: ExecutionEvent) => {
    const baseline = projectionState ?? lastLoadedProjection;
    if (!baseline) return;
    projectionState = projectExecutionEvent(baseline, event);
    model.messages = projectionState.messages;
    model.pendingInteractions = projectionState.pendingInteractions;
    model.execution = projectionState.execution
      ? { ...projectionState.execution, requestID: model.execution?.requestID ?? '' }
      : model.execution;
    notify();
  };
  const consumeStreamEvent = (captured: { generation: number }, event: ExecutionEvent) => {
    if (!input.scope.accept(captured.generation)) return;
    if (input.eventStorage) {
      void input.eventStorage.commit(event).then(() => {
        if (!input.scope.accept(captured.generation)) return;
        applyEventToProjection(event);
      }).catch(() => undefined);
      return;
    }
    applyEventToProjection(event);
  };
  /** Re-reads the durable snapshot into the visible history (no stream). W12 recovery step 1. */
  const reloadProjection = async (runID: string, signal?: AbortSignal, preCaptured?: { generation: number; signal: AbortSignal }): Promise<ProductConversationProjection | undefined> => {
    if (!input.projection) return undefined;
    const captured = preCaptured ?? input.scope.capture();
    const storedEvents = input.eventStorage ? await input.eventStorage.read(runID) : [];
    const projection = await input.projection.load(runID, signal ?? captured.signal);
    if (!input.scope.accept(captured.generation)) return undefined;
    lastLoadedProjection = projection;
    projectionState = storedEvents.length > 0 ? projectExecutionSnapshotFromStored(projection, storedEvents) : projection;
    applySnapshotProjection(projection);
    return projection;
  };
  /** Opens (or reopens) the event stream. W12 recovery step 2; single-flighted per run. */
  const openStream = (runID: string, captured: { generation: number; signal: AbortSignal }) => {
    if (!input.executions.stream || streamingRuns.has(runID)) return;
    streamingRuns.add(runID);
    // With durable storage the subscription resumes from the last committed
    // W09 cursor; without it the loaded snapshot watermark is the cursor.
    const open = input.eventStorage
      ? subscribeFromLastCommittedCursor({
          read: (id) => input.eventStorage!.read(id),
          stream: (id, lastEventID, _onEvent, signal) => input.executions.stream!(id, lastEventID, (event) => consumeStreamEvent(captured, event), signal ?? captured.signal),
        })(runID, undefined, captured.signal)
      : input.executions.stream(runID, String(projectionState?.watermark ?? lastLoadedProjection?.watermark ?? 0), (event) => consumeStreamEvent(captured, event), captured.signal);
    void Promise.resolve(open).catch(() => undefined).finally(() => streamingRuns.delete(runID));
  };
  const loadProjection = async (runID: string, signal?: AbortSignal) => {
    const captured = input.scope.capture();
    const projection = await reloadProjection(runID, signal ?? captured.signal, captured);
    if (projection === undefined) return;
    openStream(runID, captured);
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
    if (execution.runID) await loadProjection(execution.runID, captured.signal).catch(() => undefined);
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
    if (execution.runID) await loadProjection(execution.runID, captured.signal).catch(() => undefined);
  };
  // W37 protocol gate: cancel/steer are control commands and may only leave
  // the device on a 'full' compatibility verdict. An absent verdict (handshake
  // still in flight or failed) is fail-closed — the request never leaves.
  const protocolNoticeFor = (mode: ClientGateVerdict['mode']): string | undefined => {
    if (mode === 'full') return undefined;
    if (mode === 'upgrade_required') return '当前应用版本低于服务端兼容窗口，任务控制（取消/调整）已停用；登录与查询保持可用，请升级应用。';
    if (mode === 'server_upgrade_required') return '服务端版本低于当前应用所需的兼容窗口，任务控制（取消/调整）已停用；登录与查询保持可用，请联系管理员或稍后重试。';
    return '无法确认服务端兼容性，任务控制（取消/调整）已停用；登录与查询保持可用。';
  };
  const assertControlCommandsAllowed = () => {
    if (!input.protocolGate) return;
    const verdictValue = input.protocolGate.latest();
    if (!verdictValue || !verdictValue.controlCommandsAllowed) throw new Error('PROTOCOL_UPGRADE_REQUIRED');
  };
  const gateAllowsControl = () => (input.protocolGate ? input.protocolGate.latest()?.controlCommandsAllowed === true : true);
  const applyProtocolVerdict = () => {
    if (!input.protocolGate || !model) return;
    const verdictValue = input.protocolGate.latest();
    model.capabilities.canCancel = gateAllowsControl();
    model.capabilities.canSteer = gateAllowsControl();
    model.protocolNotice = verdictValue ? protocolNoticeFor(verdictValue.mode) : protocolNoticeFor('unknown_schema');
    notify();
  };
  model = createConversationViewModel({
    scope: { ...identity, spaceId: input.spaceId },
    messages: input.messages,
    pendingInteractions: input.pendingInteractions,
    // W25: the product conversation always assembles the attachment pipeline
    // when the route can authenticate; ConversationScreen still hides the
    // entries whenever no `attachments` prop reaches it.
    // W29: same contract for the voice surface — canVoice is declared here
    // and the screen still hides the hold-to-talk entry until a `dictation`
    // prop (native audio port) reaches it.
    // W37: control commands additionally require a 'full' protocol verdict.
    capabilities: { canCancel: gateAllowsControl(), canSteer: gateAllowsControl(), canAttach: true, canVoice: true },
    commands: {
      cancel: async (runID, expectedRevision = 0) => {
        assertControlCommandsAllowed();
        const captured = input.scope.capture();
        await input.executions.command(runID, { action: 'cancel', expected_revision: expectedRevision }, captured.signal);
        if (!input.scope.accept(captured.generation)) throw new Error('SCOPE_CHANGED');
      },
      steer: async (runID, text, expectedRevision = 0) => {
        assertControlCommandsAllowed();
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
  // W37: reflect a verdict that settled before this model existed and follow
  // later handshake completions (a late 'full' verdict reopens control
  // commands; a degrading window closes them again).
  if (input.protocolGate) {
    const initialVerdict = input.protocolGate.latest();
    model.capabilities.canCancel = initialVerdict?.controlCommandsAllowed === true;
    model.capabilities.canSteer = initialVerdict?.controlCommandsAllowed === true;
    model.protocolNotice = protocolNoticeFor(initialVerdict?.mode ?? 'unknown_schema');
    input.protocolGate.subscribe(() => applyProtocolVerdict());
  }
  // W12 lifecycle recovery (production assembly): the controller is built on
  // the same ports the screen drives through AppState. status consults the
  // durable snapshot API; refreshHistory re-projects it; subscribe reopens
  // the stream from the last committed W09 cursor. A missing run is already
  // terminal, and there is no start port anywhere, so a 404 can never create
  // a replacement session.
  model.recovery = createExecutionRecovery({
    ports: {
      status: async () => {
        const runID = model.execution?.runID;
        if (!runID) return { terminal: true };
        if (!input.executions.snapshot) return { terminal: false };
        const snapshot = await input.executions.snapshot(runID, input.scope.capture().signal);
        return { terminal: isTerminalExecutionStatus(snapshot.execution.execution_status) };
      },
      refreshHistory: async () => {
        const runID = model.execution?.runID;
        if (runID) await reloadProjection(runID).catch(() => undefined);
      },
      subscribe: async () => {
        const runID = model.execution?.runID;
        if (runID) openStream(runID, input.scope.capture());
      },
    },
    scope: input.scope,
  });
  if (input.requestStorage) {
    void input.requestStorage.getLatest().then((record) => {
      if (!record) return;
      latestRequestID = record.requestID;
      updateExecution({ runID: record.runID ?? '', requestID: record.requestID, status: record.status, reason: record.reason });
      if (record.status === 'unknown' || record.status === 'pending' || record.status === 'dispatching') {
        void refreshRequest(record.requestID).catch(() => undefined);
      }
      if (record.runID) void loadProjection(record.runID).catch(() => undefined);
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
    snapshot: (runID, signal) => scoped((activeSignal) => executions.snapshot(runID, activeSignal), signal),
    // W37 protocol handshake: the raw capabilities envelope over the SAME
    // authenticated transport (auth-session refresh semantics included).
    capabilities: (signal) => scoped((activeSignal) => client.request(serverCapabilitiesRequest()), signal ?? undefined),
    stream: async (runID, lastEventID, onEvent, signal) => {
      const request = executionEventsRequest(runID, lastEventID);
      const captured = input.scope.capture();
      const activeSignal = signal ?? captured.signal;
      const result = input.authSession?.transport.sendStream
        ? await input.authSession.transport.sendStream({
          method: request.method,
          url: `${input.origin}${request.path}`,
          headers: { accept: 'text/event-stream', ...(request.headers ?? {}) },
          signal: activeSignal,
        })
        : await (async () => {
          const transport = new (await import('../platform/stream-transport')).ExecutionSSEParser();
          const response = await fetch(`${input.origin}${request.path}`, { method: request.method, headers: { accept: 'text/event-stream', ...(request.headers ?? {}), authorization: `Bearer ${input.credential.accessToken}` }, signal: activeSignal });
          if (!response.ok || !response.body) throw new Error(`execution stream HTTP ${response.status}`);
          const reader = response.body.getReader();
          try {
            while (true) {
              const chunk = await reader.read();
              for (const frame of transport.push(chunk.value ?? new Uint8Array(), chunk.done)) onEvent(frame.data);
              if (chunk.done) break;
            }
          } finally { reader.releaseLock(); }
          return undefined;
        })();
      if (result) {
        if (result.status < 200 || result.status >= 300) throw new Error(`execution stream HTTP ${result.status}`);
        const parser = new ExecutionSSEParser();
        for await (const chunk of result.chunks) {
          for (const frame of parser.push(new TextEncoder().encode(chunk))) onEvent(frame.data);
        }
        for (const frame of parser.push(new Uint8Array(), true)) onEvent(frame.data);
      }
      if (!input.scope.accept(captured.generation)) throw new Error('SCOPE_CHANGED');
    },
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
