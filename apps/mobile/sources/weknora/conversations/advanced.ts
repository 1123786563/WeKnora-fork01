/**
 * W32 — advanced interaction capability ports.
 *
 * The seven Happy-origin advanced operations (goal / fork / side_chat /
 * archive / rewind / duplicate / terminal) are ports: each maps to an
 * already-verified product or remote operation, and an operation whose
 * capability is not proven refuses with CAPABILITY_UNAVAILABLE instead of
 * falling back to a shell command or another driver. `call` only receives the
 * fixed operation plus server-produced target references — the layer below
 * this gate owns no generic RPC surface.
 *
 * Driver map (H26 routing, remote flavor):
 * - goal       → sync.sessionGoalAction        (remote goal command)
 * - fork       → sync.forkAndSpawn            (copies approved history; new session id)
 * - side_chat  → sync.spawnSideChat           (forked child flagged isSideChat)
 * - archive    → sync.sessionArchive          (lifecycle archive — NEVER cancel/abort)
 * - rewind     → sync.listRewindPoints + forkAndSpawn (read points, fork from one)
 * - duplicate  → sync.listRewindPoints + forkAndSpawn (fork from selected item)
 * - terminal   → product POST /api/v1/sessions/:id/sandbox/terminal-ticket
 *
 * Rewind/duplicate are non-destructive by construction: both spawn a NEW
 * session from a selected point. They never claim to roll back external side
 * effects that already happened.
 *
 * Fork/side_chat copy approved history or references only. Pending approvals,
 * budget balances and execution leases are server-side admission state and are
 * never copied by the client.
 *
 * Product flavor (verified against packages/api-client — chat/sessions offers
 * create/update/remove/clear/pin/messages and executions offers
 * start/lookup/command/snapshot/stream/decide): only the sandbox terminal
 * ticket entry exists today. The other six stay CAPABILITY_UNAVAILABLE on the
 * product side rather than being "completed" by a remote fallback.
 */
import {
  createJsonTransport,
  createSandboxTerminalApi,
  createWeKnoraClient,
  type BearerCredential,
  type FetchLike,
} from '@weknora/api-client';
import { terminalWebSocketUrl } from '@weknora/domain/sandbox/terminal';

export type AdvancedOperation = 'goal' | 'fork' | 'side_chat' | 'archive' | 'rewind' | 'duplicate' | 'terminal';

export type AdvancedCapabilities = Partial<Record<AdvancedOperation, boolean>>;

/** The handler only ever receives the fixed operation it was gated for. */
export type AdvancedOperationCall = (operation: AdvancedOperation) => Promise<void>;

/**
 * Capability gate for every advanced operation. Refuses — synchronously,
 * before the handler can start — when `capabilities[op]` is not exactly true.
 */
export async function invokeAdvanced(
  operation: AdvancedOperation,
  capabilities: AdvancedCapabilities,
  call: AdvancedOperationCall,
): Promise<void> {
  if (capabilities[operation] !== true) throw new Error('CAPABILITY_UNAVAILABLE');
  await call(operation);
}

/** Remote-flavor driver record — the interaction matrix mirrors this map. */
export interface AdvancedDriver {
  operation: AdvancedOperation;
  driver: string;
  /** True only for drivers that claim to undo already-happened external effects. */
  claimsExternalRollback: boolean;
  /** True when the driver is (or degenerates into) a cancel/abort command. */
  cancelCommand: boolean;
}

const remoteDriver = (
  operation: AdvancedOperation,
  driver: string,
  flags: Partial<Pick<AdvancedDriver, 'claimsExternalRollback' | 'cancelCommand'>> = {},
): AdvancedDriver => ({
  operation,
  driver,
  claimsExternalRollback: flags.claimsExternalRollback ?? false,
  cancelCommand: flags.cancelCommand ?? false,
});

