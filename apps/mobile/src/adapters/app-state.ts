/** 原生生命周期 Adapter：react-native AppState('change') 映射为 active/background。
 * Node 测试环境/无 AppState 时订阅为 no-op（fail closed：无自动前台同步，手动刷新仍在）。 */
export function createNativeAppStateLifecycle(): {
  subscribe(listener: (state: 'active' | 'background') => void): () => void;
} {
  return {
    subscribe(listener) {
      try {
        const reactNative = require('react-native') as {
          AppState?: { addEventListener?: (event: 'change', handler: (state: string) => void) => { remove(): void } };
        };
        if (typeof reactNative.AppState?.addEventListener !== 'function') return () => undefined;
        const subscription = reactNative.AppState.addEventListener('change', (state: string) => {
          listener(state === 'active' ? 'active' : 'background');
        });
        return () => subscription.remove();
      } catch {
        return () => undefined;
      }
    },
  };
}
