import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { KnowledgeBaseActivityEntry, WeKnoraClient } from '@weknora/api-client';
// S6 抽屉收编：packages/ui 旧栈 离栈（T15 硬前置）——详情抽屉 Sheet→tdesign
// Drawer（footer=false/visible/header/placement/size；Sheet 的 resizable 无对应，
// 收编后宽度固定 640px，报告已注记）。Select import 原本未使用，一并移除。
import { Button as TButton, Drawer as TDrawer } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { activityActionTone, activityDateTime, activityOutcomeTone, activityTargetSummary, isCurrentActivityGeneration } from './activity.ts';
import './kb-u.css';

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
  const [me, setMe] = useState<{ user?: { id?: unknown; username?: unknown; email?: unknown } } | null>(null);
  const generationRef = useRef(0);

  // Vue actorLabel (KnowledgeBaseActivitySettings.vue:506-512) resolves the
  // actor against the signed-in user; the id never renders in full.
  useEffect(() => {
    let active = true;
    const fetchMe = (client as { auth?: { me?: () => Promise<unknown> } }).auth?.me;
    if (fetchMe) void fetchMe.call(client).then((value) => { if (active) setMe(value as { user?: { id?: unknown; username?: unknown; email?: unknown } }); }).catch(() => {});
    return () => { active = false; };
  }, [client]);

  function actorLabel(entry: KnowledgeBaseActivityEntry): string {
    const id = entry.actor_user_id;
    if (!id) return t('knowledgeEditor.activity.systemActor');
    const meUser = me?.user;
    if (meUser?.id !== undefined && meUser?.id !== null && String(meUser.id) === String(id)) {
      const name = typeof meUser.username === 'string' ? meUser.username.trim() : '';
      const email = typeof meUser.email === 'string' ? meUser.email.trim() : '';
      if (name || email) return name || email;
    }
    return String(id).slice(0, 8);
  }

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
  const filterOptions = (values: string[], selectedValue: string, onSelect: (value: string) => void, open: boolean, setOpen: (value: boolean) => void, label: string) => <div className="wk-kba-1">
    <button type="button" className={`wk-kba-29 ${selectedValue ? 'wk-kba-30' : 'wk-kba-31'}`} aria-label={label} aria-expanded={open} onClick={() => setOpen(!open)}>⌄</button>
    {open ? <div className="wk-kba-2" role="menu">
      {['', ...values].map((value) => <button key={value || '__all__'} type="button" role="menuitemradio" aria-checked={selectedValue === value} className={`wk-kba-32 ${selectedValue === value ? 'wk-kba-33' : 'wk-kba-34'}`} onClick={() => { onSelect(value); setOpen(false); }}>{value ? (label === t('knowledgeEditor.activity.columns.action') ? actionLabel(value, t) : outcomeLabel(value, t)) : (label === t('knowledgeEditor.activity.columns.action') ? t('knowledgeEditor.activity.allActions') : t('knowledgeEditor.activity.allOutcomes'))}{selectedValue === value ? <span aria-hidden="true">✓</span> : null}</button>)}
    </div> : null}
  </div>;

  return <div className="wk-kba-3" aria-label={t('knowledgeEditor.activity.title')}>
    <div className="wk-kba-4">
      {embedded ? null : <div><h3 className="wk-kba-5">{t('knowledgeEditor.activity.title')}</h3><p className="wk-kba-6">{t('knowledgeEditor.activity.description')}</p></div>}
      {/* Vue title-row refresh (suggested-questions-refresh): an icon-only
          button carrying the accessible name, no visible text. */}
      <TButton type="button" variant="text" aria-label={t('knowledgeEditor.activity.refresh')} title={t('knowledgeEditor.activity.refresh')} onClick={() => void load(true)} disabled={loading}><span aria-hidden="true">↻</span></TButton>
    </div>
    {hasFilters ? <div className="wk-kba-7"><span>{filter.action ? actionLabel(filter.action, t) : t('knowledgeEditor.activity.allActions')} · {filter.outcome ? outcomeLabel(filter.outcome, t) : t('knowledgeEditor.activity.allOutcomes')}</span><TButton type="button" onClick={clearFilters}>{t('knowledgeEditor.activity.clearFilters')}</TButton></div> : null}
    {error ? <Status tone="error">{error}<TButton type="button" onClick={() => void load(true)}>{t('knowledgeEditor.activity.retry')}</TButton></Status> : null}
    {!error && loading && entries.length === 0 ? <Status>{t('common.loading')}</Status> : null}
    {!error && !loading && entries.length === 0 ? <Status>{empty}</Status> : null}
    {entries.length > 0 ? <div className="wk-kba-8"><table className="wk-kba-9"><thead><tr className="wk-kba-10"><th className="wk-kba-11"><span className="wk-kba-12">{t('knowledgeEditor.activity.columns.action')}{filterOptions(actionValues, filter.action, (value) => setFilter((current) => ({ ...current, action: value })), actionMenuOpen, setActionMenuOpen, t('knowledgeEditor.activity.columns.action'))}</span></th><th className="wk-kba-11"><span className="wk-kba-12">{t('knowledgeEditor.activity.columns.outcome')}{filterOptions(outcomeValues, filter.outcome, (value) => setFilter((current) => ({ ...current, outcome: value })), outcomeMenuOpen, setOutcomeMenuOpen, t('knowledgeEditor.activity.columns.outcome'))}</span></th><th className="wk-kba-11">{t('knowledgeEditor.activity.columns.target')}</th><th className="wk-kba-11">{t('knowledgeEditor.activity.columns.actor')}</th><th className="wk-kba-11">{t('knowledgeEditor.activity.columns.time')}</th></tr></thead><tbody>{entries.map((entry) => { const target = activityTargetSummary(entry); const time = activityDateTime(entry.created_at, locale); return <tr key={entry.id} className="wk-kba-13" tabIndex={0} onClick={() => setSelected(entry)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(entry); } }}><td className="wk-kba-11"><span className={`wk-kba-35 ${activityActionTone(entry.action) === 'error' ? 'wk-kba-36' : activityActionTone(entry.action) === 'success' ? 'wk-kba-37' : activityActionTone(entry.action) === 'warning' ? 'wk-kba-38' : ''}`}>{actionLabel(entry.action, t)}</span></td><td className="wk-kba-11"><span className={`wk-kba-39 ${activityOutcomeTone(entry.outcome) === 'error' ? 'wk-kba-36' : activityOutcomeTone(entry.outcome) === 'success' ? 'wk-kba-37' : activityOutcomeTone(entry.outcome) === 'warning' ? 'wk-kba-38' : ''}`}>{outcomeLabel(entry.outcome, t)}</span></td><td className="wk-kba-11"><span className="wk-kba-14">{target.subject || '—'}</span>{target.change ? <small className="wk-kba-15">{target.change}</small> : null}</td><td className="wk-kba-16">{actorLabel(entry)}</td><td className="wk-kba-16"><span className="wk-kba-17">{time.date}</span><span className="wk-kba-18">{time.time}</span></td></tr>; })}</tbody></table></div> : null}
    {/* Vue audit end hint (audit-end-hint): 「没有更早的记录了」 renders once
        the cursor is exhausted; the load-more affordance only exists while
        another page remains. */}
    {nextCursor !== undefined && entries.length > 0 ? <TButton type="button" onClick={() => void load(false)} disabled={loading}>{loading ? t('knowledgeEditor.activity.loadingMore') : t('knowledgeEditor.wikiBrowser.loadMoreShort')}</TButton> : null}
    {nextCursor === undefined && entries.length > 0 && !loading ? <p className="wk-kba-19" data-activity-end-hint="">{t('knowledgeEditor.activity.end')}</p> : null}
    <TDrawer footer={false} visible={selected !== null} header={selected ? actionLabel(selected.action, t) : ''} onClose={() => setSelected(null)} placement="right" size="640px">{selected ? <div className="wk-kba-20"><section><h4 className="wk-kba-21">{t('knowledgeEditor.activity.drawer.sectionSummary')}</h4><dl className="wk-kba-22">{[['time', activityDateTime(selected.created_at, locale).date + ' ' + activityDateTime(selected.created_at, locale).time], ['action', actionLabel(selected.action, t)], ['outcome', outcomeLabel(selected.outcome, t)], ['target', targetLabel(selected, t)]].map(([key, value]) => <div key={key} className="wk-kba-23"><dt className="wk-kba-24">{key}</dt><dd className="wk-kba-25">{value}</dd></div>)}</dl></section><section><h4 className="wk-kba-21">{t('knowledgeEditor.activity.drawer.sectionIdentifiers')}</h4><div className="wk-kba-23"><span className="wk-kba-24">id</span><code className="wk-kba-26">{selected.id}</code></div></section><section><h4 className="wk-kba-21">{t('knowledgeEditor.activity.drawer.sectionTask')}</h4><pre className="wk-kba-27">{JSON.stringify(selected.details ?? {}, null, 2)}</pre></section><section><h4 className="wk-kba-21">JSON</h4><pre className="wk-kba-28">{JSON.stringify(selected, null, 2)}</pre></section></div> : null}</TDrawer>
  </div>;
}
