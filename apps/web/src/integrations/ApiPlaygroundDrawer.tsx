import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { CSSProperties, MouseEvent as ReactMouseEvent } from 'react';
// S6：@weknora/ui 离栈（T15 硬前置）。agent combobox 与 query 字段是自研
// 原生控件（ARIA 契约 + 原生驱动测试锚点，同 ModelSettingsPanel Ollama
// combobox 先例），保留原生 input/textarea，仅移除旧栈包装。

import { consumeApiPlaygroundSSE } from './apiPlaygroundSSE.ts';
import {
  DEFAULT_DIRECT_HEADER_NAME,
  agentOptionLabel,
  buildChatRequestBody,
  buildPlaygroundHeaders,
  compactText,
  ensurePlaygroundAgent,
  externalUserHintKey,
  formatJSON,
  formatResponseBody,
  hasPlaygroundResult,
  interpretSessionResponse,
  playgroundDisabledReason,
  playgroundRequestPreview,
  settlePlaygroundStatuses,
  type PlaygroundStepStatus,
} from './apiPlaygroundModel.ts';

// Vue baseline: frontend/src/views/integrations/ApiIntegrationSettings.vue
// L339-460 (playground SettingDrawer) and L1637-1735 (runPlayground state
// machine). Vue SettingDrawer teleports the overlay to document.body so it is
// not clipped or restyled by the integrations page stacking context.

export interface ApiPlaygroundAgentOption { id: string; name: string; is_builtin?: boolean }

export interface ApiPlaygroundDrawerProps {
  open: boolean;
  onClose: () => void;
  apiKey: string;
  mode: 'tenant' | 'direct_header' | 'signed_token';
  agents: readonly ApiPlaygroundAgentOption[];
  /** Vue t-select loading state while listAgents is resolving. */
  agentsLoading?: boolean;
  agentsError?: string;
  apiBaseUrl: string;
  /** Vue createAPIPrincipalTestToken (signed_token mode only). */
  mintToken?: (externalUserId: string) => Promise<{ token: string; headerName: string }>;
  fetchFn?: typeof fetch;
  t: (key: string, values?: Record<string, string | number>) => string;
}

