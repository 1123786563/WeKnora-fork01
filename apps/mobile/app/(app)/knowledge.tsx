import { useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useMobileRuntime } from '../../src/runtime.tsx';
import type { KnowledgeBase } from '@weknora/contracts';

export default function KnowledgeRoute() {
  const runtime = useMobileRuntime();
  const [items, setItems] = useState<KnowledgeBase[]>([]);
  const [error, setError] = useState('');
  useEffect(() => { let active = true; void runtime.client.knowledgeBases.list().then((next) => { if (active) setItems(next); }).catch((cause) => { if (active) setError(cause instanceof Error ? cause.message : 'Unable to load knowledge bases'); }); return () => { active = false; }; }, [runtime.client]);
  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: 12 }}><Text style={{ fontSize: 24, fontWeight: '700' }}>Knowledge bases</Text><Pressable onPress={() => void runtime.logout()}><Text style={{ color: '#2864dc' }}>Sign out</Text></Pressable></View>{error ? <Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text> : null}{!error && items.length === 0 ? <ActivityIndicator /> : <FlatList data={items} keyExtractor={(item) => item.id} renderItem={({ item }) => <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 14 }}><Text style={{ fontWeight: '600' }}>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>{item.id}</Text></View>} />}</SafeAreaView>;
}
