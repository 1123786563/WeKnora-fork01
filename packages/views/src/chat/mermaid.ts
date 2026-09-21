/// <reference path="./mermaid-type-fest.d.ts" />

// R463: the fullscreen viewer toolbar rides behind the existing engine export
// so consumers already aliasing @weknora/views/chat/mermaid get it without
// extra path wiring (vite/tsconfig pin this entry per-file).
export { attachMermaidViewerToolbar, type MermaidViewerToolbarLabels, type MermaidViewerToolbarHandle } from './mermaid-viewer.ts';

// Mirror of frontend/src/utils/mermaidShared.ts MERMAID_CONFIG: diagram
// geometry must match the Vue chat so rendered SVG sizes stay in parity.
export const MERMAID_RENDER_CONFIG = Object.freeze({
  startOnLoad: false,
  securityLevel: 'strict' as const,
  theme: 'base' as const,
  fontFamily: 'PingFang SC, Microsoft YaHei, sans-serif',
  flowchart: Object.freeze({ useMaxWidth: true, htmlLabels: true, curve: 'basis', padding: 16 }),
  sequence: Object.freeze({
    useMaxWidth: true,
    diagramMarginX: 12,
    diagramMarginY: 12,
    actorMargin: 56,
    width: 156,
    height: 68,
    boxMargin: 10,
  }),
  gantt: Object.freeze({ useMaxWidth: true, leftPadding: 80, gridLineStartPadding: 40, barHeight: 22, barGap: 6, topPadding: 56 }),
  er: Object.freeze({ useMaxWidth: true }),
  journey: Object.freeze({ useMaxWidth: true }),
});

// Mirror of MERMAID_LIGHT_THEME / MERMAID_DARK_THEME (mermaidShared.ts).
export const MERMAID_LIGHT_THEME_VARIABLES = Object.freeze({
  darkMode: false,
  background: '#ffffff',
  primaryColor: '#e2e8f0',
  primaryTextColor: '#334155',
  primaryBorderColor: '#94a3b8',
  secondaryColor: '#f1f5f9',
  secondaryTextColor: '#475569',
  secondaryBorderColor: '#cbd5e1',
  tertiaryColor: '#f8fafc',
  tertiaryTextColor: '#64748b',
  tertiaryBorderColor: '#e2e8f0',
  lineColor: '#94a3b8',
  textColor: '#334155',
  mainBkg: '#ffffff',
  nodeBorder: '#94a3b8',
  clusterBkg: '#f8fafc',
  clusterBorder: '#cbd5e1',
  titleColor: '#1e293b',
  edgeLabelBackground: '#ffffff',
  actorBorder: '#94a3b8',
  actorBkg: '#f1f5f9',
  actorTextColor: '#334155',
  actorLineColor: '#94a3b8',
  signalColor: '#94a3b8',
  labelBoxBkgColor: '#f1f5f9',
  labelBoxBorderColor: '#cbd5e1',
  labelTextColor: '#334155',
  loopTextColor: '#475569',
  noteBkgColor: '#f8fafc',
  noteTextColor: '#475569',
  noteBorderColor: '#cbd5e1',
  sectionBkgColor: '#f8fafc',
  altSectionBkgColor: '#ffffff',
  gridColor: '#e2e8f0',
  todayLineColor: '#64748b',
  taskBorderColor: '#94a3b8',
  taskBkgColor: '#e2e8f0',
  activeTaskBorderColor: '#64748b',
  activeTaskBkgColor: '#94a3b8',
  doneTaskBkgColor: '#cbd5e1',
  doneTaskBorderColor: '#94a3b8',
  critBkgColor: '#fecaca',
  critBorderColor: '#f87171',
  fontSize: '14px',
});

export const MERMAID_DARK_THEME_VARIABLES = Object.freeze({
  darkMode: true,
  background: '#1a1f28',
  primaryColor: '#475569',
  primaryTextColor: '#e2e8f0',
  primaryBorderColor: '#64748b',
  secondaryColor: '#334155',
  secondaryTextColor: '#cbd5e1',
  secondaryBorderColor: '#64748b',
  tertiaryColor: '#1e293b',
  tertiaryTextColor: '#94a3b8',
  tertiaryBorderColor: '#475569',
  lineColor: '#64748b',
  textColor: '#e2e8f0',
  mainBkg: '#1e293b',
  nodeBorder: '#64748b',
  clusterBkg: '#1a2332',
  clusterBorder: '#475569',
  titleColor: '#f1f5f9',
  edgeLabelBackground: '#1e293b',
  actorBorder: '#64748b',
  actorBkg: '#1e293b',
  actorTextColor: '#f1f5f9',
  actorLineColor: '#64748b',
  signalColor: '#64748b',
  labelBoxBkgColor: '#334155',
  labelBoxBorderColor: '#64748b',
  labelTextColor: '#e2e8f0',
  loopTextColor: '#cbd5e1',
  noteBkgColor: '#334155',
  noteTextColor: '#e2e8f0',
  noteBorderColor: '#64748b',
  sectionBkgColor: '#1a2332',
  altSectionBkgColor: '#1e293b',
  gridColor: '#334155',
  todayLineColor: '#94a3b8',
  taskBorderColor: '#64748b',
  taskBkgColor: '#475569',
  activeTaskBorderColor: '#94a3b8',
  activeTaskBkgColor: '#64748b',
  doneTaskBkgColor: '#334155',
  doneTaskBorderColor: '#64748b',
  critBkgColor: '#7f1d1d',
  critBorderColor: '#ef4444',
  fontSize: '14px',
});

/** resolveMermaidThemeVariables: theme-mode attribute on <html> decides. */
export function resolveMermaidThemeVariables(themeMode?: string | null): typeof MERMAID_LIGHT_THEME_VARIABLES {
  return themeMode === 'dark' ? MERMAID_DARK_THEME_VARIABLES : MERMAID_LIGHT_THEME_VARIABLES;
}

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
  engine.initialize({
    ...MERMAID_RENDER_CONFIG,
    themeVariables: resolveMermaidThemeVariables(
      typeof document !== 'undefined' && document.documentElement
        ? document.documentElement.getAttribute('theme-mode')
        : null,
    ),
  });
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
