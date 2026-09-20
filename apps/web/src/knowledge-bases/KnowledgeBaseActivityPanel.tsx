import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { KnowledgeBaseActivityEntry, WeKnoraClient } from '@weknora/api-client';
import { Button, Select, Sheet, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { activityActionTone, activityDateTime, activityOutcomeTone, activityTargetSummary, isCurrentActivityGeneration } from './activity.ts';

type Props = { client: WeKnoraClient; knowledgeBaseId: string; /** R484: inside the KB settings drawer the tab heading already renders the Vue title/description, so the panel header stays hidden. */ embedded?: boolean };
type Filter = { action: string; outcome: string };

const actionValues = ['kb.created', 'kb.updated', 'knowledge.created', 'knowledge.updated', 'knowledge.deleted', 'knowledge.parse_canceled', 'knowledge.reparse_started', 'knowledge.batch_deleted', 'knowledge.move_started', 'knowledge.move_completed', 'knowledge.move_failed', 'datasource.created', 'datasource.updated', 'datasource.deleted', 'datasource.sync_started', 'datasource.sync_completed', 'datasource.sync_failed', 'kb.share_added', 'kb.share_removed', 'kb.share_permission_changed'];
const outcomeValues = ['accepted', 'success', 'completed', 'partial', 'failed', 'denied', 'canceled'];

function actionLabel(action: string, t: (key: string) => string): string {
  const key = `knowledgeEditor.activity.actions.${action}`;
  const translated = t(key);
  return translated === key ? action : translated;
}

function outcomeLabel(outcome: string, t: (key: string) => string): string {
  const key = `knowledgeEditor.activity.outcomes.${outcome}`;
  const translated = t(key);
  return translated === key ? outcome : translated;
}

function targetLabel(entry: KnowledgeBaseActivityEntry, t: (key: string) => string): string {
  const targetType = typeof entry.target_type === 'string' ? entry.target_type : '';
  const key = `knowledgeEditor.activity.targets.${targetType}`;
  const translated = targetType ? t(key) : '';
  const target = translated === key ? targetType : translated;
  const name = typeof entry.target_name === 'string' ? entry.target_name : typeof entry.knowledge_base_name === 'string' ? entry.knowledge_base_name : '';
  return name || target || t('knowledgeEditor.activity.knowledgeBase');
}

export function KnowledgeBaseActivityPanel({ client, knowledgeBaseId, embedded = false }: Props) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [entries, setEntries] = useState<KnowledgeBaseActivityEntry[]>([]);
  const [filter, setFilter] = useState<Filter>({ action: '', outcome: '' });
  const [nextCursor, setNextCursor] = useState<number | undefined>();
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<KnowledgeBaseActivityEntry | null>(null);
  const [actionMenuOpen, setActionMenuOpen] = useState(false);
  const [outcomeMenuOpen, setOutcomeMenuOpen] = useState(false);
  const generationRef = useRef(0);

  const load = useCallback(async (reset: boolean) => {
    if (loading || (!reset && nextCursor === undefined && loaded)) return;
    const requestGeneration = reset ? ++generationRef.current : generationRef.current;
    setLoading(true); setError(null);
    try {
      const result = await client.knowledgeBases.settings.activity(knowledgeBaseId, {
        afterId: reset ? undefined : nextCursor,
        action: filter.action || undefined,
        outcome: filter.outcome || undefined,
        limit: 30,
      });
      if (!isCurrentActivityGeneration(requestGeneration, generationRef.current)) return;
      setEntries((current) => reset ? (result.data ?? []) : [...current, ...(result.data ?? [])]);
      // Vue KnowledgeBaseActivitySettings.vue:651 gates on !!next_cursor — a
      // zero cursor means exhaustion, not a valid cursor.
      setNextCursor(result.next_cursor || undefined);
      setLoaded(true);
    } catch (cause) {
      if (isCurrentActivityGeneration(requestGeneration, generationRef.current)) setError(cause instanceof Error ? cause.message : t('knowledgeEditor.activity.loadFailed'));
    } finally {
      if (isCurrentActivityGeneration(requestGeneration, generationRef.current)) setLoading(false);
    }
  }, [client, filter.action, filter.outcome, knowledgeBaseId, loaded, loading, nextCursor, t]);

  useEffect(() => { setLoaded(false); setNextCursor(undefined); void load(true); }, [knowledgeBaseId, filter.action, filter.outcome]);

  const hasFilters = Boolean(filter.action || filter.outcome);
  const empty = useMemo(() => hasFilters ? t('knowledgeEditor.activity.emptyFiltered') : t('knowledgeEditor.activity.empty'), [hasFilters, t]);
  const clearFilters = () => setFilter({ action: '', outcome: '' });
  const filterOptions = (values: string[], selectedValue: string, onSelect: (value: string) => void, open: boolean, setOpen: (value: boolean) => void, label: string) => <div className="relative inline-flex">
    <button type="button" className={`inline-flex h-7 w-7 items-center justify-center rounded-control border bg-transparent text-xs ${selectedValue ? 'border-accent text-accent' : 'border-transparent text-muted hover:bg-hover-wash'}`} aria-label={label} aria-expanded={open} onClick={() => setOpen(!open)}>⌄</button>
    {open ? <div className="absolute right-0 top-[calc(100%+4px)] z-20 grid min-w-[180px] rounded-control border border-line bg-surface p-1 shadow-[0_8px_20px_rgb(16_24_40/14%)]" role="menu">
      {['', ...values].map((value) => <button key={value || '__all__'} type="button" role="menuitemradio" aria-checked={selectedValue === value} className={`flex items-center justify-between rounded-[4px] border-0 bg-transparent px-2 py-1.5 text-left text-xs hover:bg-hover-wash ${selectedValue === value ? 'text-accent' : 'text-ink'}`} onClick={() => { onSelect(value); setOpen(false); }}>{value ? (label === t('knowledgeEditor.activity.columns.action') ? actionLabel(value, t) : outcomeLabel(value, t)) : (label === t('knowledgeEditor.activity.columns.action') ? t('knowledgeEditor.activity.allActions') : t('knowledgeEditor.activity.allOutcomes'))}{selectedValue === value ? <span aria-hidden="true">✓</span> : null}</button>)}
    </div> : null}
  </div>;

  return <div className="grid gap-3" aria-label={t('knowledgeEditor.activity.title')}>
    <div className="flex items-start justify-between gap-3">
      {embedded ? null : <div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.activity.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.activity.description')}</p></div>}
      {/* Vue title-row refresh (suggested-questions-refresh): an icon-only
          button carrying the accessible name, no visible text. */}
      <Button type="button" variant="text" aria-label={t('knowledgeEditor.activity.refresh')} title={t('knowledgeEditor.activity.refresh')} onClick={() => void load(true)} disabled={loading}><span aria-hidden="true">↻</span></Button>
    </div>
    {hasFilters ? <div className="flex items-center gap-2 rounded-control bg-surface-muted px-3 py-2 text-xs text-muted"><span>{filter.action ? actionLabel(filter.action, t) : t('knowledgeEditor.activity.allActions')} · {filter.outcome ? outcomeLabel(filter.outcome, t) : t('knowledgeEditor.activity.allOutcomes')}</span><Button type="button" onClick={clearFilters}>{t('knowledgeEditor.activity.clearFilters')}</Button></div> : null}
    {error ? <Status tone="error">{error}<Button type="button" onClick={() => void load(true)}>{t('knowledgeEditor.activity.retry')}</Button></Status> : null}
    {!error && loading && entries.length === 0 ? <Status>{t('common.loading')}</Status> : null}
    {!error && !loading && entries.length === 0 ? <Status>{empty}</Status> : null}
    {entries.length > 0 ? <div className="overflow-auto rounded-card border border-line-soft"><table className="w-full min-w-[620px] border-collapse text-sm"><thead><tr className="border-b border-line-soft bg-surface-muted text-left text-xs text-muted"><th className="px-3 py-2"><span className="inline-flex items-center gap-1">{t('knowledgeEditor.activity.columns.action')}{filterOptions(actionValues, filter.action, (value) => setFilter((current) => ({ ...current, action: value })), actionMenuOpen, setActionMenuOpen, t('knowledgeEditor.activity.columns.action'))}</span></th><th className="px-3 py-2"><span className="inline-flex items-center gap-1">{t('knowledgeEditor.activity.columns.outcome')}{filterOptions(outcomeValues, filter.outcome, (value) => setFilter((current) => ({ ...current, outcome: value })), outcomeMenuOpen, setOutcomeMenuOpen, t('knowledgeEditor.activity.columns.outcome'))}</span></th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.target')}</th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.actor')}</th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.time')}</th></tr></thead><tbody>{entries.map((entry) => { const target = activityTargetSummary(entry); const time = activityDateTime(entry.created_at, locale); return <tr key={entry.id} className="cursor-pointer border-b border-line-soft last:border-0 hover:bg-surface-muted" tabIndex={0} onClick={() => setSelected(entry)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(entry); } }}><td className="px-3 py-2"><span className={`rounded-pill border border-line-soft px-2 py-1 text-xs ${activityActionTone(entry.action) === 'error' ? 'text-danger' : activityActionTone(entry.action) === 'success' ? 'text-success-text' : activityActionTone(entry.action) === 'warning' ? 'text-warning-text' : ''}`}>{actionLabel(entry.action, t)}</span></td><td className="px-3 py-2"><span className={`rounded-pill bg-surface-muted px-2 py-1 text-xs ${activityOutcomeTone(entry.outcome) === 'error' ? 'text-danger' : activityOutcomeTone(entry.outcome) === 'success' ? 'text-success-text' : activityOutcomeTone(entry.outcome) === 'warning' ? 'text-warning-text' : ''}`}>{outcomeLabel(entry.outcome, t)}</span></td><td className="px-3 py-2"><span className="block truncate">{target.subject || '—'}</span>{target.change ? <small className="block text-muted">{target.change}</small> : null}</td><td className="px-3 py-2 text-muted">{typeof entry.actor_username === 'string' ? entry.actor_username : entry.actor_user_id ? String(entry.actor_user_id) : t('knowledgeEditor.activity.systemActor')}</td><td className="px-3 py-2 text-muted"><span className="block">{time.date}</span><span className="block text-xs">{time.time}</span></td></tr>; })}</tbody></table></div> : null}
    {/* Vue audit end hint (audit-end-hint): 「没有更早的记录了」 renders once
        the cursor is exhausted; the load-more affordance only exists while
        another page remains. */}
    {nextCursor !== undefined && entries.length > 0 ? <Button type="button" onClick={() => void load(false)} disabled={loading}>{loading ? t('knowledgeEditor.activity.loadingMore') : t('knowledgeEditor.wikiBrowser.loadMoreShort')}</Button> : null}
    {nextCursor === undefined && entries.length > 0 && !loading ? <p className="m-0 text-xs text-muted" data-activity-end-hint="">{t('knowledgeEditor.activity.end')}</p> : null}
    <Sheet open={selected !== null} title={selected ? actionLabel(selected.action, t) : ''} onClose={() => setSelected(null)} side="right" width="640px" resizable minWidth={480} maxWidth={960} storageKey="setting-drawer:width:kb-activity-detail">{selected ? <div className="grid gap-4 text-sm"><section><h4 className="m-0 mb-2 text-sm font-semibold">{t('knowledgeEditor.activity.drawer.sectionSummary')}</h4><dl className="m-0 grid gap-2">{[['time', activityDateTime(selected.created_at, locale).date + ' ' + activityDateTime(selected.created_at, locale).time], ['action', actionLabel(selected.action, t)], ['outcome', outcomeLabel(selected.outcome, t)], ['target', targetLabel(selected, t)]].map(([key, value]) => <div key={key} className="grid grid-cols-[9rem_minmax(0,1fr)] gap-3 border-b border-line-soft py-2"><dt className="text-muted">{key}</dt><dd className="m-0 break-words">{value}</dd></div>)}</dl></section><section><h4 className="m-0 mb-2 text-sm font-semibold">{t('knowledgeEditor.activity.drawer.sectionIdentifiers')}</h4><div className="grid grid-cols-[9rem_minmax(0,1fr)] gap-3 border-b border-line-soft py-2"><span className="text-muted">id</span><code className="break-all">{selected.id}</code></div></section><section><h4 className="m-0 mb-2 text-sm font-semibold">{t('knowledgeEditor.activity.drawer.sectionTask')}</h4><pre className="m-0 max-h-64 overflow-auto rounded-control bg-surface-muted p-3 text-xs">{JSON.stringify(selected.details ?? {}, null, 2)}</pre></section><section><h4 className="m-0 mb-2 text-sm font-semibold">JSON</h4><pre className="m-0 max-h-64 overflow-auto rounded-control border border-line-soft bg-surface-muted p-3 text-xs">{JSON.stringify(selected, null, 2)}</pre></section></div> : null}</Sheet>
  </div>;
}
