import { useEffect, useMemo, useState } from 'react';
import type { KnowledgeBase, ParserEngineInfo, StorageBackendView, VectorStoreView, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { DataSourcesPage } from '../data-sources/DataSourcesPage.tsx';
import { buildKnowledgeBaseSettingsInput, formFromKnowledgeBase, updateParserRule, type KnowledgeBaseSettingsForm } from './form.ts';

type Tab = 'settings' | 'sources';

function errorMessage(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function parserGroups(engines: ParserEngineInfo[]): string[] { return [...new Set(engines.flatMap((engine) => engine.FileTypes))].filter((type) => type !== 'url').sort(); }
function ruleFor(form: KnowledgeBaseSettingsForm, fileType: string): string { return form.parserRules.find((rule) => rule.file_types.some((type) => type.toLowerCase() === fileType.toLowerCase()))?.engine ?? ''; }

export function KnowledgeSettingsPage({ client, knowledgeBaseId }: { client: WeKnoraClient; knowledgeBaseId: string }) {
  const t = createTranslator(useAppLocale());
  const [tab, setTab] = useState<Tab>('settings');
  const [knowledgeBase, setKnowledgeBase] = useState<KnowledgeBase | null>(null);
  const [form, setForm] = useState<KnowledgeBaseSettingsForm | null>(null);
  const [engines, setEngines] = useState<ParserEngineInfo[]>([]);
  const [storageBackends, setStorageBackends] = useState<StorageBackendView[]>([]);
  const [vectorStores, setVectorStores] = useState<VectorStoreView[]>([]);
  const [activity, setActivity] = useState<Array<Record<string, unknown>>>([]);
  const [sample, setSample] = useState('## Sample\n\nPaste representative content to inspect the selected chunking strategy.');
  const [preview, setPreview] = useState<{ selected_tier: string; chunks: Array<Record<string, unknown>>; stats: Record<string, number> } | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const settings = client.knowledge.settings;

  async function load() {
    setLoading(true); setMessage(null);
    const results = await Promise.allSettled([settings.get(knowledgeBaseId), settings.parserEngines(), settings.storageBackends(), settings.vectorStores(), settings.activity(knowledgeBaseId)]);
    const kbResult = results[0];
    if (kbResult.status === 'fulfilled') { setKnowledgeBase(kbResult.value); setForm(formFromKnowledgeBase(kbResult.value)); }
    else setMessage({ tone: 'error', text: errorMessage(kbResult.reason, 'Unable to load knowledge base settings') });
    const parserResult = results[1]; if (parserResult.status === 'fulfilled') setEngines(parserResult.value.data);
    const storageResult = results[2]; if (storageResult.status === 'fulfilled') setStorageBackends(storageResult.value.data.filter((backend) => backend.status === 'active'));
    const vectorResult = results[3]; if (vectorResult.status === 'fulfilled') setVectorStores(vectorResult.value.data);
    const activityResult = results[4]; if (activityResult.status === 'fulfilled') setActivity((activityResult.value.data ?? []) as Array<Record<string, unknown>>);
    setLoading(false);
  }

  useEffect(() => { void load(); }, [client, knowledgeBaseId]);
  const fileTypes = useMemo(() => parserGroups(engines), [engines]);
  const setField = <K extends keyof KnowledgeBaseSettingsForm>(key: K, value: KnowledgeBaseSettingsForm[K]) => setForm((current) => current ? { ...current, [key]: value } : current);
  const updateIndexing = (key: keyof KnowledgeBaseSettingsForm['indexing'], value: boolean) => setForm((current) => current ? { ...current, indexing: { ...current.indexing, [key]: value } } : current);

  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!form) return;
    if (!form.name.trim()) { setMessage({ tone: 'error', text: 'Knowledge base name is required.' }); return; }
    setSaving(true); setMessage(null);
    try { const updated = await settings.update(knowledgeBaseId, buildKnowledgeBaseSettingsInput(form)); setKnowledgeBase(updated); setForm(formFromKnowledgeBase(updated)); setMessage({ tone: 'success', text: 'Knowledge base settings saved.' }); }
    catch (error) { setMessage({ tone: 'error', text: errorMessage(error, 'Unable to save settings; your form is still available.') }); }
    finally { setSaving(false); }
  }

  async function runPreview() {
    if (!form || !sample.trim()) return;
    setPreviewing(true); setMessage(null);
    try { const result = await settings.previewChunking({ text: sample, chunking_config: buildKnowledgeBaseSettingsInput(form).config?.chunking_config ?? {} }); setPreview({ selected_tier: result.selected_tier, chunks: result.chunks.slice(0, 12), stats: result.stats }); }
    catch (error) { setMessage({ tone: 'error', text: errorMessage(error, 'Unable to preview chunking') }); }
    finally { setPreviewing(false); }
  }


  if (tab === 'sources') return <><nav className="wk-settings-tabs"><button type="button" className="wk-settings-tab" onClick={() => setTab('settings')}>{t('knowledgeBase.settings.title')}</button><button type="button" className="wk-settings-tab is-active">{t('datasource.title')}</button></nav><DataSourcesPage client={client} knowledgeBaseId={knowledgeBaseId} /></>;

  return <main className="wk-page wk-knowledge-settings-page">
    <nav className="wk-settings-tabs"><button type="button" className="wk-settings-tab is-active">{t('knowledgeBase.settings.title')}</button><button type="button" className="wk-settings-tab" onClick={() => setTab('sources')}>{t('datasource.title')}</button></nav>
    <header className="wk-header"><div><p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p><h1>{t('knowledgeBase.settings.title')}</h1><p className="wk-muted">{t('knowledgeEditor.chunking.description')}</p></div><Button type="button" onClick={() => void load()} disabled={loading}>{t('knowledgeEditor.activity.refresh')}</Button></header>
    {message ? <Status tone={message.tone}>{message.text}</Status> : null}
    {loading || !form || !knowledgeBase ? <Card><Status>{t('common.loading')}</Status></Card> : <form onSubmit={save}>
      <Card className="wk-settings-section"><h2>{t('knowledgeEditor.basic.title')}</h2><div className="wk-form-grid"><label>{t('knowledgeEditor.basic.nameLabel')}<input required value={form.name} onChange={(event) => setField('name', event.target.value)} /></label><label>{t('knowledgeEditor.basic.descriptionLabel')}<textarea rows={3} value={form.description} onChange={(event) => setField('description', event.target.value)} /></label></div></Card>
      <Card className="wk-settings-section"><h2>{t('kbSettings.parser.title')}</h2><p className="wk-muted">{t('kbSettings.parser.description')}</p>{engines.length === 0 ? <Status tone="warning">{t('kbSettings.parser.noEngineAvailable')}</Status> : <div className="wk-settings-table">{fileTypes.map((fileType) => <label key={fileType}><span>.{fileType}</span><select value={ruleFor(form, fileType)} onChange={(event) => setField('parserRules', updateParserRule(form.parserRules, [fileType], event.target.value))}><option value="">{t('kbSettings.parser.default')}</option>{engines.filter((engine) => engine.FileTypes.includes(fileType)).map((engine) => <option key={engine.Name} value={engine.Name} disabled={engine.Available === false}>{engine.Name}{engine.Available === false ? ` — ${engine.UnavailableReason || t('kbSettings.parser.noEngine')}` : ''}</option>)}</select></label>)}</div>}</Card>
      <Card className="wk-settings-section"><h2>{t('knowledgeEditor.chunking.title')}</h2><div className="wk-form-grid wk-form-grid--three"><label>{t('knowledgeEditor.chunking.sizeLabel')}<input type="number" min={1} value={form.chunkSize} onChange={(event) => setField('chunkSize', Number(event.target.value))} /></label><label>{t('knowledgeEditor.chunking.overlapLabel')}<input type="number" min={0} value={form.chunkOverlap} onChange={(event) => setField('chunkOverlap', Number(event.target.value))} /></label><label>{t('knowledgeEditor.chunking.strategyLabel')}<select value={form.chunkStrategy} onChange={(event) => setField('chunkStrategy', event.target.value)}><option value="auto">{t('knowledgeEditor.chunking.strategies.auto.label')}</option><option value="heading">{t('knowledgeEditor.chunking.strategies.heading.label')}</option><option value="heuristic">{t('knowledgeEditor.chunking.strategies.heuristic.label')}</option><option value="legacy">{t('knowledgeEditor.chunking.strategies.legacy.label')}</option></select></label></div><label className="wk-checkbox"><input type="checkbox" checked={form.parentChild} onChange={(event) => setField('parentChild', event.target.checked)} /> {t('knowledgeEditor.chunking.parentChildLabel')}</label>{form.parentChild ? <div className="wk-form-grid wk-form-grid--two"><label>{t('knowledgeEditor.chunking.parentChunkSizeLabel')}<input type="number" min={1} value={form.parentChunkSize} onChange={(event) => setField('parentChunkSize', Number(event.target.value))} /></label><label>{t('knowledgeEditor.chunking.childChunkSizeLabel')}<input type="number" min={1} value={form.childChunkSize} onChange={(event) => setField('childChunkSize', Number(event.target.value))} /></label></div> : null}<label>{t('knowledgeEditor.advanced.tableMetadataInstructions.label')}<textarea rows={3} value={form.tableMetadataInstructions} onChange={(event) => setField('tableMetadataInstructions', event.target.value)} /></label><div className="wk-preview-box"><label>{t('knowledgeEditor.chunking.debug.sampleLabel')}<textarea rows={5} value={sample} onChange={(event) => setSample(event.target.value)} /></label><Button type="button" onClick={() => void runPreview()} disabled={previewing || !sample.trim()} loading={previewing}>{t('knowledgeEditor.chunking.debug.runButton')}</Button>{preview ? <div className="wk-preview-result"><p>{t('knowledgeEditor.chunking.debug.selectedTier')}: <strong>{preview.selected_tier}</strong> · {t('knowledgeEditor.chunking.debug.stats.chunks')}: {preview.stats.count ?? 0}</p><ol>{preview.chunks.map((chunk, index) => <li key={index}>{typeof chunk.content === 'string' ? chunk.content : JSON.stringify(chunk.content)}</li>)}</ol></div> : null}</div></Card>
      <Card className="wk-settings-section"><h2>{t('knowledgeEditor.indexing.title')}</h2><p className="wk-muted">{t('knowledgeEditor.indexing.description')}</p><div className="wk-toggle-grid">{([['vector_enabled', t('knowledgeEditor.indexing.searchTitle')], ['keyword_enabled', t('knowledgeEditor.indexing.searchTitle')], ['wiki_enabled', t('knowledgeEditor.indexing.wikiTitle')], ['graph_enabled', t('knowledgeEditor.indexing.graphTitle')]] as const).map(([key, label]) => <label key={key} className="wk-checkbox"><input type="checkbox" checked={form.indexing[key]} onChange={(event) => updateIndexing(key, event.target.checked)} /> {label}</label>)}</div><div className="wk-form-grid wk-form-grid--two"><label>{t('knowledgeEditor.models.embeddingLabel')}<input value={form.embeddingModelId} readOnly aria-describedby="model-binding-note" /></label><label>{t('knowledgeEditor.models.llmLabel')}<input value={form.summaryModelId} readOnly aria-describedby="model-binding-note" /></label></div><p id="model-binding-note" className="wk-muted">{t('knowledgeEditor.models.description')}</p></Card>
      <Card className="wk-settings-section"><h2>{t('kbSettings.storage.title')}</h2><p className="wk-muted">{t('kbSettings.storage.migrateHint')}</p><div className="wk-form-grid wk-form-grid--two"><label>{t('kbSettings.storage.instanceLabel')}<select value={form.storageBackendId} disabled><option value="">{t('kbSettings.parser.default')}</option>{storageBackends.map((backend) => <option key={backend.id} value={backend.id}>{backend.name} · {backend.provider}</option>)}</select></label><label>{t('kbSettings.vectorStore.boundLabel')}<select value={form.vectorStoreId} disabled><option value="">{t('kbSettings.vectorStore.systemDefault')}</option>{vectorStores.map((store) => <option key={store.id} value={store.id}>{store.name} · {store.engine_type}</option>)}</select></label></div></Card>
      <div className="wk-form-actions"><Button type="submit" loading={saving}>{t('common.save')}</Button></div>
    </form>}
    <Card className="wk-settings-section"><h2>Activity</h2>{activity.length === 0 ? <Status>No knowledge-base activity was returned, or the caller does not have activity access.</Status> : <ul className="wk-list">{activity.map((entry, index) => <li key={String(entry.id ?? index)}><strong>{String(entry.action ?? 'unknown')}</strong><span>{String(entry.outcome ?? 'unknown')}</span><small>{String(entry.created_at ?? '')}</small></li>)}</ul>}</Card>
  </main>;
}