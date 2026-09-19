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

function createRenderer(invalidImageLabel: string, imageFailedLabel: string, imagePreviewLabel: string): Renderer {
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
    return chatImageMarkup(url, escapeHtml(text || ''), titleAttribute, imageFailedLabel, imagePreviewLabel);
  };
  renderer.code = ({ text, lang }: Tokens.Code) => {
    const language = lang?.trim().toLowerCase() || '';
    const classAttribute = language ? ` class="language-${escapeHtml(language)}"` : '';
    const diagramAttribute = language === 'mermaid' ? ' data-markdown-diagram="mermaid"' : '';
    return `<pre${diagramAttribute}><code${classAttribute}>${escapeHtml(text)}\n</code></pre>`;
  };
  return renderer;
}

/**
 * Vue renders content images through TDesign's t-image: a broken/failed image
 * shows the 图片无法显示 error placeholder with a 预览 preview trigger instead of
 * the browser's default broken-image glyph. The wrapper carries both states —
 * the <img> plus a hidden fallback that installChatImageErrorWatcher reveals
 * when the load fails.
 */
function chatImageMarkup(src: string, escapedAlt: string, titleAttribute: string, imageFailedLabel: string, imagePreviewLabel: string): string {
  const fallbackStyle = 'display:none;align-items:center;gap:8px;padding:12px;background:rgba(0,0,0,0.03);border-radius:6px;font-size:13px;color:rgba(0,0,0,0.6)';
  const errorIcon = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M6 15l4-4 3 3 2-2 3 3" /><circle cx="9" cy="9" r="1" /></svg>';
  return `<span class="wk-chat-image" data-wk-chat-image>`
    + `<img src="${src}" alt="${escapedAlt}" loading="lazy" data-wk-chat-image-img${titleAttribute}>`
    + `<span class="wk-chat-image-error" hidden style="${fallbackStyle}">`
    + errorIcon
    + `<span class="wk-chat-image-error-text">${escapeHtml(imageFailedLabel)}</span>`
    + `<span class="wk-chat-image-preview" role="button" tabindex="0" data-wk-chat-image-preview style="cursor:pointer;text-decoration:underline">${escapeHtml(imagePreviewLabel)}</span>`
    + `</span></span>`;
}

/** error.invalidImageLink zh-CN — the deployment-default placeholder (Vue locale default). */
export const INVALID_IMAGE_LINK_PLACEHOLDER = '无效的图片链接';

/** Vue t-image error placeholder + viewer trigger labels (TDesign zh defaults). */
export const CHAT_IMAGE_FAILED_LABEL = '图片无法显示';
export const CHAT_IMAGE_PREVIEW_LABEL = '预览';

const RAW_IMG_RE = /<img\b[^>]*>/gi;

/**
 * Whitelist pass for RAW <img> tags in assistant content: safe-src images are
 * restored after sanitization with the same error-fallback wrapper as markdown
 * images; unsafe destinations degrade to the invalid-image placeholder. Fenced
 * code is left untouched.
 */
function preserveRawImages(
  markdown: string,
  invalidImageLabel: string,
  imageFailedLabel: string,
  imagePreviewLabel: string,
): { source: string; replacements: Map<string, string> } {
  const replacements = new Map<string, string>();
  let index = 0;
  const source = markdown.replace(RAW_IMG_RE, (tag: string, ...offsets: unknown[]) => {
    const offset = offsets[0] as number;
    const before = markdown.slice(0, offset);
    const fences = (before.match(/```/g) || []).length;
    if (fences % 2 === 1) return tag; // inside a fenced code block
    const attrs = attributes(tag.slice(4, -1));
    const url = safeUrl(attrs.src ?? '', true);
    const token = `WEKNORA_RAWIMG_${index++}_TOKEN`;
    if (!url) {
      replacements.set(token, `<p>${escapeHtml(invalidImageLabel)}</p>`);
      return token;
    }
    replacements.set(token, chatImageMarkup(url, escapeHtml(attrs.alt ?? ''), attrs.title ? ` title="${escapeHtml(attrs.title)}"` : '', imageFailedLabel, imagePreviewLabel));
    return token;
  });
  return { source, replacements };
}

/**
 * Installs (once) the capture-phase listeners that reveal the 图片无法显示
 * fallback when a content image fails to load and open the image in a new tab
 * from its 预览 trigger — Vue's t-image + image-viewer behavior.
 */
export function installChatImageErrorWatcher(): void {
  if (chatImageWatcherInstalled || typeof window === 'undefined' || typeof window.addEventListener !== 'function') return;
  chatImageWatcherInstalled = true;
  window.addEventListener('error', (event) => {
    const target = event.target as HTMLImageElement | null;
    if (target && target.tagName === 'IMG' && target.hasAttribute('data-wk-chat-image-img')) revealChatImageFailure(target);
  }, true);
  window.addEventListener('click', (event) => {
    const target = event.target as HTMLElement | null;
    const trigger = target && typeof target.closest === 'function' ? target.closest('[data-wk-chat-image-preview]') : null;
    if (!trigger) return;
    const img = trigger.closest('[data-wk-chat-image]')?.querySelector('img[data-wk-chat-image-img]');
    const src = img?.getAttribute('src');
    if (src) window.open(src, '_blank', 'noopener');
  });
}
let chatImageWatcherInstalled = false;

/** Reveals the error fallback for an image whose load failed. */
export function revealChatImageFailure(img: HTMLImageElement): void {
  img.style.display = 'none';
  const wrap = img.closest('[data-wk-chat-image]');
  const fallback = wrap ? wrap.querySelector<HTMLElement>('.wk-chat-image-error') : null;
  if (fallback) {
    fallback.removeAttribute('hidden');
    fallback.style.display = 'flex';
  }
}

export interface ChatMarkdownOptions {
  /** Localized error.invalidImageLink placeholder for invalid image destinations. */
  invalidImageLabel?: string;
  /** t-image error placeholder label (Vue TDesign zh default 图片无法显示). */
  imageFailedLabel?: string;
  /** Image viewer trigger label (Vue TDesign zh default 预览). */
  imagePreviewLabel?: string;
}

/**
 * Render assistant Markdown as a small, DOM-safe HTML contract for all web
 * clients. Raw HTML is escaped, links are allow-listed, and citation tags are
 * converted to buttons instead of being interpreted by the browser. Raw <img>
 * tags are the one whitelisted raw-HTML element: safe-src images render with
 * the TDesign-style error fallback wrapper, unsafe ones degrade to the
 * invalid-image placeholder.
 */
export function renderChatMarkdown(markdown: unknown, options: ChatMarkdownOptions = {}): string {
  const raw = typeof markdown === 'string' ? markdown : String(markdown ?? '');
  if (!raw.trim()) return '';
  const invalidImageLabel = options.invalidImageLabel?.trim() || INVALID_IMAGE_LINK_PLACEHOLDER;
  const imageFailedLabel = options.imageFailedLabel?.trim() || CHAT_IMAGE_FAILED_LABEL;
  const imagePreviewLabel = options.imagePreviewLabel?.trim() || CHAT_IMAGE_PREVIEW_LABEL;
  const rawImages = preserveRawImages(raw, invalidImageLabel, imageFailedLabel, imagePreviewLabel);
  const citations = preserveCitations(rawImages.source);
  const replacements = new Map([...rawImages.replacements, ...citations.replacements]);
  const html = marked.parse(citations.source, {
    renderer: createRenderer(invalidImageLabel, imageFailedLabel, imagePreviewLabel),
    gfm: true,
    breaks: true,
    async: false,
  }) as string;
  return restoreMath(restoreCitations(html, replacements));
}
