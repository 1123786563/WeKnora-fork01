/**
 * Sub-agent delegation section of the agent editor (Octop M3 — no Vue
 * baseline). Shows the agent's installed roles (config.subagents; delegation
 * is off while the list is empty) with one-click removal, plus a catalog
 * browser over client.subagents.catalog(): division filter chips (label +
 * count), a client-side search box (slug / name_zh / name_en) and one install
 * button per entry. Install/remove ride the agent-scoped subagents endpoints
 * and patch config.subagents straight from the response list — the backend
 * owns the persisted agent config, so the form never composes the list itself.
 *
 * TDesign 同构迁移（Task 9）：控件换 tdesign-react（Input/Button），布局值
 * 从 Tailwind utility 平移到 agents.td.css（wk-ae-sub-* 段）。
 */
import { useEffect, useMemo, useState } from 'react';
import type { SubagentCatalog, SubagentCatalogEntry, WeKnoraClient } from '@weknora/api-client';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { Button, Input } from 'tdesign-react';
import { usePreferredLocale } from '../locale.ts';
import type { AgentConfigForm, Translate } from './agent-editor.ts';

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
    <div className="section" data-editor-section="subagents">
      <div className="section-header">
        <div className="section-header-title"><h2>{t('agentEditor.subagents.title')}</h2></div>
        <p className="section-description">{t('agentEditor.subagents.desc')}</p>
      </div>
      <div className="settings-group">
        {/* installed roles — the delegate tool only registers when non-empty */}
        <Row label={t('agentEditor.subagents.installedLabel')}>
          {installed.length === 0 ? (
            <p className="desc" data-subagent-empty-hint>{t('agentEditor.subagents.delegationOff')}</p>
          ) : (
            <div className="wk-ae-sub-list">
              {installed.map((slug) => {
                const entry = bySlug.get(slug);
                const name = entry ? nameAtLocale(entry, zh) : slug;
                return (
                  <div key={slug} className="wk-ae-sub-row" data-subagent-installed-row={slug}>
                    <span aria-hidden="true" className="wk-ae-sub-dot" style={{ background: dotColor(entry?.color ?? '') }} />
                    {entry && entry.emoji !== '' ? <span aria-hidden="true">{entry.emoji}</span> : null}
                    <span className="wk-ae-persona-name">{name}</span>
                    <span className="wk-ae-sub-slug">{slug}</span>
                    <Button
                      variant="outline"
                      size="small"
                      className="wk-ae-sub-action"
                      data-subagent-remove={slug}
                      disabled={!canMutate}
                      onClick={() => {
                        if (!canMutate) return;
                        setBusySlug(slug);
                        void applyInstalled(client.subagents.remove(agentId, slug));
                      }}
                    >{busySlug === slug ? t('common.loading') : t('agentEditor.subagents.remove')}</Button>
                  </div>
                );
              })}
            </div>
          )}
        </Row>

        {/* catalog browser */}
        <Row label={t('agentEditor.subagents.catalogLabel')}>
          {catalog === null && !loadFailed ? (
            <p className="desc" role="status">{t('common.loading')}</p>
          ) : null}
          {loadFailed ? (
            <>
              <p className="desc" style={{ color: 'var(--td-error-color)' }} role="alert">{t('agentEditor.subagents.loadFailed')}</p>
              <Button variant="outline" onClick={() => setAttempt((current) => current + 1)}>{t('common.retry')}</Button>
            </>
          ) : null}
          {catalog !== null ? (
            <>
              {agentId === '' ? (
                <p className="desc" data-subagent-need-save>{t('agentEditor.subagents.needSave')}</p>
              ) : null}
              <div className="wk-ae-sub-chips">
                <button
                  type="button"
                  className={`wk-ae-sub-chip${division === '' ? ' wk-ae-sub-chip--active' : ''}`}
                  data-subagent-division=""
                  aria-pressed={division === '' ? 'true' : 'false'}
                  onClick={() => setDivision('')}
                >{t('agentEditor.subagents.allDivisions')} · {catalog.total}</button>
                {catalog.divisions.map((row) => (
                  <button
                    key={row.slug}
                    type="button"
                    className={`wk-ae-sub-chip wk-ae-sub-chip--inline${division === row.slug ? ' wk-ae-sub-chip--active' : ''}`}
                    data-subagent-division={row.slug}
                    aria-pressed={division === row.slug ? 'true' : 'false'}
                    onClick={() => setDivision(row.slug)}
                  >
                    <span aria-hidden="true" className="wk-ae-sub-dot wk-ae-sub-dot--sm" style={{ background: dotColor(row.color) }} />
                    {row.label} · {row.count}
                  </button>
                ))}
              </div>
              <Input
                type="search"
                className="wk-ae-sub-search"
                value={query}
                placeholder={t('agentEditor.subagents.searchPlaceholder')}
                aria-label={t('agentEditor.subagents.searchPlaceholder')}
                data-subagent-search
                onChange={(value) => setQuery(String(value))}
              />
              {catalog.entries.length === 0 ? (
                <p className="desc" data-subagent-catalog-empty>{t('agentEditor.subagents.emptyCatalog')}</p>
              ) : entries.length === 0 ? (
                <p className="desc" data-subagent-no-match>{t('agentEditor.subagents.noMatch')}</p>
              ) : (
                <div className="wk-ae-sub-list">
                  {entries.map((entry) => {
                    const has = installed.includes(entry.slug);
                    const open = expanded === entry.slug;
                    return (
                      <div key={entry.slug} className="wk-ae-sub-row wk-ae-sub-row--card" data-subagent-slug={entry.slug}>
                        <span aria-hidden="true" className="wk-ae-sub-dot" style={{ background: dotColor(entry.color) }} />
                        {entry.emoji !== '' ? <span aria-hidden="true">{entry.emoji}</span> : null}
                        <button
                          type="button"
                          className="wk-ae-sub-name"
                          data-subagent-name={entry.slug}
                          aria-expanded={open ? 'true' : 'false'}
                          onClick={() => toggleDetail(entry)}
                        >{nameAtLocale(entry, zh)}</button>
                        {has ? (
                          <span className="wk-ae-sub-installed-tag" data-subagent-installed-tag={entry.slug}>
                            <span aria-hidden="true">✓</span>{t('agentEditor.subagents.installedTag')}
                          </span>
                        ) : null}
                        <span className="wk-ae-sub-slug wk-ae-sub-slug--end">{entry.slug}</span>
                        {!has ? (
                          <Button
                            variant="outline"
                            size="small"
                            className="wk-ae-sub-action"
                            data-subagent-install={entry.slug}
                            disabled={!canMutate}
                            onClick={() => {
                              if (!canMutate) return;
                              setBusySlug(entry.slug);
                              // locale pins the library copy at the app locale
                              // ("zh-CN"/"en-US" normalize onto zh/en)
                              void applyInstalled(client.subagents.install(agentId, entry.slug, locale));
                            }}
                          >{busySlug === entry.slug ? t('common.loading') : t('agentEditor.subagents.install')}</Button>
                        ) : null}
                        {open ? (
                          <div className="wk-ae-sub-detail" data-subagent-detail={entry.slug}>
                            {detailErrors[entry.slug] && !pendingDetails[entry.slug] ? (
                              <p role="alert" style={{ color: 'var(--td-error-color)' }}>{t('agentEditor.subagents.detailFailed')}</p>
                            ) : details[entry.slug] === undefined ? (
                              <p role="status">{t('common.loading')}</p>
                            ) : (
                              /* renderChatMarkdown escapes raw HTML and allow-lists links */
                              <div dangerouslySetInnerHTML={{ __html: renderChatMarkdown(details[entry.slug] ?? '') }} />
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
          {actionError ? <p className="desc" role="alert" style={{ color: 'var(--td-error-color)' }}>{actionError}</p> : null}
        </Row>
      </div>
    </div>
  );
}

/** 局部 Row（同 PersonaSection，无 Vue 事实源的 setting-row 复刻）。 */
function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="setting-row setting-row-vertical">
      <div className="setting-info">
        <label>{label}</label>
      </div>
      <div className="setting-control setting-control-full">{children}</div>
    </div>
  );
}
