/// <reference path="./mermaid-type-fest.d.ts" />

// R463: the fullscreen viewer toolbar rides behind the existing engine export
// so consumers already aliasing @weknora/views/chat/mermaid get it without
// extra path wiring (vite/tsconfig pin this entry per-file).
export { attachMermaidViewerToolbar, type MermaidViewerToolbarLabels, type MermaidViewerToolbarHandle } from './mermaid-viewer.ts';

export const MERMAID_RENDER_CONFIG = Object.freeze({
  startOnLoad: false,
  securityLevel: 'strict' as const,
  theme: 'base' as const,
  flowchart: Object.freeze({ useMaxWidth: true, htmlLabels: false }),
});

export interface MermaidEngine {
  initialize(config: typeof MERMAID_RENDER_CONFIG): void;
  render(id: string, source: string): Promise<{ svg: string }>;
}

export function mermaidSource(block: Pick<HTMLElement, 'querySelector'>): string {
  return block.querySelector('code')?.textContent?.trim() ?? '';
}

function diagramId(index: number, prefix: string): string {
  return `${prefix}-${index + 1}`;
}

/**
 * Hydrate only the renderer's marked Mermaid nodes. The caller supplies the
 * dynamic engine and DOM sanitizer so this module remains harmless in SSR and
 * keeps all SVG injection behind the DOM boundary.
 */
export async function hydrateMermaidBlocks(
  root: HTMLElement,
  engine: MermaidEngine,
  sanitize: (svg: string) => string,
  prefix = 'wk-chat-mermaid',
): Promise<void> {
  const blocks = [...root.querySelectorAll<HTMLElement>('[data-markdown-diagram="mermaid"]')];
  if (blocks.length === 0) return;
  engine.initialize(MERMAID_RENDER_CONFIG);
  for (const [index, block] of blocks.entries()) {
    const source = mermaidSource(block);
    if (!source || block.dataset.mermaidError === 'true') continue;
    try {
      const result = await engine.render(diagramId(index, prefix), source);
      const svg = sanitize(result.svg);
      if (!svg) throw new Error('Mermaid sanitizer returned an empty SVG');
      const wrapper = document.createElement('div');
      wrapper.className = 'wk-chat-mermaid';
      wrapper.setAttribute('role', 'img');
      wrapper.setAttribute('aria-label', 'Mermaid diagram');
      wrapper.innerHTML = svg;
      block.replaceWith(wrapper);
    } catch {
      // Preserve the escaped source block as a readable fallback.
      block.dataset.mermaidError = 'true';
    }
  }
}

/** Load the optional browser-only renderer from this shared package boundary. */
export async function hydrateMermaidBlocksWithBrowserDefaults(root: HTMLElement, prefix?: string): Promise<void> {
  if (typeof window === 'undefined') return;
  const [mermaidModule, domPurifyModule] = await Promise.all([import('mermaid'), import('dompurify')]);
  const purifier = domPurifyModule.default(window);
  await hydrateMermaidBlocks(
    root,
    mermaidModule.default as unknown as MermaidEngine,
    (svg) => purifier.sanitize(svg, { USE_PROFILES: { svg: true, svgFilters: true } }),
    prefix,
  );
}
