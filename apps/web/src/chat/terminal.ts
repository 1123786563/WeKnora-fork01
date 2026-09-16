import {
  terminalStatusFromCode,
  terminalWebSocketUrl,
  type SandboxTerminalStatus,
} from '@weknora/domain/sandbox/terminal';
import type { SandboxTerminalTicket } from '@weknora/api-client';

const MAX_TERMINAL_OUTPUT = 1_000_000;

export interface WebTerminalSnapshot {
  status: SandboxTerminalStatus;
  output: string;
  error?: string;
}

export interface WebTerminalController {
  open(input?: { provision?: boolean; signal?: AbortSignal }): Promise<void>;
  sendInput(input: string): void;
  resize(cols: number, rows: number): void;
  close(): void;
  snapshot(): WebTerminalSnapshot;
  subscribe(listener: (next: WebTerminalSnapshot) => void): () => void;
}

export interface WebTerminalSocket {
  binaryType: string;
  onopen: (() => void) | null;
  onmessage: ((event: { data: unknown }) => void) | null;
  onerror: (() => void) | null;
  onclose: (() => void) | null;
  send(data: string | ArrayBuffer | ArrayBufferView | Blob): void;
  close(code?: number, reason?: string): void;
}

export interface WebTerminalControllerOptions {
  sessionId: string;
  wsOrigin: string;
  basePath?: string;
  issueTicket(signal?: AbortSignal): Promise<SandboxTerminalTicket>;
  socketFactory(url: string): WebTerminalSocket;
  errorCopy?: string;
  onSnapshot?(snapshot: WebTerminalSnapshot): void;
}

export interface WebSocketTarget {
  origin: string;
  basePath: string;
}

/** Convert the HTTP API base into the same-origin WebSocket endpoint root. */
export function webSocketTarget(apiBaseUrl: string, fallbackOrigin: string): WebSocketTarget {
  const parsed = new URL(apiBaseUrl || fallbackOrigin, fallbackOrigin);
  const apiSuffix = /\/api\/v1\/?$/;
  const basePath = parsed.pathname.replace(apiSuffix, '').replace(/\/$/, '');
  return { origin: parsed.origin, basePath };
}

function initialSnapshot(): WebTerminalSnapshot {
  return { status: 'idle', output: '' };
}

function readBinary(data: ArrayBuffer | ArrayBufferView): string {
  const bytes = data instanceof ArrayBuffer
    ? new Uint8Array(data)
    : new Uint8Array(data.buffer, data.byteOffset, data.byteLength);
  return new TextDecoder().decode(bytes);
}

function withProvision(url: string, provision: boolean): string {
  const parsed = new URL(url);
  if (provision) parsed.searchParams.set('provision', '1');
  return parsed.toString();
}

function appendOutput(current: string, value: string): string {
  if (!value) return current;
  const next = current + value;
  return next.length > MAX_TERMINAL_OUTPUT ? next.slice(-MAX_TERMINAL_OUTPUT) : next;
}

