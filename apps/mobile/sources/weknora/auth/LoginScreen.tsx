import * as React from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { Stack, useRouter } from 'expo-router';
import { useProductAuth } from './session';
import { useMobileHost } from '@/weknora/platform/host';

export default function LoginScreen() {
  const host = useMobileHost();
  const auth = useProductAuth();
  const router = useRouter();
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);

  const submit = async () => {
    setLoading(true);
    try {
      if (!host || !email.trim() || !password) {
        setError('Enter your email and password.');
        return;
      }
      setError(null);
      await auth.login(email.trim(), password);
      router.replace('/(app)');
    } catch {
      setError('Login failed. Check your email and password and try again.');
    } finally {
      setPassword('');
      setLoading(false);
    }
  };

  return (
    <View style={styles.container}>
      <Stack.Screen options={{ title: 'Sign in to WeKnora' }} />
      <Text style={styles.title}>Sign in</Text>
      <Text style={styles.description}>{host?.origin ?? 'Connect to a WeKnora server first.'}</Text>
      <TextInput accessibilityLabel="Email" autoCapitalize="none" autoCorrect={false} keyboardType="email-address" onChangeText={setEmail} placeholder="Email" style={styles.input} value={email} />
      <TextInput accessibilityLabel="Password" autoCapitalize="none" autoCorrect={false} onChangeText={setPassword} placeholder="Password" secureTextEntry style={styles.input} value={password} />
      {error ? <Text accessibilityRole="alert" style={styles.error}>{error}</Text> : null}
      <Pressable accessibilityRole="button" disabled={loading || auth.loading || !host} onPress={submit} style={[styles.button, (loading || auth.loading || !host) && styles.disabled]}>
        <Text style={styles.buttonText}>{loading ? 'Signing in…' : 'Sign in'}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, gap: 16, justifyContent: 'center', padding: 24 },
  title: { fontSize: 28, fontWeight: '700' },
  description: { color: '#666', fontSize: 14 },
  input: { borderColor: '#bbb', borderRadius: 8, borderWidth: 1, fontSize: 16, padding: 12 },
  error: { color: '#b00020' },
  button: { alignItems: 'center', backgroundColor: '#1f6feb', borderRadius: 8, padding: 14 },
  disabled: { opacity: 0.5 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
