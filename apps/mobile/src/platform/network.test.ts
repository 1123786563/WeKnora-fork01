import assert from 'node:assert/strict';
import test from 'node:test';
import { createNetworkRecovery, type MobileNetworkState } from './network.ts';

function subscriptionHarness() {
  let listener: ((state: MobileNetworkState) => void) | undefined;
  let removed = false;
  return {
    subscribe(next: (state: MobileNetworkState) => void) {
      listener = next;
      return () => { removed = true; };
    },
    emit(state: MobileNetworkState) {
      listener?.(state);
    },
    wasRemoved() {
      return removed;
    },
  };
}

test('recovers once when a foreground network changes from offline to online', () => {
  const harness = subscriptionHarness();
  let recoveries = 0;
  const dispose = createNetworkRecovery({
    subscribe: harness.subscribe,
    onReconnect: () => { recoveries += 1; },
  });

  harness.emit({ isConnected: true, isInternetReachable: true });
  harness.emit({ isConnected: false, isInternetReachable: false });
  harness.emit({ isConnected: true, isInternetReachable: true });
  harness.emit({ isConnected: true, isInternetReachable: true });

  assert.equal(recoveries, 1);
  dispose();
  assert.equal(harness.wasRemoved(), true);
});

test('treats a connected state with unknown reachability as online', () => {
  const harness = subscriptionHarness();
  let recoveries = 0;
  createNetworkRecovery({
    subscribe: harness.subscribe,
    onReconnect: () => { recoveries += 1; },
  });

  harness.emit({ isConnected: false, isInternetReachable: false });
  harness.emit({ isConnected: true, isInternetReachable: null });

  assert.equal(recoveries, 1);
});
