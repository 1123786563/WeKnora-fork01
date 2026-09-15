export type UploadConfirmMode = 'file' | 'manual' | 'reparse';
export type UploadConfirmSection = 'tags' | 'parser' | 'chunking' | 'multimodal' | 'asr' | 'question';

export interface UploadConfirmSource {
  name: string;
  type?: string;
}

export interface UploadConfirmInput {
  mode: UploadConfirmMode;
  files?: UploadConfirmSource[];
  urls?: string[];
  manualContent?: string;
  multimodalEnabled: boolean;
  multimodalModelId: string;
  asrEnabled: boolean;
  asrModelId: string;
}

export interface UploadConfirmValidation {
  valid: boolean;
  issues: UploadConfirmSection[];
  firstIssueSection: UploadConfirmSection | null;
}

const imageExtensions = new Set(['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp']);
const audioExtensions = new Set(['mp3', 'wav', 'm4a', 'flac', 'ogg']);

function extensionOf(name: string): string {
  return name.split(/[?#]/, 1)[0].split('.').pop()?.toLowerCase() ?? '';
}

function hasMedia(sources: UploadConfirmSource[], urls: string[], extensions: Set<string>): boolean {
  return [...sources.map((source) => source.name), ...urls].some((name) => extensions.has(extensionOf(name)));
}

export function validateUploadConfirm(input: UploadConfirmInput): UploadConfirmValidation {
  const files = input.files ?? [];
  const urls = input.urls ?? [];
  const issues: UploadConfirmSection[] = [];

  if (input.mode === 'file' && files.length + urls.length === 0) return { valid: false, issues: [], firstIssueSection: null };
  if (input.mode === 'manual' && !input.manualContent?.trim()) return { valid: false, issues: [], firstIssueSection: null };

  if (hasMedia(files, urls, imageExtensions) && (!input.multimodalEnabled || !input.multimodalModelId)) issues.push('multimodal');
  if (hasMedia(files, urls, audioExtensions) && (!input.asrEnabled || !input.asrModelId)) issues.push('asr');

  return { valid: issues.length === 0, issues, firstIssueSection: issues[0] ?? null };
}

export function getUploadConfirmDefaultSection(input: UploadConfirmInput): UploadConfirmSection {
  if (input.mode === 'reparse') return 'parser';
  return validateUploadConfirm(input).firstIssueSection ?? 'tags';
}
