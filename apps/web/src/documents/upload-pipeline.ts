/** Parity with Vue KnowledgeBase.vue multi-file upload (UploadConfirmDialog). */

import { formatMessage, type Locale } from '@weknora/i18n';

export type { Locale };

export interface UploadEntry {
  file: File;
  name: string;
  size: number;
}

export type UploadEntryStatus = 'pending' | 'uploading' | 'done' | 'error';

export interface UploadEntryState {
  entry: UploadEntry;
  status: UploadEntryStatus;
  message?: string;
}

/** Collect a FileList/drop into stable entries before the confirm dialog. */
export function toUploadEntries(files: Iterable<File>): UploadEntry[] {
  return Array.from(files).map((file) => ({ file, name: file.name, size: file.size }));
}

export function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size < 0) return '';
  if (size < 1024) return size + ' B';
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = size;
  let unit = 'B';
  for (const next of units) {
    if (value < 1024) break;
    value /= 1024;
    unit = next;
  }
  return (unit === 'B' ? String(value) : value.toFixed(1)) + ' ' + unit;
}

/** View-model for the confirm dialog: file names + sizes (+ tag_ids applied to all). */
export function uploadSummary(entries: readonly UploadEntry[]): { count: number; totalLabel: string } {
  const total = entries.reduce((sum, entry) => sum + entry.size, 0);
  return { count: entries.length, totalLabel: formatBytes(total) };
}

/** Remove one staged item without mutating the caller's list or upload state. */
export function removeUploadEntry(entries: readonly UploadEntry[], index: number): UploadEntry[] {
  return entries.filter((_, entryIndex) => entryIndex !== index);
}

/** Normalize only web URLs accepted by the URL-import endpoint. */
export function normalizeUploadUrl(value: string): string | null {
  const candidate = value.trim();
  if (!candidate) return null;
  try {
    const parsed = new URL(candidate);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:' ? parsed.toString() : null;
  } catch {
    return null;
  }
}

/**
 * Vue uploadConfirm append-files dedup key (utils/uploadSources.ts
 * getUploadFileKey): relative path when a folder upload supplied one, else the
 * file name, plus the size — same-named files from different folders stage.
 */
export function uploadEntryKey(entry: Pick<UploadEntry, 'name' | 'size'> & { file?: { webkitRelativePath?: string } }): string {
  const relativePath = entry.file?.webkitRelativePath || '';
  return `${relativePath || entry.name}\0${entry.size}`;
}

export interface MergeUploadEntriesResult {
  entries: UploadEntry[];
  addedCount: number;
  duplicateCount: number;
}

/** Vue UploadConfirmDialog.appendFiles: appending dedupes and reports counts. */
export function mergeUploadEntries(existing: readonly UploadEntry[], incoming: readonly UploadEntry[]): MergeUploadEntriesResult {
  const keys = new Set(existing.map(uploadEntryKey));
  const toAdd: UploadEntry[] = [];
  let duplicateCount = 0;
  for (const entry of incoming) {
    const key = uploadEntryKey(entry);
    if (keys.has(key)) {
      duplicateCount += 1;
      continue;
    }
    keys.add(key);
    toAdd.push(entry);
  }
  return {
    entries: [...existing, ...toAdd],
    addedCount: toAdd.length,
    duplicateCount,
  };
}

/**
 * Directory a folder-upload file came from (Vue UploadConfirmDialog.
 * fileRelativeDir): the webkitRelativePath minus the file name, so a batch of
 * same-named files stays distinguishable.
 */
export function uploadEntryRelativeDir(entry: UploadEntry): string {
  const relativePath = (entry.file as File & { webkitRelativePath?: string } | undefined)?.webkitRelativePath;
  if (!relativePath) return '';
  return relativePath.split('/').filter(Boolean).slice(0, -1).join('/');
}

/** Vue UploadConfirmDialog.fileDisplayTitle: relative path when present. */
export function uploadEntryDisplayTitle(entry: UploadEntry): string {
  const relativePath = (entry.file as File & { webkitRelativePath?: string } | undefined)?.webkitRelativePath;
  return relativePath || entry.name;
}

// --- Vue UploadConfirmDialog batch extension detection (L745-816) ------------

/** Vue getFileExt: lower-cased extension without the dot, '' when none. */
export function uploadFileExtension(name: string): string {
  const dot = name.lastIndexOf('.');
  if (dot < 0) return '';
  return name.substring(dot + 1).toLowerCase();
}

/** Vue getExtFromUrl: extension from a URL path, '' for invalid URLs. */
export function uploadUrlExtension(url: string): string {
  try {
    const pathname = new URL(url).pathname;
    return uploadFileExtension(pathname);
  } catch {
    return '';
  }
}

/** Vue inferMediaExtsFromMarkdown: image/audio extensions referenced by a manual document. */
export function inferMediaExtsFromMarkdown(content: string): string[] {
  const exts = new Set<string>();
  const patterns = [
    /\.(jpg|jpeg|png|gif|bmp|webp)(\?|#|\)|\s|$)/gi,
    /\.(mp3|wav|m4a|flac|ogg)(\?|#|\)|\s|$)/gi,
  ];
  for (const pattern of patterns) {
    let match: RegExpExecArray | null;
    while ((match = pattern.exec(content)) !== null) {
      let ext = match[1].toLowerCase();
      if (ext === 'jpeg') ext = 'jpg';
      exts.add(ext);
    }
  }
  return [...exts];
}

export interface BatchUploadExtensionInput {
  entries?: readonly UploadEntry[];
  urls?: readonly string[];
  manualContent?: string;
  reparseFileType?: string;
}

/**
 * Vue batchFileExts: every file type in the batch — staged files, URL paths,
 * media referenced by a manual document and the reparse target's type — so
 * PDF/audio surfaces (scanned-PDF override, ASR setup) react to the whole batch.
 */
export function batchUploadExtensions(input: BatchUploadExtensionInput): string[] {
  const set = new Set<string>();
  if (input.manualContent) {
    for (const ext of inferMediaExtsFromMarkdown(input.manualContent)) set.add(ext);
  }
  if (input.reparseFileType) {
    const ext = input.reparseFileType.toLowerCase();
    if (ext) set.add(ext);
  }
  for (const url of input.urls ?? []) {
    const ext = uploadUrlExtension(url);
    if (ext) set.add(ext);
  }
  for (const entry of input.entries ?? []) {
    const ext = uploadFileExtension(entry.name);
    if (ext) set.add(ext);
  }
  return [...set];
}

export const UPLOAD_IMAGE_EXTENSIONS = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp'];
export const UPLOAD_AUDIO_EXTENSIONS = ['mp3', 'wav', 'm4a', 'flac', 'ogg'];

/** Vue hasImages: embedded manual images or any image extension in the batch. */
export function batchHasImages(exts: readonly string[], manualContent?: string): boolean {
  if (manualContent && /data:image\/|!\[[^\]]*\]\([^)]+\)/i.test(manualContent)) return true;
  return exts.some((ext) => UPLOAD_IMAGE_EXTENSIONS.includes(ext));
}

