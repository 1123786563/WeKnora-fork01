/**
 * Embed chat answer markdown rendering — React port of the Vue embed chain:
 *
 *   frontend/src/views/embed/EmbedBotMessage.vue `renderedHTML`:
 *     renderChatMarkdown(text, { renderer, escapeMarkdown, sanitizeHtml })
 *     -> frontend/src/utils/chatMarkdownRenderer.ts, whose citation order is:
 *
 *     1. stripIncompleteCitationTag  (hide a trailing partial <kb/<web tag)
 *     2. joinCitationTagsToPreviousLine (keep citations inline, not new blocks)
 *     3. extractCitationHtmlPlaceholders (tags -> pill HTML -> placeholders)
 *     4. marked.parse(..., { breaks: true, gfm: true })
 *     5. restoreCitationHtmlPlaceholders (raw pill HTML re-injected)
 *     6. collapseStandaloneCitationParagraphs
 *     7. sanitizeMarkdownHTML (markdownDomPurifyConfig + security hooks)
 *
 * So the answer is: citations are converted BEFORE marked and re-injected
 * AFTER it — markdown never sees the tags, and the pills never get escaped.
 * The Vue typewriter-emphasis guards guard streaming artifacts of the Vue
 * typewriter and are omitted (the React face renders whole messages). Math
 * uses the same marked-katex-extension call as
 * chatMarkdownRenderer.configureMarkedForChatMarkdown, on a dedicated Marked
 * instance so the wiki face's global marked configuration stays untouched;
 * katex/dist/katex.min.css is imported by EmbedEntryPage.tsx like
 * EmbedBotMessage.vue does. The DOMPurify surface below is the verbatim Vue
 * chat config (its math categories cover the KaTeX output).
 */
import { Marked } from 'marked';
import markedKatex from 'marked-katex-extension';
import DOMPurify from 'dompurify';
import { domPurifyAllowedUriRegexp, escapeHTML } from '../wiki/markdown.ts';
import { resolveCitationChunkId } from './chat-data.ts';

// Vue chatMarkdownRenderer.configureMarkedForChatMarkdown:
//   marked.use({ breaks: true, gfm: true })
//   marked.use(markedKatex({ throwOnError: false, nonStandard: true }))
const embedMarked = new Marked(markedKatex({ throwOnError: false, nonStandard: true }));

// ─── citation tag helpers (frontend/src/utils/citationMarkdown.ts) ───

/** `<kb/>` / `<web/>` tags, self-closing or unclosed. */
const KB_WEB_TAG_RE = /<(?:kb|web)\b[^>]*?\s*\/?>/g;
const KB_TAG_ATTR_RE = /<kb\b([^>]*?)\s*\/?>/g;
const WEB_TAG_ATTR_RE = /<web\b([^>]*?)\s*\/?>/g;
const ATTRIBUTE_REGEX = /([\w-]+)\s*=\s*"([^"]*)"/g;

function parseTagAttributes(attrString: string): Record<string, string> {
  const attributes: Record<string, string> = {};
  if (!attrString) return attributes;
  ATTRIBUTE_REGEX.lastIndex = 0;
  let match: RegExpExecArray | null;
  while ((match = ATTRIBUTE_REGEX.exec(attrString)) !== null) attributes[match[1]] = match[2];
  return attributes;
}

/** Vue citationMarkdown stripIncompleteCitationTag. */
export function stripIncompleteCitationTag(content: string): string {
  if (!content) return content;
  const start = content.lastIndexOf('<');
  if (start < 0) return content;
  const tail = content.slice(start);
  if (tail.includes('>')) return content;
  const isCitationPrefix = tail === '<'
    || /^<k(?:b(?:\s[\s\S]*)?)?$/i.test(tail)
    || /^<w(?:e(?:b(?:\s[\s\S]*)?)?)?$/i.test(tail);
  return isCitationPrefix ? content.slice(0, start) : content;
}

/** Vue citationMarkdown truncateMiddle (13 chars, middle ellipsis). */
function truncateMiddle(text: string, maxLength = 13): string {
  if (!text) return '';
  if (text.length <= maxLength) return text;
  const half = Math.floor((maxLength - 3) / 2);
  const start = text.slice(0, half + ((maxLength - 3) % 2));
  const end = text.slice(-half);
  return `${start}...${end}`;
}

