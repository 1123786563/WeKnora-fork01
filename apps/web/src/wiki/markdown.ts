/**
 * Wiki markdown rendering — React port of the Vue WikiBrowser.vue chain:
 *
 *   frontend/src/views/knowledge/wiki/WikiBrowser.vue `renderMarkdown()`:
 *     1. preprocess [[slug|label]] wiki-links into wiki-content-link anchors
 *     2. marked.parse(html, { breaks: true, async: false })
 *     3. sanitizeMarkdownHTML(html) — DOMPurify with markdownDomPurifyConfig
 *
 * `marked` + `dompurify` are pinned to the same versions the Vue frontend
 * uses (marked ^17.0.5, dompurify ^3.4.11) so the produced DOM structure is
 * identical. No syntax highlighter is installed: the Vue chain does not
 * highlight code either.
 */
import { marked } from 'marked';
import DOMPurify from 'dompurify';

/** Vue WikiBrowser.vue: /\[\[([^\]]+)\]\]/g → wiki-content-link anchors. */
const WIKI_LINK_PATTERN = /\[\[([^\]]+)\]\]/g;

function escapeHTML(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;');
}

/**
 * Vue WikiBrowser.vue `slugDisplayName()`: prefer the loaded page's title,
 * otherwise strip the `type/` prefix and keep the rest of the slug.
 */
export function wikiSlugDisplayName(
  slug: string,
  pages: Array<{ slug: string; title: string }>,
): string {
  const page = pages.find((candidate) => candidate.slug === slug);
  if (page) return page.title;
  const parts = slug.split('/');
  return parts.length > 1 ? parts.slice(1).join('/') : slug;
}

/**
 * Vue WikiBrowser.vue `stripDuplicateLeadingTitle()`: drop a leading `# Title`
 * heading when it only repeats the page title (the reader header already
 * shows it).
 */
export function stripDuplicateLeadingTitle(content: string, title: string): string {
  if (!content || !title) return content;
  const lines = content.split('\n');
  let i = 0;
  while (i < lines.length && lines[i].trim() === '') i++;
  if (i >= lines.length) return content;
  const headingMatch = lines[i].match(/^#\s+(.+?)\s*$/);
  if (!headingMatch) return content;
  const heading = headingMatch[1].trim();
  const pageTitle = title.trim();
  if (heading !== pageTitle && heading.toLowerCase() !== pageTitle.toLowerCase()) return content;
  i++;
  while (i < lines.length && lines[i].trim() === '') i++;
  return lines.slice(i).join('\n');
}

// Verbatim port of frontend/src/utils/markdownDomPurify.ts (the config behind
// Vue's sanitizeMarkdownHTML) — wiki page bodies must survive/strip exactly
// the same way as the Vue reader.
const domPurifyAllowedUriRegexp =
  /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|cid|xmpp|blob):|data:image\/|(?:resource|storage|local|minio|cos|tos|s3|oss|ks3|obs):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i;

const domPurifyForbidTags = ['script', 'style', 'object', 'embed', 'form', 'input'];
const domPurifyForbidAttr = ['onerror', 'onload', 'onclick', 'onmouseover', 'onfocus', 'onblur'];

const markdownDomPurifyConfig = {
  ALLOWED_TAGS: [
    'p', 'br', 'strong', 'em', 'u', 'code', 'pre', 'ul', 'ol', 'li', 'blockquote',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'a', 'span', 'table', 'thead', 'tbody',
    'tr', 'th', 'td', 'img', 'figure', 'figcaption', 'div',
  ],
  ALLOWED_ATTR: [
    'href', 'title', 'target', 'rel', 'class', 'role', 'tabindex', 'src', 'alt',
    'data-slug', 'download', 'width', 'height', 'style', 'id', 'type', 'aria-label',
  ],
  USE_PROFILES: { html: true },
  ALLOWED_URI_REGEXP: domPurifyAllowedUriRegexp,
  FORBID_TAGS: [...domPurifyForbidTags],
  FORBID_ATTR: [...domPurifyForbidAttr],
  KEEP_CONTENT: true,
  RETURN_DOM: false,
  RETURN_DOM_FRAGMENT: false,
  RETURN_DOM_IMPORT: false,
  SANITIZE_DOM: true,
  SANITIZE_NAMED_PROPS: true,
  WHOLE_DOCUMENT: false,
} as const;

const markdownDomPurifyHooks = {
  beforeSanitizeElements: (currentNode: Node) => {
    if (!('tagName' in currentNode) || !('hasAttribute' in currentNode)) return;
    const element = currentNode as Element;
    domPurifyForbidAttr.forEach((attr) => {
      if (element.hasAttribute(attr)) element.removeAttribute(attr);
    });
  },
  afterSanitizeElements: (currentNode: Node) => {
    if (!('tagName' in currentNode) || !('getAttribute' in currentNode)) return;
    const element = currentNode as Element;
    if (element.tagName === 'A') {
      const href = element.getAttribute('href');
      if (href && href.startsWith('http')) {
        element.setAttribute('rel', 'noopener noreferrer');
        element.setAttribute('target', '_blank');
      }
    }
    if (element.tagName === 'IMG' && !element.getAttribute('alt')) {
      element.setAttribute('alt', '');
    }
  },
};

let hooksInstalled = false;
function ensureDomPurifyHooks() {
  if (hooksInstalled) return;
  DOMPurify.addHook('beforeSanitizeElements', markdownDomPurifyHooks.beforeSanitizeElements);
  DOMPurify.addHook('afterSanitizeElements', markdownDomPurifyHooks.afterSanitizeElements);
  hooksInstalled = true;
}

export type WikiMarkdownOptions = {
  /** Resolves a wiki slug to its display label (Vue `slugDisplayName`). */
  resolveSlugName: (slug: string) => string;
};

/** Vue WikiBrowser.vue `renderMarkdown()` — wiki-links → marked → sanitize. */
export function renderWikiMarkdown(content: string, options: WikiMarkdownOptions): string {
  if (!content) return '';
  const preprocessed = content.replace(WIKI_LINK_PATTERN, (_match: string, inner: string) => {
    const pipeIdx = inner.indexOf('|');
    const slug = pipeIdx > 0 ? inner.substring(0, pipeIdx).trim() : inner.trim();
    const display = pipeIdx > 0 ? inner.substring(pipeIdx + 1).trim() : options.resolveSlugName(slug);
    // Vue interpolates slug/display raw; escaping keeps the same DOM for real
    // slugs while making attribute break-out impossible before sanitize.
    return `<a href="#" class="wiki-content-link" data-slug="${escapeHTML(slug)}">${escapeHTML(display)}</a>`;
  });
  const html = marked.parse(preprocessed, { breaks: true, async: false }) as string;
  ensureDomPurifyHooks();
  return DOMPurify.sanitize(html, markdownDomPurifyConfig as unknown as import('dompurify').Config);
}

/**
 * Vue WikiBrowser.vue `handleContentClick()`: delegated clicks inside the
 * reader body — `.wiki-content-link` navigates to `data-slug`, everything
 * else is left alone. Uses `closest` so inline children of the anchor
 * (marked may wrap link labels in em/code) still navigate.
 */
export function handleWikiBodyClick(
  event: { target: unknown; preventDefault: () => void },
  navigate: (slug: string) => void,
): void {
  const target = event.target as Element | null;
  if (!target || typeof target.closest !== 'function') return;
  const link = target.closest('a.wiki-content-link');
  if (!link) return;
  event.preventDefault();
  const slug = link.getAttribute('data-slug');
  if (slug) navigate(slug);
}
