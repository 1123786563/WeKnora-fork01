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