/** Vue hasAudio. */
export function batchHasAudio(exts: readonly string[]): boolean {
  return exts.some((ext) => UPLOAD_AUDIO_EXTENSIONS.includes(ext));
}

// --- Vue folderTree.ts helpers (frontend/src/views/knowledge/folderTree.ts) --

/** Keep in sync with types.MaxKnowledgeFolderDepth on the server. */
export const MAX_FOLDER_DEPTH = 16;
/** Keep in sync with types.MaxKnowledgeFolderSegmentLength on the server. */
export const MAX_FOLDER_SEGMENT_LENGTH = 128;
/** Keep in sync with types.MaxKnowledgeFolderPathLength on the server. */
export const MAX_FOLDER_PATH_LENGTH = 1024;

export interface FolderOption {
  path: string;
  name: string;
  depth: number;
}

/** Vue normalizeFolderPath: backslashes to slashes, drop bad segments, clamp limits. */
export function normalizeFolderPath(path: string): string {
  const segments: string[] = [];
  for (const raw of path.replace(/\\/g, '/').split('/')) {
    let segment = raw.trim().replace(/[. ]+$/, '');
    if (!segment || segment === '.' || segment === '..') continue;
    if (segment.length > MAX_FOLDER_SEGMENT_LENGTH) {
      segment = segment.slice(0, MAX_FOLDER_SEGMENT_LENGTH).trim();
    }
    if (!segment) continue;
    segments.push(segment);
    if (segments.length >= MAX_FOLDER_DEPTH) break;
  }
  let normalized = segments.join('/');
  while (normalized.length > MAX_FOLDER_PATH_LENGTH && segments.length > 0) {
    segments.pop();
    normalized = segments.join('/');
  }
  return normalized;
}

/** Vue joinFolderPath. */
export function joinFolderPath(parent: string, name: string): string {
  return normalizeFolderPath(parent ? `${parent}/${name}` : name);
}

/** Vue folderOptionFromPath: last path segment as the row label. */
export function folderOptionFromPath(path: string): FolderOption {
  const segments = path.split('/').filter(Boolean);
  return {
    path,
    name: segments[segments.length - 1] || path,
    depth: Math.max(segments.length - 1, 0),
  };
}

/** Vue sortFolderOptions: sibling order at every level. */
export function sortFolderOptions<T extends { path: string }>(options: T[]): T[] {
  return [...options].sort((a, b) => {
    const aParts = a.path.split('/').filter(Boolean);
    const bParts = b.path.split('/').filter(Boolean);
    const max = Math.max(aParts.length, bParts.length);
    for (let i = 0; i < max; i += 1) {
      const aSeg = aParts[i];
      const bSeg = bParts[i];
      if (aSeg === undefined) return -1;
      if (bSeg === undefined) return 1;
      const cmp = aSeg.localeCompare(bSeg);
      if (cmp !== 0) return cmp;
    }
    return 0;
  });
}

/**
 * Vue pickerFolderOptions (UploadConfirmDialog L675-682): server folders plus
 * folders created in this dialog before upload, sorted for the picker list.
 */
export function mergeFolderOptions(options: readonly FolderOption[], pendingPaths: readonly string[]): FolderOption[] {
  const byPath = new Map<string, FolderOption>();
  options.forEach((option) => byPath.set(option.path, option));
  pendingPaths.forEach((path) => {
    if (!byPath.has(path)) byPath.set(path, folderOptionFromPath(path));
  });
  return sortFolderOptions([...byPath.values()]);
}

// --- Vue FolderPickerMenu render model (FolderPickerMenu.vue L123-161) -------

export type FolderPickerRow =
  | { kind: 'folder'; key: string; path: string; label: string; depth: number; isRoot: boolean }
  | { kind: 'create'; key: string; parentPath: string; depth: number };

/** childCreateDepth: a create row indents one level under its parent. */
function childCreateDepth(parentPath: string): number {
  return parentPath.split('/').filter(Boolean).length;
}

/** The picker rows: the root first, then folders with an inline create row under the parent being created. */
export function folderPickerRows(options: readonly FolderOption[], creatingUnder: string | null, rootLabel: string): FolderPickerRow[] {
  const rows: FolderPickerRow[] = [{
    kind: 'folder',
    key: 'root',
    path: '',
    label: rootLabel,
    depth: 0,
    isRoot: true,
  }];
  if (creatingUnder === '') {
    rows.push({ kind: 'create', key: 'create-root', parentPath: '', depth: childCreateDepth('') });
  }
  options.forEach((option) => {
    rows.push({ kind: 'folder', key: option.path, path: option.path, label: option.name, depth: option.depth, isRoot: false });
    if (creatingUnder === option.path) {
      rows.push({ kind: 'create', key: `create-${option.path}`, parentPath: option.path, depth: childCreateDepth(option.path) });
    }
  });
  return rows;
}

export type CommitFolderNameResult =
  | { status: 'created'; path: string }
  | { status: 'duplicate'; path: string }
  | { status: 'invalid' };

/** Vue FolderPickerMenu.commitNewFolder: normalize, join under the parent, reject duplicates. */
export function commitFolderName(options: readonly FolderOption[], parentPath: string, rawName: string): CommitFolderNameResult {
  const name = normalizeFolderPath(rawName);
  if (!name) return { status: 'invalid' };
  const path = joinFolderPath(parentPath, name);
  if (options.some((option) => option.path === path)) {
    return { status: 'duplicate', path };
  }
  return { status: 'created', path };
}

