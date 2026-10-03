import { useColorScheme } from 'react-native';
import { Stack } from 'expo-router';
import { initialWindowMetrics, SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { nativeTokens } from '../../../../packages/design-tokens/src/mobile/native-tokens.ts';

/** The router hosts one Runtime-selected mobile surface at a time. */
export default function RootLayout() {
  const scheme = useColorScheme();
  // react-native-screens containers are opaque and would otherwise stay white on iOS in dark mode.
  const bg = nativeTokens.colors[scheme === 'dark' ? 'dark' : 'light'].bg;
  return (
    // Keep the SDK57 router shell provider explicit; native initial metrics prevent a zero-inset first frame.
    <SafeAreaProvider initialMetrics={initialWindowMetrics}>
      <SafeAreaView edges={['top', 'bottom']} style={{ flex: 1, backgroundColor: bg }}>
        <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: bg } }} />
      </SafeAreaView>
    </SafeAreaProvider>
  );
}
