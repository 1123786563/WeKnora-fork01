export type SandboxTerminalStatus =
  | 'paused' | 'connecting' | 'ready' | 'needs_provision' | 'exited'
  | 'no_sandbox' | 'unsupported' | 'idle' | 'unauthorized' | 'error';

export interface TerminalConnectionState {
  status: SandboxTerminalStatus;
  sessionId: string;
  generation: number;
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