/** Vue citationMarkdown citationDomain: keep host, keep the last two labels. */
function citationDomain(url: string): string {
  try {
    const host = new URL(url).hostname || '';
    const parts = host.split('.');
    return parts.length >= 2 ? parts.slice(-2).join('.') : host || url;
  } catch {
    return url;
  }
}

type CitationKnowledgeRef = {
  id?: string;
  knowledge_title?: string;
  knowledge_filename?: string;
  chunk_index?: number;
  chunk_type?: string;
};

/** Vue citationMarkdown preprocessCitationTags (web pill + kb pill + wiki link). */
export function preprocessCitationTags(contentStr: string, refs?: CitationKnowledgeRef[] | null): string {
  if (!contentStr.trim()) return '';

  return contentStr
    .replace(WEB_TAG_ATTR_RE, (_m, attrString: string) => {
      const attrs = parseTagAttributes(attrString);
      const url = attrs.url || '';
      const title = attrs.title || '';
      if (!url) return '';
      const safeTitle = escapeHTML(title);
      const safeUrl = escapeHTML(url);
      return `<a class="citation citation-web" data-url="${safeUrl}" href="${safeUrl}" target="_blank" rel="noopener noreferrer"><span class="citation-icon citation-icon--web" aria-hidden="true"></span><span class="citation-domain">${escapeHTML(citationDomain(url))}</span><span class="citation-tip"><span class="tip-title">${safeTitle}</span><span class="tip-url">${safeUrl}</span></span></a>`;
    })
    .replace(KB_TAG_ATTR_RE, (_m, attrString: string) => {
      const attrs = parseTagAttributes(attrString);
      const doc = attrs.doc || '';
      const chunkId = resolveCitationChunkId(
        attrs.chunk_id || attrs.chunkId || '',
        { doc },
        (refs || []) as Record<string, unknown>[],
      );
      if (!doc || !chunkId) return '';
      const safeKbId = escapeHTML(attrs.kb_id || attrs.kbId || '');
      return `<span class="citation citation-kb" data-kb-id="${safeKbId}" data-chunk-id="${escapeHTML(chunkId)}" data-doc="${escapeHTML(doc)}" role="button" tabindex="0"><span class="citation-icon citation-icon--book" aria-hidden="true"></span><span class="citation-text">${escapeHTML(truncateMiddle(doc))}</span><span class="citation-tip"><span class="tip-loading">…</span></span></span>`;
    })
    .replace(/\[\[([^\]]+)\]\]/g, (match, inner: string) => {
      const pipeIdx = inner.indexOf('|');
      const slug = pipeIdx > 0 ? inner.substring(0, pipeIdx).trim() : inner.trim();
      if (!slug) return match;
      let display = slug;
      if (pipeIdx > 0) {
        display = inner.substring(pipeIdx + 1).trim();
      } else {
        const parts = slug.split('/');
        display = parts.length > 1 ? parts.slice(1).join('/') : slug;
      }
      return `<a href="#" class="wiki-content-link citation-wiki" data-slug="${escapeHTML(slug)}">${escapeHTML(display)}</a>`;
    });
}

const HTML_PLACEHOLDER_RE = /@@WEKNORA_HTML_PLACEHOLDER_(\d+)@@/g;

/** Protect citation pill HTML from the markdown parser; restore after marked. */
function extractCitationHtmlPlaceholders(
  contentStr: string,
  refs?: CitationKnowledgeRef[] | null,
): { content: string; htmlSnippets: string[] } {
  const htmlSnippets: string[] = [];
  const storeHtml = (html: string): string => {
    const idx = htmlSnippets.length;
    htmlSnippets.push(html);
    return `@@WEKNORA_HTML_PLACEHOLDER_${idx}@@`;
  };
  const content = contentStr
    .replace(KB_WEB_TAG_RE, (match) => storeHtml(preprocessCitationTags(match, refs)))
    .replace(/\[\[([^\]]+)\]\]/g, (match) => storeHtml(preprocessCitationTags(match, refs)));
  return { content, htmlSnippets };
}

function restoreCitationHtmlPlaceholders(html: string, htmlSnippets: string[]): string {
  if (!htmlSnippets.length) return html;
  return html.replace(HTML_PLACEHOLDER_RE, (_match, idx: string) => htmlSnippets[Number(idx)] || '');
}

