import { ContractError } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export interface SandboxTerminalTicket {
  ticket: string;
  expiresIn: number;
}

function responseRecord(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'invalid terminal ticket response');
  }
  return value as Record<string, unknown>;
}

function sessionPath(sessionId: string): string {
  if (typeof sessionId !== 'string' || sessionId.trim() === '') throw new Error('sessionId must not be empty');
  return encodeURIComponent(sessionId);
}

export function parseSandboxTerminalTicket(value: unknown): SandboxTerminalTicket {
  const envelope = responseRecord(value, 'terminalTicket');
  if (envelope.success !== true) {
    throw new ContractError('terminalTicket.success', 'invalid terminal ticket response');
  }
  const data = responseRecord(envelope.data, 'terminalTicket.data');
  if (typeof data.ticket !== 'string' || data.ticket.trim() === '') {
    throw new ContractError('terminalTicket.data.ticket', 'missing terminal ticket');
  }
  if (!Number.isSafeInteger(data.expires_in) || (data.expires_in as number) <= 0) {
    throw new ContractError('terminalTicket.data.expires_in', 'invalid terminal ticket expiry');
  }
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
