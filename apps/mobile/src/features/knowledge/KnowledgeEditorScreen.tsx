import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { FAQEntry, WikiPage } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { classifyMobileEditorError, createFaqDraft, createWikiDraft, validateFaqDraft, validateWikiDraft, type FaqEditorDraft, type WikiEditorDraft } from './editor.ts';

type EditorKind = 'faq' | 'wiki';

function firstParam(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

function canEdit(role: string | undefined): boolean {
  const normalized = role?.trim().toLowerCase();
  return normalized === 'owner' || normalized === 'admin';
}

export function KnowledgeEditorScreen() {
  const params = useLocalSearchParams<{ id?: string; kind?: string; slug?: string }>();
  const kbId = firstParam(params.id);
  const kind: EditorKind = firstParam(params.kind) === 'wiki' ? 'wiki' : 'faq';
  const slug = firstParam(params.slug);
  const runtime = useMobileRuntime();
  const router = useRouter();
  const role = runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role;
  const writable = canEdit(role);
  const [wiki, setWiki] = useState<WikiEditorDraft>({ title: '', summary: '', content: '', version: 1 });
  const [faq, setFaq] = useState<FaqEditorDraft>({ standardQuestion: '', answer: '', isEnabled: true, isRecommended: false });
  const [loading, setLoading] = useState(Boolean(slug));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const [conflict, setConflict] = useState(false);

  const title = useMemo(() => kind === 'wiki' ? (slug ? 'Edit Wiki page' : 'New Wiki page') : (slug ? 'Edit FAQ' : 'New FAQ'), [kind, slug]);

  const load = useCallback(async () => {
    if (!kbId || !slug) return;
    setLoading(true); setError(''); setConflict(false);
    try {
      if (kind === 'wiki') {
        setWiki(createWikiDraft(await runtime.client.wiki.get(kbId, slug)));
      } else {
        const entry = await runtime.client.knowledge.faq.get(kbId, Number(slug));
        setFaq(createFaqDraft(entry));
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : `Unable to load ${kind}`);
    } finally { setLoading(false); }
  }, [kbId, kind, runtime.client, slug]);

  useEffect(() => { void load(); }, [load]);

  async function save() {
    if (!kbId || saving) return;
    const validation = kind === 'wiki' ? validateWikiDraft(wiki) : validateFaqDraft(faq);
    if (validation.length) { setError(validation.join('. ')); return; }
    setSaving(true); setError(''); setSaved(false); setConflict(false);
    try {
      if (kind === 'wiki') {
        const result = slug
          ? await runtime.client.wiki.update(kbId, slug, { title: wiki.title.trim(), summary: wiki.summary, content: wiki.content, version: wiki.version })
          : await runtime.client.wiki.create(kbId, { title: wiki.title.trim(), summary: wiki.summary, content: wiki.content });
        setWiki(createWikiDraft(result));
      } else {
        const result = slug
          ? await runtime.client.knowledge.faq.update(kbId, Number(slug), {
            standard_question: faq.standardQuestion.trim(), answers: [faq.answer], is_enabled: faq.isEnabled, is_recommended: faq.isRecommended,
          })
          : await runtime.client.knowledge.faq.create(kbId, {
            standard_question: faq.standardQuestion.trim(), answers: [faq.answer], is_enabled: faq.isEnabled, is_recommended: faq.isRecommended,
          });
        setFaq(createFaqDraft(result));
      }
      setSaved(true);
    } catch (cause) {
      if (classifyMobileEditorError(cause) === 'conflict') setConflict(true);
      setError(cause instanceof Error ? cause.message : `Unable to save ${kind}`);
    } finally { setSaving(false); }
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}>
      <Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>Back</Text></Pressable>
      <Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{title}</Text>
      {writable ? <Pressable accessibilityRole="button" disabled={saving || loading} onPress={() => void save()}><Text style={{ color: '#2864dc', opacity: saving ? 0.5 : 1 }}>{saving ? 'Saving…' : 'Save'}</Text></Pressable> : null}
    </View>
    {!writable ? <Text style={{ color: '#667085', marginBottom: 8 }}>Editing requires an owner or admin workspace role. The server remains authoritative.</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel={`Loading ${kind}`} /> : <ScrollView keyboardShouldPersistTaps="handled">
      {error ? <View style={{ backgroundColor: '#fff4ed', padding: 10, borderRadius: 8, marginBottom: 10 }}><Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text>{conflict ? <Pressable onPress={() => void load()}><Text style={{ color: '#2864dc', marginTop: 8 }}>Reload server version</Text></Pressable> : null}</View> : null}
      {saved ? <Text accessibilityLiveRegion="polite" style={{ color: '#067647', marginBottom: 8 }}>Saved successfully</Text> : null}
      {kind === 'wiki' ? <>
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>Title</Text>
        <TextInput accessibilityLabel="Wiki title" value={wiki.title} onChangeText={(titleValue) => setWiki((current) => ({ ...current, title: titleValue }))} editable={writable} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} />
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>Summary</Text>
        <TextInput accessibilityLabel="Wiki summary" value={wiki.summary} onChangeText={(summary) => setWiki((current) => ({ ...current, summary }))} editable={writable} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} />
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>Content</Text>
        <TextInput accessibilityLabel="Wiki content" value={wiki.content} onChangeText={(content) => setWiki((current) => ({ ...current, content }))} editable={writable} multiline textAlignVertical="top" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 220 }} />
        <Text style={{ color: '#667085', fontSize: 12, marginTop: 8 }}>Version {wiki.version} · server-side conflict protection</Text>
      </> : <>
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>Question</Text>
        <TextInput accessibilityLabel="FAQ question" value={faq.standardQuestion} onChangeText={(standardQuestion) => setFaq((current) => ({ ...current, standardQuestion }))} editable={writable} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} />
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>Answer</Text>
        <TextInput accessibilityLabel="FAQ answer" value={faq.answer} onChangeText={(answer) => setFaq((current) => ({ ...current, answer }))} editable={writable} multiline textAlignVertical="top" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 160, marginBottom: 10 }} />
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingVertical: 8 }}><Text>Enabled</Text><Switch accessibilityLabel="FAQ enabled" value={faq.isEnabled} onValueChange={(isEnabled) => setFaq((current) => ({ ...current, isEnabled }))} disabled={!writable} /></View>
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingVertical: 8 }}><Text>Recommended</Text><Switch accessibilityLabel="FAQ recommended" value={faq.isRecommended} onValueChange={(isRecommended) => setFaq((current) => ({ ...current, isRecommended }))} disabled={!writable} /></View>
      </>}
    </ScrollView>}
  </SafeAreaView>;
}
