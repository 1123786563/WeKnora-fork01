export interface MobileNetworkState {
  isConnected: boolean | null;
  isInternetReachable?: boolean | null;
}

export interface NetworkRecoveryOptions {
  subscribe(listener: (state: MobileNetworkState) => void): () => void;
  onReconnect(): void;
}

function isReachable(state: MobileNetworkState): boolean | null {
  if (state.isConnected === null || state.isInternetReachable == null) return null;
  return state.isConnected && state.isInternetReachable;
}

/** Call the runtime recovery hook only after an observed offline→online edge. */
export function createNetworkRecovery(options: NetworkRecoveryOptions): () => void {
  let previous: boolean | null = null;
  return options.subscribe((state) => {
    const current = isReachable(state);
    if (previous === false && current === true) options.onReconnect();
    if (current !== null) previous = current;
  });
}
