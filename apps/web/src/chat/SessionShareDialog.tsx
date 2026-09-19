// SP13 Task 8 — 会话分享弹窗（ChatRoutePage 聊天宿主与 PlatformShell 侧栏
// 共用）。打开即调 client.queryHistory.share（Task 5：每次 mint/rotate 新
// token，旧 token 立即失效），展示只读链接 + 复制 + 撤销 + 关闭；share/
// unshare 失败 toast 由宿主注入（ChatRoutePage showAgentToast 先例）。
//
// 刻意不依赖 @weknora/ui：PlatformShell 在每个 platform 页面与全部 node
// 测试里直接 import 本模块，而 @weknora/ui 的 theme.css 会破坏 node 下的
// 模块加载（router.tsx 对 craft 的 lazy 处理同理）——这里全部用内联
// Tailwind utilities（QueryHistorySnapshotDrawer 抽屉同风格）。
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage, type Locale } from '@weknora/i18n';
import { buildShareLink, writeShareLinkClipboard } from './session-share.ts';

export interface SessionShareDialogProps {
  client: WeKnoraClient;
  sessionId: string;
  /** 侧栏行标题（仅展示；缺省渲染会话 id）。 */
  sessionTitle?: string;
  locale?: Locale;
  onClose(): void;
  /** 宿主 toast 通道（ChatRoutePage showAgentToast / shell 内同等实现）。 */
  onToast(message: string): void;
}

const SECONDARY_BUTTON = 'box-border min-h-[32px] cursor-pointer rounded-[6px] border border-[#dcdcdc] bg-white px-[12px] text-[13px] text-[rgba(0,0,0,0.9)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-55';
const PRIMARY_BUTTON = 'box-border min-h-[32px] cursor-pointer rounded-[6px] border-0 bg-[#07c05f] px-[12px] text-[13px] text-white disabled:cursor-not-allowed disabled:opacity-55';
const DANGER_BUTTON = 'box-border min-h-[32px] cursor-pointer rounded-[6px] border border-[rgba(220,60,60,0.4)] bg-white px-[12px] text-[13px] text-[#c03939] enabled:hover:bg-[#fdecee] disabled:cursor-not-allowed disabled:opacity-55';

