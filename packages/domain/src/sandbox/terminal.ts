export type SandboxTerminalStatus =
  | 'paused' | 'connecting' | 'ready' | 'needs_provision' | 'exited'
  | 'no_sandbox' | 'unsupported' | 'idle' | 'unauthorized' | 'error';

export interface TerminalConnectionState {
  status: SandboxTerminalStatus;
  sessionId: string;
  generation: number;
}

export type TerminalTicketState =
  | { status: 'idle' }
  | { status: 'requesting' }
  | { status: 'ready'; ticket: string; expiresAtMs: number }
  | { status: 'consumed' }
  | { status: 'expired' }
  | { status: 'error'; error: string };

export type TerminalTicketEvent =
  | { type: 'request' }
  | { type: 'issued'; ticket: string; expiresIn: number; nowMs: number }
  | { type: 'consume'; nowMs: number }
  | { type: 'clock'; nowMs: number }
  | { type: 'fail'; error: string }
  | { type: 'reset' };

export function initialTerminalTicketState(): TerminalTicketState {
  return { status: 'idle' };
}

/** Clear short-lived ticket material as soon as it is consumed or expires. */
export function reduceTerminalTicketState(
  state: TerminalTicketState,
  event: TerminalTicketEvent,
): TerminalTicketState {
  switch (event.type) {
    case 'request':
      return { status: 'requesting' };
    case 'issued': {
      const ticket = event.ticket.trim();
      if (!ticket || !Number.isSafeInteger(event.expiresIn) || event.expiresIn <= 0 || !Number.isFinite(event.nowMs)) {
        return { status: 'error', error: 'invalid terminal ticket' };
      }
      return { status: 'ready', ticket, expiresAtMs: event.nowMs + event.expiresIn * 1_000 };
    }
    case 'consume':
      if (state.status !== 'ready') return state;
      return event.nowMs >= state.expiresAtMs ? { status: 'expired' } : { status: 'consumed' };
    case 'clock':
      if (state.status !== 'ready' || event.nowMs < state.expiresAtMs) return state;
      return { status: 'expired' };
    case 'fail':
      return { status: 'error', error: event.error.trim() || 'terminal ticket request failed' };
    case 'reset':
      return initialTerminalTicketState();
  }
}

export function terminalWebSocketUrl(origin: string, sessionId: string, ticket: string, basePath = ''): string {
  const parsed = new URL(origin);
  parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
  const prefix = basePath.replace(/^\/+|\/+$/g, '');
  const path = `${prefix ? `/${prefix}` : ''}/api/v1/sessions/${encodeURIComponent(sessionId)}/sandbox/terminal`;
  parsed.pathname = path;
  parsed.search = new URLSearchParams({ ticket }).toString();
  return parsed.toString();
}

export function terminalStatusFromCode(code: string | undefined, provisionAttempted: boolean): SandboxTerminalStatus {
  if (code === 'SANDBOX_NOT_BOUND') return provisionAttempted ? 'no_sandbox' : 'needs_provision';
  if (code === 'SANDBOX_PAUSED') return 'paused';
  if (code === 'TERMINAL_UNSUPPORTED') return 'unsupported';
  if (code === 'IDLE_DISCONNECTED') return 'idle';
  if (code === 'AUTH_REVOKED') return 'unauthorized';
  return 'error';
}

export function canAcceptTerminalFrame(state: TerminalConnectionState, generation: number): boolean {
  return state.generation === generation && state.status !== 'exited' && state.status !== 'unauthorized';
}
