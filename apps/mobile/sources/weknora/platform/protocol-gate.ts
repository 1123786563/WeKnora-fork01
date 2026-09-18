import { CLIENT_PROTOCOL_VERSION, clientGate, type ClientGateVerdict, type WorkbenchCapabilitySnapshot } from '@weknora/domain/mobile';

/**
 * W37 carry-forward (W36 review Important): the mobile side of the protocol
 * compatibility window. The app performs this handshake once per mount from
 * the production session route, classifies itself against the window the
 * server advertises on /system/capabilities, and refuses to issue control
 * commands (cancel/steer) unless the verdict is 'full'. Unknown schema and
 * handshake failures degrade fail-closed to the same refusal while the safe
 * surface (login, reads, upgrade explanation) stays available.
 */

/** The capability request every handshake sends (Viewer-scoped, bearer). */
export function serverCapabilitiesRequest(): { method: 'GET'; path: string } {
  return { method: 'GET', path: '/api/v1/system/capabilities' };
}

/** Unwraps the standard { code, msg, data } envelope; anything else is unknown. */
function unwrapCapabilitiesResponse(payload: unknown): unknown {
  if (typeof payload !== 'object' || payload === null || Array.isArray(payload)) return undefined;
  const row = payload as { code?: unknown; data?: unknown };
  if (row.code !== 0) return undefined;
  return row.data;
}

/**
 * Maps the server's snake_case workbench switches onto the camelCase client
 * mirror. The W36 review recorded the fail-open trap of feeding raw wire
 * keys to normalizeWorkbenchCapabilitySnapshot: `read_enabled: false` would
 * read as unset (= open). This explicit mapping closes that trap; camelCase
 * keys still work for in-process callers.
 */
export function parseWorkbenchWireSnapshot(value: unknown): Partial<WorkbenchCapabilitySnapshot> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return {};
  const row = value as Record<string, unknown>;
  const pick = (wire: string, camel: string): boolean | undefined => {
    if (typeof row[wire] === 'boolean') return row[wire] as boolean;
    if (typeof row[camel] === 'boolean') return row[camel] as boolean;
    return undefined;
  };
  const readEnabled = pick('read_enabled', 'readEnabled');
  const platformAdmission = pick('platform_admission', 'platformAdmission');
  const workerDrain = pick('worker_drain', 'workerDrain');
  return {
    ...(readEnabled === undefined ? {} : { readEnabled }),
    ...(platformAdmission === undefined ? {} : { platformAdmission }),
    ...(workerDrain === undefined ? {} : { workerDrain }),
  };
}

export interface ProtocolGate {
  /** Latest handshake verdict; undefined until the first handshake settles. */
  latest(): ClientGateVerdict | undefined;
  /** Reads /system/capabilities and applies the verdict. Never rejects. */
  handshake(signal?: AbortSignal): Promise<ClientGateVerdict>;
  subscribe(listener: () => void): () => void;
}

const failClosedVerdict: ClientGateVerdict = {
  mode: 'unknown_schema',
  controlCommandsAllowed: false,
  safeSurface: 'login-upgrade',
};

/**
 * Builds the gate over an authenticated capabilities fetcher (the production
 * route uses the product execution API's `capabilities` method, which rides
 * the auth-session transport). Transport failures, non-zero envelopes and
 * unknown capability schemas all degrade to the fail-closed verdict — the
 * app never fabricates a window it was not given.
 */
export function createProtocolGate(
  fetchCapabilities: (signal?: AbortSignal) => Promise<unknown>,
): ProtocolGate {
  const listeners = new Set<() => void>();
  let current: ClientGateVerdict | undefined;
  return {
    latest: () => current,
    async handshake(signal?: AbortSignal): Promise<ClientGateVerdict> {
      let next = failClosedVerdict;
      try {
        const payload = await fetchCapabilities(signal);
        const data = unwrapCapabilitiesResponse(payload);
        next = clientGate(CLIENT_PROTOCOL_VERSION, data);
      } catch {
        next = failClosedVerdict;
      }
      current = next;
      listeners.forEach((listener) => listener());
      return next;
    },
    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}
