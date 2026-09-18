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
  /** Byte-backed upload percent (0-100) while uploading; 100 once done. */
  progress?: number;
}

/**
 * Percent from byte-level upload progress — the exact Vue api math
 * (frontend/src/api/system/index.ts:1117 Math.round(loaded*100/total)) plus the
 * KnowledgeBaseList.vue:1601 clampProgress guard for unknown totals.
 */
export function uploadProgressPercent(progress: { loaded: number; total: number }): number {
  if (!progress.total || progress.total <= 0) return 0;
  return Math.min(100, Math.max(0, Math.round((progress.loaded * 100) / progress.total)));
}

/** Clamp any reported percent into 0-100 (KnowledgeBaseList.vue:1601). */
export function clampUploadPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(100, Math.max(0, Math.round(value)));
}

/**
 * Batch percent for the upload mask — mirrors the Vue upload panel aggregate
 * (KnowledgeBaseList.vue:1586-1595): the mean of the per-task progress values,
 * rounded and clamped.
 */
export function batchUploadProgress(states: readonly UploadEntryState[]): number {
  if (states.length === 0) return 0;
  const sum = states.reduce((total, state) => total + (state.status === 'done' ? 100 : state.progress ?? 0), 0);
  return clampUploadPercent(sum / states.length);
}

/** Collect a FileList/drop into stable entries before the confirm dialog. */
export function toUploadEntries(files: Iterable<File>): UploadEntry[] {
  return Array.from(files).map((file) => ({ file, name: file.name, size: file.size }));
}

export const UPLOAD_VIDEO_EXTENSIONS = ['mp4', 'mov', 'avi', 'mkv', 'webm', 'wmv', 'flv'];
export const DEFAULT_UPLOAD_EXTENSIONS = [
  'pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'epub', 'mhtml',
  'txt', 'md', 'csv', 'json', 'xml', 'html', 'markdown', 'yaml', 'yml', 'log',
  'jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp',
  'mp3', 'wav', 'm4a', 'flac', 'ogg', 'aac',
];

export interface FilterUploadFilesOptions {
  supportedFileTypes?: Iterable<string>;
  fromFolder?: boolean;
}

export interface FilterUploadFilesResult {
  validFiles: File[];
  skippedCount: number;
  videoFilteredCount: number;
  hiddenFileCount: number;
}

