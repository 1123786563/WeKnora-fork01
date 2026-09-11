export async function stopChatRun(
  stopRemote: (() => Promise<void>) | undefined,
  abortLocal: () => void,
  markStopped: () => void,
): Promise<void> {
  const remoteStop = stopRemote?.();
  abortLocal();
  markStopped();
  if (remoteStop) await remoteStop;
}
