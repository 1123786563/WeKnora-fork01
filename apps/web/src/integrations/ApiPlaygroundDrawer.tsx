import * as React from 'react';
import { consumeApiPlaygroundSSE } from './apiPlaygroundSSE';
import {
  agentOptionLabel,
  buildChatRequestBody,
  buildPlaygroundHeaders,
  compactText,
  ensurePlaygroundAgent,
  externalUserHintKey,
  formatResponseBody,
  interpretSessionResponse,
  playgroundDisabledReason,
  playgroundRequestPreview,
} from './apiPlaygroundModel';

export type ApiPlaygroundAgentOption = { id: string; name: string; is_builtin?: boolean };
type Mode = 'tenant' | 'direct_header' | 'signed_token';
type Token = { token: string; headerName: string };
type Translator = (key: string, values?: Record<string, string | number>) => string;
type StepStatus = 'idle' | 'running' | 'success' | 'failed' | 'stopped';

export function ApiPlaygroundDrawer({ open, onClose, apiKey, mode, agents, apiBaseUrl, fetchFn = fetch, mintToken, t }: {
  open: boolean; onClose: () => void; apiKey: string; mode: Mode; agents: readonly ApiPlaygroundAgentOption[];
  apiBaseUrl: string; fetchFn?: typeof fetch; mintToken?: (externalUserId: string) => Promise<Token>; t: Translator;
}) {
  const [agentId, setAgentId] = React.useState(() => ensurePlaygroundAgent('', agents));
  const [externalUserId, setExternalUserId] = React.useState('user_123');
  const [query, setQuery] = React.useState('hello');
  const [running, setRunning] = React.useState(false);
  const [alert, setAlert] = React.useState('');
  const [token, setToken] = React.useState<Token | null>(null);
  const [sessionResponse, setSessionResponse] = React.useState('');
  const [streamOutput, setStreamOutput] = React.useState('');
  const [finalAnswer, setFinalAnswer] = React.useState('');
  const [sessionStatus, setSessionStatus] = React.useState<StepStatus>('idle');
  const [chatStatus, setChatStatus] = React.useState<StepStatus>('idle');
  const [answerStatus, setAnswerStatus] = React.useState<StepStatus>('idle');
  const abortRef = React.useRef<AbortController | null>(null);
  React.useEffect(() => setAgentId((current) => ensurePlaygroundAgent(current, agents)), [agents]);
  React.useEffect(() => () => abortRef.current?.abort('closed'), []);
  if (!open) return null;
  const signedToken = token?.token ?? '';
  const reasonKey = playgroundDisabledReason({ running, apiKey, agentId, query, mode, externalUserId });
  const headers = buildPlaygroundHeaders({ apiKey, mode, externalUserId, signedToken, maskSecrets: false });
  const run = async () => {
    if (reasonKey || running) return;
    const controller = new AbortController(); abortRef.current = controller;
    setRunning(true); setAlert(''); setSessionResponse(''); setStreamOutput(''); setFinalAnswer('');
    setSessionStatus('running'); setChatStatus('idle'); setAnswerStatus('idle');
    try {
      let activeToken = token;
      if (mode === 'signed_token') {
        if (!mintToken) throw new Error(t('integrations.api.playgroundMintTokenFailed'));
        activeToken = await mintToken(externalUserId.trim()); setToken(activeToken);
      }
      const activeHeaders = buildPlaygroundHeaders({ apiKey, mode, externalUserId, signedToken: activeToken?.token ?? '', maskSecrets: false });
      const session = await fetchFn(apiBaseUrl.replace(/\/$/, '') + '/api/v1/sessions', { method: 'POST', headers: activeHeaders.sessionHeaders, body: '{}', signal: controller.signal });
      const sessionText = await session.text(); setSessionResponse(formatResponseBody(sessionText));
      const sessionResult = interpretSessionResponse({ ok: session.ok, status: session.status, payload: (() => { try { return JSON.parse(sessionText); } catch { return sessionText; } })() });
      if (!sessionResult.ok) throw new Error(sessionResult.error);
      setSessionStatus('success'); setChatStatus('running'); setAnswerStatus('running');
      const chat = await fetchFn(apiBaseUrl.replace(/\/$/, '') + '/api/v1/agent-chat/' + encodeURIComponent(sessionResult.sessionId), { method: 'POST', headers: activeHeaders.chatHeaders, body: JSON.stringify(buildChatRequestBody({ query, agentId })), signal: controller.signal });
      if (!chat.ok || !chat.body) throw new Error(chat.body ? `HTTP ${chat.status}` : t('integrations.api.playgroundNoStream'));
      const stream = await consumeApiPlaygroundSSE(chat.body, (progress) => { setStreamOutput(compactText(progress.raw)); setFinalAnswer(progress.answer); });
      setStreamOutput(compactText(stream.raw)); setFinalAnswer(stream.answer);
      if (stream.status === 'failed') throw new Error(stream.error || t('integrations.api.playgroundFailed'));
      setChatStatus('success'); setAnswerStatus('success'); setAlert(t('integrations.api.playgroundSuccess', { ms: 0 }));
    } catch (cause) {
      const stopped = controller.signal.aborted;
      setSessionStatus((status) => stopped ? 'stopped' : (status === 'running' ? 'failed' : status));
      setChatStatus((status) => status === 'running' ? (stopped ? 'stopped' : 'failed') : status);
      setAnswerStatus((status) => status === 'running' ? (stopped ? 'stopped' : 'failed') : status);
      setAlert(stopped ? t('integrations.api.playgroundStopped') : (cause instanceof Error ? cause.message : t('integrations.api.playgroundFailed')));
    } finally { setRunning(false); abortRef.current = null; }
  };
  const stop = () => abortRef.current?.abort('stopped');
  const step = (name: 'session' | 'chat' | 'answer' | 'token', title: string, status: StepStatus, body: string) => <div className="wk-api-playground-step" data-step={name}><div className="wk-api-playground-step-header"><span>{title}</span><span className="wk-api-playground-status" data-status={status}>{status}</span></div>{body ? <pre>{body}</pre> : null}</div>;
  return <div className="wk-embed-preview-overlay" role="presentation" onClick={onClose}>
    <aside className="wk-api-playground-drawer" role="dialog" aria-modal="true" aria-label="API Playground" onClick={(event) => event.stopPropagation()}>
      <header className="wk-embed-preview-header"><div><h2>API Playground</h2><p className="wk-muted">{t('integrations.api.playgroundDrawerDesc')}</p></div><button type="button" className="wk-api-playground-close" aria-label="关闭" onClick={onClose}>×</button></header>
      <div className="wk-api-playground-body">
        <section className="wk-api-playground-section"><h4>{t('integrations.api.playgroundSectionRequest')}</h4>
          <label>{t('integrations.api.playgroundAgent')}<select className="wk-api-playground-agent" value={agentId} onChange={(event) => setAgentId(event.target.value)}><option value="">{t('integrations.api.playgroundAgentPlaceholder')}</option>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agentOptionLabel(agent.name, Boolean(agent.is_builtin), t('integrations.api.playgroundBuiltin'))}</option>)}</select></label>
          <label>{t('integrations.api.playgroundExternalUser')}<input className="wk-api-playground-external-user" value={externalUserId} disabled={mode === 'tenant'} onChange={(event) => setExternalUserId(event.target.value)} placeholder="user_123" /></label>
          <p className="wk-api-playground-hint wk-muted">{t(externalUserHintKey(mode), { headerName: 'X-External-User-ID' })}</p>
          <label>{t('integrations.api.playgroundQuestion')}<textarea className="wk-api-playground-query" rows={4} value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('integrations.api.playgroundQuestionPlaceholder')} /></label>
          {reasonKey ? <p className="wk-api-playground-disabled-reason wk-status-error">{t(reasonKey)}</p> : null}
          <button type="button" data-action={running ? 'stop' : 'run'} className="wk-button wk-button--primary" disabled={!running && Boolean(reasonKey)} onClick={running ? stop : run}>{running ? t('integrations.api.playgroundStop') : t('integrations.api.playgroundRun')}</button>
        </section>
        <section className="wk-api-playground-section wk-api-playground-preview"><h4>{t('integrations.api.playgroundSectionPreview')}</h4><pre>{playgroundRequestPreview({ query, agentId, mode, externalUserId, signedToken, apiKey })}</pre></section>
        <section className="wk-api-playground-section"><h4>{t('integrations.api.playgroundSectionResult')}</h4>
          {!sessionResponse && !streamOutput && !finalAnswer && !token ? <p className="wk-api-playground-empty">{t('integrations.api.playgroundEmptyResult')}</p> : null}
          {token ? step('token', t('integrations.api.playgroundGeneratedToken'), 'success', token.token) : null}
          {sessionResponse || sessionStatus !== 'idle' ? step('session', t('integrations.api.playgroundStepSession'), sessionStatus, sessionResponse) : null}
          {streamOutput || chatStatus !== 'idle' ? step('chat', t('integrations.api.playgroundStepChat'), chatStatus, streamOutput) : null}
          {finalAnswer || answerStatus !== 'idle' ? step('answer', t('integrations.api.playgroundFinalAnswer'), answerStatus, finalAnswer) : null}
          {alert ? <p className="wk-api-playground-alert wk-status" role="status">{alert}</p> : null}
        </section>
      </div>
    </aside>
  </div>;
}
