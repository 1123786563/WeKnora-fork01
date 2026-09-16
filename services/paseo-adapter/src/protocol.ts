export const BRIDGE_VERSION = 1 as const;

export interface StartCommand {
  commandID: string;
  runID: string;
  attemptID: string;
  targetID: string;
  workspaceRef: string;
  prompt: string;
  provider: string;
  epoch: number;
  expiresAt: number;
}

export interface PaseoPort {
  readonly supportsCancellation?: boolean;
  create(input: { cwd: string; prompt: string; provider: string }, options?: { signal?: AbortSignal }): Promise<{ id: string }>;
  observe(id: string): Promise<{ state: string }>;
  cancel(id: string): Promise<void>;
}

export interface BridgeOptions {
  timeoutMs?: number;
  signal?: AbortSignal;
  /** Trusted server-side admission; callers must not construct this from request JSON. */
  admission?: TrustedAdmission;
  admissionVerifier?: AdmissionVerifier;
}

export interface AdmissionVerifier {
  serviceIdentity: string;
  signingSecret: string;
  authorizationVersion: number;
}

export interface TrustedAdmission {
  serviceIdentity: string;
  signature: string;
  authorizationVersion: number;
  commandHash: string;
  targetID: string;
  workspaceRef: string;
  workspaceTargetID: string;
  epoch: number;
  verify: (command: StartCommand, admission: TrustedAdmission) => Promise<void>;
}

export class BridgeError extends Error {
  constructor(readonly code: string, message = code) {
    super(message);
    this.name = 'BridgeError';
  }
}

export function canonicalCommand(command: StartCommand): string {
  return JSON.stringify({ commandID: command.commandID, runID: command.runID, attemptID: command.attemptID, targetID: command.targetID, workspaceRef: command.workspaceRef, prompt: command.prompt, provider: command.provider, epoch: command.epoch, expiresAt: command.expiresAt });
}

export function commandHash(command: StartCommand): string {
  return createHash('sha256').update(canonicalCommand(command)).digest('hex');
}

export function commandSignature(command: StartCommand, secret: string): string {
  return `hmac-sha256:${createHmac('sha256', secret).update(commandHash(command)).digest('hex')}`;
}

export function verifyAdmission(command: StartCommand, admission: TrustedAdmission, verifier: AdmissionVerifier): void {
  if (admission.serviceIdentity !== verifier.serviceIdentity || admission.signature !== commandSignature(command, verifier.signingSecret) || admission.authorizationVersion !== verifier.authorizationVersion || admission.commandHash !== commandHash(command) || admission.targetID !== command.targetID || admission.workspaceRef !== command.workspaceRef || admission.workspaceTargetID !== command.targetID || admission.epoch !== command.epoch) throw new BridgeError('ADMISSION_FORBIDDEN');
}

export function encodeStartEnvelope(command: StartCommand): string {
  validateStartCommand(command);
  return JSON.stringify({ version: 1, operation: 'start', payload: JSON.parse(canonicalCommand(command)) });
}

export function decodeStartEnvelope(input: string): StartCommand {
  const value: unknown = JSON.parse(input);
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new BridgeError('INVALID_COMMAND');
  const envelope = value as Record<string, unknown>;
  if (Object.keys(envelope).sort().join(',') !== 'operation,payload,version' || envelope.version !== 1 || envelope.operation !== 'start' || !envelope.payload || typeof envelope.payload !== 'object' || Array.isArray(envelope.payload)) throw new BridgeError('INVALID_COMMAND');
  const payload = envelope.payload as Record<string, unknown>;
  const allowed = ['attemptID', 'commandID', 'epoch', 'expiresAt', 'prompt', 'provider', 'runID', 'targetID', 'workspaceRef'];
  if (Object.keys(payload).sort().join(',') !== allowed.sort().join(',')) throw new BridgeError('INVALID_COMMAND');
  const command = payload as unknown as StartCommand;
  validateStartCommand(command);
  return command;
}

export function validateStartCommand(command: StartCommand): void {
  const fields: Array<keyof StartCommand> = [
    'commandID', 'runID', 'attemptID', 'targetID', 'workspaceRef', 'prompt', 'provider',
  ];
  for (const field of fields) {
    if (typeof command[field] !== 'string' || command[field].trim() === '') {
      throw new BridgeError('INVALID_COMMAND', `missing ${field}`);
    }
  }
  if (!Number.isSafeInteger(command.epoch) || command.epoch < 1) {
    throw new BridgeError('INVALID_COMMAND', 'invalid epoch');
  }
  if (!Number.isFinite(command.expiresAt) || command.expiresAt <= 0) {
    throw new BridgeError('INVALID_COMMAND', 'invalid expiry');
  }
  if (command.prompt.length > 32_000 || command.provider.length > 256) {
    throw new BridgeError('INVALID_COMMAND', 'command exceeds size limit');
  }
  if ([command.commandID, command.runID, command.attemptID, command.targetID, command.workspaceRef].some(value => value.length > 512)) {
    throw new BridgeError('INVALID_COMMAND', 'identifier exceeds size limit');
  }
}
import { createHash, createHmac } from 'node:crypto';
