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
    return <main className="wk-page mx-auto box-border max-w-[960px] px-5 py-12" data-testid="shared-session-loading"><p role="status">{t('common.loading')}</p></main>;
  }
  if (state.phase === 'invalid') {
    return (
      <main className="wk-page mx-auto box-border max-w-[960px] px-5 py-12" data-testid="shared-session-invalid">
        <h1 className="m-0 mb-[8px] text-[20px] font-semibold text-[rgba(23,26,29,0.92)]">{t('settings.queryHistory.sharedInvalid')}</h1>
        <p className="m-0 text-[13px] text-[#8a96a8]">{t('settings.queryHistory.sharedInvalidHint')}</p>
      </main>
    );
  }
  if (state.phase === 'error') {
    return (
      <main className="wk-page mx-auto box-border max-w-[960px] px-5 py-12" data-testid="shared-session-error" role="alert">
        <p className="m-0 mb-[8px] text-[13px] text-[#b42318]">{state.message || t('settings.queryHistory.sharedLoadFailed')}</p>
        <button type="button" className="min-h-[32px] cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-white px-[12px] text-[13px] hover:bg-[#f3f3f3]" onClick={() => setReloadCount((count) => count + 1)}>{t('common.retry')}</button>
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
    <main className="wk-page mx-auto box-border max-w-[960px] px-5 py-10" data-testid="shared-session-page" aria-label={t('settings.queryHistory.sharedPageTitle')}>
      <header className="mb-[16px] flex flex-col gap-[6px] border-b border-[#eef1f5] pb-[14px]">
        <div className="flex flex-wrap items-center gap-[8px]">
          <h1 className="m-0 break-all text-[20px] font-semibold text-[rgba(23,26,29,0.92)]">{session.title}</h1>
          <span className="inline-flex shrink-0 items-center rounded-[999px] border border-[rgba(46,109,230,0.22)] bg-[rgba(46,109,230,0.08)] px-[7px] text-xs leading-[18px] text-[#2f5ca8]" data-testid="shared-session-readonly-badge">{t('settings.queryHistory.sharedReadonlyBadge')}</span>
        </div>
        <dl className="m-0 flex flex-wrap gap-x-[16px] gap-y-[4px] text-xs text-[#8a96a8]">
          {meta.map(([label, value]) => (
            <div key={label} className="flex items-center gap-[4px]">
              <dt>{label}</dt>
              <dd className="m-0 text-[rgba(23,26,29,0.75)]">{value}</dd>
            </div>
          ))}
        </dl>
      </header>
      {messages.length === 0 ? <p className="m-0 text-[13px] text-[#8a96a8]">{t('common.empty')}</p> : (
        <ol className="m-0 flex list-none flex-col gap-[10px] p-0">
          {messages.map((message) => {
            const refs = messageReferenceCount(message);
            return (
              <li key={message.id} className="rounded-[8px] border border-[rgba(120,135,155,0.18)] bg-[#f7f9fb] p-[12px]">
                <div className="mb-[6px] flex items-center gap-[8px]">
                  <span className={`inline-flex shrink-0 whitespace-nowrap rounded-[999px] border px-[7px] text-xs leading-[18px] ${ROLE_TONE[message.role]}`}>
                    {message.role === 'user' ? t('settings.queryHistory.roleUser') : message.role === 'assistant' ? t('settings.queryHistory.roleAssistant') : t('settings.queryHistory.roleSystem')}
                  </span>
                  {refs > 0 ? <span className="inline-flex shrink-0 items-center gap-[4px] rounded-[999px] border border-[rgba(120,135,155,0.2)] bg-[rgba(120,135,155,0.08)] px-[7px] text-xs leading-[18px] text-[#5c6b83]">{t('settings.queryHistory.refCount', { n: refs })}</span> : null}
                </div>
                <p className="m-0 whitespace-pre-wrap break-all text-[13px] leading-[1.55] text-[rgba(23,26,29,0.92)]">{message.content}</p>
              </li>
            );
          })}
        </ol>
      )}
      {truncated ? <p className="m-0 mt-[10px] text-xs text-[#9a6a0b]" role="note">{t('settings.queryHistory.truncated')}</p> : null}
    </main>
  );
}