function ApiPlaygroundAgentSelect({ agents, value, loading, placeholder, loadingLabel, builtinLabel, onChange }: {
  agents: readonly ApiPlaygroundAgentOption[];
  value: string;
  loading: boolean;
  placeholder: string;
  loadingLabel: string;
  builtinLabel: string;
  onChange: (value: string) => void;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const selected = agents.find((agent) => agent.id === value);
  const options = useMemo(() => {
    const needle = filter.trim().toLocaleLowerCase();
    return needle ? agents.filter((agent) => `${agent.name} ${agent.id}`.toLocaleLowerCase().includes(needle)) : [...agents];
  }, [agents, filter]);
  useEffect(() => {
    if (!open) return;
    const onOutside = (event: MouseEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') { setOpen(false); inputRef.current?.blur(); } };
    document.addEventListener('mousedown', onOutside);
    window.addEventListener('keydown', onKey);
    return () => { document.removeEventListener('mousedown', onOutside); window.removeEventListener('keydown', onKey); };
  }, [open]);
  function choose(agent: ApiPlaygroundAgentOption) {
    onChange(agent.id);
    setFilter('');
    setOpen(false);
  }
  return <div ref={rootRef} className="wk-api-playground-agent-select" style={{ position: 'relative' }}>
    <input
      ref={inputRef}
      className="wk-api-playground-agent box-border"
      role="combobox"
      aria-autocomplete="list"
      aria-controls="wk-api-playground-agent-options"
      aria-expanded={open}
      aria-busy={loading || undefined}
      value={open ? filter : (selected ? agentOptionLabel(selected.name, selected.is_builtin === true, builtinLabel) : '')}
      placeholder={loading ? loadingLabel : placeholder}
      onFocus={() => { setOpen(true); setFilter(''); setActiveIndex(0); }}
      onChange={(event) => { setFilter(event.target.value); setOpen(true); setActiveIndex(0); }}
      onKeyDown={(event: React.KeyboardEvent<HTMLInputElement>) => {
        if (event.key === 'ArrowDown') { event.preventDefault(); setOpen(true); setActiveIndex((index) => Math.min(index + 1, Math.max(options.length - 1, 0))); }
        else if (event.key === 'ArrowUp') { event.preventDefault(); setActiveIndex((index) => Math.max(index - 1, 0)); }
        else if (event.key === 'Enter' && open && options[activeIndex]) { event.preventDefault(); choose(options[activeIndex]); }
      }}
      style={fieldStyle}
    />
    {open ? <div id="wk-api-playground-agent-options" role="listbox" className="wk-api-playground-agent-options" style={{ position: 'absolute', zIndex: 2, left: 0, right: 0, maxHeight: 240, overflowY: 'auto', background: 'var(--wk-bg, #fff)', border: '1px solid var(--wk-border, #e5e7eb)', borderRadius: 6, boxShadow: '0 8px 20px rgba(15,23,42,.14)' }}>
      {loading ? <div role="status" className="wk-muted text-muted" style={{ padding: '8px 10px' }}>{loadingLabel}</div> : options.length === 0 ? <div className="wk-muted text-muted" style={{ padding: '8px 10px' }}>{placeholder}</div> : options.map((agent, index) => <div key={agent.id} role="option" aria-selected={agent.id === value} className={index === activeIndex ? 'is-active' : undefined} onMouseDown={(event) => { event.preventDefault(); choose(agent); }} style={{ padding: '8px 10px', cursor: 'pointer', background: index === activeIndex ? 'var(--wk-bg-muted, #f6f8fa)' : undefined }}>{agentOptionLabel(agent.name, agent.is_builtin === true, builtinLabel)}</div>)}
    </div> : null}
  </div>;
}

interface RunState {
  running: boolean;
  sessionStatus: PlaygroundStepStatus;
  chatStatus: PlaygroundStepStatus;
  sessionResponse: string;
  streamOutput: string;
  finalAnswer: string;
  signedToken: string;
  error: string;
  successMs: number | null;
}

const idleRun: RunState = { running: false, sessionStatus: '', chatStatus: '', sessionResponse: '', streamOutput: '', finalAnswer: '', signedToken: '', error: '', successMs: null };

// The playground opens from inside the settings modal (wks-overlay z-index
// 1100), so the drawer must stack above it or the modal backdrop swallows
// every click (Vue escapes the same way by teleporting the drawer to body).
const PLAYGROUND_OVERLAY_Z = 1200;
const PLAYGROUND_DRAWER_Z = 1201;
const PLAYGROUND_RESIZE_Z = 1202;

const overlayStyle: CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(15,23,42,.4)', zIndex: PLAYGROUND_OVERLAY_Z };
const API_PLAYGROUND_DRAWER_SPEC = { storageKey: 'setting-drawer:width:api-playground', defaultWidth: 640, minWidth: 560, maxWidth: 960 } as const;

export function clampApiPlaygroundWidth(width: number, viewportWidth = typeof window === 'undefined' ? API_PLAYGROUND_DRAWER_SPEC.maxWidth : window.innerWidth): number {
  const cap = Math.min(API_PLAYGROUND_DRAWER_SPEC.maxWidth, viewportWidth);
  const floor = Math.min(API_PLAYGROUND_DRAWER_SPEC.minWidth, cap);
  return Math.max(floor, Math.min(cap, Math.round(width)));
}

function readApiPlaygroundWidth(): number {
  try {
    const raw = typeof window === 'undefined' ? null : window.localStorage.getItem(API_PLAYGROUND_DRAWER_SPEC.storageKey);
    const parsed = raw ? Number(raw) : Number.NaN;
    return Number.isFinite(parsed) ? clampApiPlaygroundWidth(parsed) : API_PLAYGROUND_DRAWER_SPEC.defaultWidth;
  } catch {
    return API_PLAYGROUND_DRAWER_SPEC.defaultWidth;
  }
}

const bodyStyle: CSSProperties = { flex: '1 1 auto', overflowY: 'auto', padding: '0 20px 12px' };
const footerStyle: CSSProperties = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '12px 20px', borderTop: '1px solid var(--wk-border, #e5e7eb)' };
const preStyle: CSSProperties = { background: 'var(--wk-bg-muted, #f6f8fa)', padding: 10, overflowX: 'auto', fontSize: 12 };
// .wk-api-playground-preview/.wk-api-playground-step pre cascade (styles.css L604) as utilities;
// inline preStyle keeps overriding background/padding/overflow-x/font-size exactly as before.
const preClassName = 'm-0 max-h-[260px] overflow-auto whitespace-pre-wrap rounded-[6px] [font:12px/1.5_ui-monospace,SFMono-Regular,Menlo,monospace]';
const rowStyle: CSSProperties = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 };
const fieldStyle: CSSProperties = { display: 'block', margin: '10px 0', width: '100%' };