/** Vue destinationBreadcrumb: root label plus path segments joined with ' / '. */
export function destinationBreadcrumb(path: string, rootLabel: string): string {
  if (!path) return rootLabel;
  const parts = path.split('/').filter(Boolean);
  return [rootLabel, ...parts].join(' / ');
}

// --- Vue parser section status (UploadConfirmDialog L738-743, 958-965) ------

export interface ParserEngineRuleState {
  file_types: string[];
  engine: string;
  xlsx_first_row_as_header?: boolean;
}

/** Vue hasParserCustomization: any non-builtin engine or the xlsx header flag. */
export function hasParserCustomization(rules: readonly ParserEngineRuleState[] | undefined): boolean {
  if (!rules?.length) return false;
  return rules.some((rule) => rule.engine && rule.engine !== 'builtin')
    || rules.some((rule) => rule.xlsx_first_row_as_header);
}

export type ParserNavStatusKey = 'forceScanned' | 'customized' | 'default';

/** Vue getSectionNavStatus('parser'): scanned mode wins, then customized, then default. */
export function parserNavStatus(input: {
  pdfForceScanned: boolean;
  hasPdf: boolean;
  rules?: readonly ParserEngineRuleState[];
}): ParserNavStatusKey {
  if (input.pdfForceScanned && input.hasPdf) return 'forceScanned';
  if (hasParserCustomization(input.rules)) return 'customized';
  return 'default';
}

// --- Vue UploadUIState model (UploadConfirmDialog L600-615, 1057-1247) -------

export interface UploadConfirmUIState {
  chunkSize: number;
  chunkOverlap: number;
  chunkStrategy: string;
  separators: string[];
  tokenLimit: number;
  languages: string[];
  enableParentChild: boolean;
  parentChunkSize: number;
  childChunkSize: number;
  parserRules: ParserEngineRuleState[];
  multimodalEnabled: boolean;
  vllmModelId: string;
  descriptionLanguage: string;
  customInstructions: string;
  asrEnabled: boolean;
  asrModelId: string;
  asrLanguage: string;
  questionEnabled: boolean;
  questionCount: number;
  questionInstructions: string;
  pdfForceScanned: boolean;
}

/** Vue createDefaultUIState. */
export function defaultUploadConfirmUIState(): UploadConfirmUIState {
  return {
    chunkSize: 512,
    chunkOverlap: 80,
    chunkStrategy: 'auto',
    separators: ['\n\n', '\n', '。', '！', '？', ';', '；'],
    tokenLimit: 0,
    languages: [],
    enableParentChild: true,
    parentChunkSize: 4096,
    childChunkSize: 384,
    parserRules: [],
    multimodalEnabled: false,
    vllmModelId: '',
    descriptionLanguage: '',
    customInstructions: '',
    asrEnabled: false,
    asrModelId: '',
    asrLanguage: '',
    questionEnabled: true,
    questionCount: 3,
    questionInstructions: '',
    pdfForceScanned: false,
  };
}

type KbLike = Record<string, unknown> | null | undefined;

