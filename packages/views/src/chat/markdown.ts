import { marked, Renderer, type Tokens } from 'marked';
import { renderToString as renderKatex } from 'katex';

const CITATION_RE = /<(kb|web|wiki)\b([^>]*)\/>/gi;
const ATTRIBUTE_RE = /([\w-]+)\s*=\s*(["'])(.*?)\2/g;
const UNSAFE_SCHEME_RE = /^(?:javascript|vbscript|data):/i;
const SAFE_IMAGE_SCHEME_RE = /^(?:https?|resource|storage|local|minio|s3|cos|tos|oss|obs|ks3):\/\//i;

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function safeUrl(value: string, image = false): string | null {
  const candidate = value.trim();
  if (!candidate || UNSAFE_SCHEME_RE.test(candidate)) return null;
  if (candidate.startsWith('/') && !candidate.startsWith('//')) return candidate;
  if (/^mailto:/i.test(candidate)) return image ? null : candidate;
  if (image && !SAFE_IMAGE_SCHEME_RE.test(candidate)) return null;
  if (!image && !/^https?:\/\//i.test(candidate)) return null;
  return candidate;
}

function attributes(value: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const match of value.matchAll(ATTRIBUTE_RE)) result[match[1]!.toLowerCase()] = match[3]!;
  return result;
}

function citationMarkup(kind: string, rawAttributes: string): string {
  const attrs = attributes(rawAttributes);
  const id = attrs.chunk_id || attrs.id || attrs.url;
  if (!id) return '';
  const label = attrs.doc || attrs.title || attrs.url || `${kind} reference`;
  return `<button type="button" class="wk-chat-citation" data-citation-id="${escapeHtml(id)}" aria-label="引用 ${escapeHtml(label)}">${escapeHtml(label)}</button>`;
}

function preserveCitations(markdown: string): { source: string; replacements: Map<string, string> } {
  const replacements = new Map<string, string>();
  let index = 0;
  const source = markdown.replace(CITATION_RE, (match, kind: string, rawAttributes: string) => {
    const token = `WEKNORA_CITATION_${index++}_TOKEN`;
    const replacement = citationMarkup(kind, rawAttributes);
    if (replacement) replacements.set(token, replacement);
    return replacement ? token : '';
  });
  return { source, replacements };
}

function restoreCitations(html: string, replacements: Map<string, string>): string {
  if (!replacements.size) return html;
  const parts = html.split(/(<code\b[^>]*>[\s\S]*?<\/code>)/gi);
  for (let index = 0; index < parts.length; index += 2) {
    for (const [token, replacement] of replacements) parts[index] = parts[index]!.split(token).join(replacement);
  }
  return parts.join('');
}

function restoreMath(html: string): string {
  const parts = html.split(/(<pre\b[\s\S]*?<\/pre>|<code\b[^>]*>[\s\S]*?<\/code>)/gi);
  for (let index = 0; index < parts.length; index += 2) {
    parts[index] = parts[index]!
      .replace(/\$\$([\s\S]+?)\$\$/g, (_match, expression: string) => renderMath(expression, true))
      .replace(/\$([^$\n]+)\$/g, (_match, expression: string) => renderMath(expression, false));
  }
  return parts.join('');
}

function renderMath(expression: string, displayMode: boolean): string {
  try {
    return renderKatex(expression, { displayMode, throwOnError: false, trust: false });
  } catch {
    const className = displayMode ? 'math-block' : 'math-inline';
    return `<span class="${className}" role="math" data-format="tex">${escapeHtml(expression)}</span>`;
  }
}

function createRenderer(invalidImageLabel: string): Renderer {
  const renderer = new Renderer();
  renderer.html = ({ text }: Tokens.HTML | Tokens.Tag) => escapeHtml(text);
  renderer.link = ({ href, title, tokens }: Tokens.Link) => {
    const url = safeUrl(href);
    const label = renderer.parser.parseInline(tokens);
    if (!url) return label;
    const titleAttribute = title ? ` title="${escapeHtml(title)}"` : '';
    return `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer"${titleAttribute}>${label}</a>`;
  };
  renderer.image = ({ href, title, text }: Tokens.Image) => {
    const url = safeUrl(href, true);
    // Vue botmsg.vue invalidImageHtml: images failing the URL validation render
    // the localized `error.invalidImageLink` placeholder paragraph instead of
    // being dropped or replaced by the alt text.
    if (!url) return `<p>${escapeHtml(invalidImageLabel)}</p>`;
    const titleAttribute = title ? ` title="${escapeHtml(title)}"` : '';
    return `<img src="${escapeHtml(url)}" alt="${escapeHtml(text || '')}" loading="lazy"${titleAttribute}>`;
  };
  renderer.code = ({ text, lang }: Tokens.Code) => {
    const language = lang?.trim().toLowerCase() || '';
    const classAttribute = language ? ` class="language-${escapeHtml(language)}"` : '';
    const diagramAttribute = language === 'mermaid' ? ' data-markdown-diagram="mermaid"' : '';
    return `<pre${diagramAttribute}><code${classAttribute}>${escapeHtml(text)}\n</code></pre>`;
  };
  return renderer;
}

/** error.invalidImageLink zh-CN — the deployment-default placeholder (Vue locale default). */
export const INVALID_IMAGE_LINK_PLACEHOLDER = '无效的图片链接';

export interface ChatMarkdownOptions {
  /** Localized error.invalidImageLink placeholder for invalid image destinations. */
  invalidImageLabel?: string;
}

/**
 * Render assistant Markdown as a small, DOM-safe HTML contract for all web
 * clients. Raw HTML is escaped, links are allow-listed, and citation tags are
 * converted to buttons instead of being interpreted by the browser.
 */
export function renderChatMarkdown(markdown: unknown, options: ChatMarkdownOptions = {}): string {
  const raw = typeof markdown === 'string' ? markdown : String(markdown ?? '');
  if (!raw.trim()) return '';
  const { source, replacements } = preserveCitations(raw);
  const html = marked.parse(source, {
    renderer: createRenderer(options.invalidImageLabel?.trim() || INVALID_IMAGE_LINK_PLACEHOLDER),
    gfm: true,
    breaks: true,
    async: false,
  }) as string;
  return restoreMath(restoreCitations(html, replacements));
}
