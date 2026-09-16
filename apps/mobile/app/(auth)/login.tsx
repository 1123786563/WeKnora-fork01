import { useState } from 'react';
import { Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import { useMobileRuntime } from '../../src/runtime.tsx';
import { formatMessage } from '@weknora/i18n';

export default function LoginRoute() {
  const router = useRouter();
  const runtime = useMobileRuntime();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(runtime.locale, key, values);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  async function submit() {
    setBusy(true); setError('');
    try { await runtime.login(email, password); router.replace('/(app)/knowledge'); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t('auth.loginErrorRetry')); }
    finally { setBusy(false); }
  }
  return <SafeAreaView style={{ flex: 1, justifyContent: 'center', padding: 24 }}><View style={{ gap: 14 }}><Text style={{ fontSize: 30, fontWeight: '700' }}>WeKnora</Text><Text style={{ color: '#667085' }}>{t('auth.loginHint')}</Text><TextInput autoCapitalize="none" keyboardType="email-address" placeholder={t('auth.emailPlaceholder')} value={email} onChangeText={setEmail} style={inputStyle} /><TextInput placeholder={t('auth.passwordPlaceholder')} secureTextEntry value={password} onChangeText={setPassword} style={inputStyle} />{error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : null}{runtime.oidcError ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{runtime.oidcError}</Text> : null}<Pressable disabled={busy || !email || !password} onPress={() => void submit()} style={buttonStyle}><Text style={{ color: '#fff', textAlign: 'center', fontWeight: '700' }}>{busy ? t('auth.loggingIn') : t('auth.login')}</Text></Pressable><Pressable disabled={busy} onPress={() => void runtime.startOIDC().catch((cause) => setError(cause instanceof Error ? cause.message : t('auth.loginErrorRetry')))}><Text style={{ color: '#2864dc', textAlign: 'center' }}>{t('auth.oidcLogin')}</Text></Pressable><Pressable disabled={busy} onPress={() => router.push('/(auth)/register')}><Text style={{ color: '#2864dc', textAlign: 'center' }}>{t('auth.createAccount')}</Text></Pressable><Pressable disabled={busy} onPress={() => router.push('/(auth)/register?token=')}><Text style={{ color: '#2864dc', textAlign: 'center' }}>{t('auth.workspaceOnboarding.invitations')}</Text></Pressable><Pressable disabled={busy} onPress={() => router.push('/(auth)/server')}><Text style={{ color: '#2864dc', textAlign: 'center' }}>{t('auth.changeServer')}</Text></Pressable></View></SafeAreaView>;
}

const inputStyle = { borderColor: '#d0d5dd', borderRadius: 10, borderWidth: 1, padding: 12 } as const;
const buttonStyle = { backgroundColor: '#2864dc', borderRadius: 10, padding: 13 } as const;