/** Verified remote (Happy CLI) drivers for the seven operations. */
export const REMOTE_ADVANCED_DRIVER: Record<AdvancedOperation, AdvancedDriver> = {
  goal: remoteDriver('goal', 'sync.sessionGoalAction'),
  fork: remoteDriver('fork', 'sync.forkAndSpawn'),
  side_chat: remoteDriver('side_chat', 'sync.spawnSideChat'),
  archive: remoteDriver('archive', 'sync.sessionArchive', { cancelCommand: false }),
  rewind: remoteDriver('rewind', 'sync.listRewindPoints+forkAndSpawn'),
  duplicate: remoteDriver('duplicate', 'sync.listRewindPoints+forkAndSpawn'),
  terminal: remoteDriver('terminal', 'product.sandboxTerminalTicket'),
};

/** Session facts the remote flavor derives its capabilities from. */
export interface RemoteAdvancedFacts {
  /** Goal commands are dispatchable (goal bar visible / goal state present). */
  goalActions: boolean;
  /** A verified fork source exists (machine + directory + claude/codex ids). */
  forkSource: boolean;
  /** The remote session is present — archive operates on a real session. */
  sessionPresent: boolean;
  /** The flavor exposes rewind points (Claude sessions / Codex threads). */
  rewindSupported: boolean;
  /** The flavor exposes duplicate-from-item. */
  duplicateSupported: boolean;
}

/**
 * Remote (Happy CLI) capabilities. `terminal` is deliberately absent: the
 * remote machine terminal is a different provider from the product sandbox
 * terminal (H33 — paths and tickets must not mix), so a remote session alone
 * never unlocks the product terminal operation.
 */
export function deriveRemoteAdvancedCapabilities(facts: RemoteAdvancedFacts): AdvancedCapabilities {
  return {
    goal: facts.goalActions,
    fork: facts.forkSource,
    side_chat: facts.forkSource,
    archive: facts.sessionPresent,
    rewind: facts.rewindSupported,
    duplicate: facts.duplicateSupported,
  };
}

/** Product-flavor facts: an authenticated product transport plus its session. */
export interface ProductAdvancedFacts {
  authenticated: boolean;
  sessionId: string;
}

/**
 * Product capabilities. Only the sandbox terminal entry is a verified product
 * endpoint today; the six Happy-origin operations return no capability so
 * they refuse at the gate (no remote fallback, no simulated success).
 */
export function deriveProductAdvancedCapabilities(facts: ProductAdvancedFacts): AdvancedCapabilities {
  if (!facts.authenticated || facts.sessionId.trim() === '') return {};
  return { terminal: true };
}

/** Server-produced terminal target reference (scoped WS url + ticket expiry). */
export interface SandboxTerminalTarget {
  webSocketUrl: string;
  ticketExpiresIn: number;
}

export interface SandboxTerminalOperationInput {
  origin: string;
  sessionId: string;
  credential: BearerCredential;
  fetcher?: FetchLike;
  /** Receives the server-produced target reference once the ticket is issued. */
  onTarget?(target: SandboxTerminalTarget): void;
}

/**
 * The 'terminal' operation port. Reuses the existing verified sandbox
 * terminal-ticket entry (POST /api/v1/sessions/:id/sandbox/terminal-ticket
 * through @weknora/api-client) over the product bearer. The issued ticket is
 * one-time and short-term (server TTL); the scoped WS url embeds the ticket
 * — never the bearer. No generic shell RPC is exposed through this port.
 */
export function createSandboxTerminalOperation(input: SandboxTerminalOperationInput): AdvancedOperationCall {
  // Same product-transport pattern as createProductExecutionApi: the bearer
  // rides only this authenticated client, never the produced target.
  const transport = createJsonTransport(input.fetcher ?? fetch);
  const client = createWeKnoraClient({
    baseURL: input.origin,
    transport: {
      send: (request) => transport.send({
        ...request,
        headers: { ...request.headers, authorization: `Bearer ${input.credential.accessToken}` },
      }),
    },
  });
  const api = createSandboxTerminalApi((request) => client.request(request));
  return async (operation) => {
    if (operation !== 'terminal') throw new Error('OPERATION_MISMATCH');
    const ticket = await api.issueTicket(input.sessionId);
    const target: SandboxTerminalTarget = {
      webSocketUrl: terminalWebSocketUrl(input.origin, input.sessionId, ticket.ticket),
      ticketExpiresIn: ticket.expiresIn,
    };
    input.onTarget?.(target);
  };
}
