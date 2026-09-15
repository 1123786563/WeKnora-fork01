import { useCallback, useEffect, useMemo, useState } from 'react';
import type { KnowledgeBaseActivityEntry, WeKnoraClient } from '@weknora/api-client';
import { Button, Select, Sheet, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';

type Props = { client: WeKnoraClient; knowledgeBaseId: string };
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

function formatTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export function KnowledgeBaseActivityPanel({ client, knowledgeBaseId }: Props) {
  const t = createTranslator(useAppLocale());
  const [entries, setEntries] = useState<KnowledgeBaseActivityEntry[]>([]);
  const [filter, setFilter] = useState<Filter>({ action: '', outcome: '' });
  const [nextCursor, setNextCursor] = useState<number | undefined>();
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<KnowledgeBaseActivityEntry | null>(null);

  const load = useCallback(async (reset: boolean) => {
    if (loading || (!reset && nextCursor === undefined && loaded)) return;
    setLoading(true); setError(null);
    try {
      const result = await client.knowledgeBases.settings.activity(knowledgeBaseId, {
        afterId: reset ? undefined : nextCursor,
        action: filter.action || undefined,
        outcome: filter.outcome || undefined,
        limit: 30,
      });
      setEntries((current) => reset ? (result.data ?? []) : [...current, ...(result.data ?? [])]);
      setNextCursor(result.next_cursor);
      setLoaded(true);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('knowledgeEditor.activity.loadFailed'));
    } finally { setLoading(false); }
  }, [client, filter.action, filter.outcome, knowledgeBaseId, loaded, loading, nextCursor, t]);

  useEffect(() => { setLoaded(false); setNextCursor(undefined); void load(true); }, [knowledgeBaseId, filter.action, filter.outcome]);

  const hasFilters = Boolean(filter.action || filter.outcome);
  const empty = useMemo(() => hasFilters ? t('knowledgeEditor.activity.emptyFiltered') : t('knowledgeEditor.activity.empty'), [hasFilters, t]);
  const clearFilters = () => setFilter({ action: '', outcome: '' });

  return <div className="grid gap-3" aria-label={t('knowledgeEditor.activity.title')}>
    <div className="flex items-start justify-between gap-3">
      <div><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('knowledgeEditor.activity.title')}</h3><p className="m-0 mt-1 text-sm leading-[22px] text-muted">{t('knowledgeEditor.activity.description')}</p></div>
      <Button type="button" aria-label={t('knowledgeEditor.activity.refresh')} title={t('knowledgeEditor.activity.refresh')} onClick={() => void load(true)} disabled={loading}>{t('knowledgeEditor.activity.refresh')}</Button>
    </div>
    {hasFilters ? <div className="flex items-center gap-2 rounded-control bg-surface-muted px-3 py-2 text-xs text-muted"><span>{filter.action ? actionLabel(filter.action, t) : t('knowledgeEditor.activity.allActions')} · {filter.outcome ? outcomeLabel(filter.outcome, t) : t('knowledgeEditor.activity.allOutcomes')}</span><Button type="button" onClick={clearFilters}>{t('knowledgeEditor.activity.clearFilters')}</Button></div> : null}
    <div className="grid grid-cols-2 gap-2 max-[680px]:grid-cols-1">
      <label className="grid gap-1 text-xs font-medium text-muted">{t('knowledgeEditor.activity.columns.action')}<Select value={filter.action} onChange={(event) => setFilter((current) => ({ ...current, action: event.target.value }))}><option value="">{t('knowledgeEditor.activity.allActions')}</option>{actionValues.map((value) => <option key={value} value={value}>{actionLabel(value, t)}</option>)}</Select></label>
      <label className="grid gap-1 text-xs font-medium text-muted">{t('knowledgeEditor.activity.columns.outcome')}<Select value={filter.outcome} onChange={(event) => setFilter((current) => ({ ...current, outcome: event.target.value }))}><option value="">{t('knowledgeEditor.activity.allOutcomes')}</option>{outcomeValues.map((value) => <option key={value} value={value}>{outcomeLabel(value, t)}</option>)}</Select></label>
    </div>
    {error ? <Status tone="error">{error}<Button type="button" onClick={() => void load(true)}>{t('knowledgeEditor.activity.retry')}</Button></Status> : null}
    {!error && loading && entries.length === 0 ? <Status>{t('common.loading')}</Status> : null}
    {!error && !loading && entries.length === 0 ? <Status>{empty}</Status> : null}
    {entries.length > 0 ? <div className="overflow-auto rounded-card border border-line-soft"><table className="w-full min-w-[620px] border-collapse text-sm"><thead><tr className="border-b border-line-soft bg-surface-muted text-left text-xs text-muted"><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.action')}</th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.outcome')}</th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.target')}</th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.actor')}</th><th className="px-3 py-2">{t('knowledgeEditor.activity.columns.time')}</th></tr></thead><tbody>{entries.map((entry) => <tr key={entry.id} className="cursor-pointer border-b border-line-soft last:border-0 hover:bg-surface-muted" tabIndex={0} onClick={() => setSelected(entry)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(entry); } }}><td className="px-3 py-2"><span className="rounded-pill border border-line-soft px-2 py-1 text-xs">{actionLabel(entry.action, t)}</span></td><td className="px-3 py-2"><span className="rounded-pill bg-surface-muted px-2 py-1 text-xs">{outcomeLabel(entry.outcome, t)}</span></td><td className="px-3 py-2">{targetLabel(entry, t)}</td><td className="px-3 py-2 text-muted">{typeof entry.actor_username === 'string' ? entry.actor_username : entry.actor_user_id ? String(entry.actor_user_id) : t('knowledgeEditor.activity.systemActor')}</td><td className="px-3 py-2 text-muted">{formatTime(entry.created_at)}</td></tr>)}</tbody></table></div> : null}
    {nextCursor !== undefined && entries.length > 0 ? <Button type="button" onClick={() => void load(false)} disabled={loading}>{loading ? t('knowledgeEditor.activity.loadingMore') : t('wikiBrowser.loadMoreShort')}</Button> : null}
    <Sheet open={selected !== null} title={selected ? actionLabel(selected.action, t) : ''} onClose={() => setSelected(null)} side="right" width="640px" resizable minWidth={480} maxWidth={960} storageKey="setting-drawer:width:kb-activity-detail">{selected ? <div className="grid gap-3 text-sm"><p className="m-0 text-muted">{targetLabel(selected, t)}</p><dl className="m-0 grid gap-2">{[['time', formatTime(selected.created_at)], ['action', actionLabel(selected.action, t)], ['outcome', outcomeLabel(selected.outcome, t)], ['target', targetLabel(selected, t)], ['id', String(selected.id)]].map(([key, value]) => <div key={key} className="grid grid-cols-[9rem_minmax(0,1fr)] gap-3 border-b border-line-soft py-2"><dt className="text-muted">{key}</dt><dd className="m-0 break-words">{value}</dd></div>)}</dl><pre className="max-h-64 overflow-auto rounded-control bg-surface-muted p-3 text-xs">{JSON.stringify(selected, null, 2)}</pre></div> : null}</Sheet>
  </div>;
}
