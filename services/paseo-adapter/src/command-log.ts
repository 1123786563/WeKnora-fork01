/** Durable command-log port used by the remote dispatcher.
 *
 * `begin` must atomically persist the started intent and commit it before the
 * caller invokes the provider. It must return `unknown` after a crash or a
 * receipt write failure; callers must never guess that a provider did not
 * start. A durable implementation must also reject the same id with a new
 * payload hash.
 */
export interface CommandLog {
  begin(id: string, hash: string): Promise<'new' | 'unknown' | { externalID: string }>;
  complete(id: string, externalID: string): Promise<void>;
}

export async function dispatchOnce(
  log: CommandLog,
  id: string,
  hash: string,
  start: () => Promise<string>,
): Promise<string> {
  const previous = await log.begin(id, hash);
  if (previous === 'unknown') throw new Error('DISPATCH_UNKNOWN');
  if (previous !== 'new') return previous.externalID;

  // A rejected start intentionally leaves the durable intent untouched. A
  // rejected complete also leaves the intent recoverable as unknown.
  const externalID = await start();
  await log.complete(id, externalID);
  return externalID;
}
