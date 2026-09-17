/**
 * W36 native release compatibility window.
 *
 * The server advertises the protocol compatibility window [minimum, maximum]
 * it serves; every app build carries the protocol generation it speaks.
 * `protocolMode` classifies the pair into the compatibility tri-state that
 * drives the client behaviour contract:
 *
 * - 'full'                    — every surface is usable, control commands
 *                               (cancel/steer) included.
 * - 'upgrade_required'        — the app is older than the server's window.
 *                               Safe surface only: login plus the upgrade
 *                               explanation; no control commands.
 * - 'server_upgrade_required' — the app is newer than the server's window
 *                               (server rollback / lagging deploy). Same
 *                               safe surface; no control commands.
 */

export type ProtocolMode = 'full' | 'upgrade_required' | 'server_upgrade_required';

/** The protocol generation this build speaks. */
export const CLIENT_PROTOCOL_VERSION = 3;

/**
 * The window this server release advertises. Expand/contract: the minimum
 * covers the PREVIOUS protocol generation, so the last released app keeps
 * working; contracting the window (dropping an old minimum) is a deliberate
 * release decision taken only after old-app field compatibility has been
 * validated, never an accident of a new maximum.
 */
export const SERVER_PROTOCOL_WINDOW = { minimum: 2, maximum: 3 } as const;

/** Classifies (client, window) into the compatibility tri-state. */
export function protocolMode(client: number, minimum: number, maximum: number): ProtocolMode {
  if (
    !Number.isSafeInteger(client) || !Number.isSafeInteger(minimum) || !Number.isSafeInteger(maximum) ||
    client < 1 || minimum < 1 || maximum < minimum
  ) {
    throw new Error('INVALID_PROTOCOL_RANGE');
  }
  if (client < minimum) return 'upgrade_required';
  if (client > maximum) return 'server_upgrade_required';
  return 'full';
}

/** Control commands are allowed in 'full' mode only. */
export function mayIssueControlCommands(mode: ProtocolMode): boolean {
  return mode === 'full';
}

export interface ServerCapabilityWindow {
  minimum: number;
  maximum: number;
}

/**
 * Parses the compatibility window out of the server capability payload.
 * Unknown or malformed schema yields undefined: an unrecognized capability
 * payload must never be guessed into an allow decision.
 */
export function parseServerCapabilityWindow(value: unknown): ServerCapabilityWindow | undefined {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
  const row = value as { protocol_minimum?: unknown; protocol_maximum?: unknown };
  const { protocol_minimum: minimum, protocol_maximum: maximum } = row;
  if (typeof minimum !== 'number' || typeof maximum !== 'number') return undefined;
  if (!Number.isSafeInteger(minimum) || !Number.isSafeInteger(maximum) || minimum < 1 || maximum < minimum) {
    return undefined;
  }
  return { minimum, maximum };
}

export type ClientGateVerdict = {
  mode: ProtocolMode | 'unknown_schema';
  /** Whether cancel/steer may be sent to this server. */
  controlCommandsAllowed: boolean;
  /** Login always stays; a non-'full' verdict additionally carries the upgrade explanation. */
  safeSurface: 'login' | 'login-upgrade';
};

/**
 * The single decision the client takes before issuing a control command.
 * Unknown capability schema → no control commands, safe login/upgrade
 * surface retained (the app never fabricates a window it was not given).
 */
export function clientGate(clientVersion: number, serverCapabilities: unknown): ClientGateVerdict {
  const window = parseServerCapabilityWindow(serverCapabilities);
  if (!window) {
    return { mode: 'unknown_schema', controlCommandsAllowed: false, safeSurface: 'login-upgrade' };
  }
  const mode = protocolMode(clientVersion, window.minimum, window.maximum);
  return {
    mode,
    controlCommandsAllowed: mayIssueControlCommands(mode),
    safeSurface: mode === 'full' ? 'login' : 'login-upgrade',
  };
}

/**
 * Client-side mirror of the W34 workbench capability switches
 * (`internal/config` `workbench.*`) as published by the server. Semantics
 * mirror the server's nil-safe accessors exactly:
 *
 * - `readEnabled`  ← `workbench.read_enabled`: unset = on. Reads (list/get/
 *   snapshot/events/lookup) answer unless explicitly closed.
 * - `workerDrain`  ← `workbench.worker_drain`: unset = off. ON refuses NEW
 *   admissions in EVERY lane while already-admitted runs finish.
 * - `platformAdmission` ← `workbench.platform_admission`: unset = on.
 *   Closing it refuses NEW platform-target admissions only.
 *
 * One switch never cuts query and cleanup at the same time: admission
 * switches leave read and cleanup paths available (W34 drain contract).
 */
export interface WorkbenchCapabilitySnapshot {
  readEnabled: boolean;
  platformAdmission: boolean;
  workerDrain: boolean;
}

/** Applies the W34 unset-defaults to a partial/absent snapshot. */
export function normalizeWorkbenchCapabilitySnapshot(
  partial?: Partial<WorkbenchCapabilitySnapshot> | null,
): WorkbenchCapabilitySnapshot {
  return {
    readEnabled: partial?.readEnabled !== false,
    platformAdmission: partial?.platformAdmission !== false,
    workerDrain: partial?.workerDrain === true,
  };
}

/** W34 `AreWorkbenchReadsEnabled`: query paths answer unless read_enabled was explicitly closed. */
export function mayQueryRuns(snapshot: Partial<WorkbenchCapabilitySnapshot> | null | undefined): boolean {
  return normalizeWorkbenchCapabilitySnapshot(snapshot).readEnabled;
}

/** W34 admission contract: NEW runs are refused while draining or while platform admission is closed. */
export function mayAdmitNewRun(snapshot: Partial<WorkbenchCapabilitySnapshot> | null | undefined): boolean {
  const caps = normalizeWorkbenchCapabilitySnapshot(snapshot);
  return caps.platformAdmission && !caps.workerDrain;
}

/**
 * W34 cleanup contract: cleanup/reconciliation of EXISTING runs stays
 * available under drain and under closed admission — an admission switch
 * never cuts the cleanup path (only `read_enabled` governs reads, and no
 * switch cuts both).
 */
export function mayCleanUpRuns(_snapshot?: Partial<WorkbenchCapabilitySnapshot> | null): boolean {
  return true;
}
