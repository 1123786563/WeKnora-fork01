import * as React from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { Stack, useRouter } from 'expo-router';
import { Linking } from 'react-native';
import * as SecureStore from 'expo-secure-store';
import { createOIDCApi } from '@weknora/api-client';
import { useProductAuth } from './session';
import { useMobileHost } from '@/weknora/platform/host';
import { validateLoginCredentials } from './loginValidation';
import { generatePKCE } from '@/utils/oauth';

const NATIVE_PKCE_VERIFIER_KEY = 'weknora:native-oidc:pkce-verifier';
const NATIVE_OIDC_STATE_KEY = 'weknora:native-oidc:state';
const AUTH_RETURN_REDIRECT = 'weknora://oidc';

export default function LoginScreen() {
  const host = useMobileHost();
  const auth = useProductAuth();
  const router = useRouter();
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [oidcLoading, setOidcLoading] = React.useState(false);

  const submit = async () => {
    setLoading(true);
    try {
      if (!host || !email.trim() || !password) {
        setError('Enter your email and password.');
        return;
      }
      const validation = validateLoginCredentials(email, password);
      const validationError = validation.email ?? validation.password;
      if (validationError) {
        setError(validationError);
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

  const startNativeOIDC = async () => {
    if (!host) return;
    setOidcLoading(true);
    try {
      const { verifier, challenge } = await generatePKCE();
      const api = createOIDCApi(async (request) => {
        const response = await fetch(`${host.origin}${request.path}`, { method: request.method });
        if (!response.ok) throw new Error(`oidc_start_${response.status}`);
        return await response.json();
      });
      // The identity provider must call the server. The server validates the
      // provider response, creates a short-lived application code, then
      // redirects to this native marker. Passing the custom scheme as the
      // provider redirect would bypass that callback and cannot be exchanged.
      const providerRedirect = `${host.origin.replace(/\/+$/, '')}/api/v1/auth/oidc/callback`;
      const result = await api.startNative(providerRedirect, challenge, AUTH_RETURN_REDIRECT);
      if (!result.authorization_url) throw new Error('OIDC_AUTHORIZATION_URL_MISSING');
      await SecureStore.setItemAsync(NATIVE_PKCE_VERIFIER_KEY, verifier);
      if (!result.state) throw new Error('OIDC_STATE_MISSING');
      await SecureStore.setItemAsync(NATIVE_OIDC_STATE_KEY, JSON.stringify({ state: result.state, redirect_uri: AUTH_RETURN_REDIRECT, issued_at: Date.now() }));
      await Linking.openURL(result.authorization_url);
    } catch {
      await SecureStore.deleteItemAsync(NATIVE_PKCE_VERIFIER_KEY);
      await SecureStore.deleteItemAsync(NATIVE_OIDC_STATE_KEY);
      setError('Provider sign-in could not be started.');
    } finally { setOidcLoading(false); }
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
      <Pressable accessibilityRole="button" disabled={oidcLoading || loading || !host} onPress={startNativeOIDC} style={[styles.buttonSecondary, (oidcLoading || loading || !host) && styles.disabled]}>
        <Text style={styles.buttonTextSecondary}>{oidcLoading ? 'Opening provider…' : 'Continue with provider'}</Text>
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
  buttonSecondary: { alignItems: 'center', borderColor: '#1f6feb', borderRadius: 8, borderWidth: 1, padding: 14 },
  buttonTextSecondary: { color: '#1f6feb', fontSize: 16, fontWeight: '600' },
});
