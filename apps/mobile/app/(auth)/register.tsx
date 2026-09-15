import { useState } from 'react';
import { Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMobileRuntime } from '../../src/runtime.tsx';
import { formatMessage } from '@weknora/i18n';

export default function RegisterRoute() {
  const router = useRouter();
  const runtime = useMobileRuntime();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(runtime.locale, key, values);
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
    catch (cause) { setError(cause instanceof Error ? cause.message : t('auth.join.invitationRegistrationFailed')); }
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
        setNotice(t('auth.registerSuccess'));
      }
    } catch (cause) { setError(cause instanceof Error ? cause.message : t('auth.registerError')); }
    finally { setBusy(false); }
  }

  return <SafeAreaView style={{ flex: 1, justifyContent: 'center', padding: 24 }}><View style={{ gap: 12 }}>
    <Text style={{ fontSize: 30, fontWeight: '700' }}>{token.trim() ? t('auth.join.title') : t('auth.register')}</Text>
    <Text style={{ color: '#667085' }}>{token.trim() ? t('auth.join.title') : t('auth.registerSubtitle')}</Text>
    {token.trim() ? <><TextInput autoCapitalize="none" autoCorrect={false} placeholder={t('mobileAuth.invitationToken')} value={token} onChangeText={setToken} style={inputStyle} /><Pressable disabled={busy || !token.trim()} onPress={() => void lookup()}><Text style={{ color: '#2864dc' }}>{t('auth.join.checkingInvitation')}</Text></Pressable>{invite ? <Text style={{ color: '#667085' }}>{t('auth.join.joinPrefix')} {invite.tenantName || t('auth.join.workspaceFallback', { id: '?' })}{t('auth.join.asRole', { role: invite.role })}{t('mobileAuth.invitationExpires', { expiresAt: invite.expiresAt })}</Text> : null}</> : null}
    <TextInput autoCapitalize="none" placeholder={t('auth.usernamePlaceholder')} value={username} onChangeText={setUsername} style={inputStyle} />
    <TextInput autoCapitalize="none" keyboardType="email-address" placeholder={t('auth.emailPlaceholder')} value={email} onChangeText={setEmail} style={inputStyle} />
    <TextInput placeholder={t('auth.passwordPlaceholder')} secureTextEntry value={password} onChangeText={setPassword} style={inputStyle} />
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : null}
    {notice ? <Text accessibilityLiveRegion="polite" style={{ color: '#16803c' }}>{notice}</Text> : null}
    <Pressable disabled={busy || !username.trim() || !email.trim() || !password} onPress={() => void submit()} style={buttonStyle}><Text style={{ color: '#fff', textAlign: 'center', fontWeight: '700' }}>{busy ? t('common.loading') : token.trim() ? t('auth.join.title') : t('auth.register')}</Text></Pressable>
    <Pressable disabled={busy} onPress={() => router.replace('/(auth)/login')}><Text style={{ color: '#2864dc', textAlign: 'center' }}>{t('auth.backToLogin')}</Text></Pressable>
  </View></SafeAreaView>;
}

const inputStyle = { borderColor: '#d0d5dd', borderRadius: 10, borderWidth: 1, padding: 12 } as const;
const buttonStyle = { backgroundColor: '#2864dc', borderRadius: 10, padding: 13 } as const;