export function createWebTerminalController(options: WebTerminalControllerOptions) {
  let snapshot = initialSnapshot();
  let socket: WebTerminalSocket | null = null;
  let generation = 0;
  const listeners = new Set<(next: WebTerminalSnapshot) => void>();
  if (options.onSnapshot) listeners.add(options.onSnapshot);

  function publish(next: WebTerminalSnapshot): void {
    snapshot = next;
    for (const listener of listeners) listener(snapshot);
  }

  function currentSocket(expected: number): WebTerminalSocket | null {
    return expected === generation ? socket : null;
  }

  function handleControl(expected: number, raw: string, provisionAttempted: boolean): void {
    if (!currentSocket(expected)) return;
    let frame: Record<string, unknown>;
    try {
      const value: unknown = JSON.parse(raw);
      if (value === null || typeof value !== 'object' || Array.isArray(value)) return;
      frame = value as Record<string, unknown>;
    } catch {
      publish({ ...snapshot, output: appendOutput(snapshot.output, raw) });
      return;
    }
    const type = typeof frame.type === 'string' ? frame.type : '';
    if (type === 'ready') {
      publish({ ...snapshot, status: 'ready', error: undefined });
      return;
    }
    if (type === 'exited') {
      publish({ ...snapshot, status: 'exited' });
      return;
    }
    if (type === 'error') {
      const code = typeof frame.code === 'string' ? frame.code : undefined;
      const message = typeof frame.message === 'string' && frame.message.trim()
        ? frame.message
        : options.errorCopy ?? 'Terminal connection failed';
      publish({ ...snapshot, status: terminalStatusFromCode(code, provisionAttempted), error: message });
    }
  }

  async function open(input: { provision?: boolean; signal?: AbortSignal } = {}): Promise<void> {
    const expected = ++generation;
    const provisionAttempted = input.provision === true;
    socket?.close(1000, 'replaced');
    socket = null;
    publish({ status: 'connecting', output: '', error: undefined });
    let ticket: SandboxTerminalTicket;
    try {
      ticket = await options.issueTicket(input.signal);
    } catch (cause) {
      if (expected !== generation) return;
      const error = cause instanceof Error ? cause.message : options.errorCopy ?? 'Unable to issue terminal ticket';
      publish({ status: 'error', output: '', error });
      throw cause;
    }
    if (expected !== generation) return;
    const url = withProvision(
      terminalWebSocketUrl(options.wsOrigin, options.sessionId, ticket.ticket, options.basePath),
      provisionAttempted,
    );
    const nextSocket = options.socketFactory(url);
    socket = nextSocket;
    nextSocket.binaryType = 'arraybuffer';
    publish({ status: 'connecting', output: '', error: undefined });
    nextSocket.onopen = () => {
      if (!currentSocket(expected)) return;
      publish({ ...snapshot, status: 'connecting', error: undefined });
    };
    nextSocket.onmessage = (event) => {
      if (!currentSocket(expected)) return;
      if (typeof event.data === 'string') {
        handleControl(expected, event.data, provisionAttempted);
        return;
      }
      if (event.data instanceof ArrayBuffer || ArrayBuffer.isView(event.data)) {
        publish({ ...snapshot, output: appendOutput(snapshot.output, readBinary(event.data)) });
        return;
      }
      if (typeof Blob !== 'undefined' && event.data instanceof Blob) {
        void event.data.arrayBuffer().then((buffer) => {
          if (currentSocket(expected)) publish({ ...snapshot, output: appendOutput(snapshot.output, readBinary(buffer)) });
        });
      }
    };
    nextSocket.onerror = () => {
      if (currentSocket(expected)) publish({ ...snapshot, status: 'error', error: options.errorCopy ?? 'Terminal connection failed' });
    };
    nextSocket.onclose = () => {
      if (!currentSocket(expected)) return;
      socket = null;
      if (snapshot.status === 'error' || snapshot.status === 'paused' || snapshot.status === 'no_sandbox' || snapshot.status === 'unsupported' || snapshot.status === 'unauthorized' || snapshot.status === 'exited') return;
      publish({ ...snapshot, status: 'idle' });
    };
  }

  function sendInput(input: string): void {
    if (!socket || snapshot.status !== 'ready') return;
    const value = input.trim();
    if (!value) return;
    socket.send(new TextEncoder().encode(`${value}\r\n`));
  }

  function resize(cols: number, rows: number): void {
    if (!socket || snapshot.status !== 'ready' || !Number.isSafeInteger(cols) || !Number.isSafeInteger(rows) || cols <= 0 || rows <= 0 || cols > 500 || rows > 500) return;
    socket.send(JSON.stringify({ type: 'resize', cols, rows }));
  }

  function close(): void {
    generation += 1;
    const current = socket;
    socket = null;
    current?.close(1000, 'closed by user');
    publish(initialSnapshot());
  }

  return {
    open,
    sendInput,
    resize,
    close,
    snapshot: () => snapshot,
    subscribe(listener: (next: WebTerminalSnapshot) => void): () => void {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}