/** Vue KbUploadSourceDropdown filtering before files enter UploadConfirmDialog. */
export function filterUploadFiles(files: Iterable<File>, options: FilterUploadFilesOptions = {}): FilterUploadFilesResult {
  const supported = new Set(Array.from(options.supportedFileTypes ?? []).map((type) => type.replace(/^\./, '').toLowerCase()));
  const accepted = supported.size > 0 ? supported : new Set(DEFAULT_UPLOAD_EXTENSIONS);
  const validFiles: File[] = [];
  let skippedCount = 0;
  let videoFilteredCount = 0;
  let hiddenFileCount = 0;

  for (const file of files) {
    const relativePath = (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name;
    if (options.fromFolder && relativePath.split('/').some((part) => part.startsWith('.'))) {
      hiddenFileCount += 1;
      continue;
    }
    const extension = uploadFileExtension(file.name);
    if (UPLOAD_VIDEO_EXTENSIONS.includes(extension)) {
      videoFilteredCount += 1;
      continue;
    }
    if (!accepted.has(extension)) {
      skippedCount += 1;
      continue;
    }
    validFiles.push(file);
  }
  return { validFiles, skippedCount, videoFilteredCount, hiddenFileCount };
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

/** Vue GraphSettings node row (extract_config.nodes). */
export interface UploadGraphNodeState {
  name: string;
  attributes: string[];
}

/** Vue GraphSettings relation row (extract_config.relations). */
export interface UploadGraphRelationState {
  node1: string;
  node2: string;
  type: string;
}

/** Vue UploadUIState.nodeExtractConfig (extract_config). */
export interface UploadNodeExtractState {
  enabled: boolean;
  text: string;
  tags: string[];
  nodes: UploadGraphNodeState[];
  relations: UploadGraphRelationState[];
  customInstructions: string;
}

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
  /** Vue uiState.nodeExtractConfig (kb extract_config). */
  nodeExtract: UploadNodeExtractState;
  /** Vue uiState.graphEnabled (kb indexing_strategy.graph_enabled). */
  graphEnabled: boolean;
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
    nodeExtract: {
      enabled: false,
      text: '',
      tags: [],
      nodes: [],
      relations: [],
      customInstructions: '',
    },
    graphEnabled: false,
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

/**
 * Vue `value || fallback` seeding (UploadConfirmDialog L1096-1102): live KBs
 * store 0 for "not customized", so a falsy number falls back to the default
 * instead of surfacing an invalid 0 in the dialog state.
 */
function vueNumOr(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value !== 0 ? value : fallback;
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

/** Vue initFromKbInfo node mapping: keep the name, default missing attributes. */
function parseGraphNodes(value: unknown): UploadGraphNodeState[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((node): UploadGraphNodeState[] => {
    if (!node || typeof node !== 'object') return [];
    const row = node as Record<string, unknown>;
    if (typeof row.name !== 'string') return [];
    return [{
      name: row.name,
      attributes: Array.isArray(row.attributes) && row.attributes.every((item) => typeof item === 'string')
        ? row.attributes as string[]
        : [],
    }];
  });
}

/** Vue relation passthrough: node1/node2/type strings only. */
function parseGraphRelations(value: unknown): UploadGraphRelationState[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((relation): UploadGraphRelationState[] => {
    if (!relation || typeof relation !== 'object') return [];
    const row = relation as Record<string, unknown>;
    if (typeof row.node1 !== 'string' || typeof row.node2 !== 'string' || typeof row.type !== 'string') return [];
    return [{ node1: row.node1, node2: row.node2, type: row.type }];
  });
}

/** Vue initFromKbInfo: seed the dialog from the knowledge base defaults. */
export function uploadConfirmStateFromKb(kb: KbLike): UploadConfirmUIState {
  const state = defaultUploadConfirmUIState();
  if (!kb) return state;
  const chunking = kbRecord(kb, 'chunking_config');
  if (chunking) {
    // Vue initFromKbInfo seeds with `||` semantics: the live KB "not
    // customized" zeros (chunk_size 0, chunk_overlap 0) fall back to the
    // Vue defaults instead of entering the dialog as invalid values.
    state.chunkSize = vueNumOr(chunking.chunk_size, 512);
    state.chunkOverlap = vueNumOr(chunking.chunk_overlap, 80);
    state.separators = strList(chunking.separators, state.separators);
    state.parserRules = parseEngineRules(chunking.parser_engine_rules);
    if (Array.isArray(chunking.parser_engine_rules) && chunking.parser_engine_rules.length === 0) state.parserRules = [];
    state.enableParentChild = chunking.enable_parent_child === true;
    state.parentChunkSize = vueNumOr(chunking.parent_chunk_size, 4096);
    state.childChunkSize = vueNumOr(chunking.child_chunk_size, 384);
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
  // Vue L1124-1135: graphEnabled from indexing_strategy.graph_enabled ?? false;
  // extract.enabled requires the kb graph flag too.
  const indexing = kbRecord(kb, 'indexing_strategy');
  state.graphEnabled = indexing?.graph_enabled === true;
  const extract = kbRecord(kb, 'extract_config');
  state.nodeExtract = {
    enabled: extract?.enabled === true && state.graphEnabled,
    text: str(extract?.text),
    tags: strList(extract?.tags, []),
    nodes: parseGraphNodes(extract?.nodes),
    relations: parseGraphRelations(extract?.relations),
    customInstructions: str(extract?.custom_instructions),
  };
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
  // Vue L1231-1241: extract_config fields, then graph_enabled, then the clamp
  // nodeExtract.enabled = enabled && graphEnabled. Copy-on-write so the base
  // state's nested graph config is never mutated.
  const nodeExtract: UploadNodeExtractState = { ...state.nodeExtract };
  const ec = overrides.extract_config;
  if (ec) {
    if (typeof ec.enabled === 'boolean') nodeExtract.enabled = ec.enabled;
    if (typeof ec.text === 'string') nodeExtract.text = ec.text;
    if (Array.isArray(ec.tags) && ec.tags.every((item) => typeof item === 'string')) nodeExtract.tags = ec.tags as string[];
    if (Array.isArray(ec.nodes)) nodeExtract.nodes = parseGraphNodes(ec.nodes);
    if (Array.isArray(ec.relations)) nodeExtract.relations = parseGraphRelations(ec.relations);
    if (typeof ec.custom_instructions === 'string') nodeExtract.customInstructions = ec.custom_instructions;
  }
  if (typeof overrides.graph_enabled === 'boolean') state.graphEnabled = overrides.graph_enabled;
  nodeExtract.enabled = nodeExtract.enabled && state.graphEnabled;
  state.nodeExtract = nodeExtract;
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
    // Vue L1175-1183: graph_enabled = extract.enabled && graphEnabled; the
    // extract config itself passes through untouched.
    graph_enabled: state.nodeExtract.enabled && state.graphEnabled,
    extract_config: {
      enabled: state.nodeExtract.enabled,
      text: state.nodeExtract.text,
      tags: state.nodeExtract.tags,
      nodes: state.nodeExtract.nodes,
      relations: state.nodeExtract.relations,
      custom_instructions: state.nodeExtract.customInstructions,
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

/** Vue ConfigSectionKey (UploadConfirmDialog L579). */
export type UploadConfirmSectionKey = 'tags' | 'parser' | 'chunking' | 'multimodal' | 'asr' | 'question' | 'graph';

/**
 * Per-section nav status text (Vue getSectionNavStatus, L946-1022).
 */
export function uploadSectionStatus(section: UploadConfirmSectionKey, input: SectionNavStatusInput): UploadSectionStatus | null {
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
    case 'graph': {
      // Vue L1009-1018: off until both flags hold, then the tag count (or plain on).
      if (!input.state.graphEnabled || !input.state.nodeExtract.enabled) {
        return { key: 'uploadConfirm.statusOff', tone: 'muted' };
      }
      const tagCount = input.state.nodeExtract.tags.length;
      if (tagCount > 0) {
        return { key: 'uploadConfirm.summaryGraphTagsValue', values: { count: tagCount } };
      }
      return { key: 'uploadConfirm.statusOn' };
    }
    default:
      return null;
  }
}

// --- Graph section availability (Vue isGraphSectionAvailable L861-868) --------

/** Loose SystemInfo shape; only graph_database_engine matters here. */
export type UploadSystemInfo = Record<string, unknown> | null | undefined;

/** Vue isGraphDatabaseEnabled: an engine string that is set and not "Not Enabled". */
export function graphDatabaseEnabled(systemInfo: UploadSystemInfo): boolean {
  const engine = systemInfo?.graph_database_engine;
  return !!engine && engine !== 'Not Enabled';
}

/** Vue isGraphSectionAvailable: engine enabled AND the kb graph flag. */
export function graphSectionAvailable(input: { systemInfo: UploadSystemInfo; graphEnabled: boolean }): boolean {
  return graphDatabaseEnabled(input.systemInfo) && input.graphEnabled;
}

/** Vue getDefaultSection (L1043-1048). */
export function defaultUploadConfirmSection(input: { mode: 'file' | 'manual' | 'reparse'; multimodalIssue: boolean; asrIssue: boolean }): UploadConfirmSectionKey {
  if (input.mode === 'reparse') return 'parser';
  if (input.multimodalIssue) return 'multimodal';
  if (input.asrIssue) return 'asr';
  return 'tags';
}

/** Vue watch(isGraphSectionAvailable) (L1309-1313): leaving graph falls back. */
export function sectionAfterGraphAvailabilityChange(current: UploadConfirmSectionKey, graphAvailable: boolean, fallback: UploadConfirmSectionKey): UploadConfirmSectionKey {
  if (!graphAvailable && current === 'graph') return fallback;
  return current;
}

/**
 * Vue GraphSettings canRunGraphExtract: tenant role at least admin (admin=30,
 * owner=40; cross-tenant superuser grants temporary admin). UI gating only —
 * the server routes (internal/router/routes_infra.go:123-125) re-check authz.
 */
export function hasGraphAdminRole(me: unknown): boolean {
  const record = me as { user?: { role?: unknown; is_superuser?: unknown }; memberships?: { role?: unknown }[]; can_access_all_tenants?: unknown } | null | undefined;
  if (!record) return false;
  const role = typeof record.user?.role === 'string' ? record.user.role : '';
  if (role === 'admin' || role === 'owner' || role === 'system_admin') return true;
  if (record.user?.is_superuser === true || record.can_access_all_tenants === true) return true;
  return Array.isArray(record.memberships)
    && record.memberships.some((membership) => membership?.role === 'admin' || membership?.role === 'owner' || membership?.role === 'system_admin');
}

/**
 * Vue GraphSettings defaultExtractExample (L549-565): the Shakespeare sample
 * loaded by the "default example" button, byte-exact.
 */
export const GRAPH_EXTRACT_DEFAULT_EXAMPLE: Readonly<{
  text: string;
  tags: string[];
  nodes: UploadGraphNodeState[];
  relations: UploadGraphRelationState[];
}> = {
  text: '"Romeo and Juliet" is a tragedy written by William Shakespeare early in his career, and is one of the most frequently performed plays in world literature. The play follows two young lovers from feuding families in Verona, Italy — the Montagues and the Capulets. Written around 1594-1596, it was first published in quarto in 1597. The full title is "The Most Excellent and Lamentable Tragedy of Romeo and Juliet." The story has been adapted countless times for stage, film, and other media.',
  tags: ['Author', 'Alias'],
  nodes: [
    { name: 'Romeo and Juliet', attributes: ['One of the most frequently performed plays', 'Written around 1594-1596', 'A tragedy'] },
    { name: 'The Most Excellent and Lamentable Tragedy of Romeo and Juliet', attributes: ['Full title of Romeo and Juliet'] },
    { name: 'William Shakespeare', attributes: ['English playwright', 'Author of Romeo and Juliet'] },
    { name: 'Verona', attributes: ['City in Italy', 'Setting of the play'] },
  ],
  relations: [
    { node1: 'Romeo and Juliet', node2: 'The Most Excellent and Lamentable Tragedy of Romeo and Juliet', type: 'Alias' },
    { node1: 'Romeo and Juliet', node2: 'William Shakespeare', type: 'Author' },
    { node1: 'Romeo and Juliet', node2: 'Verona', type: 'Setting' },
  ],
};

// Dialog copy now resolves through the shared i18n bundle
// (packages/i18n/src/generated/uploadConfirm.ts - the local byte-exact table
// that used to live here was migrated there per this file's own header note).
/** t function for dialog copy via the shared i18n bundle. */
export function uploadConfirmT(locale: Locale): (key: string, values?: Record<string, string | number>) => string {
  return (key, values) => formatMessage(locale, key, values);
}

/** Same contract as the former local table: locale resolve, en-US fallback, key echo. */
export function uploadConfirmMessage(locale: Locale, key: string, values: Record<string, string | number> = {}): string {
  return formatMessage(locale, key, values);
}


export interface RunUploadPipelineInput {
  entries: readonly UploadEntry[];
  tagIds?: string[];
  signal?: AbortSignal;
  /**
   * One call per file; resolves with the created document. onProgress receives
   * a clamped 0-100 percent derived from the transport's byte-level upload
   * events (Vue uploadKnowledgeFile onProgress parity).
   */
  upload(
    entry: UploadEntry,
    tagIds: string[] | undefined,
    signal: AbortSignal | undefined,
    onProgress?: (percent: number) => void,
  ): Promise<unknown>;
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
    state.progress = 0;
    emit();
    try {
      await input.upload(state.entry, input.tagIds, input.signal, (percent) => {
        const next = clampUploadPercent(percent);
        if (next === state.progress) return;
        state.progress = next;
        emit();
      });
      state.status = 'done';
      state.progress = 100;
    } catch (cause) {
      state.status = 'error';
      state.message = cause instanceof Error ? cause.message : 'Upload failed';
    }
    emit();
  }
  return states;
}

/** Return the staged file subset that is safe to send through a retry. */
export function retryableUploadEntries(states: readonly UploadEntryState[]): UploadEntry[] {
  return states.filter((state) => state.status === 'error').map((state) => state.entry);
}