function kbRecord(kb: KbLike, key: string): Record<string, unknown> | undefined {
  const value = (kb as Record<string, unknown> | undefined)?.[key];
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

function num(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function str(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function strList(value: unknown, fallback: string[]): string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string') ? (value as string[]) : fallback;
}

/** Parse stored parser_engine_rules, keeping the xlsx flag Vue round-trips. */
function parseEngineRules(value: unknown): ParserEngineRuleState[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((rule): ParserEngineRuleState[] => {
    if (!rule || typeof rule !== 'object') return [];
    const row = rule as Record<string, unknown>;
    if (typeof row.engine !== 'string' || !Array.isArray(row.file_types)) return [];
    if (!row.file_types.every((item) => typeof item === 'string')) return [];
    return [{
      file_types: row.file_types as string[],
      engine: row.engine,
      ...(typeof row.xlsx_first_row_as_header === 'boolean' ? { xlsx_first_row_as_header: row.xlsx_first_row_as_header } : {}),
    }];
  });
}

/** Vue initFromKbInfo: seed the dialog from the knowledge base defaults. */
export function uploadConfirmStateFromKb(kb: KbLike): UploadConfirmUIState {
  const state = defaultUploadConfirmUIState();
  if (!kb) return state;
  const chunking = kbRecord(kb, 'chunking_config');
  if (chunking) {
    state.chunkSize = num(chunking.chunk_size, 512);
    state.chunkOverlap = num(chunking.chunk_overlap, 80);
    state.separators = strList(chunking.separators, state.separators);
    state.parserRules = parseEngineRules(chunking.parser_engine_rules);
    if (Array.isArray(chunking.parser_engine_rules) && chunking.parser_engine_rules.length === 0) state.parserRules = [];
    state.enableParentChild = chunking.enable_parent_child === true;
    state.parentChunkSize = num(chunking.parent_chunk_size, 4096);
    state.childChunkSize = num(chunking.child_chunk_size, 384);
    state.chunkStrategy = str(chunking.strategy, 'auto') || 'auto';
    state.tokenLimit = num(chunking.token_limit, 0);
    state.languages = strList(chunking.languages, []);
  }
  const vlm = kbRecord(kb, 'vlm_config');
  if (vlm) {
    state.multimodalEnabled = vlm.enabled === true;
    state.vllmModelId = str(vlm.model_id);
    state.descriptionLanguage = str(vlm.description_language);
    state.customInstructions = str(vlm.custom_instructions);
  } else if ((kb as Record<string, unknown>).enable_multimodel === true) {
    state.multimodalEnabled = true;
  }
  const asr = kbRecord(kb, 'asr_config');
  if (asr) {
    state.asrEnabled = asr.enabled === true;
    state.asrModelId = str(asr.model_id);
    state.asrLanguage = str(asr.language);
  }
  const question = kbRecord(kb, 'question_generation_config');
  if (question) {
    state.questionEnabled = question.enabled !== false;
    state.questionCount = num(question.question_count, 3);
    state.questionInstructions = str(question.custom_instructions);
  }
  // Vue always resets pdfForceScanned per dialog open; scanned mode is a
  // per-batch override, never a knowledge base default.
  state.pdfForceScanned = false;
  return state;
}

/** Snake-case override payload shape (backend types.KnowledgeProcessOverrides). */
export interface UploadConfirmOverrides {
  parser_engine_rules?: ParserEngineRuleState[];
  chunking_config?: Record<string, unknown>;
  enable_multimodel?: boolean;
  vlm_config?: Record<string, unknown>;
  asr_config?: Record<string, unknown>;
  question_generation_config?: Record<string, unknown>;
  graph_enabled?: boolean;
  extract_config?: Record<string, unknown>;
  parser_engine_overrides?: Record<string, string>;
}

/** Vue applyOverridesToState, as an immutable fold over a base state. */
export function applyUploadOverrides(base: UploadConfirmUIState, overrides: UploadConfirmOverrides | null | undefined): UploadConfirmUIState {
  if (!overrides) return base;
  const state: UploadConfirmUIState = { ...base };
  const cc = overrides.chunking_config;
  if (cc) {
    if (typeof cc.chunk_size === 'number') state.chunkSize = cc.chunk_size;
    if (typeof cc.chunk_overlap === 'number') state.chunkOverlap = cc.chunk_overlap;
    if (Array.isArray(cc.separators) && cc.separators.every((item) => typeof item === 'string')) state.separators = cc.separators as string[];
    if (typeof cc.enable_parent_child === 'boolean') state.enableParentChild = cc.enable_parent_child;
    if (typeof cc.parent_chunk_size === 'number') state.parentChunkSize = cc.parent_chunk_size;
    if (typeof cc.child_chunk_size === 'number') state.childChunkSize = cc.child_chunk_size;
    if (typeof cc.strategy === 'string') state.chunkStrategy = cc.strategy;
    if (typeof cc.token_limit === 'number') state.tokenLimit = cc.token_limit;
    if (Array.isArray(cc.languages) && cc.languages.every((item) => typeof item === 'string')) state.languages = cc.languages as string[];
    if (Array.isArray(cc.parser_engine_rules)) state.parserRules = parseEngineRules(cc.parser_engine_rules);
  }
  if (overrides.parser_engine_rules) state.parserRules = parseEngineRules(overrides.parser_engine_rules);
  if (typeof overrides.enable_multimodel === 'boolean') state.multimodalEnabled = overrides.enable_multimodel;
  const vlm = overrides.vlm_config;
  if (vlm) {
    if (typeof vlm.enabled === 'boolean') state.multimodalEnabled = vlm.enabled;
    if (typeof vlm.model_id === 'string') state.vllmModelId = vlm.model_id;
    if (typeof vlm.description_language === 'string') state.descriptionLanguage = vlm.description_language;
    if (typeof vlm.custom_instructions === 'string') state.customInstructions = vlm.custom_instructions;
  }
  const asr = overrides.asr_config;
  if (asr) {
    if (typeof asr.enabled === 'boolean') state.asrEnabled = asr.enabled;
    if (typeof asr.model_id === 'string') state.asrModelId = asr.model_id;
    if (typeof asr.language === 'string') state.asrLanguage = asr.language;
  }
  const qg = overrides.question_generation_config;
  if (qg) {
    if (typeof qg.enabled === 'boolean') state.questionEnabled = qg.enabled;
    if (typeof qg.question_count === 'number') state.questionCount = qg.question_count;
    if (typeof qg.custom_instructions === 'string') state.questionInstructions = qg.custom_instructions;
  }
  state.pdfForceScanned = overrides.parser_engine_overrides?.pdf_force_scanned === 'true';
  return state;
}

/** Vue buildProcessOverrides: the per-batch process_config payload. */
export function buildUploadConfirmOverrides(state: UploadConfirmUIState): UploadConfirmOverrides {
  const overrides: UploadConfirmOverrides = {
    parser_engine_rules: state.parserRules.length > 0 ? state.parserRules : undefined,
    chunking_config: {
      chunk_size: state.chunkSize,
      chunk_overlap: state.chunkOverlap,
      separators: state.separators,
      enable_parent_child: state.enableParentChild,
      parent_chunk_size: state.parentChunkSize,
      child_chunk_size: state.childChunkSize,
      strategy: state.chunkStrategy,
      token_limit: state.tokenLimit,
      languages: state.languages,
    },
    enable_multimodel: state.multimodalEnabled,
    vlm_config: {
      enabled: state.multimodalEnabled,
      model_id: state.vllmModelId,
      description_language: state.descriptionLanguage,
      custom_instructions: state.customInstructions,
    },
    asr_config: {
      enabled: state.asrEnabled,
      model_id: state.asrModelId,
      language: state.asrLanguage,
    },
    question_generation_config: {
      enabled: state.questionEnabled,
      question_count: state.questionCount,
      custom_instructions: state.questionInstructions,
    },
  };
  if (state.pdfForceScanned) {
    overrides.parser_engine_overrides = { pdf_force_scanned: 'true' };
  }
  return overrides;
}

// --- Section nav status model (Vue UploadConfirmDialog L897-1022) ------------

export interface UploadSectionStatus {
  /** Message key inside the dialog copy table. */
  key: string;
  values?: Record<string, string | number>;
  tone?: 'warning' | 'error' | 'muted';
  /** Literal status text (a model name) rendered instead of the keyed message. */
  text?: string;
  /** Extra key appended with ' · ' (the chunking parent-child marker). */
  suffixKey?: string;
}

export interface SectionNavStatusInput {
  state: UploadConfirmUIState;
  selectedTagCount: number;
  hasPdf: boolean;
  hasImages: boolean;
  hasAudio: boolean;
  vllmModelName?: string;
  asrModelName?: string;
}

/** True when the section blocks confirm (Vue issueSectionKeys + model errors). */
export function multimodalSectionIssue(input: Pick<SectionNavStatusInput, 'state' | 'hasImages'>): boolean {
  const modelMissing = input.state.multimodalEnabled && !input.state.vllmModelId;
  if (input.hasImages) return !input.state.multimodalEnabled || !input.state.vllmModelId;
  return modelMissing;
}

export function asrSectionIssue(input: Pick<SectionNavStatusInput, 'state' | 'hasAudio'>): boolean {
  const modelMissing = input.state.asrEnabled && !input.state.asrModelId;
  if (input.hasAudio) return !input.state.asrEnabled || !input.state.asrModelId;
  return modelMissing;
}

/**
 * Per-section nav status text (Vue getSectionNavStatus). Returns null for
 * sections the React dialog does not render (graph).
 */
export function uploadSectionStatus(section: 'tags' | 'parser' | 'chunking' | 'multimodal' | 'asr' | 'question', input: SectionNavStatusInput): UploadSectionStatus | null {
  switch (section) {
    case 'tags':
      return input.selectedTagCount === 0
        ? { key: 'uploadConfirm.summaryNoTags', tone: 'muted' }
        : { key: 'uploadConfirm.summaryTagsCount', values: { count: input.selectedTagCount } };
    case 'parser': {
      const status = parserNavStatus({ pdfForceScanned: input.state.pdfForceScanned, hasPdf: input.hasPdf, rules: input.state.parserRules });
      if (status === 'forceScanned') return { key: 'uploadConfirm.summaryParserForceScanned' };
      if (status === 'customized') return { key: 'uploadConfirm.navParserCustomized' };
      return { key: 'uploadConfirm.navParserDefault', tone: 'muted' };
    }
    case 'chunking':
      return {
        key: 'uploadConfirm.navChunkingSummary',
        values: { size: input.state.chunkSize },
        suffixKey: input.state.enableParentChild ? 'uploadConfirm.summaryParentChildShort' : undefined,
      };
    case 'multimodal':
      if (multimodalSectionIssue(input)) return { key: 'uploadConfirm.statusNeedsSetup', tone: 'error' };
      if (!input.state.multimodalEnabled) return { key: 'uploadConfirm.statusOff', tone: 'muted' };
      return input.state.vllmModelId
        ? { key: 'model', text: input.vllmModelName || input.state.vllmModelId }
        : { key: 'uploadConfirm.notSet', tone: 'warning' };
    case 'asr':
      if (asrSectionIssue(input)) return { key: 'uploadConfirm.statusNeedsSetup', tone: 'error' };
      if (!input.state.asrEnabled) return { key: 'uploadConfirm.statusOff', tone: 'muted' };
      return input.state.asrModelId
        ? { key: 'model', text: input.asrModelName || input.state.asrModelId }
        : { key: 'uploadConfirm.notSet', tone: 'warning' };
    case 'question':
      return input.state.questionEnabled
        ? { key: 'uploadConfirm.summaryQuestionCountValue', values: { count: input.state.questionCount } }
        : { key: 'uploadConfirm.statusOff', tone: 'muted' };
    default:
      return null;
  }
}

// --- Dialog copy (Vue locale uploadConfirm block; five locales) --------------
//
// The shared i18n package has not migrated the uploadConfirm block yet; these
// strings are byte-exact ports from frontend/src/i18n/locales/*.ts so the
// dialog is fully localized today. When packages/i18n gains the keys, swap
// uploadConfirmT for formatMessage and delete this table.

const uploadConfirmCopy: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'uploadConfirm.title': '上传文档确认',
    'uploadConfirm.titleManual': '在线编辑发布确认',
    'uploadConfirm.titleReparse': '重新解析确认',
    'uploadConfirm.parseConfig': '解析配置',
    'uploadConfirm.configNav': '解析配置导航',
    'uploadConfirm.navParserDefault': '默认',
    'uploadConfirm.navParserCustomized': '已自定义',
    'uploadConfirm.moreOptions': '更多处理选项',
    'uploadConfirm.summaryParentChildShort': '父子分块',
    'uploadConfirm.summaryParserForceScanned': '扫描件解析',
    'uploadConfirm.summaryQuestionCountValue': '{count} 个',
    'uploadConfirm.navChunkingSummary': '分块 {size}',
    'uploadConfirm.statusOn': '已开启',
    'uploadConfirm.statusOff': '未开启',
    'uploadConfirm.notSet': '未设置',
    'uploadConfirm.summaryNoTags': '未设置',
    'uploadConfirm.summaryTagsCount': '{count} 个标签',
    'uploadConfirm.confirm': '确认上传并解析',
    'uploadConfirm.cancel': '取消',
    'uploadConfirm.tabTags': '文档标签',
    'uploadConfirm.tagsDescription': '为本批次导入的所有文档选择标签，可多选',
    'uploadConfirm.tagsPlaceholder': '请选择标签（可多选）',
    'uploadConfirm.tagsEmpty': '当前知识库暂无标签，可上传后在标签管理中创建',
    'uploadConfirm.tagsLoadFailed': '标签加载失败，可稍后在文档列表中设置',
    'uploadConfirm.noItems': '请至少添加一个文件或 URL',
    'uploadConfirm.urlItemLabel': 'URL',
    'uploadConfirm.urlAdded': '已添加 URL',
    'uploadConfirm.urlDuplicate': '该 URL 已在列表中',
    'uploadConfirm.statusNeedsSetup': '待配置',
    'uploadConfirm.multimodalSetupHint': '含图片内容，请启用多模态并选择模型',
    'uploadConfirm.asrSetupHint': '含音频内容，请启用语音识别并选择模型',
    'uploadConfirm.vlmModelRequired': '请配置多模态模型',
    'uploadConfirm.asrModelRequired': '请配置语音识别模型',
    'uploadConfirm.vlmModelSelectRequired': '已启用多模态，请选择 VLM 模型',
    'uploadConfirm.asrModelSelectRequired': '已启用语音识别，请选择 ASR 模型',
    'uploadConfirm.continueAdd': '继续添加',
    'uploadConfirm.destinationLabel': '上传位置',
    'uploadConfirm.destinationChange': '更改上传位置',
    'uploadConfirm.filesAdded': '已添加 {count} 个文件',
    'uploadConfirm.filesAllDuplicate': '所选文件已在列表中',
    'uploadConfirm.confirmManual': '确认发布并解析',
    'uploadConfirm.confirmReparse': '确认并重新解析',
    'uploadConfirm.reparseSource': '待重新解析文档',
    'uploadConfirm.reparseHint': '将沿用上次解析的配置，可在此调整',
    'uploadConfirm.manualCharCount': '{count} 个字符',
    'uploadConfirm.pdfForceScanned.label': '按扫描件解析 PDF',
    'uploadConfirm.pdfForceScanned.description': '适用于网页打印、扫描件、图片型 PDF。开启后会逐页 OCR，解析更完整但耗时和模型调用更多。',
    'common.remove': '移除',
  },
  'en-US': {
    'uploadConfirm.title': 'Confirm Upload',
    'uploadConfirm.titleManual': 'Confirm online publish',
    'uploadConfirm.titleReparse': 'Confirm reparse',
    'uploadConfirm.parseConfig': 'Parse settings',
    'uploadConfirm.configNav': 'Parse settings navigation',
    'uploadConfirm.navParserDefault': 'Default',
    'uploadConfirm.navParserCustomized': 'Customized',
    'uploadConfirm.moreOptions': 'More processing options',
    'uploadConfirm.summaryParentChildShort': 'Parent-child',
    'uploadConfirm.summaryParserForceScanned': 'Scanned mode',
    'uploadConfirm.summaryQuestionCountValue': '{count}',
    'uploadConfirm.navChunkingSummary': 'Chunk {size}',
    'uploadConfirm.statusOn': 'On',
    'uploadConfirm.statusOff': 'Off',
    'uploadConfirm.notSet': 'Not set',
    'uploadConfirm.summaryNoTags': 'Not set',
    'uploadConfirm.summaryTagsCount': '{count} tags',
    'uploadConfirm.confirm': 'Upload and parse',
    'uploadConfirm.cancel': 'Cancel',
    'uploadConfirm.tabTags': 'Document tags',
    'uploadConfirm.tagsDescription': 'Select one or more tags for every document in this import batch',
    'uploadConfirm.tagsPlaceholder': 'Select tags (multiple allowed)',
    'uploadConfirm.tagsEmpty': 'This knowledge base has no tags. You can create them in tag management after upload.',
    'uploadConfirm.tagsLoadFailed': 'Failed to load tags. You can set them later from the document list.',
    'uploadConfirm.noItems': 'Add at least one file or URL',
    'uploadConfirm.urlItemLabel': 'URL',
    'uploadConfirm.urlAdded': 'URL added',
    'uploadConfirm.urlDuplicate': 'This URL is already in the list',
    'uploadConfirm.statusNeedsSetup': 'Needs setup',
    'uploadConfirm.multimodalSetupHint': 'Images detected. Enable multimodal and select a model.',
    'uploadConfirm.asrSetupHint': 'Audio detected. Enable speech recognition and select a model.',
    'uploadConfirm.vlmModelRequired': 'Configure a multimodal model',
    'uploadConfirm.asrModelRequired': 'Configure a speech recognition model',
    'uploadConfirm.vlmModelSelectRequired': 'Multimodal is enabled. Please select a VLM model.',
    'uploadConfirm.asrModelSelectRequired': 'Speech recognition is enabled. Please select an ASR model.',
    'uploadConfirm.continueAdd': 'Add more',
    'uploadConfirm.destinationLabel': 'Upload location',
    'uploadConfirm.destinationChange': 'Change upload location',
    'uploadConfirm.filesAdded': 'Added {count} file(s)',
    'uploadConfirm.filesAllDuplicate': 'Selected files are already in the list',
    'uploadConfirm.confirmManual': 'Publish and parse',
    'uploadConfirm.confirmReparse': 'Confirm and reparse',
    'uploadConfirm.reparseSource': 'Document to reparse',
    'uploadConfirm.reparseHint': 'Reuses the last parse settings; adjust them here',
    'uploadConfirm.manualCharCount': '{count} characters',
    'uploadConfirm.pdfForceScanned.label': 'Force scanned PDF parsing',
    'uploadConfirm.pdfForceScanned.description': 'Useful for web-print, scanned, or image-heavy PDFs. Every page will be rendered as an image and processed via OCR/VLM. May increase processing time and model costs.',
    'common.remove': 'Remove',
  },
  'ja-JP': {
    'uploadConfirm.title': 'アップロードの確認',
    'uploadConfirm.titleManual': 'オンライン作成の確認',
    'uploadConfirm.titleReparse': '再解析の確認',
    'uploadConfirm.parseConfig': '解析設定',
    'uploadConfirm.configNav': '解析設定のナビゲーション',
    'uploadConfirm.navParserDefault': 'デフォルト',
    'uploadConfirm.navParserCustomized': 'カスタム',
    'uploadConfirm.moreOptions': 'その他の処理オプション',
    'uploadConfirm.summaryParentChildShort': '親子チャンク',
    'uploadConfirm.summaryParserForceScanned': 'スキャンモード',
    'uploadConfirm.summaryQuestionCountValue': '{count}',
    'uploadConfirm.navChunkingSummary': 'チャンク{size}',
    'uploadConfirm.statusOn': 'オン',
    'uploadConfirm.statusOff': 'オフ',
    'uploadConfirm.notSet': '未設定',
    'uploadConfirm.summaryNoTags': '未設定',
    'uploadConfirm.summaryTagsCount': 'タグ{count}件',
    'uploadConfirm.confirm': 'アップロードして解析',
    'uploadConfirm.cancel': 'キャンセル',
    'uploadConfirm.tabTags': 'ドキュメントのタグ',
    'uploadConfirm.tagsDescription': '今回のインポート対象すべてのドキュメントに付けるタグを選択してください',
    'uploadConfirm.tagsPlaceholder': 'タグを選択（複数選択可）',
    'uploadConfirm.tagsEmpty': 'このナレッジベースにはタグがありません。アップロード後にタグ管理から作成できます。',
    'uploadConfirm.tagsLoadFailed': 'タグの読み込みに失敗しました。後からドキュメント一覧で設定できます。',
    'uploadConfirm.noItems': 'ファイルまたはURLを1件以上追加してください',
    'uploadConfirm.urlItemLabel': 'URL',
    'uploadConfirm.urlAdded': 'URLを追加しました',
    'uploadConfirm.urlDuplicate': 'このURLは既に一覧にあります',
    'uploadConfirm.statusNeedsSetup': '設定が必要',
    'uploadConfirm.multimodalSetupHint': '画像が含まれています。マルチモーダルを有効にしてモデルを選択してください。',
    'uploadConfirm.asrSetupHint': '音声が含まれています。音声認識を有効にしてモデルを選択してください。',
    'uploadConfirm.vlmModelRequired': 'マルチモーダルモデルを設定してください',
    'uploadConfirm.asrModelRequired': '音声認識モデルを設定してください',
    'uploadConfirm.vlmModelSelectRequired': 'マルチモーダルが有効です。VLMモデルを選択してください。',
    'uploadConfirm.asrModelSelectRequired': '音声認識が有効です。ASRモデルを選択してください。',
    'uploadConfirm.continueAdd': '続けて追加',
    'uploadConfirm.destinationLabel': 'アップロード先',
    'uploadConfirm.destinationChange': 'アップロード先を変更',
    'uploadConfirm.filesAdded': '{count}件のファイルを追加しました',
    'uploadConfirm.filesAllDuplicate': '選択したファイルは既に一覧にあります',
    'uploadConfirm.confirmManual': '作成して解析',
    'uploadConfirm.confirmReparse': '確定して再解析',
    'uploadConfirm.reparseSource': '再解析するドキュメント',
    'uploadConfirm.reparseHint': '前回の解析設定を引き継ぎます。ここで変更できます',
    'uploadConfirm.manualCharCount': '{count}文字',
    'uploadConfirm.pdfForceScanned.label': 'スキャンPDFとして強制解析',
    'uploadConfirm.pdfForceScanned.description': 'Web印刷、スキャン、画像が多いPDFに有効です。全ページを画像としてレンダリングし、OCR／VLMで処理します。処理時間とモデル費用が増える場合があります。',
    'common.remove': '削除',
  },
  'ko-KR': {
    'uploadConfirm.title': '문서 업로드 확인',
    'uploadConfirm.titleManual': '온라인 편집 게시 확인',
    'uploadConfirm.titleReparse': '재파싱 확인',
    'uploadConfirm.parseConfig': '파싱 설정',
    'uploadConfirm.configNav': '파싱 설정 탐색',
    'uploadConfirm.navParserDefault': '기본',
    'uploadConfirm.navParserCustomized': '사용자 지정',
    'uploadConfirm.moreOptions': '추가 처리 옵션',
    'uploadConfirm.summaryParentChildShort': '부모-자식 청킹',
    'uploadConfirm.summaryParserForceScanned': '스캔 문서 파싱',
    'uploadConfirm.summaryQuestionCountValue': '{count}개',
    'uploadConfirm.navChunkingSummary': '청크 {size}',
    'uploadConfirm.statusOn': '켜짐',
    'uploadConfirm.statusOff': '꺼짐',
    'uploadConfirm.notSet': '미설정',
    'uploadConfirm.summaryNoTags': '미설정',
    'uploadConfirm.summaryTagsCount': '태그 {count}개',
    'uploadConfirm.confirm': '업로드 및 파싱 확인',
    'uploadConfirm.cancel': '취소',
    'uploadConfirm.tabTags': '문서 태그',
    'uploadConfirm.tagsDescription': '이 배치로 가져오는 모든 문서에 적용할 태그를 여러 개 선택할 수 있습니다',
    'uploadConfirm.tagsPlaceholder': '태그 선택(복수 선택 가능)',
    'uploadConfirm.tagsEmpty': '현재 지식 베이스에 태그가 없습니다. 업로드 후 태그 관리에서 만들 수 있습니다.',
    'uploadConfirm.tagsLoadFailed': '태그를 불러오지 못했습니다. 나중에 문서 목록에서 설정할 수 있습니다.',
    'uploadConfirm.noItems': '파일 또는 URL을 하나 이상 추가하세요',
    'uploadConfirm.urlItemLabel': 'URL',
    'uploadConfirm.urlAdded': 'URL이 추가되었습니다',
    'uploadConfirm.urlDuplicate': '이 URL은 이미 목록에 있습니다',
    'uploadConfirm.statusNeedsSetup': '설정 필요',
    'uploadConfirm.multimodalSetupHint': '이미지가 포함되어 있습니다. 멀티모달을 활성화하고 모델을 선택하세요',
    'uploadConfirm.asrSetupHint': '오디오가 포함되어 있습니다. 음성 인식을 활성화하고 모델을 선택하세요',
    'uploadConfirm.vlmModelRequired': '멀티모달 모델을 설정하세요',
    'uploadConfirm.asrModelRequired': '음성 인식 모델을 설정하세요',
    'uploadConfirm.vlmModelSelectRequired': '멀티모달이 활성화되었습니다. VLM 모델을 선택하세요',
    'uploadConfirm.asrModelSelectRequired': '음성 인식이 활성화되었습니다. ASR 모델을 선택하세요',
    'uploadConfirm.continueAdd': '계속 추가',
    'uploadConfirm.destinationLabel': '업로드 위치',
    'uploadConfirm.destinationChange': '업로드 위치 변경',
    'uploadConfirm.filesAdded': '{count}개 파일이 추가되었습니다',
    'uploadConfirm.filesAllDuplicate': '선택한 파일이 이미 목록에 있습니다',
    'uploadConfirm.confirmManual': '게시 및 파싱',
    'uploadConfirm.confirmReparse': '확인 후 재파싱',
    'uploadConfirm.reparseSource': '재파싱할 문서',
    'uploadConfirm.reparseHint': '이전 파싱 설정을 사용하며 여기서 조정할 수 있습니다',
    'uploadConfirm.manualCharCount': '{count}자',
    'uploadConfirm.pdfForceScanned.label': '스캔 PDF로 파싱',
    'uploadConfirm.pdfForceScanned.description': '웹 인쇄, 스캔본, 이미지 위주 PDF에 적합합니다. 모든 페이지를 이미지로 렌더링한 뒤 OCR/VLM으로 처리합니다. 처리 시간과 모델 호출 비용이 늘어날 수 있습니다.',
    'common.remove': '제거',
  },
  'ru-RU': {
    'uploadConfirm.title': 'Подтверждение загрузки',
    'uploadConfirm.titleManual': 'Подтверждение публикации',
    'uploadConfirm.titleReparse': 'Подтверждение повторной обработки',
    'uploadConfirm.parseConfig': 'Настройки разбора',
    'uploadConfirm.configNav': 'Навигация по настройкам разбора',
    'uploadConfirm.navParserDefault': 'По умолчанию',
    'uploadConfirm.navParserCustomized': 'Настроено',
    'uploadConfirm.moreOptions': 'Дополнительные параметры обработки',
    'uploadConfirm.summaryParentChildShort': 'Родительско-дочернее',
    'uploadConfirm.summaryParserForceScanned': 'режим скана',
    'uploadConfirm.summaryQuestionCountValue': '{count}',
    'uploadConfirm.navChunkingSummary': 'Чанк {size}',
    'uploadConfirm.statusOn': 'Вкл.',
    'uploadConfirm.statusOff': 'Выкл.',
    'uploadConfirm.notSet': 'Не задано',
    'uploadConfirm.summaryNoTags': 'Не задано',
    'uploadConfirm.summaryTagsCount': 'Тегов: {count}',
    'uploadConfirm.confirm': 'Загрузить и обработать',
    'uploadConfirm.cancel': 'Отмена',
    'uploadConfirm.tabTags': 'Теги документов',
    'uploadConfirm.tagsDescription': 'Выберите один или несколько тегов для всех документов в этой партии',
    'uploadConfirm.tagsPlaceholder': 'Выберите теги (можно несколько)',
    'uploadConfirm.tagsEmpty': 'В этой базе знаний пока нет тегов. Их можно создать после загрузки.',
    'uploadConfirm.tagsLoadFailed': 'Не удалось загрузить теги. Их можно назначить позже в списке документов.',
    'uploadConfirm.noItems': 'Добавьте хотя бы один файл или URL',
    'uploadConfirm.urlItemLabel': 'URL',
    'uploadConfirm.urlAdded': 'URL добавлен',
    'uploadConfirm.urlDuplicate': 'Этот URL уже в списке',
    'uploadConfirm.statusNeedsSetup': 'Нужна настройка',
    'uploadConfirm.multimodalSetupHint': 'Обнаружены изображения. Включите мультимодальность и выберите модель.',
    'uploadConfirm.asrSetupHint': 'Обнаружено аудио. Включите распознавание речи и выберите модель.',
    'uploadConfirm.vlmModelRequired': 'Настройте мультимодальную модель',
    'uploadConfirm.asrModelRequired': 'Настройте модель распознавания речи',
    'uploadConfirm.vlmModelSelectRequired': 'Мультимодальность включена. Выберите модель VLM.',
    'uploadConfirm.asrModelSelectRequired': 'Распознавание речи включено. Выберите модель ASR.',
    'uploadConfirm.continueAdd': 'Добавить ещё',
    'uploadConfirm.destinationLabel': 'Куда загрузить',
    'uploadConfirm.destinationChange': 'Изменить папку загрузки',
    'uploadConfirm.filesAdded': 'Добавлено файлов: {count}',
    'uploadConfirm.filesAllDuplicate': 'Выбранные файлы уже в списке',
    'uploadConfirm.confirmManual': 'Опубликовать и обработать',
    'uploadConfirm.confirmReparse': 'Подтвердить и обработать заново',
    'uploadConfirm.reparseSource': 'Документ для повторной обработки',
    'uploadConfirm.reparseHint': 'Используются настройки прошлой обработки; их можно изменить здесь',
    'uploadConfirm.manualCharCount': '{count} символов',
    'uploadConfirm.pdfForceScanned.label': 'Разбор PDF как сканированного документа',
    'uploadConfirm.pdfForceScanned.description': 'Подходит для PDF с веб-печати, сканов и документов с большим числом изображений. Каждая страница будет отрендерена в изображение и обработана через OCR/VLM. Может увеличить время обработки и расходы на модели.',
    'common.remove': 'Удалить',
  },
};

