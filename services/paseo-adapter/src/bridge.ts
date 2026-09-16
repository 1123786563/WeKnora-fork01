import type { PaseoPort, StartCommand, BridgeOptions } from './protocol.ts';
import { BridgeError, validateStartCommand } from './protocol.ts';

function abortError(signal?: AbortSignal): BridgeError | undefined {
  if (!signal?.aborted) return undefined;
  return new BridgeError('BRIDGE_CANCELLED');
}

async function bounded<T>(work: Promise<T>, options: BridgeOptions): Promise<T> {
  const aborted = abortError(options.signal);
  if (aborted) throw aborted;
  const timeoutMs = options.timeoutMs ?? 30_000;
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) throw new BridgeError('INVALID_COMMAND', 'invalid timeout');
  let timer: ReturnType<typeof setTimeout> | undefined;
  let onAbort: (() => void) | undefined;
  const cancellation = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new BridgeError('BRIDGE_TIMEOUT')), timeoutMs);
    onAbort = () => reject(new BridgeError('BRIDGE_CANCELLED'));
    options.signal?.addEventListener('abort', onAbort, { once: true });
  });
  try {
    return await Promise.race([work, cancellation]);
  } catch (error) {
    if (error instanceof BridgeError) throw error;
    throw new BridgeError('PASEO_UNAVAILABLE');
  } finally {
    if (timer) clearTimeout(timer);
    if (onAbort) options.signal?.removeEventListener('abort', onAbort);
  }
}

/** Execute the only command currently admitted by the Paseo bridge. */
export async function startViaBridge(
  command: StartCommand,
  port: PaseoPort,
  resolveWorkspace: (ref: string) => Promise<string>,
  now = Date.now(),
  options: BridgeOptions = {},
): Promise<{ id: string }> {
  validateStartCommand(command);
  if (command.expiresAt <= now) throw new BridgeError('COMMAND_EXPIRED');
  if (!options.admit) throw new BridgeError('ADMISSION_REQUIRED');
  const startedAt = Date.now();
  const admission = await options.admit(command).catch(() => { throw new BridgeError('ADMISSION_FORBIDDEN'); });
  void admission;
  if (options.signal?.aborted) throw new BridgeError('BRIDGE_CANCELLED');
  const timeoutMs = options.timeoutMs ?? 30_000;
  if (timeoutMs <= 0 || Date.now() - startedAt >= timeoutMs) throw new BridgeError('BRIDGE_TIMEOUT');
  let cwd: string;
  try {
    cwd = await bounded(resolveWorkspace(command.workspaceRef), { ...options, timeoutMs: timeoutMs - (Date.now() - startedAt) });
  } catch (error) {
    if (error instanceof BridgeError && ['BRIDGE_TIMEOUT', 'BRIDGE_CANCELLED'].includes(error.code)) throw error;
    throw new BridgeError('WORKSPACE_FORBIDDEN');
  }
  if (!cwd || cwd.includes('\0')) throw new BridgeError('WORKSPACE_FORBIDDEN');
  if (command.expiresAt <= Date.now()) throw new BridgeError('COMMAND_EXPIRED');
  if (options.signal?.aborted) throw new BridgeError('BRIDGE_CANCELLED');
  if (Date.now() - startedAt >= timeoutMs) throw new BridgeError('BRIDGE_TIMEOUT');
  try {
    return await bounded(port.create({ cwd, prompt: command.prompt, provider: command.provider }, options), { ...options, timeoutMs: timeoutMs - (Date.now() - startedAt) });
  } catch (error) {
    if (error instanceof BridgeError) throw error;
    throw new BridgeError('PASEO_UNAVAILABLE');
  }
}
