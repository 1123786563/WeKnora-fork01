import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, SafeAreaView, ScrollView, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { WikiGraphData } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { knowledgeListLabel } from './list.ts';
import { graphNodePositions } from './graph-layout.ts';

function firstParam(value: string | string[] | undefined): string | undefined { return Array.isArray(value) ? value[0] : value; }

const graphTypeLabels: Record<string, string> = {
  summary: 'wikiBrowser.filterSummary',
  knowledge: 'wikiBrowser.filterKnowledge',
  entity: 'wikiBrowser.filterEntity',
  concept: 'wikiBrowser.filterConcept',
  synthesis: 'wikiBrowser.filterSynthesis',
  comparison: 'wikiBrowser.filterComparison',
};

export function KnowledgeGraphScreen() {
  const params = useLocalSearchParams<{ id?: string; slug?: string }>();
  const knowledgeBaseId = firstParam(params.id);
  const initialSlug = firstParam(params.slug);
  const runtime = useMobileRuntime();
  const router = useRouter();
  const label = (key: string, values: Record<string, string | number> = {}) => knowledgeListLabel(runtime.locale, key, values);
  const [graph, setGraph] = useState<WikiGraphData | null>(null);
  const [mode, setMode] = useState<'overview' | 'ego'>(initialSlug ? 'ego' : 'overview');
  const [center, setCenter] = useState(initialSlug ?? '');
  const [depth, setDepth] = useState(1);
  const [query, setQuery] = useState('');
  const [type, setType] = useState('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const requestIdRef = useRef(0);
  const load = useCallback(async (nextMode: 'overview' | 'ego', nextCenter = center) => {
    if (!knowledgeBaseId) return;
    const requestId = ++requestIdRef.current;
    setLoading(true); setError('');
    try {
      const result = await runtime.client.wiki.graph(knowledgeBaseId, { mode: nextMode, ...(nextMode === 'ego' && nextCenter ? { center: nextCenter, depth } : {}), ...(type === 'all' ? {} : { types: [type] }), limit: 500 });
      if (requestId !== requestIdRef.current) return;
      setGraph(result); setMode(nextMode); setCenter(nextCenter);
    } catch (cause) {
      if (requestId !== requestIdRef.current) return;
      const message = cause instanceof Error ? cause.message : '';
      setError(/feature is not enabled/i.test(message) ? label("knowledgeBase.graph.disabled") : message || label("knowledgeBase.graph.loadFailed"));
    } finally {
      if (requestId === requestIdRef.current) setLoading(false);
    }
  }, [center, depth, knowledgeBaseId, runtime.client, type]);
  // Node presses call `load` directly and update `center`; including center
  // here would immediately issue the same graph request a second time.
  useEffect(() => { void load(mode, mode === 'ego' ? center : ''); }, [knowledgeBaseId, mode, depth, type]);
  const types = useMemo(() => [...new Set(graph?.nodes.map((node) => node.page_type) ?? [])].sort(), [graph]);
  const visibleNodes = useMemo(() => { const needle = query.trim().toLocaleLowerCase(); return (graph?.nodes ?? []).filter((node) => !needle || `${node.title} ${node.slug}`.toLocaleLowerCase().includes(needle)); }, [graph, query]);
  const labelForType = (value: string) => graphTypeLabels[value] ? label(graphTypeLabels[value]) : value;
  const graphPositions = useMemo(() => graphNodePositions(visibleNodes, center), [center, visibleNodes]);
  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}><Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.detail.back")}</Text></Pressable><Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{label("wikiBrowser.tabGraph")}</Text>{mode === 'ego' ? <Pressable accessibilityRole="button" onPress={() => void load('overview', '')}><Text style={{ color: '#2864dc' }}>{label("common.all")}</Text></Pressable> : null}<Pressable accessibilityRole="button" disabled={loading} onPress={() => void load(mode)}><Text style={{ color: loading ? '#98a2b3' : '#2864dc' }}>{label("common.refresh")}</Text></Pressable></View>
    <TextInput accessibilityLabel={label("wikiBrowser.searchPlaceholder")} value={query} onChangeText={setQuery} placeholder={label("wikiBrowser.searchPlaceholder")} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 8 }} />
    <ScrollView horizontal showsHorizontalScrollIndicator={false} style={{ maxHeight: 42, marginBottom: 6 }}>{[{ value: 'all', label: label("common.all") }, ...types.map((item) => ({ value: item, label: labelForType(item) }))].map((item) => <Pressable key={item.value} accessibilityRole="button" accessibilityState={{ selected: type === item.value }} onPress={() => setType(item.value)} style={{ paddingHorizontal: 10, paddingVertical: 7, borderRadius: 14, backgroundColor: type === item.value ? '#dbeafe' : '#f2f4f7', marginRight: 6 }}><Text>{item.label}</Text></Pressable>)}</ScrollView>
    <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: 8, gap: 8 }}><Text>{label("knowledgeBase.graph.depth")}</Text>{[1, 2, 3].map((value) => <Pressable key={value} accessibilityRole="button" accessibilityLabel={`${label("knowledgeBase.graph.depth")} ${value}`} accessibilityState={{ selected: depth === value }} onPress={() => setDepth(value)} style={{ paddingHorizontal: 10, paddingVertical: 6, borderRadius: 8, backgroundColor: depth === value ? '#dcfce7' : '#f2f4f7' }}><Text>{value}</Text></Pressable>)}</View>
    {error ? <View style={{ backgroundColor: '#fff4ed', padding: 10, borderRadius: 8, marginBottom: 10 }}><Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text><Pressable onPress={() => void load(mode)}><Text style={{ color: '#2864dc', marginTop: 8 }}>{label("common.retry")}</Text></Pressable></View> : null}
    {loading ? <ActivityIndicator accessibilityLabel={label("common.loading")} /> : graph ? <ScrollView><Text style={{ color: '#667085', marginBottom: 8 }}>{label("knowledgeBase.graph.showing", { visible: visibleNodes.length, total: graph.meta.total })}{graph.meta.truncated ? ` · ${label("knowledgeBase.graph.overviewBounded")}` : ''}.</Text>{graph.nodes.length === 0 ? <Text style={{ color: '#667085' }}>{label("wikiBrowser.graphNoData")}</Text> : visibleNodes.length === 0 ? <Text style={{ color: '#667085' }}>{label("wikiBrowser.searchNoResults")}</Text> : <><View accessibilityLabel={label("knowledgeBase.graph.ariaLinks")} style={{ minHeight: Math.max(92, Math.ceil(graphPositions.length / 2) * 76), marginBottom: 8, borderRadius: 12, backgroundColor: '#f8fafc', borderColor: '#e4e7ec', borderWidth: 1, padding: 8, flexDirection: 'row', flexWrap: 'wrap', alignContent: 'flex-start' }}>{graphPositions.map((position) => <Pressable key={position.slug} accessibilityRole="button" onPress={() => void load('ego', position.slug)} style={{ width: '50%', padding: 4 }}><View style={{ minHeight: 60, borderRadius: 10, borderWidth: 1, borderColor: position.tone === 'primary' ? '#16a34a' : '#cbd5e1', backgroundColor: position.tone === 'primary' ? '#dcfce7' : '#fff', padding: 8 }}><Text numberOfLines={2} style={{ fontWeight: '600', fontSize: 13 }}>{position.title}</Text><Text style={{ color: '#667085', fontSize: 11, marginTop: 4 }}>{label("wikiBrowser.expandNeighbors")}</Text></View></Pressable>)}</View>{visibleNodes.map((node) => <View key={node.slug} style={{ borderBottomColor: '#eaecf0', borderBottomWidth: 1, paddingVertical: 12 }}><Text style={{ fontWeight: '600' }}>{node.title}</Text><Text accessibilityLabel={label("knowledgeBase.graph.ariaLinks")} style={{ color: '#667085', fontSize: 12, marginTop: 3 }}>{node.slug} · {labelForType(node.page_type)} · {node.link_count} {label("knowledgeBase.graph.links")}{node.familiar ? ` · ${label("knowledgeBase.graph.familiar")}` : ''}</Text><Pressable accessibilityRole="button" onPress={() => void load('ego', node.slug)}><Text style={{ color: '#2864dc', marginTop: 7 }}>{label("wikiBrowser.expandNeighbors")}</Text></Pressable></View>)}</>}</ScrollView> : null}
  </SafeAreaView>;
}