export { uploadConfirmCopy };

/**
 * Same contract as formatMessage: locale table, en-US fallback, then the key
 * itself, with {placeholder} interpolation.
 */
export function uploadConfirmMessage(locale: Locale, key: string, values: Record<string, string | number> = {}): string {
  const template = uploadConfirmCopy[locale][key] ?? uploadConfirmCopy['en-US'][key] ?? key;
  return template.replace(/\{(\w+)\}/g, (_match, name: string) => String(values[name] ?? `{${name}}`));
}

/** t function for dialog copy that first tries shared i18n, then the local table. */
export function uploadConfirmT(locale: Locale): (key: string, values?: Record<string, string | number>) => string {
  return (key, values) => {
    const shared = formatMessage(locale, key, values);
    if (shared !== key) return shared;
    return uploadConfirmMessage(locale, key, values);
  };
}

export interface RunUploadPipelineInput {
  entries: readonly UploadEntry[];
  tagIds?: string[];
  signal?: AbortSignal;
  /** One call per file; resolves with the created document. */
  upload(entry: UploadEntry, tagIds: string[] | undefined, signal: AbortSignal | undefined): Promise<unknown>;
  onStateChange?(states: readonly UploadEntryState[]): void;
}

/**
 * Sequential multi-file upload (Vue parity: one upload per file with a
 * progress panel). A per-file failure does not stop the remaining files;
 * an aborted signal stops the batch at the next file boundary.
 */
export async function runUploadPipeline(input: RunUploadPipelineInput): Promise<readonly UploadEntryState[]> {
  const states: UploadEntryState[] = input.entries.map((entry) => ({ entry, status: 'pending' }));
  const emit = () => input.onStateChange?.([...states]);
  emit();
  for (let index = 0; index < states.length; index += 1) {
    if (input.signal?.aborted) break;
    const state = states[index];
    state.status = 'uploading';
    emit();
    try {
      await input.upload(state.entry, input.tagIds, input.signal);
      state.status = 'done';
    } catch (cause) {
      state.status = 'error';
      state.message = cause instanceof Error ? cause.message : 'Upload failed';
    }
    emit();
  }
  return states;
}