/** Opening/closing fence for GFM fenced code blocks (up to 3 spaces indent). */
const FENCED_CODE_DELIMITER_RE = /^ {0,3}(`{3,}|~{3,})(\s*\S.*)?\s*$/;

function isFencedCodeDelimiterLine(line: string): boolean {
  return FENCED_CODE_DELIMITER_RE.test(line);
}

/** Vue citationMarkdown joinCitationTagsToPreviousLine. */
export function joinCitationTagsToPreviousLine(content: string): string {
  if (!content) return content;

  let result = content;

  // Newlines between consecutive citation tags
  let prev = '';
  while (result !== prev) {
    prev = result;
    result = result.replace(
      /(<(?:kb|web)\b[^>]*?\s*\/?>)\s*\n+\s*(<(?:kb|web)\b)/gi,
      '$1 $2',
    );
  }

  // Blank lines before citations: join to the previous content. Fenced-code
  // delimiters are the only exception because ``` / ~~~ must stay on their own line.
  result = result.replace(/\n[ \t]*\n+([ \t]*<(?:kb|web)\b)/gi, (match, kbStart: string, offset: number, full: string) => {
    const before = full.slice(0, offset);
    const lastLine = before.split('\n').filter((line: string) => line.trim()).pop() || '';
    if (isFencedCodeDelimiterLine(lastLine)) {
      return `\n\n${kbStart}`;
    }
    return ` ${kbStart.trimStart()}`;
  });

  // Single newline before citation when it follows text or another citation (not after a blank line)
  result = result.replace(
    /(?<!\n)(<(?:kb|web)\b[^>]*?\s*\/?>|[ \t]*\S[^\n]*?)\n([ \t]*<(?:kb|web)\b)/g,
    (match: string, beforePart: string, kbStart: string, offset: number, full: string) => {
      const lineStart = full.lastIndexOf('\n', offset - 1) + 1;
      const fullPrevLine = full.slice(lineStart, offset + beforePart.length);
      if (isFencedCodeDelimiterLine(fullPrevLine)) {
        return match;
      }
      return `${beforePart} ${kbStart.trimStart()}`;
    },
  );

  return result;
}

const CITATION_HTML_FRAGMENT =
  '(?:<span class="citation\\b[^]*?</span>|<a class="citation\\b[^]*?</a>)';

/** Vue citationMarkdown collapseStandaloneCitationParagraphs. */
export function collapseStandaloneCitationParagraphs(html: string): string {
  if (!html || !html.includes('citation')) return html;

  const mergePattern = new RegExp(
    `(<\\/(?:p|li)>)\\s*(?:<p>\\s*<\\/p>\\s*)*<p>\\s*(${CITATION_HTML_FRAGMENT})\\s*<\\/p>`,
    'g',
  );

  let result = html;
  let prev = '';
  while (result !== prev) {
    prev = result;
    result = result.replace(mergePattern, (_match, closeTag: string, citation: string) => {
      return ` ${citation}${closeTag}`;
    });
  }

  return result;
}

// ─── sanitize (frontend/src/utils/markdownDomPurify.ts, verbatim chat config) ───

const domPurifyForbidTags = ['script', 'style', 'object', 'embed', 'form', 'input'];
const domPurifyForbidAttr = ['onerror', 'onload', 'onclick', 'onmouseover', 'onfocus', 'onblur'];

