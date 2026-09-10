import type { ClientRequest } from '../client.ts';

export interface SandboxTerminalTicket {
  ticket: string;
  expiresIn: number;
}

function sessionPath(sessionId: string): string {
  if (typeof sessionId !== 'string' || sessionId.trim() === '') throw new Error('sessionId must not be empty');
  return encodeURIComponent(sessionId);
}

export function parseSandboxTerminalTicket(value: unknown): SandboxTerminalTicket {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('invalid terminal ticket response');
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw new Error('invalid terminal ticket response');
  }
  const data = envelope.data as Record<string, unknown>;
  if (typeof data.ticket !== 'string' || data.ticket.trim() === '') throw new Error('missing terminal ticket');
  if (!Number.isSafeInteger(data.expires_in) || (data.expires_in as number) <= 0) throw new Error('invalid terminal ticket expiry');
  return { ticket: data.ticket, expiresIn: data.expires_in as number };
}

export function createSandboxTerminalApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async issueTicket(sessionId: string, signal?: AbortSignal): Promise<SandboxTerminalTicket> {
      return parseSandboxTerminalTicket(await request({
        method: 'POST', path: `/api/v1/sessions/${sessionPath(sessionId)}/sandbox/terminal-ticket`, body: {},
        ...(signal === undefined ? {} : { signal }),
      }));
    },
  };
}

export type SandboxTerminalApi = ReturnType<typeof createSandboxTerminalApi>;
