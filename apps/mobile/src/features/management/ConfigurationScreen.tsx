import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, SafeAreaView, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import type { AgentConfiguration, McpConfiguration, ModelConfiguration, SkillConfiguration } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';

interface ConfigurationRows { agents: AgentConfiguration[]; models: ModelConfiguration[]; mcp: McpConfiguration[]; skills: SkillConfiguration[] }

export function ConfigurationScreen() {
  const runtime = useMobileRuntime();
  const router = useRouter();
  const [rows, setRows] = useState<ConfigurationRows>({ agents: [], models: [], mcp: [], skills: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    setError('');
    const [agents, models, mcp, skills] = await Promise.allSettled([
      runtime.client.configuration.agents.list(), runtime.client.configuration.models.list(), runtime.client.configuration.mcp.list(), runtime.client.configuration.skills.list(),
    ]);
    const failures = [agents, models, mcp, skills].filter((result): result is PromiseRejectedResult => result.status === 'rejected');
    if (failures.length) setError(`${failures.length} configuration surface(s) are unavailable or forbidden`);
    setRows({ agents: agents.status === 'fulfilled' ? agents.value : [], models: models.status === 'fulfilled' ? models.value : [], mcp: mcp.status === 'fulfilled' ? mcp.value : [], skills: skills.status === 'fulfilled' ? skills.value : [] });
    setLoading(false);
  }, [runtime.client]);
  useEffect(() => { void load(); }, [load]);
  const sections = [
    ['Agents', rows.agents], ['Models', rows.models], ['MCP services', rows.mcp], ['Skills', rows.skills],
  ] as const;
  const data = sections.flatMap(([section, items]) => [{ id: `header:${section}`, section, header: true as const }, ...items.map((item) => ({ id: `${section}:${item.id}`, section, name: item.name, header: false as const }))]);
  return <SafeAreaView style={{ flex: 1, padding: 16 }}><View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable><Text accessibilityRole="header" style={{ fontSize: 22, fontWeight: '700' }}>Configuration</Text></View>{loading ? <ActivityIndicator accessibilityLabel="Loading configuration" /> : null}{error ? <Text accessibilityRole="alert" style={{ color: '#b42318', marginBottom: 8 }}>{error}</Text> : null}<FlatList data={data} keyExtractor={(item) => item.id} ListEmptyComponent={<Text style={{ color: '#667085' }}>No readable configuration is available.</Text>} renderItem={({ item }) => item.header ? <Text style={{ fontSize: 16, fontWeight: '700', marginTop: 12, marginBottom: 4 }}>{item.section}</Text> : <View style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 10 }}><Text>{item.name}</Text><Text style={{ color: '#667085', fontSize: 12 }}>Read-only on mobile</Text></View>} /></SafeAreaView>;
}