/** Vue markdownDomPurifyConfig — the config behind Vue's sanitizeMarkdownHTML. */
const markdownDomPurifyConfig = {
  ALLOWED_TAGS: [
    'p', 'br', 'strong', 'em', 'u', 'code', 'pre', 'ul', 'ol', 'li', 'blockquote',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'a', 'span', 'table', 'thead', 'tbody',
    'tr', 'th', 'td', 'img', 'figure', 'figcaption', 'div',
    'svg', 'g', 'path', 'rect', 'circle', 'ellipse', 'line', 'polygon',
    'polyline', 'text', 'tspan', 'defs', 'marker', 'filter', 'use',
    'clippath', 'lineargradient', 'radialgradient', 'stop', 'pattern',
    'image', 'foreignobject', 'desc', 'title', 'switch', 'symbol', 'mask',
    'math', 'annotation', 'semantics', 'mo', 'mi', 'mn', 'msup', 'mrow', 'mfrac', 'msqrt', 'mroot', 'mstyle',
    'button',
  ],
  ALLOWED_ATTR: [
    'href', 'title', 'target', 'rel', 'data-tooltip', 'data-url', 'data-kb-id',
    'data-chunk-id', 'data-doc', 'data-slug', 'class', 'role', 'tabindex', 'src', 'alt', 'data-protected-src', 'data-img-loading',
    'data-artifact-index', 'data-protected-resource', 'download',
    'width', 'height', 'style', 'id', 'type', 'aria-label', 'data-mermaid', 'disabled',
    'd', 'fill', 'stroke', 'stroke-width', 'stroke-linecap', 'stroke-linejoin',
    'stroke-dasharray', 'stroke-dashoffset', 'stroke-miterlimit', 'stroke-opacity',
    'fill-opacity', 'opacity', 'transform', 'viewbox', 'preserveaspectratio',
    'x', 'y', 'x1', 'y1', 'x2', 'y2', 'cx', 'cy', 'rx', 'ry', 'r',
    'dx', 'dy', 'text-anchor', 'dominant-baseline', 'font-family', 'font-size',
    'font-weight', 'font-style', 'letter-spacing', 'word-spacing',
    'marker-start', 'marker-mid', 'marker-end', 'markerunits', 'markerwidth',
    'markerheight', 'refx', 'refy', 'orient', 'points', 'offset',
    'gradientunits', 'gradienttransform', 'spreadmethod', 'stop-color', 'stop-opacity',
    'patternunits', 'patterntransform', 'clippathunits', 'maskunits',
    'filterunits', 'primitiveunits', 'xmlns', 'xmlns:xlink', 'xlink:href',
    'version', 'baseprofile', 'enable-background', 'overflow', 'visibility',
    'display', 'pointer-events', 'cursor', 'data-emit', 'direction',
    'mathvariant', 'encoding', 'aria-hidden',
  ],
  USE_PROFILES: { html: true, svg: true, mathMl: true },
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

/** Vue markdownDomPurifySecurityHooks (strip event attrs, harden links).
 * Vue's hook additionally calls element.remove() on <script>; DOMPurify's own
 * FORBID_TAGS already removes script elements with their content, and removing
 * a node inside beforeSanitizeElements makes DOMPurify abort ("refusing to
 * sanitize in place" — observed under jsdom), so the explicit removal is
 * delegated to FORBID_TAGS with identical output. */
const markdownDomPurifySecurityHooks = {
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
    if (element.tagName === 'A' && element.getAttribute('href')) {
      const className = element.getAttribute('class') || '';
      if (/\bprotected-resource-card\b/.test(className) || element.hasAttribute('download')) {
        element.removeAttribute('target');
        return;
      }
      element.setAttribute('rel', 'noopener noreferrer');
      element.setAttribute('target', '_blank');
    }
  },
};

let hooksInstalled = false;
function ensureDomPurifyHooks() {
  if (hooksInstalled) return;
  DOMPurify.addHook('beforeSanitizeElements', markdownDomPurifySecurityHooks.beforeSanitizeElements);
  DOMPurify.addHook('afterSanitizeElements', markdownDomPurifySecurityHooks.afterSanitizeElements);
  hooksInstalled = true;
}

/** Vue EmbedBotMessage: sanitizeMarkdownHTML(markdownDomPurifyConfig). */
export function sanitizeEmbedMarkdownHTML(html: string): string {
  if (!html || typeof html !== 'string') return '';
  try {
    ensureDomPurifyHooks();
    return DOMPurify.sanitize(html, markdownDomPurifyConfig as unknown as import('dompurify').Config);
  } catch {
    return escapeHTML(html);
  }
}

/**
 * The full Vue embed order (renderChatMarkdown): strip a trailing partial
 * citation tag, join tags to their line, shield pill HTML behind placeholders,
 * run marked with the Vue chat options, restore + collapse the pills, then
 * sanitize with the Vue chat DOMPurify config.
 */
export function renderEmbedChatMarkdown(content: string, refs?: CitationKnowledgeRef[] | null): string {
  const rawText = typeof content === 'string' ? content : String(content || '');
  if (!rawText.trim()) return '';

  const citationSafeText = stripIncompleteCitationTag(rawText);
  const inlineTags = joinCitationTagsToPreviousLine(citationSafeText);
  const { content: markdownWithPlaceholders, htmlSnippets } =
    extractCitationHtmlPlaceholders(inlineTags, refs);
  const html = embedMarked.parse(markdownWithPlaceholders, {
    breaks: true,
    gfm: true,
    async: false,
  }) as string;
  const restoredHtml = restoreCitationHtmlPlaceholders(html, htmlSnippets);
  const citationHtml = collapseStandaloneCitationParagraphs(restoredHtml);
  return sanitizeEmbedMarkdownHTML(citationHtml);
}
