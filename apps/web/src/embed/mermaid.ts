/**
 * Embed answer mermaid hydration — R446/A3 port of the Vue embed chain:
 *
 *   frontend/src/views/embed/EmbedBotMessage.vue:
 *     - createMermaidCodeRenderer('mermaid-embed-botmsg') marks ```mermaid
 *       blocks in the markdown pipeline (apps/web/src/embed/markdown.ts);
 *     - watch/onUpdated run renderMermaidDiagrams() ONLY when
 *       session.is_completed, so streaming answers keep the escaped source
 *       block; enhanceMarkdownContainer -> renderMermaidInContainer renders
 *       the SVG and engine failures are caught (code block stays visible).
 *
 * The renderer itself is the shared views engine
 * (packages/views/src/chat/mermaid.ts, mermaid 11.15.0 pinned there): the
 * embed face only owns the is_completed gate and hands the answer root to
 * hydrateMermaidBlocksWithBrowserDefaults, which dynamic-imports mermaid +
 * dompurify and injects the sanitized SVG container (.wk-chat-mermaid).
 */
import { hydrateMermaidBlocksWithBrowserDefaults } from '@weknora/views/chat/mermaid';

/** Render-id namespace for embed answers (Vue prefix: 'mermaid-embed-botmsg'). */
export const EMBED_MERMAID_PREFIX = 'wk-embed-mermaid';

/** The shared views loader (mermaid + DOMPurify svg profile, strict mode). */
export const defaultEmbedMermaidLoader = hydrateMermaidBlocksWithBrowserDefaults;

type EmbedMermaidLoader = typeof hydrateMermaidBlocksWithBrowserDefaults;

/** Header/viewer copy for the embed mermaid chrome (Vue mermaid.* labels). */
export interface EmbedMermaidLabels {
  badge: string;
  expand: string;
  close: string;
}

// Vue EXPAND_ICON (frontend/src/utils/markdownEnhancements.ts).
const EXPAND_ICON =
  '<svg class="embed-mermaid-block__expand-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>';
const CLOSE_ICON =
  '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';

/**
 * Fullscreen viewer for one embed diagram (Vue openMermaidFullscreen, minimal
 * face): a fixed overlay dialog that shows the sanitized SVG and dismisses on
 * the close button, an overlay click, or Escape.
 */
export function openEmbedMermaidFullscreen(svgHtml: string, labels: EmbedMermaidLabels): void {
  if (typeof document === 'undefined') return;
  const overlay = document.createElement('div');
  overlay.className = 'embed-mermaid-viewer';
  overlay.setAttribute('role', 'dialog');
  overlay.setAttribute('aria-label', labels.expand);

  const stage = document.createElement('div');
  stage.className = 'embed-mermaid-viewer__stage';
  stage.innerHTML = svgHtml;

  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.className = 'embed-mermaid-viewer__close';
  closeBtn.setAttribute('aria-label', labels.close);
  closeBtn.setAttribute('title', labels.close);
  closeBtn.innerHTML = CLOSE_ICON;

  const onKeydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.stopPropagation();
      close();
    }
  };
  const close = () => {
    overlay.remove();
    document.removeEventListener('keydown', onKeydown, true);
  };
  closeBtn.addEventListener('click', (event) => {
    event.stopPropagation();
    close();
  });
  overlay.addEventListener('click', (event) => {
    if (event.target === overlay) close();
  });
  document.addEventListener('keydown', onKeydown, true);

  overlay.append(stage, closeBtn);
  document.body.appendChild(overlay);
}

/**
 * Wrap each hydrated .wk-chat-mermaid figure with the Vue embed header chrome:
 * a mermaid.diagram badge plus a mermaid.expand fullscreen button
 * (EmbedBotMessage.vue -> buildMermaidBlockHtml). Idempotent, and the expand
 * action stays disabled until an svg exists (syncMermaidExpandButtons).
 */
export function decorateEmbedMermaidChrome(root: HTMLElement, labels: EmbedMermaidLabels): void {
  root.querySelectorAll('.wk-chat-mermaid').forEach((figure) => {
    if (figure.closest('.embed-mermaid-block')) return;
    const block = document.createElement('div');
    block.className = 'embed-mermaid-block';
    const header = document.createElement('div');
    header.className = 'embed-mermaid-block__header';
    const badge = document.createElement('span');
    badge.className = 'embed-mermaid-block__badge';
    badge.textContent = labels.badge;
    const expand = document.createElement('button');
    expand.type = 'button';
    expand.className = 'embed-mermaid-block__expand';
    expand.setAttribute('aria-label', labels.expand);
    expand.setAttribute('title', labels.expand);
    expand.innerHTML = EXPAND_ICON;
    const svg = figure.querySelector('svg');
    expand.disabled = !svg;
    expand.addEventListener('click', (event) => {
      event.preventDefault();
      event.stopPropagation();
      if (expand.disabled) return;
      const diagramSvg = figure.querySelector('svg');
      if (diagramSvg) openEmbedMermaidFullscreen(diagramSvg.outerHTML, labels);
    });
    header.append(badge, expand);
    figure.replaceWith(block);
    block.append(header, figure);
  });
}

/**
 * Hydrate the ```mermaid blocks of one answer, mirroring the Vue
 * is_completed gate: streaming answers (isCompleted=false) and answers without
 * diagram blocks never reach the loader; loader failures are contained so the
 * escaped source block stays visible instead of surfacing an error.
 */
export async function hydrateEmbedAnswerMermaid(
  root: HTMLElement | null | undefined,
  isCompleted: boolean,
  loader: EmbedMermaidLoader = defaultEmbedMermaidLoader,
  labels?: EmbedMermaidLabels,
): Promise<void> {
  if (!root || !isCompleted || typeof window === 'undefined') return;
  if (!root.querySelector('[data-markdown-diagram="mermaid"]')) return;
  try {
    await loader(root, EMBED_MERMAID_PREFIX);
    if (labels) decorateEmbedMermaidChrome(root, labels);
  } catch {
    // The escaped Mermaid source remains as the readable fallback when the
    // optional renderer or sanitizer cannot load (Vue renderMermaidInContainer
    // also swallows engine errors per block).
  }
}
