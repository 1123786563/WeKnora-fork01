import { useEffect, useState } from 'react';

// Vue baseline: frontend/src/components/EmbedChannelPreview.vue (720px preview
// drawer, iframe mode L5-19) + AgentEmbedChannelPanel.vue openPreviewForChannel
// (L993-1035): the deploy-step 预览 button mints a short-lived preview session
// (previewSession port, wired on IntegrationsRoutePage) and opens an in-page
// modal whose iframe loads the embed URL with the session token in the hash
// fragment. This component carries the same modality for the route shell: the
// in-page deploy-step preview panel in packages/views page.tsx keeps its own
// markup but shares these URL/close semantics.
// Markup reuses the committed wk-embed-preview-* styles (styles.css) so the
// modal and the deploy-step preview panel stay visually identical.

export interface EmbedPreviewModalProps {
  open: boolean;
  channelId: string;
  /** Short-lived preview session token (embedPublish previewSession). */
  token: string;
  title?: string;
  apiBaseUrl?: string;
  /** Channel default locale — passed in the iframe URL (Vue previewLocale L1021). */
  locale?: string;
  /** Bumped by the parent on reopen so the embed page fully reloads (Vue previewNonce). */
  refreshKey?: number;
  onClose: () => void;
}

/** Vue buildEmbedURL (frontend/src/api/embed/index.ts L563-577) with the origin explicit. */
export function buildEmbedPreviewSrc(input: { channelId: string; token: string; apiBaseUrl?: string; locale?: string; refreshKey?: number }): string {
  const base = (input.apiBaseUrl || '').replace(/\/+$/, '');
  let path = base + '/embed/' + encodeURIComponent(input.channelId);
  const params = new URLSearchParams();
  if (input.locale?.trim()) params.set('locale', input.locale.trim());
  if (input.refreshKey) params.set('r', String(input.refreshKey));
  const qs = params.toString();
  if (qs) path += '?' + qs;
  if (input.token) path += '#token=' + encodeURIComponent(input.token);
  return path;
}

export function EmbedPreviewModal({ open, channelId, token, title, apiBaseUrl, locale, refreshKey, onClose }: EmbedPreviewModalProps) {
  const [ready, setReady] = useState(false);
  // Vue defers the iframe until the drawer has laid out (EmbedChannelPreview.vue
  // watch visible L95-104 nextTick; same gate as the views preview panel) so the
  // embed autosize pass never measures a 0-size frame.
  const [layoutReady, setLayoutReady] = useState(false);
  useEffect(() => { setReady(false); }, [channelId, token, refreshKey]);
  useEffect(() => {
    if (!open) { setLayoutReady(false); return; }
    const timer = window.setTimeout(() => setLayoutReady(true), 0);
    return () => window.clearTimeout(timer);
  }, [open, channelId, token, refreshKey]);
  // Vue t-drawer @close + host Escape semantics (page.tsx preview panel parity).
  useEffect(() => {
    if (!open) return;
    const handleEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [open, onClose]);
  // Vue EmbedChannelPreview iframeSrc: no channelId/token -> no iframe (L78-84).
  if (!open || !channelId || !token) return null;

  const label = title || '预览';
  const src = buildEmbedPreviewSrc({ channelId, token, apiBaseUrl, locale, refreshKey });
  return <div className="wk-embed-preview-overlay" role="presentation" onClick={onClose}>
    <aside className="wk-embed-preview-drawer" role="dialog" aria-modal="true" aria-label={label} onClick={(event) => event.stopPropagation()}>
      <header className="wk-embed-preview-header">
        <h2>{label}</h2>
        {/* Former .wk-integration-drawer-close css (styles.css) as utilities;
            the class name stays as a DOM hook. */}
        <button type="button" className="wk-integration-drawer-close absolute top-[14px] right-[16px] z-[2] h-[28px] w-[28px] cursor-pointer rounded-[4px] border-0 bg-transparent text-[22px] leading-none text-[#667085] hover:bg-[#f3f4f6] hover:text-ink focus-visible:bg-[#f3f4f6] focus-visible:text-ink focus-visible:outline-none" aria-label="关闭" title="关闭" onClick={onClose}>×</button>
      </header>
      <div className="wk-embed-preview-body">
        <div className="wk-embed-preview-device">
          <div className="wk-embed-preview-chrome"><span>●</span><span>●</span><span>●</span><code>/embed/{encodeURIComponent(channelId)}</code></div>
          <div className="wk-embed-preview-screen">
            {!ready ? <span className="wk-muted">正在加载预览…</span> : null}
            {layoutReady ? <iframe title={label} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'is-loading'} allow="clipboard-write" /> : null}
          </div>
        </div>
      </div>
    </aside>
  </div>;
}
