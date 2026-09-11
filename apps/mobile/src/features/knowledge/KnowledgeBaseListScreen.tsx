import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { KnowledgeBase } from '@weknora/contracts';
import { useMobileRuntime } from '../../runtime.tsx';
import { knowledgeHeaderLayout } from './header-layout.ts';
import { signOutAndRedirect } from './sign-out.ts';

export function KnowledgeBaseListScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [items, setItems] = useState<KnowledgeBase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try { setItems(await runtime.client.knowledgeBases.list()); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load knowledge bases'); }
    finally { setLoading(false); }
  }, [runtime.client]);

  useEffect(() => { void load(); }, [load]);

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={knowledgeHeaderLayout.container}>
      <Text accessibilityRole="header" style={knowledgeHeaderLayout.title}>Knowledge bases</Text>
      <View style={knowledgeHeaderLayout.actions}>
        <Pressable accessibilityRole="button" onPress={() => router.push(knowledgeHeaderLayout.chatRoute)}><Text style={knowledgeHeaderLayout.actionText}>Chat</Text></Pressable>
        <Pressable accessibilityRole="button" onPress={() => router.push('/workspace')}><Text style={knowledgeHeaderLayout.actionText}>Workspace</Text></Pressable>
        <Pressable accessibilityRole="button" onPress={() => router.push('/management')}><Text style={knowledgeHeaderLayout.actionText}>Manage</Text></Pressable>
        <Pressable accessibilityRole="button" onPress={() => void signOutAndRedirect(runtime.logout, (path) => router.replace(path))}><Text style={knowledgeHeaderLayout.actionText}>Sign out</Text></Pressable>
      </View>
    </View>
    {error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 12 }}>{error}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel="Loading knowledge bases" /> : <FlatList
      data={items}
      keyExtractor={(item) => item.id}
      ListEmptyComponent={<Text style={{ color: '#667085' }}>No knowledge bases available.</Text>}
      renderItem={({ item }) => <Pressable accessibilityRole="button" onPress={() => router.push(`/knowledge/${item.id}`)} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 14 }}>
        <Text style={{ fontWeight: '600', fontSize: 16 }}>{item.name}</Text>
        <Text style={{ color: '#667085', fontSize: 12 }}>{typeof item.description === 'string' ? item.description : item.id}</Text>
      </Pressable>}
    />}
  </SafeAreaView>;
}
