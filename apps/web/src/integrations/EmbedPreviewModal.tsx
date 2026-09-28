import { useEffect, useState } from 'react';
import { integrationsLocale, integrationsT } from '../../../../packages/views/src/integrations/messages.ts';
import './integrations-u.css';

// Vue baseline: frontend/src/components/EmbedChannelPreview.vue (720px preview
// drawer, iframe mode L5-19) + AgentEmbedChannelPanel.vue openPreviewForChannel
// (L993-1035): the deploy-step 预览 button mints a short-lived preview session
// (previewSession port, wired on IntegrationsRoutePage) and opens an in-page
// modal whose iframe loads the embed URL with the session token in the hash
// fragment. This component carries the same modality for the route shell: the
// in-page deploy-step preview panel in packages/views page.tsx keeps its own
// markup but shares these URL/close semantics.
// Markup carries the former wk-embed-preview-* rules (styles.css) as Tailwind
// utilities — the same utilities the deploy-step preview panel uses in
// packages/views page.tsx — so the modal and the panel stay visually identical.

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

function closeLabel(): string {
  try { return integrationsT(integrationsLocale(window.localStorage.getItem('locale')), 'common.close'); } catch { return integrationsT(integrationsLocale(null), 'common.close'); }
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
  // Former .wk-embed-preview-overlay/.wk-embed-preview-drawer (styles.css):
  // the @media (max-width: 720px) drawer width became the max-[720px] variant.
  return <div className="wk-epm-1" role="presentation" onClick={onClose}>
    <aside className="wk-epm-2" role="dialog" aria-modal="true" aria-label={label} onClick={(event) => event.stopPropagation()}>
      {/* Former .wk-embed-preview-header + .wk-embed-preview-header h2. */}
      <header className="wk-epm-3">
        <h2 className="wk-epm-4">{label}</h2>
        {/* Former .wk-integration-drawer-close css (styles.css) as utilities;
            the class name stays as a DOM hook. */}
        <button type="button" className="wk-integration-drawer-close wk-epm-5" aria-label={closeLabel()} title={closeLabel()} onClick={onClose}>×</button>
      </header>
      {/* Former .wk-embed-preview-body (incl. the ≤720px padding-inline tweak). */}
      <div className="wk-epm-6">
        {/* Former .wk-embed-preview-device. */}
        <div className="wk-epm-7">
          {/* Former .wk-embed-preview-chrome + span:nth-child(1..3) + code. */}
          <div className="wk-epm-8"><span className="wk-epm-9">●</span><span className="wk-epm-10">●</span><span className="wk-epm-11">●</span><code className="wk-epm-12">/embed/{encodeURIComponent(channelId)}</code></div>
          {/* Former .wk-embed-preview-screen + iframe rules; the old dynamic
              .is-loading class became this static 'invisible' condition. */}
          <div className="wk-epm-13">
            {!ready ? <span className="wk-muted wk-epm-14">正在加载预览…</span> : null}
            {layoutReady ? <iframe title={label} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'wk-epm-15'} allow="clipboard-write" /> : null}
          </div>
        </div>
      </div>
    </aside>
  </div>;
}
