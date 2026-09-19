/**
 * Sub-agent delegation section of the agent editor (Octop M3 — no Vue
 * baseline). Shows the agent's installed roles (config.subagents; delegation
 * is off while the list is empty) with one-click removal, plus a catalog
 * browser over client.subagents.catalog(): division filter chips (label +
 * count), a client-side search box (slug / name_zh / name_en) and one install
 * button per entry. Install/remove ride the agent-scoped subagents endpoints
 * and patch config.subagents straight from the response list — the backend
 * owns the persisted agent config, so the form never composes the list itself.
 */
import { useEffect, useMemo, useState } from 'react';
import type { SubagentCatalog, SubagentCatalogEntry, WeKnoraClient } from '@weknora/api-client';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { usePreferredLocale } from '../locale.ts';
import type { AgentConfigForm, Translate } from './agent-editor.ts';

// Mirrors the modal's private constants (AgentEditorModal.tsx FIELD_BASE /
// AE_BTN) so the section matches sibling styling without widening exports.
const FIELD_BASE = 'w-full rounded-md border border-[var(--td-component-stroke,#dcdcdc)] px-2.5 py-1.5 font-[family-name:inherit] text-[14px] text-inherit bg-[var(--td-bg-color-container,#fff)]';
const FIELD_DISABLED_BG = 'disabled:bg-[var(--td-bg-color-secondarycontainer,#f2f3f5)]';
const AE_BTN = 'cursor-pointer rounded-md px-4 py-1.5 text-[14px]';
const AE_BTN_SMALL = 'cursor-pointer rounded-md border border-[var(--td-brand-color,#0052d9)] bg-[var(--td-bg-color-container,#fff)] px-2.5 py-[2px] text-[12px] text-[var(--td-brand-color,#0052d9)] hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)] disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:bg-[var(--td-bg-color-container,#fff)]';

/** catalog color fields are server-controlled; only plain hex literals reach
 * the style attribute (ExpertsPage dotColor precedent). */
const FALLBACK_DOT_COLOR = 'rgba(127,127,127,0.35)';
function dotColor(color: string): string {
  return /^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(color) ? color : FALLBACK_DOT_COLOR;
}

/** Display name at the app locale: zh name on zh locale, en otherwise, with
 * fallback to whichever is non-empty (binding locale fallback semantics). */
function nameAtLocale(entry: { name_zh: string; name_en: string }, zh: boolean): string {
  if (zh) return entry.name_zh !== '' ? entry.name_zh : entry.name_en;
  return entry.name_en !== '' ? entry.name_en : entry.name_zh;
}

export interface SubagentsSectionProps {
  config: AgentConfigForm;
  patchConfig: <K extends keyof AgentConfigForm>(key: K, value: AgentConfigForm[K]) => void;
  client: WeKnoraClient;
  t: Translate;
  /** Agent id; empty while the editor is a create session that has not been
   * saved yet — install/remove need the persisted agent, so they stay gated. */
  agentId: string;
}

