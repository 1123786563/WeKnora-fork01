import '../theme.css';
import * as React from 'react';
import { Redirect, Slot, useSegments } from 'expo-router';
import { ThemeProvider, DarkTheme, DefaultTheme } from '@react-navigation/native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { KeyboardProvider } from 'react-native-keyboard-controller';
import { SafeAreaProvider, initialWindowMetrics } from 'react-native-safe-area-context';
import { useColorScheme } from 'react-native';
import { createMobileHost, MobileHostProvider, useMobileHost } from '@/weknora/platform/host';

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
  const segments = useSegments();
  const isServerEntry = segments.some((segment) => segment === 'server');

  if (!host && !isServerEntry) {
    return <Redirect href="/(app)/server" />;
  }
  return <Slot />;
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
              <ProductHostGate />
            </MobileHostProvider>
          </ThemeProvider>
        </KeyboardProvider>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
