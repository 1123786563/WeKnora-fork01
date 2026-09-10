import { useState } from 'react';
import { Pressable, SafeAreaView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import { useMobileRuntime } from '../../src/runtime.tsx';

export default function ServerRoute() {
  const router = useRouter();
  const runtime = useMobileRuntime();
  const [value, setValue] = useState(runtime.baseURL);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true); setError('');
    try { await runtime.setServerAddress(value); router.replace('/(auth)/login'); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to save server address'); }
    finally { setBusy(false); }
  }

  return <SafeAreaView style={{ flex: 1, justifyContent: 'center', padding: 24 }}>
    <View style={{ gap: 14 }}>
      <Text style={{ fontSize: 30, fontWeight: '700' }}>Server address</Text>
      <Text style={{ color: '#667085' }}>Use the HTTP(S) address of your WeKnora server. Do not use phone localhost for a desktop server.</Text>
      <TextInput autoCapitalize="none" autoCorrect={false} keyboardType="url" placeholder="https://weknora.example.com" value={value} onChangeText={setValue} style={inputStyle} />
      {error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : null}
      <Pressable disabled={busy} onPress={() => void save()} style={buttonStyle}><Text style={{ color: '#fff', textAlign: 'center', fontWeight: '700' }}>{busy ? 'Saving…' : 'Save server'}</Text></Pressable>
      <Pressable disabled={busy} onPress={() => router.back()}><Text style={{ color: '#2864dc', textAlign: 'center' }}>Cancel</Text></Pressable>
    </View>
  </SafeAreaView>;
}

const inputStyle = { borderColor: '#d0d5dd', borderRadius: 10, borderWidth: 1, padding: 12 } as const;
const buttonStyle = { backgroundColor: '#2864dc', borderRadius: 10, padding: 13 } as const;
