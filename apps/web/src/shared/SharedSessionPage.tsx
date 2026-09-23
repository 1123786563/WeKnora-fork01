// SP13 Task 8 — 会话只读分享页（/platform/shared/:token，挂在 platformRoute
// 下复用登录守卫与 shell；GET /api/v1/shared/sessions/:token，Viewer+）。
// 渲染模式照 Task 7 QueryHistoryPanel 抽屉：role 徽标 + 纯文本消息流 +
// truncated 提示；无输入框、无反馈、无任何写操作。token 无效或已撤销
// （404）渲染「链接无效或已撤销」占位；其余失败渲染错误 + 重试。
import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { SharedSessionSnapshot } from '@weknora/contracts';
import { formatMessage, type Locale } from '@weknora/i18n';
import { useAppLocale } from '../i18n.ts';
import { isShareLinkInvalidError, normalizeShareToken } from '../chat/session-share.ts';
import { messageReferenceCount, queryHistoryDateParts, sessionSourceText } from '../settings/QueryHistoryPanel.tsx';
import './shared-u.css';

type SharedLoadState =
  | { phase: 'loading' }
  | { phase: 'invalid' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; snapshot: SharedSessionSnapshot };

const ROLE_TONE: Record<'user' | 'assistant' | 'system', string> = {
  user: 'bg-[rgba(7,192,95,0.1)] border-[rgba(7,192,95,0.24)] text-[#078a45]',
  assistant: 'bg-[rgba(46,109,230,0.08)] border-[rgba(46,109,230,0.22)] text-[#2f5ca8]',
  system: 'bg-[rgba(120,135,155,0.1)] border-[rgba(120,135,155,0.2)] text-[#5c6b83]',
};

export function SharedSessionPage({ client, token }: { client: WeKnoraClient; token: string }) {
  const locale: Locale = useAppLocale();
  const t = (key: string, values?: Record<string, string | number>): string => formatMessage(locale, key, values);
  const [state, setState] = useState<SharedLoadState>({ phase: 'loading' });
  const [reloadCount, setReloadCount] = useState(0);
  const shareToken = normalizeShareToken(token);

  useEffect(() => {
    // 空 token（坏路由参数）与 404 同一张占位，不发请求。
    if (!shareToken) {
      setState({ phase: 'invalid' });
      return;
    }
    let active = true;
    setState({ phase: 'loading' });
    client.queryHistory.shared(shareToken)
      .then((snapshot) => { if (active) setState({ phase: 'ready', snapshot }); })
      .catch((reason: unknown) => {
        if (!active) return;
        setState(isShareLinkInvalidError(reason)
          ? { phase: 'invalid' }
          : { phase: 'error', message: reason instanceof Error ? reason.message : t('settings.queryHistory.sharedLoadFailed') });
      });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, shareToken, reloadCount]);

  if (state.phase === 'loading') {
    return <main className="wk-page wk-page--std" data-testid="shared-session-loading"><p role="status">{t('common.loading')}</p></main>;
  }
  if (state.phase === 'invalid') {
    return (
      <main className="wk-page wk-page--std" data-testid="shared-session-invalid">
        <h1 className="wk-shared-1">{t('settings.queryHistory.sharedInvalid')}</h1>
        <p className="wk-shared-2">{t('settings.queryHistory.sharedInvalidHint')}</p>
      </main>
    );
  }
  if (state.phase === 'error') {
    return (
      <main className="wk-page wk-page--std" data-testid="shared-session-error" role="alert">
        <p className="wk-shared-3">{state.message || t('settings.queryHistory.sharedLoadFailed')}</p>
        <button type="button" className="wk-shared-4" onClick={() => setReloadCount((count) => count + 1)}>{t('common.retry')}</button>
      </main>
    );
  }

  const { session, messages, truncated } = state.snapshot;
  const created = queryHistoryDateParts(session.created_at, locale);
  const meta: Array<[string, string]> = [
    [t('settings.queryHistory.colCreated'), created.date === '-' ? '—' : `${created.date} ${created.time}`],
    [t('settings.queryHistory.colSource'), sessionSourceText(session)],
    [t('settings.queryHistory.colEngine'), typeof session.engine_type === 'string' && session.engine_type ? session.engine_type : '—'],
  ];
  return (
    <main className="wk-page wk-page--std wk-shared-main" data-testid="shared-session-page" aria-label={t('settings.queryHistory.sharedPageTitle')}>
      <header className="wk-shared-5">
        <div className="wk-shared-6">
          <h1 className="wk-shared-7">{session.title}</h1>
          <span className="wk-shared-8" data-testid="shared-session-readonly-badge">{t('settings.queryHistory.sharedReadonlyBadge')}</span>
        </div>
        <dl className="wk-shared-9 wk-shared-1">
          {meta.map(([label, value]) => (
            <div key={label} className="wk-shared-10">
              <dt>{label}</dt>
              <dd className="wk-shared-11">{value}</dd>
            </div>
          ))}
        </dl>
      </header>
      {messages.length === 0 ? <p className="wk-shared-2">{t('common.empty')}</p> : (
        <ol className="wk-shared-12">
          {messages.map((message) => {
            const refs = messageReferenceCount(message);
            return (
              <li key={message.id} className="wk-shared-13">
                <div className="wk-shared-14">
                  <span className={`wk-shared-18${ROLE_TONE[message.role]}`}>
                    {message.role === 'user' ? t('settings.queryHistory.roleUser') : message.role === 'assistant' ? t('settings.queryHistory.roleAssistant') : t('settings.queryHistory.roleSystem')}
                  </span>
                  {refs > 0 ? <span className="wk-shared-15">{t('settings.queryHistory.refCount', { n: refs })}</span> : null}
                </div>
                <p className="wk-shared-16">{message.content}</p>
              </li>
            );
          })}
        </ol>
      )}
      {truncated ? <p className="wk-shared-17" role="note">{t('settings.queryHistory.truncated')}</p> : null}
    </main>
  );
}
