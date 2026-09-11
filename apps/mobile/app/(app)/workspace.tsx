import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, SafeAreaView, Text, View } from 'react-native';
import { Redirect, useRouter } from 'expo-router';
import { useMobileRuntime } from '../../src/runtime.tsx';
import { shouldLoadWorkspaceRoute } from '../../src/platform/workspace.ts';

export default function WorkspaceRoute() {
  const router = useRouter();
  const { credential, hydrating, refreshWorkspaces, switchWorkspace, tenantId, workspaces } = useMobileRuntime();
  const ready = shouldLoadWorkspaceRoute({ hydrating, credentialKind: credential.kind });
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!ready) return;
    setLoading(true); setError('');
    try { await refreshWorkspaces(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load workspaces'); }
    finally { setLoading(false); }
  }, [ready, refreshWorkspaces]);

  useEffect(() => { if (ready) void load(); }, [load, ready]);

  async function select(id: number) {
    if (String(id) === tenantId) return;
    setBusyId(id); setError('');
    try { await switchWorkspace(id); router.back(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to switch workspace'); }
    finally { setBusyId(null); }
  }

  if (!ready) {
    if (hydrating) return <SafeAreaView style={{ flex: 1, justifyContent: 'center' }}><ActivityIndicator accessibilityLabel="Restoring session" /></SafeAreaView>;
    return <Redirect href="/(auth)/login" />;
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
      <Text accessibilityRole="header" style={{ fontSize: 24, fontWeight: '700' }}>Workspaces</Text>
      <Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable>
    </View>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 12 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading workspaces" /> : workspaces.length === 0 ? <Text style={{ color: '#667085' }}>No workspaces were returned for this account.</Text> : workspaces.map((workspace) => <Pressable accessibilityRole="button" key={workspace.id} disabled={busyId !== null} onPress={() => void select(workspace.id)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 14, opacity: busyId !== null && busyId !== workspace.id ? 0.5 : 1 }}><Text style={{ fontWeight: '600', fontSize: 16 }}>{workspace.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{workspace.role}{String(workspace.id) === tenantId ? ' · Current' : ''}{busyId === workspace.id ? ' · Switching…' : ''}</Text></Pressable>)}
  </SafeAreaView>;
}
