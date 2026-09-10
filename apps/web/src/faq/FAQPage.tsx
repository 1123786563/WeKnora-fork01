import { useEffect, useRef, useState } from 'react';
import type { FAQEntry, FAQEntryFieldsUpdate, FAQEntryPayload, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { normalizeFAQPayload, parseFAQImportText } from './import-export.ts';

type FormState = { question: string; similar: string; negative: string; answers: string; tagId: string; enabled: boolean; recommended: boolean };
const emptyForm: FormState = { question: '', similar: '', negative: '', answers: '', tagId: '', enabled: true, recommended: false };

function formFrom(entry: FAQEntry | null): FormState {
  return entry ? { question: entry.standard_question, similar: entry.similar_questions.join('\n'), negative: entry.negative_questions.join('\n'), answers: entry.answers.join('\n'), tagId: typeof entry.tag_id === 'number' ? String(entry.tag_id) : '', enabled: entry.is_enabled, recommended: entry.is_recommended } : emptyForm;
}
function payloadFrom(form: FormState): FAQEntryPayload {
  return normalizeFAQPayload({ standard_question: form.question, similar_questions: form.similar.split('\n'), negative_questions: form.negative.split('\n'), answers: form.answers.split('\n'), tag_id: form.tagId.trim() ? Number(form.tagId) : null, is_enabled: form.enabled, is_recommended: form.recommended });
}
function downloadText(text: string, format: 'csv' | 'json') {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([text], { type: format === 'json' ? 'application/json' : 'text/csv' }));
  link.download = `faq-export.${format}`;
  link.click();
  URL.revokeObjectURL(link.href);
}

