// Octop M2 — expert-template catalog (/platform/experts).
//
// Skeleton follows AnalyticsPage.tsx (props { client }, useState + load() +
// useEffect, formatMessage via the resolved locale, Tailwind class constants);
// data comes from the Task 6 client.experts.* endpoints (packages/api-client
// experts.ts). The detail surface follows the AgentsPage AgentDetailDrawer
// shape (fixed right-side aside, role=dialog), and persona_markdown renders
// through the shared chat markdown boundary (renderChatMarkdown — raw HTML is
// escaped and links allow-listed there). The toast is the OrganizationsPage
// fixed top-center role=status pill; navigation reuses the platform
// navigation sink so the router picks up the ?edit deep link.
//
// icon_name is a lucide-style name the React client has no mapping for yet —
// cards render a color dot (expert.color when it is a plain hex literal, a
// neutral fallback otherwise) instead of a glyph. No emoji fallback.
import { useCallback, useEffect, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { ExpertDetail, ExpertSummary } from '@weknora/api-client';
import { formatMessage, isLocale } from '@weknora/i18n';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { usePreferredLocale } from '../locale.ts';
import { navigate } from '../platform/navigation.ts';

/* Tailwind v4 utility recipes shared across the page (AnalyticsPage constants). */
const XP_PAGE = 'wk-page box-border h-full overflow-y-auto px-[28px] pt-[24px] pb-[32px]';
const XP_HEADER = 'mb-[20px] flex flex-col gap-[12px]';
const XP_TITLE = 'm-0 text-[24px] font-semibold leading-[32px] text-[rgba(23,26,29,0.92)]';
const XP_SUBTITLE = 'm-0 text-[14px] font-normal leading-[20px] text-[rgba(23,26,29,0.6)]';
const XP_GRID = 'grid grid-cols-1 gap-[16px] min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4';
const XP_CARD = 'box-border flex cursor-pointer flex-col gap-[10px] rounded-[10px] border border-[#e7e7ea] bg-surface px-[16px] py-[14px] text-left shadow-[0_1px_3px_rgba(0,0,0,0.04)] [transition:all_.2s_ease] hover:border-accent hover:shadow-[0_4px_14px_rgba(0,0,0,0.08)]';
const XP_CARD_TITLE = 'm-0 flex items-center gap-[8px] text-[16px] font-semibold leading-[24px] text-[rgba(23,26,29,0.92)]';
const XP_CARD_DESC = 'm-0 line-clamp-2 text-[13px] font-normal leading-[19px] text-[rgba(23,26,29,0.6)]';
const XP_CHIP = 'inline-flex shrink-0 items-center rounded-[10px] bg-[rgba(127,127,127,0.1)] px-[8px] py-[2px] text-[11px] font-medium text-[rgba(23,26,29,0.75)]';
const XP_STATE = 'flex h-[240px] items-center justify-center text-[13px] text-[rgba(23,26,29,0.4)]';
const XP_ERROR = 'flex h-[240px] flex-col items-center justify-center gap-[10px] text-[13px] text-[#d54941]';
const XP_BTN_PRIMARY = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border-0 bg-accent px-[15px] font-[inherit] text-[14px] font-medium text-white shadow-[0_2px_8px_rgba(7,192,95,0.25)] [transition:all_.2s_ease] hover:shadow-[0_4px_14px_rgba(7,192,95,0.35)] disabled:cursor-not-allowed disabled:opacity-55';
const XP_BTN_OUTLINE = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border border-[rgba(7,192,95,0.5)] bg-surface px-[15px] font-[inherit] text-[14px] font-medium text-accent [transition:all_.2s_ease] hover:border-accent hover:bg-accent-wash';
const XP_INPUT = 'box-border h-[32px] w-[200px] rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] font-[inherit] text-[13px] text-[rgba(23,26,29,0.92)] placeholder:text-[rgba(23,26,29,0.35)] focus:border-accent focus:outline-none';
const XP_SECTION_TITLE = 'm-0 mb-[8px] text-[13px] font-semibold uppercase tracking-[0.04em] text-[rgba(23,26,29,0.45)]';

/** expert.color is server-controlled; only plain hex literals reach the style
 * attribute so a hostile catalog value cannot smuggle CSS into the DOM. */
const FALLBACK_DOT_COLOR = 'rgba(127,127,127,0.35)';
function dotColor(color: string): string {
  return /^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(color) ? color : FALLBACK_DOT_COLOR;
}

function t(locale: string, key: string, values?: Record<string, string | number>): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key, values);
}

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

