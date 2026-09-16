import { BridgeError } from './protocol.ts';

export type ControlCommand =
  | { action: 'cancel'; commandID: string; runID: string; externalID: string; epoch: number }
  | { action: 'steer'; commandID: string; runID: string; text: string; epoch: number }
  | { action: 'submitInteraction'; commandID: string; runID: string; externalPendingID: string; actionValue: 'approve' | 'reject'; argsHash: string; credentialVersion: number; expectedRevision: number };

export type ControlObservation = { processState: string; fresh: boolean; epoch?: number };

export interface ControlPort {
  command(command: ControlCommand, options?: { signal?: AbortSignal }): Promise<{ accepted: boolean }>;
  observe(externalID: string, options?: { signal?: AbortSignal }): Promise<ControlObservation>;
}

const commandKeys: Record<ControlCommand['action'], string[]> = {
  cancel: ['action', 'commandID', 'epoch', 'externalID', 'runID'],
  steer: ['action', 'commandID', 'epoch', 'runID', 'text'],
  submitInteraction: ['action', 'actionValue', 'argsHash', 'commandID', 'credentialVersion', 'expectedRevision', 'externalPendingID', 'runID'],
};

export function encodeControlCommand(command: ControlCommand): string {
  if (!command || !command.action || !command.commandID || !command.runID) throw new BridgeError('INVALID_COMMAND');
  if (command.action !== 'submitInteraction' && (!Number.isSafeInteger(command.epoch) || command.epoch < 1)) throw new BridgeError('INVALID_COMMAND');
  if (command.action === 'cancel' && !command.externalID) throw new BridgeError('INVALID_COMMAND');
  if (command.action === 'steer' && !command.text.trim()) throw new BridgeError('INVALID_COMMAND');
  if (command.action === 'submitInteraction' && (!command.externalPendingID || !/^[a-f0-9]{64}$/i.test(command.argsHash) || !Number.isSafeInteger(command.credentialVersion) || command.credentialVersion < 1 || !Number.isSafeInteger(command.expectedRevision) || command.expectedRevision < 0)) throw new BridgeError('INVALID_COMMAND');
  const payload = Object.fromEntries(Object.entries(command).sort(([a], [b]) => a.localeCompare(b)));
  const keys = Object.keys(payload).sort();
  if (keys.join(',') !== commandKeys[command.action].slice().sort().join(',')) throw new BridgeError('INVALID_COMMAND');
  return JSON.stringify({ version: 1, operation: 'control', payload });
}

export async function cancelAndObserve(
  port: ControlPort,
  command: Extract<ControlCommand, { action: 'cancel' }>,
  timeoutMs = 10_000,
  signal?: AbortSignal,
): Promise<{ state: 'confirmed' | 'unconfirmed' | 'unknown'; observation?: ControlObservation }> {
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) throw new BridgeError('INVALID_COMMAND');
  if (signal?.aborted) throw new BridgeError('BRIDGE_CANCELLED');
  await port.command(command, { signal });
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (signal?.aborted) throw new BridgeError('BRIDGE_CANCELLED');
    const observation = await port.observe(command.externalID, { signal });
    const state = observation.processState.toLowerCase();
    if (observation.fresh && (observation.epoch === undefined || observation.epoch === command.epoch) && (state === 'exited' || state === 'destroyed')) return { state: 'confirmed', observation };
    if (state === 'unknown' || state === 'not_found') return { state: 'unknown', observation };
    await new Promise(resolve => setTimeout(resolve, Math.min(25, Math.max(1, deadline - Date.now()))));
  }
  return { state: 'unconfirmed' };
}
