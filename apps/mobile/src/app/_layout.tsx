import { Stack } from 'expo-router';
import { initialWindowMetrics, SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';

/** The router hosts one Runtime-selected mobile surface at a time. */
export default function RootLayout() {
  return (
    // Keep the SDK57 router shell provider explicit; native initial metrics prevent a zero-inset first frame.
    <SafeAreaProvider initialMetrics={initialWindowMetrics}>
      <SafeAreaView edges={['top', 'bottom']} style={{ flex: 1 }}>
        <Stack screenOptions={{ headerShown: false }} />
      </SafeAreaView>
    </SafeAreaProvider>
  );
}