/** OrganizationsPage toast pill (fixed top-center, auto-dismiss, role=status). */
type ToastState = { tone: 'success' | 'error'; text: string } | null;

/** The success toast stays perceivable for a beat before the editor deep link
 * takes the page away — the experts page owns its toast, so the navigation is
 * deferred past the toast paint instead of flashing it for zero frames. */
const NAVIGATE_DELAY_MS = 600;

interface ExpertsPageProps {
  client: WeKnoraClient;
}

export function ExpertsPage({ client }: ExpertsPageProps) {
  const locale = usePreferredLocale();
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [experts, setExperts] = useState<ExpertSummary[]>([]);
  // Detail drawer state: summaryId drives the fetch, detail the rendered body.
  const [detailId, setDetailId] = useState('');
  const [detail, setDetail] = useState<ExpertDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [agentName, setAgentName] = useState('');
  const [instantiating, setInstantiating] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const navigateTimer = useRef<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setListError('');
    try {
      setExperts(await client.experts.list());
    } catch (reason) {
      setExperts([]);
      setListError(errorText(reason, t(locale, 'common.error')));
    } finally {
      setLoading(false);
    }
  }, [client, locale]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3000);
    return () => window.clearTimeout(timer);
  }, [toast]);

  // Deferred editor deep link (see NAVIGATE_DELAY_MS) must not fire after the
  // page unmounted for an unrelated reason.
  useEffect(() => () => { if (navigateTimer.current !== null) window.clearTimeout(navigateTimer.current); }, []);

  const openDetail = useCallback(async (expert: ExpertSummary) => {
    setDetailId(expert.id);
    setDetail(null);
    setDetailError('');
    setDetailLoading(true);
    setAgentName(expert.label);
    try {
      setDetail(await client.experts.get(expert.id));
    } catch (reason) {
      setDetailError(errorText(reason, t(locale, 'experts.detail.loadFailed')));
    } finally {
      setDetailLoading(false);
    }
  }, [client, locale]);

  function closeDetail(): void {
    setDetailId('');
    setDetail(null);
    setDetailError('');
  }

  async function instantiate(): Promise<void> {
    if (!detail || instantiating) return;
    setInstantiating(true);
    try {
      const name = agentName.trim() || detail.label;
      const result = await client.experts.instantiate(detail.id, { agentName: name });
      setInstantiating(false);
      setToast({
        tone: 'success',
        text: result.pending_skills.length > 0
          ? t(locale, 'experts.createdPending', { count: result.pending_skills.length })
          : t(locale, 'experts.created'),
      });
      // Editor deep link (apps/web/src/agents/route.ts buildAgentPath contract):
      // /platform/agents?edit=<id> opens the created agent in the editor.
      const target = `/platform/agents?edit=${encodeURIComponent(result.agent.id)}`;
      navigateTimer.current = window.setTimeout(() => {
        navigateTimer.current = null;
        navigate(target);
      }, NAVIGATE_DELAY_MS);
    } catch (reason) {
      setInstantiating(false);
      const message = errorText(reason, '');
      setToast({ tone: 'error', text: message !== '' ? t(locale, 'experts.createFailedMessage', { message }) : t(locale, 'experts.createFailed') });
    }
  }

  return (
    <main className={XP_PAGE}>
      <header className={XP_HEADER}>
        <div>
          <h2 className={XP_TITLE}>{t(locale, 'experts.title')}</h2>
          <p className={XP_SUBTITLE}>{t(locale, 'experts.subtitle')}</p>
        </div>
      </header>

      {toast ? (
        <div
          role="status"
          className={'fixed left-1/2 top-[24px] z-[3000] box-border flex max-w-[420px] -translate-x-1/2 items-center rounded-[8px] bg-[rgba(23,26,29,0.86)] px-[18px] py-[10px] text-[13px] shadow-[0_6px_20px_rgba(0,0,0,0.18)] ' + (toast.tone === 'success' ? 'text-[#7bf2b6]' : 'text-[#ffb4ae]')}
        >{toast.text}</div>
      ) : null}

      {loading ? <div className={XP_STATE} role="status">{t(locale, 'common.loading')}</div> : null}
      {!loading && listError ? (
        <div className={XP_ERROR} role="alert">
          <span>{listError}</span>
          <button type="button" className={XP_BTN_OUTLINE} onClick={() => void load()}>{t(locale, 'common.retry')}</button>
        </div>
      ) : null}
      {!loading && !listError && experts.length === 0 ? <div className={XP_STATE}>{t(locale, 'experts.empty')}</div> : null}
      {!loading && !listError && experts.length > 0 ? (
        <div className={XP_GRID}>
          {experts.map((expert) => (
            <article
              key={expert.id}
              data-expert-id={expert.id}
              className={XP_CARD}
              onClick={() => void openDetail(expert)}
            >
              <h3 className={XP_CARD_TITLE}>
                <span aria-hidden="true" className="inline-block h-[10px] w-[10px] shrink-0 rounded-full" style={{ background: dotColor(expert.color) }} />
                <span className="truncate" title={expert.label}>{expert.label}</span>
              </h3>
              <p className={XP_CARD_DESC}>{expert.description}</p>
              <div className="mt-auto flex flex-wrap items-center gap-[6px]">
                <span className={XP_CHIP}>{t(locale, 'experts.skillCount', { count: expert.skills.length })}</span>
                {expert.persona_mbti ? <span className={XP_CHIP}>{expert.persona_mbti}</span> : null}
              </div>
            </article>
          ))}
        </div>
      ) : null}

      {detailId !== '' ? (
        <div
          className="fixed inset-0 z-[1000] flex justify-end bg-[rgba(0,0,0,0.4)]"
          onClick={(event) => { if (event.target === event.currentTarget) closeDetail(); }}
        >
          <aside
            className="flex h-full w-[420px] max-w-[90vw] flex-col bg-[var(--wk-bg,#fff)] shadow-[-4px_0_24px_rgba(0,0,0,0.12)]"
            role="dialog"
            aria-label={t(locale, 'experts.title')}
            data-expert-detail={detailId}
          >
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[rgba(127,127,127,0.2)] px-6 py-5">
              <h3 className="m-0 flex min-w-0 items-center gap-[8px] text-[18px] font-semibold">
                {detail ? (
                  <>
                    <span aria-hidden="true" className="inline-block h-[10px] w-[10px] shrink-0 rounded-full" style={{ background: dotColor(detail.color) }} />
                    <span className="truncate">{detail.label}</span>
                  </>
                ) : t(locale, 'experts.title')}
              </h3>
              <button
                type="button"
                className="flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-md border-none bg-[rgba(127,127,127,0.1)] text-[15px]"
                aria-label={t(locale, 'common.cancel')}
                onClick={closeDetail}
              >✕</button>
            </div>

            <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-6">
              {detailLoading ? <div role="status">{t(locale, 'common.loading')}</div> : null}
              {!detailLoading && detailError ? (
                <div className="flex flex-col items-start gap-[10px] text-[13px] text-[#d54941]" role="alert">
                  <span>{detailError}</span>
                  {detailId ? <button type="button" className={XP_BTN_OUTLINE} onClick={() => { const id = detailId; const summary = experts.find((row) => row.id === id); if (summary) void openDetail(summary); }}>{t(locale, 'common.retry')}</button> : null}
                </div>
              ) : null}
              {!detailLoading && !detailError && detail ? (
                <>
                  <p className="m-0 text-[14px] leading-[22px] text-[rgba(23,26,29,0.6)]">{detail.description}</p>
                  <div className="flex flex-wrap items-center gap-[6px]">
                    {detail.persona_mbti ? <span className={XP_CHIP}>{detail.persona_mbti}</span> : null}
                    {detail.skills.map((skill) => <span key={skill} className={XP_CHIP}>{skill}</span>)}
                  </div>

                  <section aria-label={t(locale, 'experts.detail.persona')}>
                    <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.persona')}</h4>
                    {/* renderChatMarkdown escapes raw HTML and allow-lists links */}
                    <div
                      className="wk-experts-persona text-[13px] leading-[21px] text-[rgba(23,26,29,0.85)] [&_a]:text-accent [&_a]:underline [&_h1]:mt-[12px] [&_h1]:mb-[6px] [&_h1]:text-[16px] [&_h1]:font-semibold [&_h2]:mt-[12px] [&_h2]:mb-[6px] [&_h2]:text-[15px] [&_h2]:font-semibold [&_h3]:mt-[12px] [&_h3]:mb-[6px] [&_h3]:text-[14px] [&_h3]:font-semibold [&_p]:m-0 [&_p]:mb-[8px] [&_ul]:m-0 [&_ul]:mb-[8px] [&_ul]:list-disc [&_ul]:pl-[18px] [&_ol]:m-0 [&_ol]:mb-[8px] [&_ol]:list-decimal [&_ol]:pl-[18px] [&_code]:rounded-[4px] [&_code]:bg-[rgba(127,127,127,0.1)] [&_code]:px-[4px] [&_pre]:overflow-x-auto [&_pre]:rounded-[6px] [&_pre]:bg-[rgba(127,127,127,0.08)] [&_pre]:p-[10px]"
                      data-expert-persona
                      dangerouslySetInnerHTML={{ __html: renderChatMarkdown(detail.persona_markdown) }}
                    />
                  </section>

                  {detail.quick_prompts.length > 0 ? (
                    <section aria-label={t(locale, 'experts.detail.quickPrompts')}>
                      <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.quickPrompts')}</h4>
                      <ul className="m-0 flex list-none flex-col gap-[8px] p-0">
                        {detail.quick_prompts.map((prompt) => (
                          <li key={prompt.title} className="rounded-[8px] border border-[#e7e7ea] px-[12px] py-[10px]" data-expert-prompt>
                            <div className="flex items-center gap-[8px]">
                              <span aria-hidden="true" className="inline-block h-[8px] w-[8px] shrink-0 rounded-full" style={{ background: dotColor(prompt.color) }} />
                              <span className="text-[14px] font-semibold leading-[20px]">{prompt.title}</span>
                            </div>
                            {prompt.description ? <p className="m-0 mt-[4px] text-[12px] leading-[18px] text-[rgba(23,26,29,0.6)]">{prompt.description}</p> : null}
                          </li>
                        ))}
                      </ul>
                    </section>
                  ) : null}
                </>
              ) : null}
            </div>

            <div className="flex shrink-0 items-center justify-between gap-[10px] border-t border-[rgba(127,127,127,0.2)] bg-[var(--wk-bg,#fff)] px-6 py-4">
              <label className="flex min-w-0 flex-1 flex-col gap-[4px] text-[12px] leading-[16px] text-[rgba(23,26,29,0.6)]">
                {t(locale, 'experts.create.nameLabel')}
                <input
                  className={XP_INPUT + ' w-full'}
                  value={agentName}
                  placeholder={t(locale, 'experts.create.namePlaceholder')}
                  onChange={(event) => setAgentName(event.target.value)}
                />
              </label>
              <button
                type="button"
                className={XP_BTN_PRIMARY + ' shrink-0'}
                disabled={!detail || detailError !== '' || instantiating}
                data-expert-instantiate={detailId}
                onClick={() => void instantiate()}
              >{instantiating ? t(locale, 'experts.create.creating') : t(locale, 'experts.createAgent')}</button>
            </div>
          </aside>
        </div>
      ) : null}
    </main>
  );
}
