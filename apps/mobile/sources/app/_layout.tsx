import '../theme.css';
import * as React from 'react';
import { Redirect, Slot, usePathname, useSegments } from 'expo-router';
import { ThemeProvider, DarkTheme, DefaultTheme } from '@react-navigation/native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { KeyboardProvider } from 'react-native-keyboard-controller';
import { SafeAreaProvider, initialWindowMetrics } from 'react-native-safe-area-context';
import { Platform, Text, useColorScheme } from 'react-native';
import { createMobileHost, MobileHostProvider, useMobileHost, useSetMobileHost } from '@/weknora/platform/host';
import { ProductAuthProvider, useProductAuth } from '@/weknora/auth/session';
import { nativeOriginStorage } from '@/weknora/platform/native-origin-storage';
import { hasNativeExecutionStorageProvider } from '@/weknora/platform/native-execution-storage';
import { NotificationRouter } from '@/weknora/notifications/NotificationRouter';

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
  // Hooks must all run before any early return（Rules of Hooks——设备实测教训）
  const pathname = usePathname();
  const isServerEntry = segments.some((segment) => segment === 'server');

  const isLoginEntry = segments.some((segment) => segment === 'login');
  const isAuthEntry = isLoginEntry || segments.some((segment) => segment === 'auth-return' || segment === 'oidc' || segment === 'invitation');
  if (!host && !isServerEntry) {
    return <Redirect href="/(app)/server" />;
  }
  // 凭据读取期间不渲染业务路由：legacy (app)/index 依赖 Happy AuthProvider，
  // 身份未定就渲染会崩（设备实测）——loading 是 gate 的显式中间态。
  if (host && auth.loading) return null;
  if (host && !auth.credential && !isAuthEntry) return <Redirect href="/(app)/login" />;
  // 产品落地页（MX-013/D-023）：认证且身份已引导的用户访问索引时进入产品壳
  const identity = auth.scope.identity();
  // 实测：expo-router 在索引路由的 segments 为 ['']（而非空数组）——以 pathname 判定
  const atIndex = pathname === '/' || (segments as string[]).every((segment) => segment === '' || segment === undefined);
  if (host && !auth.loading && auth.credential && identity.userId && atIndex) {
    return <Redirect href="/(app)/product" />;
  }
  return <Slot key={scopeGeneration} />;
}

function ProductHostBootstrap() {
  const current = useMobileHost();
  const setHost = useSetMobileHost();
  const [restoring, setRestoring] = React.useState(!current);
  const [storageReady] = React.useState(Platform.OS === 'web' || hasNativeExecutionStorageProvider());
  React.useEffect(() => {
    if (current) { setRestoring(false); return; }
    nativeOriginStorage.read().then((origin) => {
      if (!origin) return;
      try { setHost(createMobileHost(origin)); } catch { /* ignore invalid persisted origin */ }
    }).finally(() => setRestoring(false));
  }, [current, setHost]);
  if (restoring) return null;
  if (!storageReady && Platform.OS !== 'web') return <Text accessibilityRole="alert">Native encrypted execution storage is unavailable on this build.</Text>;
  return <ProductAuthProvider><NotificationRouter><ProductHostGate /></NotificationRouter></ProductAuthProvider>;
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
