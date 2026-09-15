import { Redirect, Stack } from 'expo-router';
import { ActivityIndicator } from 'react-native';
import { useMobileRuntime } from '../../src/runtime.tsx';

export default function AppLayout() {
  const runtime = useMobileRuntime();
  if (runtime.hydrating) return <ActivityIndicator />;
  if (runtime.credential.kind !== 'bearer') return <Redirect href="/(auth)/login" />;
  if (!runtime.tenantId) return <Redirect href="../onboarding" />;
  // Every app screen owns its Vue-shaped header. Leaving Expo Router's
  // generated pathname header visible duplicates the title and leaks route
  // placeholders such as `knowledge/[id]` into the business UI.
  return <Stack screenOptions={{ headerShown: false }} />;
}
