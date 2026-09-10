import { Redirect } from 'expo-router';
import { ActivityIndicator, View } from 'react-native';
import { useMobileRuntime } from '../src/runtime.tsx';

export default function IndexRoute() {
  const runtime = useMobileRuntime();
  if (runtime.hydrating) return <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}><ActivityIndicator /></View>;
  return <Redirect href={runtime.credential?.kind === 'bearer' ? '/(app)/knowledge' : '/(auth)/login'} />;
}
