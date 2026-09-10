import { useState } from 'react';
import { Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMobileRuntime } from '../../src/runtime.tsx';

export default function RegisterRoute() {
  const router = useRouter();
  const runtime = useMobileRuntime();
  const params = useLocalSearchParams<{ token?: string }>();
  const [token, setToken] = useState(typeof params.token === 'string' ? params.token : '');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [invite, setInvite] = useState<{ tenantName?: string; role: string; expiresAt: string } | null>(null);
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function lookup() {
    setBusy(true); setError(''); setNotice('');
    try { setInvite(await runtime.client.auth.lookupInvitation(token.trim())); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Invitation lookup failed'); }
    finally { setBusy(false); }
  }

  async function submit() {
    setBusy(true); setError(''); setNotice('');
    try {
      if (token.trim()) {
        await runtime.registerByInvite(token.trim(), username.trim(), email.trim(), password);
        router.replace('/(app)/knowledge');
      } else {
        await runtime.register(username.trim(), email.trim(), password);
        setNotice('Account created. Sign in to continue.');
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Registration failed'); }
    finally { setBusy(false); }
  }

  return <SafeAreaView style={{ flex: 1, justifyContent: 'center', padding: 24 }}><View style={{ gap: 12 }}>
    <Text style={{ fontSize: 30, fontWeight: '700' }}>{token.trim() ? 'Join workspace' : 'Create account'}</Text>
    <Text style={{ color: '#667085' }}>{token.trim() ? 'Accept the invitation with your new account.' : 'Create an account on the configured WeKnora server.'}</Text>
    {token.trim() ? <><TextInput autoCapitalize="none" autoCorrect={false} placeholder="Invitation token" value={token} onChangeText={setToken} style={inputStyle} /><Pressable disabled={busy || !token.trim()} onPress={() => void lookup()}><Text style={{ color: '#2864dc' }}>Check invitation</Text></Pressable>{invite ? <Text style={{ color: '#667085' }}>Join {invite.tenantName || 'workspace'} as {invite.role}; expires {invite.expiresAt}.</Text> : null}</> : null}
    <TextInput autoCapitalize="none" placeholder="Username" value={username} onChangeText={setUsername} style={inputStyle} />
    <TextInput autoCapitalize="none" keyboardType="email-address" placeholder="Email" value={email} onChangeText={setEmail} style={inputStyle} />
    <TextInput placeholder="Password" secureTextEntry value={password} onChangeText={setPassword} style={inputStyle} />
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : null}
    {notice ? <Text accessibilityLiveRegion="polite" style={{ color: '#16803c' }}>{notice}</Text> : null}
    <Pressable disabled={busy || !username.trim() || !email.trim() || !password} onPress={() => void submit()} style={buttonStyle}><Text style={{ color: '#fff', textAlign: 'center', fontWeight: '700' }}>{busy ? 'Working…' : token.trim() ? 'Join workspace' : 'Create account'}</Text></Pressable>
    <Pressable disabled={busy} onPress={() => router.replace('/(auth)/login')}><Text style={{ color: '#2864dc', textAlign: 'center' }}>Back to sign in</Text></Pressable>
  </View></SafeAreaView>;
}

const inputStyle = { borderColor: '#d0d5dd', borderRadius: 10, borderWidth: 1, padding: 12 } as const;
const buttonStyle = { backgroundColor: '#2864dc', borderRadius: 10, padding: 13 } as const;
