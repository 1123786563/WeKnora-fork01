import '../theme.css';
import * as React from 'react';
import { Redirect, Slot, useSegments } from 'expo-router';
import { ThemeProvider, DarkTheme, DefaultTheme } from '@react-navigation/native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { KeyboardProvider } from 'react-native-keyboard-controller';
import { SafeAreaProvider, initialWindowMetrics } from 'react-native-safe-area-context';
import { useColorScheme } from 'react-native';
import { createMobileHost, MobileHostProvider, useMobileHost, useSetMobileHost } from '@/weknora/platform/host';
import { ProductAuthProvider, useProductAuth } from '@/weknora/auth/session';
import { nativeOriginStorage } from '@/weknora/platform/native-origin-storage';
import { registerNativeExecutionStorage } from '@/weknora/platform/native-execution-storage';

export { ErrorBoundary } from 'expo-router';

function configuredProductHost() {
  const origin = process.env.EXPO_PUBLIC_WEKNORA_ORIGIN ?? '';
  try {
    return createMobileHost(origin);
  } catch {
    // Missing or invalid configuration stays inside the server selection entry.
    return null;
  }
}

function ProductHostGate() {
  const host = useMobileHost();
  const auth = useProductAuth();
  // Route content is scoped to the product identity generation. A tenant or
  // account switch tears down voice/remote subscriptions while server-side
  // executions continue independently and can be looked up after remount.
  const scopeGeneration = auth.scope.capture().generation;
  const segments = useSegments();
  const isServerEntry = segments.some((segment) => segment === 'server');

  const isLoginEntry = segments.some((segment) => segment === 'login');
  const isAuthEntry = isLoginEntry || segments.some((segment) => segment === 'auth-return' || segment === 'oidc' || segment === 'invitation');
  if (!host && !isServerEntry) {
    return <Redirect href="/(app)/server" />;
  }
  if (host && !auth.loading && !auth.credential && !isAuthEntry) return <Redirect href="/(app)/login" />;
  return <Slot key={scopeGeneration} />;
}

function ProductHostBootstrap() {
  const current = useMobileHost();
  const setHost = useSetMobileHost();
  const [restoring, setRestoring] = React.useState(!current);
  React.useEffect(() => registerNativeExecutionStorage(), []);
  React.useEffect(() => {
    if (current) { setRestoring(false); return; }
    nativeOriginStorage.read().then((origin) => {
      if (!origin) return;
      try { setHost(createMobileHost(origin)); } catch { /* ignore invalid persisted origin */ }
    }).finally(() => setRestoring(false));
  }, [current, setHost]);
  if (restoring) return null;
  return <ProductAuthProvider><ProductHostGate /></ProductAuthProvider>;
}

/** Native shell boundary. Product identity and network clients are connected explicitly. */
export default function RootLayout() {
  const scheme = useColorScheme();
  const host = React.useMemo(configuredProductHost, []);
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider initialMetrics={initialWindowMetrics}>
        <KeyboardProvider>
          <ThemeProvider value={scheme === 'dark' ? DarkTheme : DefaultTheme}>
            <MobileHostProvider initialHost={host}>
              <ProductHostBootstrap />
            </MobileHostProvider>
          </ThemeProvider>
        </KeyboardProvider>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
