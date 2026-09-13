import { useEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';

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
// machine). Drawer shell is an inline overlay panel pending the global Drawer
// primitive (same convention as the embed preview panel).

export interface ApiPlaygroundAgentOption { id: string; name: string; is_builtin?: boolean }

export interface ApiPlaygroundDrawerProps {
  open: boolean;
  onClose: () => void;
  apiKey: string;
  mode: 'tenant' | 'direct_header' | 'signed_token';
  agents: readonly ApiPlaygroundAgentOption[];
  agentsError?: string;
  apiBaseUrl: string;
  /** Vue createAPIPrincipalTestToken (signed_token mode only). */
  mintToken?: (externalUserId: string) => Promise<{ token: string; headerName: string }>;
  fetchFn?: typeof fetch;
  t: (key: string, values?: Record<string, string | number>) => string;
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

const overlayStyle: CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(15,23,42,.4)', zIndex: 59 };
const panelStyle: CSSProperties = { position: 'fixed', top: 0, right: 0, bottom: 0, width: 'min(640px, 94vw)', background: 'var(--wk-bg, #fff)', boxShadow: '-12px 0 32px rgba(0,0,0,.18)', display: 'flex', flexDirection: 'column', zIndex: 60 };
const bodyStyle: CSSProperties = { flex: '1 1 auto', overflowY: 'auto', padding: '0 20px 12px' };
const footerStyle: CSSProperties = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '12px 20px', borderTop: '1px solid var(--wk-border, #e5e7eb)' };
const preStyle: CSSProperties = { background: 'var(--wk-bg-muted, #f6f8fa)', padding: 10, overflowX: 'auto', fontSize: 12 };
const rowStyle: CSSProperties = { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 };
const fieldStyle: CSSProperties = { display: 'block', margin: '10px 0', width: '100%' };

export function ApiPlaygroundDrawer({ open, onClose, apiKey, mode, agents, agentsError, apiBaseUrl, mintToken, fetchFn, t }: ApiPlaygroundDrawerProps) {
  const [form, setForm] = useState({ agentId: '', query: 'hello', externalUserId: 'user_123' });
  const [run, setRun] = useState<RunState>(idleRun);
  const controllerRef = useRef<AbortController | null>(null);

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
  const close = () => { controllerRef.current?.abort(); onClose(); };
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
    <span className="wk-api-playground-status" data-status={status || 'none'}>{status || '-'}</span>
  );

  return (
    <div className="wk-api-playground-overlay" role="presentation" style={overlayStyle} onClick={close}>
      <aside className="wk-api-playground-drawer" role="dialog" aria-modal="true" aria-label={t('integrations.api.playgroundTitle')} style={panelStyle} onClick={(event) => event.stopPropagation()}>
        <header className="wk-api-playground-header" style={rowStyle}>
          <div>
            <h2 style={{ margin: 0, fontSize: 16 }}>{t('integrations.api.playgroundTitle')}</h2>
            <p className="wk-muted" style={{ margin: '4px 0 0' }}>{t('integrations.api.playgroundDrawerDesc')}</p>
          </div>
          <button type="button" className="wk-api-playground-close" aria-label="关闭" title="关闭" onClick={close} style={{ border: 'none', background: 'transparent', fontSize: 18, cursor: 'pointer' }}>×</button>
        </header>

        <div className="wk-api-playground-body" style={bodyStyle}>
          <section className="wk-api-playground-section">
            <h4>{t('integrations.api.playgroundSectionRequest')}</h4>
            <label className="wk-api-playground-field" style={{ display: 'block', margin: '10px 0' }}>
              {t('integrations.api.playgroundAgent')}
              <select className="wk-api-playground-agent" value={form.agentId} onChange={(event) => setForm((prev) => ({ ...prev, agentId: event.target.value }))} style={fieldStyle}>
                <option value="" disabled>{t('integrations.api.playgroundAgentPlaceholder')}</option>
                {agents.map((agent) => (
                  <option key={agent.id} value={agent.id}>{agentOptionLabel(agent.name, agent.is_builtin === true, t('integrations.api.playgroundBuiltin'))}</option>
                ))}
              </select>
            </label>
            {agentsError ? <p className="wk-api-playground-field-error" role="alert">{agentsError}</p> : null}
            <label className="wk-api-playground-field" style={{ display: 'block', margin: '10px 0' }}>
              {t('integrations.api.playgroundExternalUser')}
              <input className="wk-api-playground-external-user" type="text" value={form.externalUserId} disabled={mode === 'tenant'} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, externalUserId: event.target.value }))} style={fieldStyle} />
            </label>
            <p className="wk-api-playground-hint wk-muted">{t(externalUserHintKey(mode), { headerName: DEFAULT_DIRECT_HEADER_NAME })}</p>
            <label className="wk-api-playground-field" style={{ display: 'block', margin: '10px 0' }}>
              {t('integrations.api.playgroundQuestion')}
              <textarea className="wk-api-playground-query" rows={3} value={form.query} placeholder={t('integrations.api.playgroundQuestionPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, query: event.target.value }))} style={fieldStyle} />
            </label>
          </section>

          <section className="wk-api-playground-section">
            <h4>{t('integrations.api.playgroundSectionPreview')}</h4>
            <div className="wk-api-playground-preview">
              <div className="wk-api-playground-preview-toolbar" style={rowStyle}>
                <span className="wk-muted">{t('integrations.api.playgroundRequestPreview')}</span>
                <button type="button" className="wk-button wk-button--text" onClick={() => copy(preview)}>{t('integrations.api.copy')}</button>
              </div>
              <pre style={preStyle}>{preview}</pre>
            </div>
          </section>

          <section className="wk-api-playground-section">
            <h4>{t('integrations.api.playgroundSectionResult')}</h4>
            {run.error ? <p className="wk-api-playground-alert" role="alert">{run.error}</p> : null}
            {run.successMs !== null ? <p className="wk-status wk-status-ok" role="status">{t('integrations.api.playgroundSuccess', { ms: run.successMs })}</p> : null}
            {hasResult ? (
              <div className="wk-api-playground-results">
                {mode === 'signed_token' && run.signedToken ? (
                  <div className="wk-api-playground-step" data-step="token">
                    <div className="wk-api-playground-step-header" style={rowStyle}>
                      <span>{t('integrations.api.playgroundGeneratedToken')}</span>
                      <button type="button" className="wk-button wk-button--text" onClick={() => copy(run.signedToken)}>{t('integrations.api.copy')}</button>
                    </div>
                    <pre style={preStyle}>{run.signedToken}</pre>
                  </div>
                ) : null}
                <div className="wk-api-playground-step" data-step="session">
                  <div className="wk-api-playground-step-header" style={rowStyle}>
                    <span>{t('integrations.api.playgroundStepSession')}</span>
                    {statusTag(run.sessionStatus)}
                  </div>
                  <pre style={{ ...preStyle, maxHeight: 180 }}>{run.sessionResponse || '-'}</pre>
                </div>
                <div className="wk-api-playground-step" data-step="chat">
                  <div className="wk-api-playground-step-header" style={rowStyle}>
                    <span>{t('integrations.api.playgroundStepChat')}</span>
                    {statusTag(run.chatStatus)}
                  </div>
                  <pre style={{ ...preStyle, maxHeight: 240 }}>{run.streamOutput || '-'}</pre>
                </div>
                {run.finalAnswer ? (
                  <div className="wk-api-playground-step" data-step="answer">
                    <div className="wk-api-playground-step-header" style={rowStyle}>
                      <span>{t('integrations.api.playgroundFinalAnswer')}</span>
                    </div>
                    <pre style={preStyle}>{run.finalAnswer}</pre>
                  </div>
                ) : null}
              </div>
            ) : (
              <p className="wk-api-playground-empty wk-muted">{t('integrations.api.playgroundEmptyResult')}</p>
            )}
          </section>
        </div>

        <footer className="wk-api-playground-footer" style={footerStyle}>
          <div className="wk-api-playground-footer-left" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {run.running ? <button type="button" className="wk-button" data-action="stop" onClick={stop}>{t('integrations.api.playgroundStop')}</button> : null}
            {disabledReason ? <span className="wk-api-playground-disabled-reason wk-muted">{t(disabledReason)}</span> : null}
          </div>
          <button type="button" className="wk-button wk-button--primary" data-action="run" disabled={run.running || Boolean(disabledReason)} onClick={() => void start()}>{t('integrations.api.playgroundRun')}</button>
        </footer>
      </aside>
    </div>
  );
}
