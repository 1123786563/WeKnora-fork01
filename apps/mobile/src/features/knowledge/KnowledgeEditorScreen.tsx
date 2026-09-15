import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, SafeAreaView, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import type { FAQEntry, WikiPage } from '@weknora/api-client';
import { useMobileRuntime } from '../../runtime.tsx';
import { knowledgeListLabel } from './list.ts';
import { canEditKnowledgeBase } from './access.ts';
import { classifyMobileEditorError, createFaqDraft, createWikiDraft, validateFaqDraft, validateWikiDraft, type FaqEditorDraft, type WikiEditorDraft } from './editor.ts';

type EditorKind = 'faq' | 'wiki';

function firstParam(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

export function KnowledgeEditorScreen() {
  const params = useLocalSearchParams<{ id?: string; kind?: string; slug?: string }>();
  const kbId = firstParam(params.id);
  const kind: EditorKind = firstParam(params.kind) === 'wiki' ? 'wiki' : 'faq';
  const slug = firstParam(params.slug);
  const runtime = useMobileRuntime();
  const router = useRouter();
  const label = useCallback((key: string, values: Record<string, string | number> = {}) => knowledgeListLabel(runtime.locale, key, values), [runtime.locale]);
  const role = runtime.workspaces.find((workspace) => String(workspace.id) === runtime.tenantId)?.role;
  const [permission, setPermission] = useState<unknown>();
  const [viaShare, setViaShare] = useState(false);
  const writable = canEditKnowledgeBase({ permission, viaShare, workspaceRole: role });
  const [wiki, setWiki] = useState<WikiEditorDraft>({ title: '', summary: '', content: '', version: 1 });
  const [faq, setFaq] = useState<FaqEditorDraft>({ standardQuestion: '', answer: '', isEnabled: true, isRecommended: false });
  const [loading, setLoading] = useState(Boolean(kbId));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const [conflict, setConflict] = useState(false);
  const loadGeneration = useRef(0);

  const title = useMemo(() => kind === 'wiki'
    ? (slug ? label('knowledgeEditor.mobile.editWikiTitle') : label('wikiBrowser.newPageTitle'))
    : (slug ? label('knowledgeEditor.faq.editorEdit') : label('knowledgeEditor.faq.editorCreate')), [kind, label, slug]);

  const load = useCallback(async () => {
    if (!kbId) return;
    const generation = ++loadGeneration.current;
    setLoading(true); setError(''); setConflict(false);
    try {
      const settings = await runtime.client.knowledge.settings?.get?.(kbId);
      if (generation !== loadGeneration.current) return;
      const record = settings as Record<string, unknown> | null | undefined;
      setPermission(record?.my_permission ?? record?.permission);
      setViaShare(record?.isMine === false || record?.is_mine === false);
      if (slug) {
        if (kind === 'wiki') {
          const next = await runtime.client.wiki.get(kbId, slug);
          if (generation === loadGeneration.current) setWiki(createWikiDraft(next));
        } else {
          const entry = await runtime.client.knowledge.faq.get(kbId, Number(slug));
          if (generation === loadGeneration.current) setFaq(createFaqDraft(entry));
        }
      }
    } catch (cause) {
      if (generation === loadGeneration.current) setError(cause instanceof Error ? cause.message : label(kind === 'wiki' ? 'knowledgeEditor.mobile.loadWikiFailed' : 'knowledgeEditor.mobile.loadFaqFailed'));
    } finally {
      if (generation === loadGeneration.current) setLoading(false);
    }
  }, [kbId, kind, label, runtime.client, slug]);

  useEffect(() => { void load(); }, [load]);

  async function save() {
    if (!kbId || saving) return;
    const validation = kind === 'wiki' ? validateWikiDraft(wiki) : validateFaqDraft(faq);
    if (validation.length) { setError(validation.map((key) => label(key)).join('. ')); return; }
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
      setError(cause instanceof Error ? cause.message : label(kind === 'wiki' ? 'knowledgeEditor.mobile.saveWikiFailed' : 'knowledgeEditor.mobile.saveFaqFailed'));
    } finally { setSaving(false); }
  }

  return <SafeAreaView style={{ flex: 1, padding: 16 }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12, marginBottom: 10 }}>
      <Pressable accessibilityRole="button" onPress={() => router.back()}><Text style={{ color: '#2864dc' }}>{label("knowledgeBase.detail.back")}</Text></Pressable>
      <Text accessibilityRole="header" style={{ flex: 1, fontSize: 21, fontWeight: '700' }}>{title}</Text>
      {writable ? <Pressable accessibilityRole="button" disabled={saving || loading} onPress={() => void save()}><Text style={{ color: '#2864dc', opacity: saving ? 0.5 : 1 }}>{saving ? label("common.loading") : label("common.save")}</Text></Pressable> : null}
    </View>
    {!writable ? <Text style={{ color: '#667085', marginBottom: 8 }}>{label('knowledgeEditor.mobile.editPermission')}</Text> : null}
    {loading ? <ActivityIndicator accessibilityLabel={label('common.loading')} /> : <ScrollView keyboardShouldPersistTaps="handled">
      {error ? <View style={{ backgroundColor: '#fff4ed', padding: 10, borderRadius: 8, marginBottom: 10 }}><Text accessibilityRole="alert" style={{ color: '#b42318' }}>{error}</Text>{conflict ? <Pressable onPress={() => void load()}><Text style={{ color: '#2864dc', marginTop: 8 }}>{label("wikiBrowser.editConflictReload")}</Text></Pressable> : null}</View> : null}
      {saved ? <Text accessibilityLiveRegion="polite" style={{ color: '#067647', marginBottom: 8 }}>{kind === 'wiki' ? label("wikiBrowser.editSaveSuccess") : label('knowledgeEditor.mobile.saved')}</Text> : null}
      {kind === 'wiki' ? <>
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>{label("wikiBrowser.editTitlePlaceholder")}</Text>
        <TextInput accessibilityLabel={label('wikiBrowser.editTitlePlaceholder')} value={wiki.title} onChangeText={(titleValue) => setWiki((current) => ({ ...current, title: titleValue }))} editable={writable} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} />
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>{label("wikiBrowser.editSummaryPlaceholder")}</Text>
        <TextInput accessibilityLabel={label('wikiBrowser.editSummaryPlaceholder')} value={wiki.summary} onChangeText={(summary) => setWiki((current) => ({ ...current, summary }))} editable={writable} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} />
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>{label("wikiBrowser.editContentPlaceholder")}</Text>
        <TextInput accessibilityLabel={label('wikiBrowser.editContentPlaceholder')} value={wiki.content} onChangeText={(content) => setWiki((current) => ({ ...current, content }))} editable={writable} multiline textAlignVertical="top" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 220 }} />
        <Text style={{ color: '#667085', fontSize: 12, marginTop: 8 }}>{label('knowledgeEditor.mobile.versionConflict', { version: wiki.version })}</Text>
      </> : <>
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>{label('knowledgeEditor.faq.standardQuestion')}</Text>
        <TextInput accessibilityLabel={label('knowledgeEditor.faq.standardQuestion')} value={faq.standardQuestion} onChangeText={(standardQuestion) => setFaq((current) => ({ ...current, standardQuestion }))} editable={writable} style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, marginBottom: 10 }} />
        <Text style={{ fontWeight: '600', marginBottom: 4 }}>{label('knowledgeEditor.faq.answers')}</Text>
        <TextInput accessibilityLabel={label('knowledgeEditor.faq.answers')} value={faq.answer} onChangeText={(answer) => setFaq((current) => ({ ...current, answer }))} editable={writable} multiline textAlignVertical="top" style={{ borderColor: '#d0d5dd', borderWidth: 1, borderRadius: 8, padding: 10, minHeight: 160, marginBottom: 10 }} />
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingVertical: 8 }}><Text>{label('knowledgeEditor.faq.statusEnabled')}</Text><Switch accessibilityLabel={label('knowledgeEditor.faq.statusEnabled')} value={faq.isEnabled} onValueChange={(isEnabled) => setFaq((current) => ({ ...current, isEnabled }))} disabled={!writable} /></View>
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', paddingVertical: 8 }}><Text>{label('knowledgeEditor.faq.recommended')}</Text><Switch accessibilityLabel={label('knowledgeEditor.faq.recommended')} value={faq.isRecommended} onValueChange={(isRecommended) => setFaq((current) => ({ ...current, isRecommended }))} disabled={!writable} /></View>
      </>}
    </ScrollView>}
  </SafeAreaView>;
}