export function FAQPage({ client, knowledgeBaseId }: { client: WeKnoraClient; knowledgeBaseId: string }) {
  const [entries, setEntries] = useState<FAQEntry[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [editing, setEditing] = useState<FAQEntry | null | undefined>(undefined);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ tone: 'error' | 'success' | 'warning'; text: string } | null>(null);
  const [importMode, setImportMode] = useState<'append' | 'replace'>('append');
  const [batchTag, setBatchTag] = useState('');
  const fileInput = useRef<HTMLInputElement>(null);
  const faq = client.knowledge.faq;

  async function load() {
    setLoading(true);
    try { const result = await faq.list(knowledgeBaseId, { page: 1, page_size: 50, keyword: keyword || undefined }); setEntries(result.data); setSelected(new Set()); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to load FAQ entries' }); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); }, [client, knowledgeBaseId, keyword]);

  function openEditor(entry: FAQEntry | null = null) { setEditing(entry); setForm(formFrom(entry)); setMessage(null); }
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setMessage(null);
    try { const payload = payloadFrom(form); if (editing) await faq.update(knowledgeBaseId, editing.id, payload); else await faq.create(knowledgeBaseId, payload); setMessage({ tone: 'success', text: editing ? 'FAQ entry updated.' : 'FAQ entry created.' }); setEditing(undefined); await load(); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to save FAQ entry' }); }
    finally { setSaving(false); }
  }
  async function updateSelection(input: FAQEntryFieldsUpdate) {
    if (!selected.size) return;
    try { await faq.updateFields(knowledgeBaseId, { by_id: Object.fromEntries([...selected].map((id) => [id, input])) }); await load(); setMessage({ tone: 'success', text: 'Selected FAQ entries updated.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to update selected entries' }); }
  }
  async function updateSelectedTag() {
    if (!selected.size) return;
    const tagId = batchTag.trim() ? Number(batchTag) : null;
    if (tagId !== null && (!Number.isSafeInteger(tagId) || tagId < 0)) { setMessage({ tone: 'error', text: 'Tag ID must be a non-negative integer.' }); return; }
    try { await faq.updateTags(knowledgeBaseId, { updates: Object.fromEntries([...selected].map((id) => [id, tagId])) }); await load(); setBatchTag(''); setMessage({ tone: 'success', text: 'Selected FAQ tags updated.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to update selected tags' }); }
  }
  async function removeSelected() {
    if (!selected.size) return;
    try { await faq.removeMany(knowledgeBaseId, [...selected]); await load(); setMessage({ tone: 'success', text: 'Selected FAQ entries deleted.' }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to delete selected entries' }); }
  }
  async function importFile(file: File) {
    try { const text = await file.text(); const format = file.name.toLowerCase().endsWith('.json') ? 'json' : 'csv'; const entries = parseFAQImportText(text, format); const result = await faq.upsert(knowledgeBaseId, { entries, mode: importMode }); await load(); setMessage({ tone: 'success', text: `Import ${importMode} queued (${result.task_id}).` }); }
    catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to import FAQ entries' }); }
    finally { if (fileInput.current) fileInput.current.value = ''; }
  }
  async function exportEntries(format: 'csv' | 'json') { try { downloadText(await faq.exportEntries(knowledgeBaseId, format), format); } catch (error) { setMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Unable to export FAQ entries' }); } }

  return <main className="wk-page wk-faq-page"><header className="wk-header"><div><p className="wk-eyebrow">Knowledge base · {knowledgeBaseId}</p><h1>FAQ library</h1><p className="wk-muted">Create, import, filter and maintain question-answer entries.</p></div><div className="wk-list-actions"><Button type="button" onClick={() => openEditor()}>New FAQ</Button><label>Import mode <select value={importMode} onChange={(event) => setImportMode(event.target.value as 'append' | 'replace')}><option value="append">Append</option><option value="replace">Replace</option></select></label><Button type="button" onClick={() => fileInput.current?.click()}>Import CSV/JSON</Button><input ref={fileInput} hidden type="file" accept=".csv,.json,application/json,text/csv" onChange={(event) => { const file = event.target.files?.[0]; if (file) void importFile(file); }} /></div></header>
    <Card><div className="wk-toolbar" role="search"><label>Search <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="Standard question" /></label><Button type="button" onClick={() => void exportEntries('csv')}>Export CSV</Button><Button type="button" onClick={() => void exportEntries('json')}>Export JSON</Button></div>
      {message ? <Status tone={message.tone}>{message.text}</Status> : null}
      {selected.size ? <div className="wk-list-actions" aria-label="FAQ batch actions"><span>{selected.size} selected</span><Button type="button" onClick={() => void updateSelection({ is_enabled: true })}>Enable</Button><Button type="button" onClick={() => void updateSelection({ is_enabled: false })}>Disable</Button><Button type="button" onClick={() => void updateSelection({ is_recommended: true })}>Recommend</Button><label>Tag <input className="wk-batch-tag" inputMode="numeric" value={batchTag} onChange={(event) => setBatchTag(event.target.value)} placeholder="ID or blank" /></label><Button type="button" onClick={() => void updateSelectedTag()}>Set tag</Button><Button type="button" onClick={() => void removeSelected()}>Delete</Button></div> : null}
      {loading ? <Status>Loading FAQ entries…</Status> : entries.length === 0 ? <Status>No FAQ entries match the current search.</Status> : <ul className="wk-list wk-faq-list"><li><label><input type="checkbox" checked={selected.size === entries.length} onChange={(event) => setSelected(event.target.checked ? new Set(entries.map((entry) => entry.id)) : new Set())} /> Select all</label><span>{entries.length} shown</span></li>{entries.map((entry) => <li key={entry.id} className="wk-faq-item"><label><input type="checkbox" checked={selected.has(entry.id)} onChange={(event) => setSelected((current) => { const next = new Set(current); if (event.target.checked) next.add(entry.id); else next.delete(entry.id); return next; })} /></label><div className="wk-list-item-copy"><strong>{entry.standard_question}</strong><span>{entry.answers.join(' · ')}</span><small>{entry.similar_questions.length} similar · {entry.negative_questions.length} negative · {entry.is_enabled ? 'enabled' : 'disabled'}{entry.is_recommended ? ' · recommended' : ''}{typeof entry.tag_id === 'number' ? ` · tag ${entry.tag_id}` : ''}</small></div><Button type="button" onClick={() => openEditor(entry)}>Edit</Button></li>)}</ul>}
    </Card>
    {editing !== undefined ? <section className="wk-faq-editor" aria-label="FAQ editor"><Card><div className="wk-header"><h2>{editing ? 'Edit FAQ' : 'New FAQ'}</h2><Button type="button" onClick={() => setEditing(undefined)}>Close</Button></div><form className="wk-wiki-editor" onSubmit={save}><label>Standard question <input required value={form.question} onChange={(event) => setForm({ ...form, question: event.target.value })} /></label><label>Similar questions <textarea rows={3} value={form.similar} onChange={(event) => setForm({ ...form, similar: event.target.value })} placeholder="One per line" /></label><label>Negative questions <textarea rows={3} value={form.negative} onChange={(event) => setForm({ ...form, negative: event.target.value })} placeholder="One per line" /></label><label>Answers <textarea required rows={4} value={form.answers} onChange={(event) => setForm({ ...form, answers: event.target.value })} placeholder="One answer per line" /></label><label>Tag ID <input inputMode="numeric" value={form.tagId} onChange={(event) => setForm({ ...form, tagId: event.target.value })} /></label><label><input type="checkbox" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} /> Enabled</label><label><input type="checkbox" checked={form.recommended} onChange={(event) => setForm({ ...form, recommended: event.target.checked })} /> Recommended</label><Button type="submit" loading={saving}>Save FAQ</Button></form></Card></section> : null}
  </main>;
}
