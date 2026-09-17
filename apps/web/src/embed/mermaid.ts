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
): Promise<void> {
  if (!root || !isCompleted || typeof window === 'undefined') return;
  if (!root.querySelector('[data-markdown-diagram="mermaid"]')) return;
  try {
    await loader(root, EMBED_MERMAID_PREFIX);
  } catch {
    // The escaped Mermaid source remains as the readable fallback when the
    // optional renderer or sanitizer cannot load (Vue renderMermaidInContainer
    // also swallows engine errors per block).
  }
}
