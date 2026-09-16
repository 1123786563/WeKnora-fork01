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

export interface UploadConfirmSourceItem {
  kind: 'file' | 'url';
  index: number;
  label: string;
  meta: 'File' | 'URL';
}

const uploadConfirmSections: UploadConfirmSection[] = ['tags', 'parser', 'chunking', 'multimodal', 'asr', 'question'];
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

export function getUploadConfirmSections(mode: UploadConfirmMode): UploadConfirmSection[] {
  return mode === 'reparse' ? uploadConfirmSections.filter((section) => section !== 'tags') : [...uploadConfirmSections];
}

export function isUploadConfirmDismissible(loading: boolean): boolean {
  return !loading;
}

export function getUploadConfirmSourceItems(input: Pick<UploadConfirmInput, 'files' | 'urls'>): UploadConfirmSourceItem[] {
  return [
    ...(input.urls ?? []).map((label, index) => ({ kind: 'url' as const, index, label, meta: 'URL' as const })),
    ...(input.files ?? []).map((file, index) => ({ kind: 'file' as const, index, label: file.name, meta: 'File' as const })),
  ];
}

export function getUploadConfirmButtonOrder(): ['cancel', 'confirm'] {
  return ['cancel', 'confirm'];
}

export function requestUploadConfirmClose(loading: boolean, onCancel: () => void): void {
  if (isUploadConfirmDismissible(loading)) onCancel();
}

export function getUploadConfirmDefaultSection(input: UploadConfirmInput): UploadConfirmSection {
  if (input.mode === 'reparse') return 'parser';
  return validateUploadConfirm(input).firstIssueSection ?? 'tags';
}
