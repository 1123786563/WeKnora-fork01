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
  create(input: { cwd: string; prompt: string; provider: string }, options?: { signal?: AbortSignal }): Promise<{ id: string }>;
  observe(id: string): Promise<{ state: string }>;
  cancel(id: string): Promise<void>;
}

export interface BridgeOptions {
  timeoutMs?: number;
  signal?: AbortSignal;
  /** Trusted server-side admission; callers must not construct this from request JSON. */
  admit?: (command: StartCommand) => Promise<void>;
}

export class BridgeError extends Error {
  constructor(readonly code: string, message = code) {
    super(message);
    this.name = 'BridgeError';
  }
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
