import * as React from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { Stack, useRouter } from 'expo-router';
import { createMobileHost, useMobileHost, useSetMobileHost } from '@/weknora/platform/host';
import { nativeOriginStorage } from '@/weknora/platform/native-origin-storage';

export default function ServerConfigScreen() {
  const router = useRouter();
  const currentHost = useMobileHost();
  const setMobileHost = useSetMobileHost();
  const [origin, setOrigin] = React.useState(currentHost?.origin ?? '');
  const [error, setError] = React.useState<string | null>(null);

  const save = async () => {
    try {
      const selected = createMobileHost(origin);
      await nativeOriginStorage.write(selected.origin);
      setMobileHost(selected);
      setError(null);
      router.replace('/(app)/login');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'INVALID_SERVER');
    }
  };

  return (
    <View style={styles.container}>
      <Stack.Screen options={{ title: 'Connect to WeKnora' }} />
      <Text style={styles.title}>WeKnora server</Text>
      <Text style={styles.description}>Enter the address of your WeKnora server to continue.</Text>
      <TextInput
        accessibilityLabel="WeKnora server address"
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="url"
        onChangeText={(value) => { setOrigin(value); setError(null); }}
        placeholder="https://weknora.example.com"
        style={styles.input}
        value={origin}
      />
      {error ? <Text style={styles.error}>{error}</Text> : null}
      <Pressable accessibilityRole="button" onPress={save} style={styles.button}>
        <Text style={styles.buttonText}>Continue</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, gap: 16, justifyContent: 'center', padding: 24 },
  title: { fontSize: 28, fontWeight: '700' },
  description: { color: '#666', fontSize: 16 },
  input: { borderColor: '#bbb', borderRadius: 8, borderWidth: 1, fontSize: 16, padding: 12 },
  error: { color: '#b00020' },
  button: { alignItems: 'center', backgroundColor: '#1f6feb', borderRadius: 8, padding: 14 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
