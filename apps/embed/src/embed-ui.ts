import { isLocale, messages as i18nMessages, type Locale } from '@weknora/i18n';

// Embed visitor helpers ported from the Vue embed implementation:
// - channel default_locale wins over the URL locale (EmbedPage.vue)
// - knowledge_references render as a flat source list (EmbedBotMessage.vue)
// - picked files become data-URI payloads (useEmbedChatSession.ts)

export interface EmbedSource { title: string; knowledgeId: string; chunkId: string }

export interface EmbedUploadCapabilities {
  allowFileUpload: boolean;
  allowImageUpload: boolean;
}

export interface EmbedFileIssue {
  file: File;
  reason: 'type' | 'size' | 'count';
}

export const EMBED_MAX_IMAGES = 5;
export const EMBED_MAX_ATTACHMENTS = 5;
export const EMBED_MAX_IMAGE_BYTES = 10 * 1024 * 1024;
export const EMBED_MAX_ATTACHMENT_BYTES = 20 * 1024 * 1024;
export const EMBED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'] as const;

export function isEmbedImage(file: File): boolean {
  return file.type.startsWith('image/');
}

export function partitionUploadFiles(files: readonly File[], imageCount = 0, attachmentCount = 0): {
  images: File[];
  attachments: File[];
  rejected: EmbedFileIssue[];
} {
  const images: File[] = [];
  const attachments: File[] = [];
  const rejected: EmbedFileIssue[] = [];
  for (const file of files) {
    if (isEmbedImage(file)) {
      if (!EMBED_IMAGE_TYPES.includes(file.type as (typeof EMBED_IMAGE_TYPES)[number])) rejected.push({ file, reason: 'type' });
      else if (file.size > EMBED_MAX_IMAGE_BYTES) rejected.push({ file, reason: 'size' });
      else if (imageCount + images.length >= EMBED_MAX_IMAGES) rejected.push({ file, reason: 'count' });
      else images.push(file);
    } else if (file.size > EMBED_MAX_ATTACHMENT_BYTES) {
      rejected.push({ file, reason: 'size' });
    } else if (attachmentCount + attachments.length >= EMBED_MAX_ATTACHMENTS) {
      rejected.push({ file, reason: 'count' });
    } else {
      attachments.push(file);
    }
  }
  return { images, attachments, rejected };
}

export function formatEmbedFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function formatEmbedConversationTimestamp(value: unknown, locale: string, now = new Date()): string {
  if (typeof value !== 'string' || !value.trim()) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (part: number) => String(part).padStart(2, '0');
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  const sameDay = date.getFullYear() === now.getFullYear() && date.getMonth() === now.getMonth() && date.getDate() === now.getDate();
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  const isYesterday = date.getFullYear() === yesterday.getFullYear() && date.getMonth() === yesterday.getMonth() && date.getDate() === yesterday.getDate();
  if (locale === 'zh-CN') {
    if (sameDay) return `今天 ${time}`;
    if (isYesterday) return `昨天 ${time}`;
    return date.getFullYear() === now.getFullYear()
      ? `${date.getMonth() + 1}月${date.getDate()}日 ${time}`
      : `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日 ${time}`;
  }
  if (locale === 'ja-JP') {
    if (sameDay) return `今日 ${time}`;
    if (isYesterday) return `昨日 ${time}`;
    return `${date.getFullYear()}/${date.getMonth() + 1}/${date.getDate()} ${time}`;
  }
  if (locale === 'ko-KR') {
    if (sameDay) return `오늘 ${time}`;
    if (isYesterday) return `어제 ${time}`;
    return `${date.getFullYear()}.${date.getMonth() + 1}.${date.getDate()} ${time}`;
  }
  if (locale === 'ru-RU') {
    if (sameDay) return `Сегодня ${time}`;
    if (isYesterday) return `Вчера ${time}`;
    return `${date.getDate()}.${date.getMonth() + 1}.${date.getFullYear()} ${time}`;
  }
  if (sameDay) return `Today ${time}`;
  if (isYesterday) return `Yesterday ${time}`;
  const month = new Intl.DateTimeFormat('en-US', { month: 'short' }).format(date);
  return date.getFullYear() === now.getFullYear()
    ? `${month} ${date.getDate()}, ${time}`
    : `${month} ${date.getDate()}, ${date.getFullYear()} ${time}`;
}

export function shouldShowEmbedTimestamp(messages: readonly { role?: unknown; created_at?: unknown }[], index: number, gapMs = 5 * 60 * 1000): boolean {
  const current = messages[index];
  if (!current || typeof current.created_at !== 'string' || Number.isNaN(new Date(current.created_at).getTime())) return false;
  const previous = messages[index - 1];
  if (!previous) return true;
  if (current.role === 'assistant' && previous.role === 'user') return false;
  if (typeof previous.created_at !== 'string' || Number.isNaN(new Date(previous.created_at).getTime())) return true;
  const currentDate = new Date(current.created_at);
  const previousDate = new Date(previous.created_at);
  return currentDate.toDateString() !== previousDate.toDateString() || currentDate.getTime() - previousDate.getTime() >= gapMs;
}

