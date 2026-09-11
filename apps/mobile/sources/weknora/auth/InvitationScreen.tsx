import * as React from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { Stack, useRouter, useLocalSearchParams } from 'expo-router';
import { createInvitationsApi, createJsonTransport, createWeKnoraClient, ApiError } from '@weknora/api-client';
import { useMobileHost } from '@/weknora/platform/host';
import { useProductAuth } from './session';

type Lookup = { tenantName: string; role: string; expiresAt: string };

function describeError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.message.includes('404') || error.message.includes('not found')) return 'This invitation does not exist.';
    if (error.message.includes('410')) return 'This invitation has expired.';
    return 'Server rejected the request (' + error.message + ').';
  }
  return 'Could not reach the server. Check the connection and try again.';
}

/**
 * Invitation entry: look up a tenant invitation token (public endpoint) and,
 * once the user is signed in, accept it with the product credential. An
 * invalid, revoked, or expired invitation is reported from the server
 * response and never silently treated as membership.
 */
export default function InvitationScreen() {
  const router = useRouter();
  const host = useMobileHost();
  const auth = useProductAuth();
  const params = useLocalSearchParams() as { token?: string | string[] };
  const initialToken = typeof params.token === 'string' ? params.token : Array.isArray(params.token) ? params.token[0] ?? '' : '';
  const [token, setToken] = React.useState(initialToken);
  const [lookup, setLookup] = React.useState<Lookup | null>(null);
  const [joined, setJoined] = React.useState<string | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [busy, setBusy] = React.useState<'lookup' | 'accept' | null>(null);

  const authorizedRequest = React.useCallback(
    (path: string, body: unknown) => {
      if (!host) throw new Error('SERVER_REQUIRED');
      const client = createWeKnoraClient({ baseURL: host.origin, transport: createJsonTransport(fetch) });
      return client.request({
        method: 'POST',
        path,
        body,
        headers: auth.credential?.kind === 'bearer' ? { authorization: 'Bearer ' + auth.credential.accessToken } : undefined,
      });
    },
    [auth.credential, host],
  );

  const submitLookup = async () => {
    const trimmed = token.trim();
    if (!host || !trimmed) {
      setError(host ? 'Enter an invitation token.' : 'Connect to a WeKnora server first.');
      return;
    }
    setBusy('lookup');
    setError(null);
    setLookup(null);
    try {
      const api = createInvitationsApi((input) =>
        createWeKnoraClient({ baseURL: host.origin, transport: createJsonTransport(fetch) }).request(input),
      );
      const response = await api.lookup(trimmed);
      const data = response?.data;
      if (!response?.success || !data?.tenant_id || !data?.expires_at) throw new Error('INVALID_INVITATION');
      setLookup({
        tenantName: data.tenant_name ?? 'Tenant ' + data.tenant_id,
        role: data.role,
        expiresAt: data.expires_at,
      });
    } catch (cause) {
      setError(describeError(cause));
    } finally {
      setBusy(null);
    }
  };

  const submitAccept = async () => {
    const trimmed = token.trim();
    if (!host || !trimmed) return;
    if (!auth.credential) {
      setError('Sign in first, then accept the invitation from this screen.');
      return;
    }
    setBusy('accept');
    setError(null);
    try {
      const response = await authorizedRequest('/api/v1/me/invitations/accept-by-token', { token: trimmed });
      const result = response as { success?: boolean; data?: { tenant_name?: string }; message?: string };
      if (!result?.success) throw new Error(result?.message ?? 'INVITATION_REJECTED');
      setJoined(result.data?.tenant_name ?? 'the tenant');
    } catch (cause) {
      setError(describeError(cause));
    } finally {
      setBusy(null);
    }
  };

  return (
    <View style={styles.container}>
      <Stack.Screen options={{ title: 'Join a workspace' }} />
      <Text style={styles.title}>Invitation</Text>
      <Text style={styles.description}>{host?.origin ?? 'Connect to a WeKnora server first.'}</Text>
      <TextInput
        accessibilityLabel="Invitation token"
        autoCapitalize="none"
        autoCorrect={false}
        onChangeText={setToken}
        placeholder="Invitation token"
        style={styles.input}
        value={token}
      />
      {lookup ? (
        <View style={styles.summary}>
          <Text style={styles.summaryTitle}>{lookup.tenantName}</Text>
          <Text style={styles.summaryText}>Role: {lookup.role}</Text>
          <Text style={styles.summaryText}>Expires: {lookup.expiresAt}</Text>
        </View>
      ) : null}
      {joined ? <Text style={styles.success}>Joined {joined}. You can sign in to continue.</Text> : null}
      {error ? <Text accessibilityRole="alert" style={styles.error}>{error}</Text> : null}
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Look up invitation"
        disabled={busy !== null || !host}
        onPress={submitLookup}
        style={[styles.button, (busy !== null || !host) && styles.disabled]}
      >
        <Text style={styles.buttonText}>{busy === 'lookup' ? 'Checking…' : 'Check invitation'}</Text>
      </Pressable>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Accept invitation"
        disabled={busy !== null || !lookup || !auth.credential}
        onPress={submitAccept}
        style={[styles.buttonSecondary, (busy !== null || !lookup || !auth.credential) && styles.disabled]}
      >
        <Text style={styles.buttonSecondaryText}>{busy === 'accept' ? 'Joining…' : 'Join workspace'}</Text>
      </Pressable>
      <Pressable accessibilityRole="button" accessibilityLabel="Back to sign in" onPress={() => router.replace('/(app)/login')}>
        <Text style={styles.link}>Back to sign in</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, gap: 16, justifyContent: 'center', padding: 24 },
  title: { fontSize: 28, fontWeight: '700' },
  description: { color: '#666', fontSize: 14 },
  input: { borderColor: '#bbb', borderRadius: 8, borderWidth: 1, fontSize: 16, padding: 12 },
  summary: { borderColor: '#ddd', borderRadius: 8, borderWidth: 1, gap: 4, padding: 12 },
  summaryTitle: { fontSize: 16, fontWeight: '600' },
  summaryText: { color: '#444', fontSize: 13 },
  success: { color: '#1a7f37' },
  error: { color: '#b00020' },
  button: { alignItems: 'center', backgroundColor: '#1f6feb', borderRadius: 8, padding: 14 },
  buttonSecondary: { alignItems: 'center', borderColor: '#1f6feb', borderRadius: 8, borderWidth: 1, padding: 14 },
  disabled: { opacity: 0.5 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
  buttonSecondaryText: { color: '#1f6feb', fontSize: 16, fontWeight: '600' },
  link: { color: '#1f6feb', textAlign: 'center' },
});