export function SubagentsSection({ config, patchConfig, client, t, agentId }: SubagentsSectionProps) {
  const locale = usePreferredLocale();
  const zh = locale === 'zh-CN';
  const [catalog, setCatalog] = useState<SubagentCatalog | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [division, setDivision] = useState('');
  const [query, setQuery] = useState('');
  const [busySlug, setBusySlug] = useState('');
  const [actionError, setActionError] = useState<string | null>(null);
  // detail peek: slug → rendered body markdown, fetched once per slug; an
  // in-flight flag (not the error) gates refetch so reopening retries a failure
  const [expanded, setExpanded] = useState('');
  const [details, setDetails] = useState<Record<string, string>>({});
  const [pendingDetails, setPendingDetails] = useState<Record<string, boolean>>({});
  const [detailErrors, setDetailErrors] = useState<Record<string, boolean>>({});

  // Catalog load, once per mount; stale responses dropped via `active`
  // (PersonaSection mbti.types precedent).
  useEffect(() => {
    let active = true;
    setLoadFailed(false);
    void client.subagents.catalog().then((rows) => {
      if (!active) return;
      setCatalog(rows);
    }).catch(() => {
      if (!active) return;
      setLoadFailed(true);
    });
    return () => { active = false; };
  }, [client, attempt]);

  const installed = config.subagents ?? [];
  const bySlug = useMemo(() => {
    const map = new Map<string, SubagentCatalogEntry>();
    for (const entry of catalog?.entries ?? []) map.set(entry.slug, entry);
    return map;
  }, [catalog]);

  const entries = useMemo(() => {
    const all = catalog?.entries ?? [];
    const needle = query.trim().toLowerCase();
    return all.filter((entry) => {
      if (division !== '' && entry.division !== division) return false;
      if (needle === '') return true;
      return entry.slug.toLowerCase().includes(needle)
        || entry.name_zh.toLowerCase().includes(needle)
        || entry.name_en.toLowerCase().includes(needle);
    });
  }, [catalog, division, query]);

  async function applyInstalled(promise: Promise<string[]>): Promise<void> {
    try {
      const next = await promise;
      setActionError(null);
      patchConfig('subagents', next);
    } catch {
      setActionError(t('agentEditor.subagents.actionFailed'));
    } finally {
      setBusySlug('');
    }
  }

  function toggleDetail(entry: SubagentCatalogEntry): void {
    if (expanded === entry.slug) { setExpanded(''); return; }
    setExpanded(entry.slug);
    if (details[entry.slug] !== undefined || pendingDetails[entry.slug]) return;
    setPendingDetails((current) => ({ ...current, [entry.slug]: true }));
    setDetailErrors((current) => ({ ...current, [entry.slug]: false }));
    void client.subagents.get(entry.slug).then((detail) => {
      setDetails((current) => ({ ...current, [entry.slug]: zh ? detail.body_zh : detail.body_en }));
    }).catch(() => {
      setDetailErrors((current) => ({ ...current, [entry.slug]: true }));
    }).finally(() => {
      setPendingDetails((current) => ({ ...current, [entry.slug]: false }));
    });
  }

  const canMutate = agentId !== '' && busySlug === '';

  return (
    <section className="wk-ae-section" data-editor-section="subagents">
      <header className="[&_h2]:m-0 [&_h2]:mb-1 [&_h2]:text-[16px]">
        <h2>{t('agentEditor.subagents.title')}</h2>
        <p className="m-0 mb-4 text-[12px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]">{t('agentEditor.subagents.desc')}</p>
      </header>
      <div className="flex flex-col gap-[18px]">
        {/* installed roles — the delegate tool only registers when non-empty */}
        <div data-subagent-installed>
          <p className="m-0 mb-1.5 text-[13px] font-semibold">{t('agentEditor.subagents.installedLabel')}</p>
          {installed.length === 0 ? (
            <p className="m-0 text-[13px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-subagent-empty-hint>{t('agentEditor.subagents.delegationOff')}</p>
          ) : (
            <div className="flex w-full max-w-[560px] flex-col gap-1">
              {installed.map((slug) => {
                const entry = bySlug.get(slug);
                const name = entry ? nameAtLocale(entry, zh) : slug;
                return (
                  <div key={slug} className="flex items-center gap-2 rounded-md border border-[var(--td-component-stroke,#e7e7e7)] px-2 py-1.5 text-[13px]" data-subagent-installed-row={slug}>
                    <span aria-hidden="true" className="inline-block h-[8px] w-[8px] shrink-0 rounded-full" style={{ background: dotColor(entry?.color ?? '') }} />
                    {entry && entry.emoji !== '' ? <span aria-hidden="true">{entry.emoji}</span> : null}
                    <span className="font-medium">{name}</span>
                    <span className="text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{slug}</span>
                    <button
                      type="button"
                      className="ml-auto cursor-pointer rounded-md border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] px-2.5 py-[2px] text-[12px] text-inherit hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)] disabled:cursor-not-allowed disabled:opacity-60"
                      data-subagent-remove={slug}
                      disabled={!canMutate}
                      onClick={() => {
                        if (!canMutate) return;
                        setBusySlug(slug);
                        void applyInstalled(client.subagents.remove(agentId, slug));
                      }}
                    >{busySlug === slug ? t('common.loading') : t('agentEditor.subagents.remove')}</button>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* catalog browser */}
        <div data-subagent-catalog>
          <p className="m-0 mb-1.5 text-[13px] font-semibold">{t('agentEditor.subagents.catalogLabel')}</p>
          {catalog === null && !loadFailed ? (
            <p className="m-0 text-[13px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" role="status">{t('common.loading')}</p>
          ) : null}
          {loadFailed ? (
            <>
              <p className="m-0 text-[13px] text-[var(--td-error-color,#d54941)]" role="alert">{t('agentEditor.subagents.loadFailed')}</p>
              <button
                type="button"
                className={`${AE_BTN} border border-[var(--td-component-stroke,#dcdcdc)] bg-[var(--td-bg-color-container,#fff)] text-inherit hover:bg-[var(--td-bg-color-container-hover,#f3f3f3)]`}
                onClick={() => setAttempt((current) => current + 1)}
              >{t('common.retry')}</button>
            </>
          ) : null}
          {catalog !== null ? (
            <>
              {agentId === '' ? (
                <p className="m-0 mb-1.5 text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]" data-subagent-need-save>{t('agentEditor.subagents.needSave')}</p>
              ) : null}
              <div className="mb-2 flex flex-wrap items-center gap-1.5">
                <button
                  type="button"
                  className={`cursor-pointer rounded-[10px] border px-2.5 py-[3px] text-[12px] ${division === '' ? 'border-[var(--td-brand-color,#0052d9)] text-[var(--td-brand-color,#0052d9)]' : 'border-[var(--td-component-stroke,#e7e7e7)] text-inherit'}`}
                  data-subagent-division=""
                  aria-pressed={division === '' ? 'true' : 'false'}
                  onClick={() => setDivision('')}
                >{t('agentEditor.subagents.allDivisions')} · {catalog.total}</button>
                {catalog.divisions.map((row) => (
                  <button
                    key={row.slug}
                    type="button"
                    className={`inline-flex cursor-pointer items-center gap-1.5 rounded-[10px] border px-2.5 py-[3px] text-[12px] ${division === row.slug ? 'border-[var(--td-brand-color,#0052d9)] text-[var(--td-brand-color,#0052d9)]' : 'border-[var(--td-component-stroke,#e7e7e7)] text-inherit'}`}
                    data-subagent-division={row.slug}
                    aria-pressed={division === row.slug ? 'true' : 'false'}
                    onClick={() => setDivision(row.slug)}
                  >
                    <span aria-hidden="true" className="inline-block h-[7px] w-[7px] shrink-0 rounded-full" style={{ background: dotColor(row.color) }} />
                    {row.label} · {row.count}
                  </button>
                ))}
              </div>
              <input
                type="search"
                className={`${FIELD_BASE} ${FIELD_DISABLED_BG} max-w-[320px]`}
                value={query}
                placeholder={t('agentEditor.subagents.searchPlaceholder')}
                aria-label={t('agentEditor.subagents.searchPlaceholder')}
                data-subagent-search
                onChange={(event) => setQuery(event.target.value)}
              />
              {catalog.entries.length === 0 ? (
                <p className="m-0 mt-2 text-[13px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-subagent-catalog-empty>{t('agentEditor.subagents.emptyCatalog')}</p>
              ) : entries.length === 0 ? (
                <p className="m-0 mt-2 text-[13px] text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" data-subagent-no-match>{t('agentEditor.subagents.noMatch')}</p>
              ) : (
                <div className="mt-2 flex w-full max-w-[560px] flex-col gap-1">
                  {entries.map((entry) => {
                    const has = installed.includes(entry.slug);
                    const open = expanded === entry.slug;
                    return (
                      <div key={entry.slug} className="rounded-md border border-[var(--td-component-stroke,#e7e7e7)]" data-subagent-slug={entry.slug}>
                        <div className="flex items-center gap-2 px-2 py-1.5 text-[13px]">
                          <span aria-hidden="true" className="inline-block h-[8px] w-[8px] shrink-0 rounded-full" style={{ background: dotColor(entry.color) }} />
                          {entry.emoji !== '' ? <span aria-hidden="true">{entry.emoji}</span> : null}
                          <button
                            type="button"
                            className="cursor-pointer border-none bg-transparent p-0 font-medium text-inherit"
                            data-subagent-name={entry.slug}
                            aria-expanded={open ? 'true' : 'false'}
                            onClick={() => toggleDetail(entry)}
                          >{nameAtLocale(entry, zh)}</button>
                          {has ? (
                            <span className="inline-flex items-center gap-0.5 text-[12px] text-[var(--td-success-color,#2ba471)]" data-subagent-installed-tag={entry.slug}>
                              <span aria-hidden="true">✓</span>{t('agentEditor.subagents.installedTag')}
                            </span>
                          ) : null}
                          <span className="ml-auto text-[12px] text-[var(--td-text-color-placeholder,rgba(0,0,0,0.4))]">{entry.slug}</span>
                          {!has ? (
                            <button
                              type="button"
                              className={AE_BTN_SMALL}
                              data-subagent-install={entry.slug}
                              disabled={!canMutate}
                              onClick={() => {
                                if (!canMutate) return;
                                setBusySlug(entry.slug);
                                // locale pins the library copy at the app locale
                                // ("zh-CN"/"en-US" normalize onto zh/en)
                                void applyInstalled(client.subagents.install(agentId, entry.slug, locale));
                              }}
                            >{busySlug === entry.slug ? t('common.loading') : t('agentEditor.subagents.install')}</button>
                          ) : null}
                        </div>
                        {open ? (
                          <div className="border-t border-[var(--td-component-stroke,#e7e7e7)] px-2 py-1.5 text-[12px] leading-5 text-[var(--td-text-color-primary,rgba(0,0,0,0.9))]" data-subagent-detail={entry.slug}>
                            {detailErrors[entry.slug] && !pendingDetails[entry.slug] ? (
                              <p className="m-0 text-[var(--td-error-color,#d54941)]" role="alert">{t('agentEditor.subagents.detailFailed')}</p>
                            ) : details[entry.slug] === undefined ? (
                              <p className="m-0 text-[var(--td-text-color-secondary,rgba(0,0,0,0.6))]" role="status">{t('common.loading')}</p>
                            ) : (
                              /* renderChatMarkdown escapes raw HTML and allow-lists links */
                              <div className="[&_a]:text-[var(--td-brand-color,#0052d9)] [&_a]:underline [&_p]:m-0 [&_p]:mb-1 [&_ul]:m-0 [&_ul]:mb-1 [&_ul]:list-disc [&_ul]:pl-[18px] [&_ol]:m-0 [&_ol]:mb-1 [&_ol]:list-decimal [&_ol]:pl-[18px]" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(details[entry.slug] ?? '') }} />
                            )}
                          </div>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              )}
            </>
          ) : null}
          {actionError ? <p className="m-0 mt-1.5 text-[12px] text-[var(--td-error-color,#d54941)]" role="alert">{actionError}</p> : null}
        </div>
      </div>
    </section>
  );
}