const uploadLabels: Record<string, { file: string; image: string }> = {
  'zh-CN': { file: '上传附件', image: '上传图片' },
  'en-US': { file: 'Upload file', image: 'Upload image' },
  'ja-JP': { file: '添付ファイルをアップロード', image: '画像をアップロード' },
  'ko-KR': { file: '파일 업로드', image: '이미지 업로드' },
  'ru-RU': { file: 'Загрузить файл', image: 'Загрузить изображение' },
};

export function embedUploadLabel(locale: string, kind: 'file' | 'image'): string {
  return uploadLabels[locale]?.[kind] ?? uploadLabels['en-US']![kind];
}

export function embedAssistantLabel(locale: string): string {
  return ({
    'zh-CN': 'AI 助手',
    'en-US': 'AI Assistant',
    'ja-JP': 'AIアシスタント',
    'ko-KR': 'AI 어시스턴트',
    'ru-RU': 'ИИ-ассистент',
  } as Record<string, string>)[locale] ?? 'AI Assistant';
}

export function embedErrorPrefix(locale: string): string {
  return ({ 'zh-CN': '错误：', 'en-US': 'Error: ', 'ja-JP': 'エラー：', 'ko-KR': '오류: ', 'ru-RU': 'Ошибка: ' } as Record<string, string>)[locale] ?? 'Error: ';
}

export function embedMessageError(locale: string, error: string): string {
  const message = error.trim();
  return message ? embedErrorPrefix(locale) + message : '';
}

export function resolveEmbedLocale(config: Record<string, unknown> | undefined | null, urlLocale: string): string {
  const fromConfig = typeof config?.default_locale === 'string' ? config.default_locale : '';
  if (isLocale(fromConfig)) return fromConfig;
  if (isLocale(urlLocale)) return urlLocale;
  return 'en-US';
}

// Keep the independent entry point aligned with EmbedChatCore.vue:
// file/image controls are one capability and require both channel flags.
export function resolveEmbedUploadCapabilities(config: Record<string, unknown> | undefined | null): EmbedUploadCapabilities {
  const enabled = config?.allow_file_upload === true && config?.agent_image_upload_enabled === true;
  return { allowFileUpload: enabled, allowImageUpload: enabled };
}

function textOf(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function sourceListFromReferences(references: unknown): EmbedSource[] {
  if (!Array.isArray(references)) return [];
  const sources: EmbedSource[] = [];
  for (const entry of references) {
    if (!entry || typeof entry !== 'object') continue;
    const row = entry as Record<string, unknown>;
    const knowledgeId = textOf(row.knowledge_id);
    const rawTitle = textOf(row.knowledge_title);
    const title = rawTitle !== '' ? rawTitle : knowledgeId;
    if (title === '' && textOf(row.chunk_id) === '') continue;
    sources.push({ title, knowledgeId, chunkId: textOf(row.chunk_id) });
  }
  return sources;
}

export async function fileToDataUri(file: File): Promise<string> {
  // Node 22+ and browsers both expose File.arrayBuffer(); encoding here keeps
  // the helper testable outside a DOM FileReader environment.
  const buffer = Buffer.from(await file.arrayBuffer());
  const mime = file.type !== '' ? file.type : 'application/octet-stream';
  return 'data:' + mime + ';base64,' + buffer.toString('base64');
}

export interface AttachmentUpload { data: string; file_name: string; file_size: number }

export async function attachmentUploadsFromFiles(files: readonly File[]): Promise<AttachmentUpload[]> {
  const uploads: AttachmentUpload[] = [];
  for (const file of files) {
    uploads.push({ data: await fileToDataUri(file), file_name: file.name, file_size: file.size });
  }
  return uploads;
}

export async function imageDataUrisFromFiles(files: readonly File[]): Promise<string[]> {
  const images: string[] = [];
  for (const file of files) images.push(await fileToDataUri(file));
  return images;
}

// Localized composer/header strings with an English fallback (the i18n
// table only carries the visitor subset of the Vue chat block).
export function translate(locale: string, key: string, fallback: string, values?: Record<string, string | number>): string {
  const table = i18nMessages[locale as Locale] ?? i18nMessages['en-US'];
  const template = table[key] ?? i18nMessages['en-US'][key];
  if (template === undefined) return fallback;
  return template.replace(/{(w+)}/g, (_match: string, name: string) => String(values?.[name] ?? '{' + name + '}'));
}