export function ApiPlaygroundDrawer({ open, onClose, apiKey, mode, agents, agentsError, agentsLoading = false, apiBaseUrl, mintToken, fetchFn, t }: ApiPlaygroundDrawerProps) {
  const [form, setForm] = useState({ agentId: '', query: 'hello', externalUserId: 'user_123' });
  const [run, setRun] = useState<RunState>(idleRun);
  const [drawerWidth, setDrawerWidth] = useState<number>(API_PLAYGROUND_DRAWER_SPEC.defaultWidth);
  const [drawerResizing, setDrawerResizing] = useState(false);
  const drawerWidthRef = useRef(drawerWidth);
  const controllerRef = useRef<AbortController | null>(null);
  const queryRef = useRef<HTMLTextAreaElement | null>(null);
  drawerWidthRef.current = drawerWidth;

  // Vue openPlaygroundDrawer -> ensurePlaygroundAgent (also reruns when the
  // agent list lands after the drawer opened).
  useEffect(() => {
    if (!open) {
      controllerRef.current?.abort();
      return;
    }
    setForm((prev) => {
      const agentId = ensurePlaygroundAgent(prev.agentId, agents);
      return agentId === prev.agentId ? prev : { ...prev, agentId };
    });
  }, [open, agents]);
  useEffect(() => () => controllerRef.current?.abort(), []);
  useEffect(() => {
    if (!open) return;
    setDrawerWidth(readApiPlaygroundWidth());
    const onResize = () => setDrawerWidth((current) => clampApiPlaygroundWidth(current));
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [open]);
  useEffect(() => {
    const textarea = queryRef.current;
    if (!textarea || !open) return;
    textarea.style.height = 'auto';
    const computed = window.getComputedStyle(textarea);
    const lineHeight = Number.parseFloat(computed.lineHeight) || 22;
    const padding = (Number.parseFloat(computed.paddingTop) || 0) + (Number.parseFloat(computed.paddingBottom) || 0);
    const minHeight = lineHeight * 2 + padding;
    const maxHeight = lineHeight * 4 + padding;
    const measured = textarea.scrollHeight || minHeight;
    textarea.style.height = `${Math.min(maxHeight, Math.max(minHeight, measured))}px`;
    textarea.style.overflowY = measured > maxHeight ? 'auto' : 'hidden';
  }, [form.query, open]);
  const onResizeStart = (event: ReactMouseEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = drawerWidthRef.current;
    setDrawerResizing(true);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    const onMove = (move: MouseEvent) => setDrawerWidth(clampApiPlaygroundWidth(startWidth + startX - move.clientX));
    const onEnd = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onEnd);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      setDrawerResizing(false);
      try { window.localStorage.setItem(API_PLAYGROUND_DRAWER_SPEC.storageKey, String(clampApiPlaygroundWidth(drawerWidthRef.current))); } catch { /* storage is optional */ }
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onEnd);
  };
  const close = () => {
    // Vue SettingDrawer blurs before destroy-on-close. This avoids leaving a
    // soon-to-be-removed textarea/select as the active element in the host.
    if (typeof document !== 'undefined' && document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    controllerRef.current?.abort();
    onClose();
  };
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') close(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  if (!open) return null;

  const disabledReason = playgroundDisabledReason({ running: run.running, apiKey, agentId: form.agentId, query: form.query, mode, externalUserId: form.externalUserId });
  const preview = playgroundRequestPreview({ query: form.query, agentId: form.agentId, mode, externalUserId: form.externalUserId, signedToken: run.signedToken, apiKey });
  const hasResult = hasPlaygroundResult({ signedToken: run.signedToken, sessionResponse: run.sessionResponse, streamOutput: run.streamOutput, finalAnswer: run.finalAnswer });
  const copy = (value: string) => { void navigator.clipboard?.writeText(value).catch(() => undefined); };
  const stop = () => { controllerRef.current?.abort(); };

  const start = async () => {
    if (run.running || disabledReason) return;
    const controller = new AbortController();
    let aborted = false;
    controller.signal.addEventListener('abort', () => { aborted = true; });
    controllerRef.current = controller;
    setRun({ ...idleRun, running: true, sessionStatus: 'running' });
    const startedAt = performance.now();
    const base = apiBaseUrl.replace(/\/+$/, '');
    const doFetch = fetchFn ?? ((input: string, init?: RequestInit) => fetch(input, init));
    try {
      let signedToken = '';
      if (mode === 'signed_token') {
        if (!mintToken) throw new Error(t('integrations.api.playgroundMintTokenFailed'));
        const minted = await mintToken(form.externalUserId.trim());
        signedToken = minted.token;
        setRun((prev) => ({ ...prev, signedToken }));
      }
      const headers = buildPlaygroundHeaders({ apiKey, mode, externalUserId: form.externalUserId, signedToken, maskSecrets: false });
      const sessionResp = await doFetch(base + '/api/v1/sessions', { method: 'POST', headers: headers.sessionHeaders, body: '{}', signal: controller.signal, credentials: 'omit' });
      const sessionRaw = await sessionResp.text();
      let sessionPayload: unknown = null;
      try { sessionPayload = sessionRaw ? JSON.parse(sessionRaw) : null; } catch { sessionPayload = null; }
      setRun((prev) => ({ ...prev, sessionResponse: sessionPayload ? formatJSON(sessionPayload) : compactText(sessionRaw) }));
      const session = interpretSessionResponse({ ok: sessionResp.ok, status: sessionResp.status, payload: sessionPayload });
      if (!session.ok) {
        setRun((prev) => ({ ...prev, sessionStatus: 'failed' }));
        throw new Error(session.error);
      }
      setRun((prev) => ({ ...prev, sessionStatus: 'success', chatStatus: 'running' }));
      const chatResp = await doFetch(base + '/api/v1/agent-chat/' + encodeURIComponent(session.sessionId), {
        method: 'POST',
        headers: headers.chatHeaders,
        body: JSON.stringify(buildChatRequestBody({ query: form.query, agentId: form.agentId })),
        signal: controller.signal,
        credentials: 'omit',
      });
      if (!chatResp.ok) {
        setRun((prev) => ({ ...prev, chatStatus: 'failed' }));
        const body = formatResponseBody(await chatResp.text());
        setRun((prev) => ({ ...prev, streamOutput: body }));
        throw new Error(body || 'HTTP ' + chatResp.status);
      }
      if (!chatResp.body) throw new Error(t('integrations.api.playgroundNoStream'));

      const result = await consumeApiPlaygroundSSE(chatResp.body, ({ raw, answer }) => {
        setRun((prev) => ({ ...prev, streamOutput: compactText(raw), finalAnswer: answer }));
      });
      setRun((prev) => ({ ...prev, streamOutput: compactText(result.raw), finalAnswer: result.answer }));
      if (aborted) {
        setRun((prev) => ({ ...prev, ...settlePlaygroundStatuses(prev, true), error: t('integrations.api.playgroundStopped') }));
        return;
      }
      if (result.status === 'failed') {
        setRun((prev) => ({ ...prev, chatStatus: 'failed' }));
        throw new Error(result.error || t('integrations.api.playgroundFailed'));
      }
      setRun((prev) => ({ ...prev, chatStatus: 'success', successMs: Math.round(performance.now() - startedAt) }));
    } catch (cause) {
      // Vue L1717: AbortError means the user stopped the run — stopped, not failed.
      const stopped = aborted || (cause as { name?: string } | null)?.name === 'AbortError';
      setRun((prev) => ({
        ...prev,
        ...settlePlaygroundStatuses(prev, stopped),
        error: stopped ? t('integrations.api.playgroundStopped') : ((cause as Error | null)?.message || t('integrations.api.playgroundFailed')),
      }));
    } finally {
      setRun((prev) => ({ ...prev, running: false }));
      controllerRef.current = null;
    }
  };

  const statusTag = (status: PlaygroundStepStatus) => (
    <span className={status === 'success' ? 'text-[#16803c]' : status === 'failed' ? 'text-[#b42318]' : status === 'stopped' ? 'text-[#a15c00]' : 'text-[var(--wk-muted-foreground,#64748b)]'} data-status={status || 'none'}>{status || '-'}</span>
  );

  const drawer = (
    <div className="wk-api-playground-overlay" role="presentation" style={overlayStyle} onClick={close}>
      <div className={`wk-api-playground-resize-handle${drawerResizing ? ' is-active' : ''}`} role="separator" aria-orientation="vertical" aria-label="调整抽屉宽度" onMouseDown={onResizeStart} style={{ position: 'fixed', top: 0, bottom: 0, right: drawerWidth, width: 8, cursor: 'col-resize', zIndex: PLAYGROUND_RESIZE_Z }}><span aria-hidden style={{ display: 'block', height: '100%', width: 1, margin: '0 auto', background: drawerResizing ? 'var(--wk-accent, #4a7dff)' : 'transparent' }} /></div>
      <aside className="overflow-auto h-screen" role="dialog" aria-modal="true" aria-label={t('integrations.api.playgroundTitle')} style={{ position: 'fixed', top: 0, right: 0, bottom: 0, width: `${drawerWidth}px`, background: 'var(--wk-bg, #fff)', boxShadow: '-12px 0 32px rgba(0,0,0,.18)', display: 'flex', flexDirection: 'column', zIndex: PLAYGROUND_DRAWER_Z }} onClick={(event) => event.stopPropagation()}>
        <header className="wk-api-playground-header" style={rowStyle}>
          <div>
            <h2 style={{ margin: 0, fontSize: 16 }}>{t('integrations.api.playgroundTitle')}</h2>
            <p className="wk-muted text-muted" style={{ margin: '4px 0 0' }}>{t('integrations.api.playgroundDrawerDesc')}</p>
          </div>
          <button type="button" className="wk-api-playground-close" aria-label={t('common.close')} title={t('common.close')} onClick={close} style={{ border: 'none', background: 'transparent', fontSize: 18, cursor: 'pointer' }}>×</button>
        </header>

        <div className="grid gap-[18px]" style={bodyStyle}>
          <section className="grid gap-3 rounded-[10px] border border-[var(--wk-border,#e5e7eb)] p-4">
            <h4 className="m-0 text-[14px]">{t('integrations.api.playgroundSectionRequest')}</h4>
            <label className="wk-api-playground-field text-[13px]" style={{ display: 'block', margin: '10px 0' }}>
              {t('integrations.api.playgroundAgent')}
              <ApiPlaygroundAgentSelect agents={agents} value={form.agentId} loading={agentsLoading} placeholder={t('integrations.api.playgroundAgentPlaceholder')} loadingLabel={t('common.loading')} builtinLabel={t('integrations.api.playgroundBuiltin')} onChange={(agentId) => setForm((prev) => ({ ...prev, agentId }))} />
            </label>
            {agentsError ? <p className="wk-api-playground-field-error" role="alert">{agentsError}</p> : null}
            <label className="wk-api-playground-field text-[13px]" style={{ display: 'block', margin: '10px 0' }}>
              {t('integrations.api.playgroundExternalUser')}
              <input className="wk-api-playground-external-user box-border" type="text" value={form.externalUserId} disabled={mode === 'tenant'} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, externalUserId: event.target.value }))} style={fieldStyle} />
            </label>
            <p className="wk-api-playground-hint wk-muted text-muted">{t(externalUserHintKey(mode), { headerName: DEFAULT_DIRECT_HEADER_NAME })}</p>
            <label className="wk-api-playground-field text-[13px]" style={{ display: 'block', margin: '10px 0' }}>
              {t('integrations.api.playgroundQuestion')}
              <textarea ref={queryRef} className="wk-api-playground-query box-border" rows={2} value={form.query} placeholder={t('integrations.api.playgroundQuestionPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, query: event.target.value }))} style={fieldStyle} />
            </label>
          </section>

          <section className="grid gap-3 rounded-[10px] border border-[var(--wk-border,#e5e7eb)] p-4">
            <h4 className="m-0 text-[14px]">{t('integrations.api.playgroundSectionPreview')}</h4>
            <div className="wk-api-playground-preview">
              <div className="wk-api-playground-preview-toolbar" style={rowStyle}>
                <span className="wk-muted text-muted">{t('integrations.api.playgroundRequestPreview')}</span>
                <button type="button" className="wk-button wk-button--text cursor-pointer rounded-control border border-solid border-transparent! bg-transparent px-[0.5rem]! py-[0.3rem]! text-muted-strong! [font:inherit] disabled:cursor-not-allowed disabled:opacity-55! hover:bg-hover-wash focus-visible:bg-hover-wash enabled:hover:border-primary!" onClick={() => copy(preview)}>{t('integrations.api.copy')}</button>
              </div>
              <pre className={preClassName} style={preStyle}>{preview}</pre>
            </div>
          </section>

          <section className="grid gap-3 rounded-[10px] border border-[var(--wk-border,#e5e7eb)] p-4">
            <h4 className="m-0 text-[14px]">{t('integrations.api.playgroundSectionResult')}</h4>
            {run.error ? <p className="m-0" role="alert">{run.error}</p> : null}
            {run.successMs !== null ? <p className="wk-status wk-status-ok my-[0.25rem]! text-[13px] text-success-text!" role="status">{t('integrations.api.playgroundSuccess', { ms: run.successMs })}</p> : null}
            {hasResult ? (
              <div className="wk-api-playground-results">
                {mode === 'signed_token' && run.signedToken ? (
                  <div className="grid gap-2" data-step="token">
                    <div className="text-[13px]" style={rowStyle}>
                      <span>{t('integrations.api.playgroundGeneratedToken')}</span>
                      <button type="button" className="wk-button wk-button--text cursor-pointer rounded-control border border-solid border-transparent! bg-transparent px-[0.5rem]! py-[0.3rem]! text-muted-strong! [font:inherit] disabled:cursor-not-allowed disabled:opacity-55! hover:bg-hover-wash focus-visible:bg-hover-wash enabled:hover:border-primary!" onClick={() => copy(run.signedToken)}>{t('integrations.api.copy')}</button>
                    </div>
                    <pre className={preClassName} style={preStyle}>{run.signedToken}</pre>
                  </div>
                ) : null}
                <div className="grid gap-2" data-step="session">
                  <div className="text-[13px]" style={rowStyle}>
                    <span>{t('integrations.api.playgroundStepSession')}</span>
                    {statusTag(run.sessionStatus)}
                  </div>
                  <pre className={preClassName} style={{ ...preStyle, maxHeight: 180 }}>{run.sessionResponse || '-'}</pre>
                </div>
                <div className="grid gap-2" data-step="chat">
                  <div className="text-[13px]" style={rowStyle}>
                    <span>{t('integrations.api.playgroundStepChat')}</span>
                    {statusTag(run.chatStatus)}
                  </div>
                  <pre className={preClassName} style={{ ...preStyle, maxHeight: 240 }}>{run.streamOutput || '-'}</pre>
                </div>
                {run.finalAnswer ? (
                  <div className="grid gap-2" data-step="answer">
                    <div className="text-[13px]" style={rowStyle}>
                      <span>{t('integrations.api.playgroundFinalAnswer')}</span>
                    </div>
                    <pre className={preClassName} style={preStyle}>{run.finalAnswer}</pre>
                  </div>
                ) : null}
              </div>
            ) : (
              <p className="wk-api-playground-empty wk-muted text-muted m-0">{t('integrations.api.playgroundEmptyResult')}</p>
            )}
          </section>
        </div>

        <footer className="wk-api-playground-footer" style={footerStyle}>
          <div className="wk-api-playground-footer-left" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {run.running ? <button type="button" className="wk-button cursor-pointer rounded-control border border-solid border-line-control! bg-surface px-[0.85rem]! py-[0.45rem]! text-ink [font:inherit] disabled:cursor-not-allowed disabled:opacity-55! enabled:hover:border-primary!" data-action="stop" onClick={stop}>{t('integrations.api.playgroundStop')}</button> : null}
            {disabledReason ? <span className="wk-api-playground-disabled-reason wk-muted text-muted m-0">{t(disabledReason)}</span> : null}
          </div>
          <button type="button" className="wk-button wk-button--primary cursor-pointer rounded-control border border-solid border-line-control! bg-surface px-[0.85rem]! py-[0.45rem]! text-ink [font:inherit] disabled:cursor-not-allowed disabled:opacity-55! enabled:hover:border-primary!" data-action="run" disabled={run.running || Boolean(disabledReason)} onClick={() => void start()}>{t('integrations.api.playgroundRun')}</button>
        </footer>
      </aside>
    </div>
  );
  return typeof document === 'undefined' ? drawer : createPortal(drawer, document.body);
}
