import type {
  NativeEvent,
  NativeFailure,
  NativeOutcome,
  NativePendingReference,
  NativePublicArtifact,
  NativePublicUsage,
} from '@weknora/contracts/chat/native';

export interface NativeRunScope {
  tenant_id: string;
  session_id: string;
  run_id: string;
}

export interface NativeAttemptState {
  id: string;
  text: string;
  reasoning: string;
  replaced_by?: string;
  finished: boolean;
}

export interface NativeToolState {
  call_id: string;
  plan_version?: number;
  tool_name?: string;
  outcome?: NativeOutcome;
}

export interface NativePendingState extends NativePendingReference {
  call_id: string;
  plan_version: number;
  args_hash: string;
  expires_at: string;
  wait_kind: string;
}

export interface NativeTerminalState {
  status: 'succeeded' | 'failed' | 'cancelled';
}

export interface NativeRunState {
  scope: NativeRunScope;
  cursor: bigint;
  resync_required: boolean;
  status: string;
  active_attempt_id?: string;
  attempts: Readonly<Record<string, NativeAttemptState>>;
  text: string;
  reasoning: string;
  tools: Readonly<Record<string, NativeToolState>>;
  pending: Readonly<Record<string, NativePendingState>>;
  usage: Readonly<Record<string, NativePublicUsage>>;
  artifacts: Readonly<Record<string, NativePublicArtifact>>;
  error?: NativeFailure;
  terminal?: NativeTerminalState;
}

export function shouldApplySequence(last: bigint, incoming: bigint): 'apply' | 'duplicate' | 'resync' {
  if (incoming <= last) return 'duplicate';
  return incoming === last + 1n ? 'apply' : 'resync';
}

export function createNativeRunState(scope: NativeRunScope, cursor = 0n): NativeRunState {
  return {
    scope: { ...scope }, cursor, resync_required: false, status: 'queued', attempts: {}, text: '', reasoning: '', tools: {}, pending: {}, usage: {}, artifacts: {},
  };
}

function sameScope(scope: NativeRunScope, event: NativeEvent): boolean {
  return scope.tenant_id === event.tenant_id && scope.session_id === event.session_id && scope.run_id === event.run_id;
}

function byteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}

function attempt(state: NativeRunState, id: string): NativeAttemptState {
  return state.attempts[id] ?? { id, text: '', reasoning: '', finished: false };
}

function withCursor(state: NativeRunState, event: NativeEvent): NativeRunState {
  return { ...state, cursor: BigInt(event.seq) };
}

function activeText(state: NativeRunState, activeAttemptID: string | undefined): Pick<NativeRunState, 'text' | 'reasoning'> {
  if (activeAttemptID === undefined) return { text: '', reasoning: '' };
  const current = state.attempts[activeAttemptID];
  return { text: current?.text ?? '', reasoning: current?.reasoning ?? '' };
}

export function reduceNativeEvent(state: NativeRunState, event: NativeEvent): NativeRunState {
  if (!sameScope(state.scope, event)) return state;
  const sequence = BigInt(event.seq);
  if (state.resync_required || shouldApplySequence(state.cursor, sequence) === 'duplicate') return state;
  if (shouldApplySequence(state.cursor, sequence) === 'resync') return { ...state, resync_required: true };

  let next = withCursor(state, event);
  switch (event.kind) {
    case 'run.status': {
      next = { ...next, status: event.payload.status };
      if (event.payload.status === 'succeeded' || event.payload.status === 'failed' || event.payload.status === 'cancelled') {
        next = { ...next, terminal: { status: event.payload.status } };
      }
      return next;
    }
    case 'attempt.started': {
      if (event.attempt_id === undefined) return next;
      const current = attempt(state, event.attempt_id);
      const attempts = { ...state.attempts, [event.attempt_id]: { ...current, finished: false } };
      return { ...next, attempts, active_attempt_id: event.attempt_id, ...activeText({ ...next, attempts }, event.attempt_id) };
    }
    case 'attempt.replaced': {
      const old = attempt(state, event.payload.replaces_attempt_id);
      const attempts: Record<string, NativeAttemptState> = { ...state.attempts, [old.id]: { ...old, ...(event.attempt_id === undefined ? {} : { replaced_by: event.attempt_id }) } };
      if (event.attempt_id !== undefined) attempts[event.attempt_id] = attempt(state, event.attempt_id);
      const activeAttemptID = event.attempt_id;
      return { ...next, attempts, active_attempt_id: activeAttemptID, ...activeText({ ...next, attempts }, activeAttemptID) };
    }
    case 'attempt.finished': {
      if (event.attempt_id === undefined) return next;
      const current = attempt(state, event.attempt_id);
      const attempts = { ...state.attempts, [event.attempt_id]: { ...current, finished: true } };
      return { ...next, attempts, ...activeText({ ...next, attempts }, state.active_attempt_id) };
    }
    case 'text.delta':
    case 'reasoning.delta': {
      if (event.attempt_id === undefined || event.attempt_id !== state.active_attempt_id) return next;
      const current = attempt(state, event.attempt_id);
      const field = event.kind === 'text.delta' ? 'text' : 'reasoning';
      if (byteLength(current[field]) !== event.payload.offset) return { ...state, resync_required: true };
      const updated = { ...current, [field]: current[field] + event.payload.text };
      const attempts = { ...state.attempts, [event.attempt_id]: updated };
      return { ...next, attempts, ...activeText({ ...next, attempts }, event.attempt_id) };
    }
    case 'tool.planned': {
      const previous = state.tools[event.payload.call_id] ?? { call_id: event.payload.call_id };
      return { ...next, tools: { ...state.tools, [event.payload.call_id]: { ...previous, plan_version: event.payload.plan_version, tool_name: event.payload.tool_name } } };
    }
    case 'tool.result': {
      const previous = state.tools[event.payload.call_id] ?? { call_id: event.payload.call_id };
      return { ...next, tools: { ...state.tools, [event.payload.call_id]: { ...previous, outcome: event.payload.outcome } } };
    }
    case 'decision.required':
      return {
        ...next,
        pending: {
          ...state.pending,
          [event.payload.pending.pending_id]: {
            ...event.payload.pending,
            call_id: event.payload.call_id,
            plan_version: event.payload.plan_version,
            args_hash: event.payload.args_hash,
            expires_at: event.payload.expires_at,
            wait_kind: event.payload.wait_kind,
          },
        },
      };
    case 'usage.observed':
      return { ...next, usage: { ...state.usage, [event.payload.usage.observation_id]: event.payload.usage } };
    case 'artifact.available':
      return { ...next, artifacts: { ...state.artifacts, [event.payload.artifact.id]: event.payload.artifact } };
    case 'error':
      return { ...next, error: event.payload.failure };
  }
}