export function SessionShareDialog({ client, sessionId, sessionTitle, locale = 'zh-CN', onClose, onToast }: SessionShareDialogProps) {
  const t = (key: string): string => formatMessage(locale, key);
  const [phase, setPhase] = useState<'creating' | 'ready' | 'failed'>('creating');
  const [link, setLink] = useState('');
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState(false);
  const [retryCount, setRetryCount] = useState(0);
  const copiedTimer = useRef<number | null>(null);
  // 迟到的 share 响应不得覆盖新一次打开/重试的结果（generation 守卫）。
  const generation = useRef(0);

  useEffect(() => () => { if (copiedTimer.current !== null) window.clearTimeout(copiedTimer.current); }, []);

  // 打开（或重试）即 mint：share 每次 rotate token，旧链接立即失效，因此
  // 不缓存上一次结果。
  useEffect(() => {
    const current = ++generation.current;
    setPhase('creating');
    setLink('');
    setCopied(false);
    client.queryHistory.share(sessionId)
      .then((result) => {
        if (current !== generation.current) return;
        setLink(buildShareLink(window.location.origin, result.share_token));
        setPhase('ready');
      })
      .catch(() => {
        if (current !== generation.current) return;
        setPhase('failed');
        onToast(t('settings.queryHistory.shareFailedToast'));
      });
    // t/onToast 是宿主闭包；重建会话请求已由 generation 守卫兜底。
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [client, sessionId, retryCount]);

  // Escape 关窗（QueryHistorySnapshotDrawer 同惯例）。
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  async function copyLink(): Promise<void> {
    if (!link) return;
    try {
      await writeShareLinkClipboard(link);
      setCopied(true);
      if (copiedTimer.current !== null) window.clearTimeout(copiedTimer.current);
      copiedTimer.current = window.setTimeout(() => setCopied(false), 2000);
    } catch {
      onToast(t('settings.queryHistory.shareCopyFailedToast'));
    }
  }

  async function revokeShare(): Promise<void> {
    if (revoking) return;
    setRevoking(true);
    try {
      await client.queryHistory.unshare(sessionId);
      onToast(t('settings.queryHistory.shareRevokedToast'));
      onClose();
    } catch {
      onToast(t('settings.queryHistory.shareRevokeFailedToast'));
    } finally {
      setRevoking(false);
    }
  }

  return createPortal(
    <div
      className="fixed inset-0 z-[3400] flex items-center justify-center bg-[rgba(15,23,42,0.45)] p-[16px]"
      role="presentation"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t('settings.queryHistory.shareTitle')}
        data-testid="session-share-dialog"
        className="w-[min(520px,100%)] rounded-[12px] bg-white p-[20px] shadow-[0_18px_50px_rgba(15,23,42,0.22)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="mb-[4px] flex items-start justify-between gap-[8px]">
          <h3 className="m-0 text-[16px] font-semibold text-[rgba(23,26,29,0.92)]">{t('settings.queryHistory.shareTitle')}</h3>
          <button type="button" aria-label={t('settings.queryHistory.close')} className="min-h-[28px] cursor-pointer rounded-[6px] border-0 bg-transparent px-[6px] text-[20px] leading-[1] text-[rgba(23,26,29,0.55)] hover:bg-[#f3f3f3]" onClick={onClose}>×</button>
        </div>
        <p className="m-0 mb-[12px] break-all text-[13px] text-[rgba(23,26,29,0.6)]">{sessionTitle || sessionId}</p>

        {phase === 'creating' ? (
          <p role="status" className="m-0 mb-[12px] text-[13px] text-[#8a96a8]">{t('settings.queryHistory.shareCreating')}</p>
        ) : phase === 'failed' ? (
          <div role="alert" className="mb-[12px] flex flex-wrap items-center gap-[8px]">
            <p className="m-0 text-[13px] text-[#b42318]">{t('settings.queryHistory.shareFailedToast')}</p>
            <button type="button" className={SECONDARY_BUTTON} onClick={() => setRetryCount((count) => count + 1)}>{t('common.retry')}</button>
          </div>
        ) : (
          <>
            <label className="m-0 mb-[6px] block text-[12px] font-semibold text-[#5c6b83]" htmlFor="wk-session-share-link">{t('settings.queryHistory.shareLinkLabel')}</label>
            <div className="mb-[6px] flex items-center gap-[8px]">
              <input
                id="wk-session-share-link"
                type="text"
                readOnly
                value={link}
                data-testid="session-share-link"
                onFocus={(event) => event.currentTarget.select()}
                className="box-border min-w-0 flex-1 rounded-[6px] border border-[#e7e7ea] bg-[#f7f9fb] px-[8px] py-[6px] font-mono text-[12px] text-[rgba(23,26,29,0.92)]"
              />
              <button
                type="button"
                className={PRIMARY_BUTTON}
                data-testid="session-share-copy"
                onClick={() => void copyLink()}
              >
                {copied ? t('settings.queryHistory.shareCopied') : t('settings.queryHistory.shareCopy')}
              </button>
            </div>
            <p className="m-0 mb-[12px] text-[12px] leading-[1.5] text-[#8a96a8]">{t('settings.queryHistory.shareHint')}</p>
          </>
        )}

        <div className="mt-[4px] flex items-center justify-end gap-[8px] border-t border-[#eef1f5] pt-[14px]">
          <button
            type="button"
            className={DANGER_BUTTON}
            data-testid="session-share-revoke"
            disabled={revoking || phase !== 'ready'}
            onClick={() => void revokeShare()}
          >
            {t('settings.queryHistory.shareRevoke')}
          </button>
          <button type="button" className={SECONDARY_BUTTON} onClick={onClose}>{t('settings.queryHistory.close')}</button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
