import assert from 'node:assert/strict';
import test from 'node:test';

import {
  applyUploadOverrides,
  asrSectionIssue,
  batchHasAudio,
  batchHasImages,
  batchUploadExtensions,
  buildUploadConfirmOverrides,
  commitFolderName,
  defaultUploadConfirmUIState,
  destinationBreadcrumb,
  folderOptionFromPath,
  folderPickerRows,
  formatBytes,
  GRAPH_EXTRACT_DEFAULT_EXAMPLE,
  hasParserCustomization,
  inferMediaExtsFromMarkdown,
  joinFolderPath,
  mergeFolderOptions,
  mergeUploadEntries,
  multimodalSectionIssue,
  normalizeFolderPath,
  normalizeUploadUrl,
  parserNavStatus,
  removeUploadEntry,
  runUploadPipeline,
  retryableUploadEntries,
  sortFolderOptions,
  toUploadEntries,
  uploadConfirmMessage,
  uploadConfirmStateFromKb,
  uploadConfirmT,
  uploadEntryDisplayTitle,
  uploadEntryRelativeDir,
  uploadFileExtension,
  uploadSectionStatus,
  uploadSummary,
  uploadUrlExtension,
  uploadProgressPercent,
  batchUploadProgress,
  filterUploadFiles,
  type UploadConfirmUIState,
  type UploadEntry,
} from './upload-pipeline.ts';

function entry(name: string): File {
  return new File(['x'], name);
}

function entryWithSize(name: string, size: number): UploadEntry {
  return { file: new File(['x'], name), name, size };
}

test('multi-file pipeline uploads once per file with tag_ids', async () => {
  const entries = toUploadEntries([entry('a.pdf'), entry('b.docx'), entry('c.txt')]);
  const uploaded: string[] = [];
  const seenTags: (string[] | undefined)[] = [];
  const snapshots: number[] = [];
  const final = await runUploadPipeline({
    entries,
    tagIds: ['tag-1', 'tag-2'],
    upload: async (item, tagIds) => {
      uploaded.push(item.name);
      seenTags.push(tagIds);
      snapshots.push(snapshots.length);
    },
  });
  assert.deepEqual(uploaded, ['a.pdf', 'b.docx', 'c.txt']);
  for (const tags of seenTags) assert.deepEqual(tags, ['tag-1', 'tag-2']);
  assert.ok(final.every((state) => state.status === 'done'));
});

test('per-file error surfaces and does not stop the remaining uploads', async () => {
  const entries = toUploadEntries([entry('ok.txt'), entry('bad.txt'), entry('later.txt')]);
  const uploaded: string[] = [];
  const final = await runUploadPipeline({
    entries,
    upload: async (item) => {
      uploaded.push(item.name);
      if (item.name === 'bad.txt') throw new Error('413 too large');
    },
  });
  assert.deepEqual(uploaded, ['ok.txt', 'bad.txt', 'later.txt']);
  assert.equal(final[1].status, 'error');
  assert.equal(final[1].message, '413 too large');
  assert.equal(final[2].status, 'done');
});

test('cancel (pre-aborted signal) clears without uploading', async () => {
  const controller = new AbortController();
  controller.abort();
  let calls = 0;
  const final = await runUploadPipeline({
    entries: toUploadEntries([entry('a.pdf'), entry('b.pdf')]),
    signal: controller.signal,
    upload: async () => { calls += 1; },
  });
  assert.equal(calls, 0);
  assert.ok(final.every((state) => state.status === 'pending'));
});

test('abort mid-batch stops at the next file boundary', async () => {
  const controller = new AbortController();
  const uploaded: string[] = [];
  const final = await runUploadPipeline({
    entries: toUploadEntries([entry('a.pdf'), entry('b.pdf'), entry('c.pdf')]),
    signal: controller.signal,
    upload: async (item) => {
      uploaded.push(item.name);
      if (item.name === 'a.pdf') controller.abort();
    },
  });
  assert.deepEqual(uploaded, ['a.pdf']);
  assert.equal(final[0].status, 'done');
  assert.ok(final.slice(1).every((state) => state.status === 'pending'));
});

test('retryable upload entries contain only files that failed in the previous batch', async () => {
  const entries = toUploadEntries([entry('ok.txt'), entry('bad.txt')]);
  const final = await runUploadPipeline({
    entries,
    upload: async (item) => { if (item.name === 'bad.txt') throw new Error('temporary failure'); },
  });
  assert.deepEqual(retryableUploadEntries(final).map((item) => item.name), ['bad.txt']);
});

test('upload summary lists file names plus sizes for the confirm dialog', () => {
  const entries: UploadEntry[] = [entryWithSize('a.pdf', 1024), entryWithSize('b.txt', 512)];
  const summary = uploadSummary(entries);
  assert.equal(summary.count, 2);
  assert.equal(summary.totalLabel, '1.5 KB');
  assert.equal(formatBytes(512), '512 B');
});

test('removing a staged item preserves order and does not mutate the batch', () => {
  const entries = toUploadEntries([entry('a.pdf'), entry('b.pdf'), entry('c.pdf')]);
  assert.deepEqual(removeUploadEntry(entries, 1).map((item) => item.name), ['a.pdf', 'c.pdf']);
  assert.deepEqual(entries.map((item) => item.name), ['a.pdf', 'b.pdf', 'c.pdf']);
});

test('URL staging accepts HTTP(S), trims it, and rejects unsafe schemes', () => {
  assert.equal(normalizeUploadUrl('  https://example.com/docs?q=1  '), 'https://example.com/docs?q=1');
  assert.equal(normalizeUploadUrl('javascript:alert(1)'), null);
  assert.equal(normalizeUploadUrl(''), null);
});

test('filters upload sources like Vue before staging the confirmation batch', () => {
  const hidden = new File(['x'], 'notes.md');
  Object.defineProperty(hidden, 'webkitRelativePath', { value: 'docs/.private/notes.md' });
  const result = filterUploadFiles([
    new File(['x'], 'guide.pdf'),
    new File(['x'], 'movie.mp4'),
    new File(['x'], 'unknown.bin'),
    hidden,
  ], { supportedFileTypes: ['pdf', 'md'], fromFolder: true });

  assert.deepEqual(result.validFiles.map((file) => file.name), ['guide.pdf']);
  assert.equal(result.videoFilteredCount, 1);
  assert.equal(result.skippedCount, 1);
  assert.equal(result.hiddenFileCount, 1);
});

// --- Vue append/add-more parity ------------------------------------------------

test('appending staged files dedupes by (relative path|name)+size and reports counts', () => {
  const folderFile = new File(['x'], 'README.md');
  Object.defineProperty(folderFile, 'webkitRelativePath', { value: 'docs/README.md' });
  const existing: UploadEntry[] = [{ file: folderFile, name: 'README.md', size: 1 }, entryWithSize('a.pdf', 10)];
  const sameFolderFile = new File(['x'], 'README.md');
  Object.defineProperty(sameFolderFile, 'webkitRelativePath', { value: 'docs/README.md' });
  const otherFolderFile = new File(['x'], 'README.md');
  Object.defineProperty(otherFolderFile, 'webkitRelativePath', { value: 'spec/README.md' });
  const merged = mergeUploadEntries(existing, [
    { file: sameFolderFile, name: 'README.md', size: 1 },
    { file: otherFolderFile, name: 'README.md', size: 1 },
  ]);
  assert.equal(merged.addedCount, 1);
  assert.equal(merged.duplicateCount, 1);
  assert.equal(merged.entries.length, 3);
});

test('folder-upload rows show the relative directory and path-qualified title', () => {
  const file = new File(['x'], 'README.md');
  Object.defineProperty(file, 'webkitRelativePath', { value: 'docs/spec/README.md' });
  const staged: UploadEntry = { file, name: 'README.md', size: 1 };
  assert.equal(uploadEntryRelativeDir(staged), 'docs/spec');
  assert.equal(uploadEntryDisplayTitle(staged), 'docs/spec/README.md');
  const plain = entryWithSize('a.pdf', 1);
  assert.equal(uploadEntryRelativeDir(plain), '');
  assert.equal(uploadEntryDisplayTitle(plain), 'a.pdf');
});

// --- Vue batchFileExts parity (PDF/audio surfaces react to the whole batch) ----

test('batch extensions combine files, URL paths, manual markdown media and the reparse type', () => {
  assert.equal(uploadFileExtension('Report.PDF'), 'pdf');
  assert.equal(uploadUrlExtension('https://example.com/a/b.Report.pdf?x=1'), 'pdf');
  assert.equal(uploadUrlExtension('not a url'), '');
  assert.deepEqual(
    [...inferMediaExtsFromMarkdown('see ![logo](./img/logo.jpeg) and audio.mp3 here')].sort(),
    ['jpg', 'mp3'],
  );
  const exts = batchUploadExtensions({
    entries: toUploadEntries([entry('a.docx')]),
    urls: ['https://example.com/spec.pdf'],
    manualContent: '![](pic.png)',
    reparseFileType: 'PDF',
  });
  assert.deepEqual([...exts].sort(), ['docx', 'pdf', 'png']);
  assert.ok(batchHasImages(exts, '![](pic.png)'));
  assert.ok(!batchHasAudio(exts));
  assert.ok(batchHasAudio(batchUploadExtensions({ entries: toUploadEntries([entry('call.mp3')]) })));
});

// --- Vue folderTree helpers ----------------------------------------------------

test('folder path helpers normalize, join and derive picker options like Vue', () => {
  // Vue drops '.', '..' and empty segments without popping the previous one.
  assert.equal(normalizeFolderPath(' /a/./b/../c/ '), 'a/b/c');
  assert.equal(normalizeFolderPath('a\\b'), 'a/b');
  assert.equal(joinFolderPath('docs', 'New Folder'), 'docs/New Folder');
  assert.equal(joinFolderPath('', 'root'), 'root');
  assert.deepEqual(folderOptionFromPath('docs/spec'), { path: 'docs/spec', name: 'spec', depth: 1 });
  assert.deepEqual(
    sortFolderOptions([{ path: 'b/z' }, { path: 'a' }, { path: 'b/a' }]).map((option) => option.path),
    ['a', 'b/a', 'b/z'],
  );
});

test('picker options merge dialog-created pending folders with server folders', () => {
  const merged = mergeFolderOptions(
    [{ path: 'docs', name: 'docs', depth: 0 }],
    ['docs', 'new/spec'],
  );
  assert.deepEqual(merged.map((option) => option.path), ['docs', 'new/spec']);
  assert.deepEqual(merged.map((option) => option.depth), [0, 1]);
});

// --- Vue FolderPickerMenu rows + commit ----------------------------------------

test('picker rows render root, depth-indented folders and the inline create row under its parent', () => {
  const options = [
    { path: 'docs', name: 'docs', depth: 0 },
    { path: 'docs/spec', name: 'spec', depth: 1 },
  ];
  const idle = folderPickerRows(options, null, '根目录');
  assert.deepEqual(
    idle.map((row) => row.kind === 'folder' ? `${row.label}@${row.depth}` : `create@${row.parentPath}`),
    ['根目录@0', 'docs@0', 'spec@1'],
  );
  assert.ok(idle[0].kind === 'folder' && idle[0].isRoot);
  const creating = folderPickerRows(options, 'docs', '根目录');
  assert.equal(creating[2].kind, 'create');
  assert.ok(creating[2].kind === 'create' && creating[2].parentPath === 'docs' && creating[2].depth === 1);
  const creatingRoot = folderPickerRows(options, '', '根目录');
  assert.ok(creatingRoot[1].kind === 'create' && creatingRoot[1].parentPath === '' && creatingRoot[1].depth === 0);
});

test('committing a picker folder name normalizes, joins and rejects duplicates', () => {
  const options = [{ path: 'docs', name: 'docs', depth: 0 }];
  assert.deepEqual(commitFolderName(options, 'docs', '  Spec 21 . '), { status: 'created', path: 'docs/Spec 21' });
  assert.deepEqual(commitFolderName(options, '', 'root'), { status: 'created', path: 'root' });
  assert.deepEqual(commitFolderName(options, '', 'docs'), { status: 'duplicate', path: 'docs' });
  assert.equal(commitFolderName(options, '', '  .. ').status, 'invalid');
});

test('destination breadcrumb shows root / segment / segment', () => {
  assert.equal(destinationBreadcrumb('', '根目录'), '根目录');
  assert.equal(destinationBreadcrumb('docs/spec', '根目录'), '根目录 / docs / spec');
});

// --- Vue parser nav status ------------------------------------------------------

test('parser customization and nav status follow Vue rules', () => {
  assert.equal(hasParserCustomization(undefined), false);
  assert.equal(hasParserCustomization([{ file_types: ['pdf'], engine: 'builtin' }]), false);
  assert.equal(hasParserCustomization([{ file_types: ['pdf'], engine: 'mineru' }]), true);
  assert.equal(hasParserCustomization([{ file_types: ['xlsx'], engine: 'builtin', xlsx_first_row_as_header: true }]), true);
  assert.equal(parserNavStatus({ pdfForceScanned: true, hasPdf: true, rules: [] }), 'forceScanned');
  assert.equal(parserNavStatus({ pdfForceScanned: true, hasPdf: false, rules: [] }), 'default');
  assert.equal(parserNavStatus({ pdfForceScanned: false, hasPdf: true, rules: [{ file_types: ['pdf'], engine: 'mineru' }] }), 'customized');
  assert.equal(parserNavStatus({ pdfForceScanned: false, hasPdf: true, rules: [] }), 'default');
});

// --- Vue UploadUIState seeding and overrides ------------------------------------

test('dialog state seeds from knowledge base defaults like Vue initFromKbInfo', () => {
  const state = uploadConfirmStateFromKb({
    chunking_config: { chunk_size: 800, chunk_overlap: 120, strategy: 'heading', separators: ['\n'], parser_engine_rules: [{ file_types: ['pdf'], engine: 'mineru', xlsx_first_row_as_header: true }] },
    vlm_config: { enabled: true, model_id: 'vlm-1', description_language: 'English', custom_instructions: 'ctx' },
    asr_config: { enabled: true, model_id: 'asr-1', language: 'en' },
    question_generation_config: { enabled: false, question_count: 5, custom_instructions: 'q' },
  });
  assert.equal(state.chunkSize, 800);
  assert.equal(state.chunkOverlap, 120);
  assert.equal(state.chunkStrategy, 'heading');
  assert.deepEqual(state.separators, ['\n']);
  assert.deepEqual(state.parserRules, [{ file_types: ['pdf'], engine: 'mineru', xlsx_first_row_as_header: true }]);
  assert.equal(state.multimodalEnabled, true);
  assert.equal(state.vllmModelId, 'vlm-1');
  assert.equal(state.descriptionLanguage, 'English');
  assert.equal(state.asrEnabled, true);
  assert.equal(state.asrModelId, 'asr-1');
  assert.equal(state.questionEnabled, false);
  assert.equal(state.questionCount, 5);
  // pdfForceScanned is per-batch, never a KB default.
  assert.equal(state.pdfForceScanned, false);
  assert.equal(uploadConfirmStateFromKb(null).chunkSize, 512);
});

test('reparse seeding applies stored overrides and reads back pdf_force_scanned', () => {
  const base = uploadConfirmStateFromKb({ chunking_config: { chunk_size: 512 } });
  const seeded = applyUploadOverrides(base, {
    chunking_config: { chunk_size: 1024, chunk_overlap: 200, separators: ['。'], enable_parent_child: false, strategy: 'heuristic', token_limit: 512, languages: ['zh'] },
    parser_engine_rules: [{ file_types: ['pdf'], engine: 'paddleocr-vl' }],
    vlm_config: { enabled: true, model_id: 'vlm-9' },
    asr_config: { enabled: true, model_id: 'asr-9', language: 'zh' },
    question_generation_config: { enabled: true, question_count: 7 },
    parser_engine_overrides: { pdf_force_scanned: 'true' },
  });
  assert.equal(seeded.chunkSize, 1024);
  assert.equal(seeded.enableParentChild, false);
  assert.equal(seeded.chunkStrategy, 'heuristic');
  assert.equal(seeded.tokenLimit, 512);
  assert.deepEqual(seeded.languages, ['zh']);
  assert.deepEqual(seeded.parserRules, [{ file_types: ['pdf'], engine: 'paddleocr-vl' }]);
  assert.equal(seeded.vllmModelId, 'vlm-9');
  assert.equal(seeded.asrModelId, 'asr-9');
  assert.equal(seeded.questionCount, 7);
  assert.equal(seeded.pdfForceScanned, true);
  assert.equal(applyUploadOverrides(base, null), base);
  assert.equal(applyUploadOverrides(base, {}).pdfForceScanned, false);
});

test('confirm overrides build the full process_config payload like Vue buildProcessOverrides', () => {
  const state: UploadConfirmUIState = {
    ...defaultUploadConfirmUIState(),
    chunkSize: 640,
    chunkOverlap: 100,
    chunkStrategy: 'heading',
    parserRules: [{ file_types: ['pdf'], engine: 'mineru' }],
    multimodalEnabled: true,
    vllmModelId: 'vlm-1',
    questionEnabled: true,
    questionCount: 4,
    pdfForceScanned: true,
  };
  const overrides = buildUploadConfirmOverrides(state);
  assert.equal(overrides.enable_multimodel, true);
  assert.deepEqual(overrides.vlm_config, { enabled: true, model_id: 'vlm-1', description_language: '', custom_instructions: '' });
  assert.deepEqual(overrides.chunking_config?.chunk_size, 640);
  assert.deepEqual(overrides.chunking_config?.strategy, 'heading');
  assert.deepEqual(overrides.question_generation_config, { enabled: true, question_count: 4, custom_instructions: '' });
  assert.deepEqual(overrides.parser_engine_overrides, { pdf_force_scanned: 'true' });
  assert.equal(buildUploadConfirmOverrides({ ...state, pdfForceScanned: false }).parser_engine_overrides, undefined);
  assert.equal(buildUploadConfirmOverrides({ ...state, parserRules: [] }).parser_engine_rules, undefined);
});

// --- Section nav status + required-model issues ---------------------------------

function navInput(state: UploadConfirmUIState) {
  return {
    state,
    selectedTagCount: 2,
    hasPdf: true,
    hasImages: false,
    hasAudio: false,
    vllmModelName: undefined,
    asrModelName: undefined,
  };
}

test('section nav statuses mirror Vue getSectionNavStatus', () => {
  const state = defaultUploadConfirmUIState();
  assert.deepEqual(uploadSectionStatus('tags', navInput(state)), { key: 'uploadConfirm.summaryTagsCount', values: { count: 2 } });
  assert.equal(uploadSectionStatus('tags', { ...navInput(state), selectedTagCount: 0 })?.key, 'uploadConfirm.summaryNoTags');
  assert.equal(uploadSectionStatus('parser', navInput(state))?.key, 'uploadConfirm.navParserDefault');
  assert.equal(uploadSectionStatus('parser', navInput({ ...state, pdfForceScanned: true }))?.key, 'uploadConfirm.summaryParserForceScanned');
  assert.equal(uploadSectionStatus('parser', navInput({ ...state, parserRules: [{ file_types: ['pdf'], engine: 'mineru' }] }))?.key, 'uploadConfirm.navParserCustomized');
  const chunking = uploadSectionStatus('chunking', navInput(state));
  assert.equal(chunking?.key, 'uploadConfirm.navChunkingSummary');
  assert.equal(chunking?.values?.size, 512);
  assert.equal(chunking?.suffixKey, 'uploadConfirm.summaryParentChildShort');
  assert.equal(uploadSectionStatus('multimodal', navInput(state))?.key, 'uploadConfirm.statusOff');
  // Vue routes enabled-without-model through the issue flag (needs setup, error tone).
  const needsSetup = uploadSectionStatus('multimodal', navInput({ ...state, multimodalEnabled: true }));
  assert.equal(needsSetup?.key, 'uploadConfirm.statusNeedsSetup');
  assert.equal(needsSetup?.tone, 'error');
  assert.equal(uploadSectionStatus('multimodal', navInput({ ...state, multimodalEnabled: true, vllmModelId: 'vlm-1' }))?.text, 'vlm-1');
  assert.equal(uploadSectionStatus('question', navInput({ ...state, questionEnabled: true, questionCount: 3 }))?.values?.count, 3);
});

test('multimodal and ASR section issues block confirm exactly when Vue does', () => {
  const state = defaultUploadConfirmUIState();
  assert.equal(multimodalSectionIssue({ state, hasImages: true }), true);
  assert.equal(multimodalSectionIssue({ state: { ...state, multimodalEnabled: true, vllmModelId: 'm' }, hasImages: true }), false);
  assert.equal(multimodalSectionIssue({ state: { ...state, multimodalEnabled: true, vllmModelId: '' }, hasImages: false }), true);
  assert.equal(multimodalSectionIssue({ state: { ...state, multimodalEnabled: false }, hasImages: false }), false);
  assert.equal(asrSectionIssue({ state, hasAudio: true }), true);
  assert.equal(asrSectionIssue({ state: { ...state, asrEnabled: true, asrModelId: 'a' }, hasAudio: true }), false);
  assert.equal(asrSectionIssue({ state: { ...state, asrEnabled: true, asrModelId: '' }, hasAudio: false }), true);
});

// --- Dialog copy table ----------------------------------------------------------

test('dialog copy resolves five locales with en-US fallback and interpolation', () => {
  assert.equal(uploadConfirmMessage('zh-CN', 'uploadConfirm.title'), '上传文档确认');
  assert.equal(uploadConfirmMessage('en-US', 'uploadConfirm.title'), 'Confirm Upload');
  assert.equal(uploadConfirmMessage('ja-JP', 'uploadConfirm.title'), 'アップロードの確認');
  assert.equal(uploadConfirmMessage('ko-KR', 'uploadConfirm.title'), '문서 업로드 확인');
  assert.equal(uploadConfirmMessage('ru-RU', 'uploadConfirm.title'), 'Подтверждение загрузки');
  assert.equal(uploadConfirmMessage('zh-CN', 'uploadConfirm.pdfForceScanned.label'), '按扫描件解析 PDF');
  assert.equal(uploadConfirmMessage('en-US', 'uploadConfirm.filesAdded', { count: 3 }), 'Added 3 file(s)');
  assert.equal(uploadConfirmMessage('zh-CN', 'uploadConfirm.filesAdded', { count: 3 }), '已添加 3 个文件');
  assert.equal(uploadConfirmMessage('ru-RU', 'nonexistent.key'), 'nonexistent.key');
});

test('uploadConfirmT prefers shared i18n keys before the local dialog table', () => {
  const t = uploadConfirmT('zh-CN');
  // Shared knowledgeEditor key resolves through @weknora/i18n.
  assert.equal(t('knowledgeEditor.sidebar.chunking'), '分块设置');
  // Dialog-only key falls back to the ported Vue copy.
  assert.equal(t('uploadConfirm.destinationLabel'), '上传位置');
  assert.equal(t('uploadConfirm.reparseHint'), '将沿用上次解析的配置，可在此调整');
});

// --- N007: graph section state, payload and gating (Vue UploadConfirmDialog) -------

test('graph state seeds from kb extract_config and indexing_strategy like Vue initFromKbInfo', () => {
  const state = uploadConfirmStateFromKb({
    indexing_strategy: { graph_enabled: true },
    extract_config: {
      enabled: true,
      text: 'sample',
      tags: ['Author'],
      nodes: [{ name: 'Verona', attributes: ['City in Italy'] }, { name: 'NoAttrs' }],
      relations: [{ node1: 'A', node2: 'B', type: 'Alias' }],
      custom_instructions: 'ctx',
    },
  });
  assert.equal(state.graphEnabled, true);
  assert.equal(state.nodeExtract.enabled, true);
  assert.equal(state.nodeExtract.text, 'sample');
  assert.deepEqual(state.nodeExtract.tags, ['Author']);
  assert.deepEqual(state.nodeExtract.nodes, [
    { name: 'Verona', attributes: ['City in Italy'] },
    { name: 'NoAttrs', attributes: [] },
  ]);
  assert.deepEqual(state.nodeExtract.relations, [{ node1: 'A', node2: 'B', type: 'Alias' }]);
  assert.equal(state.nodeExtract.customInstructions, 'ctx');

  // Vue: nodeExtractConfig.enabled = extract.enabled && indexing.graph_enabled.
  const graphOff = uploadConfirmStateFromKb({
    indexing_strategy: { graph_enabled: false },
    extract_config: { enabled: true, text: '', tags: [], nodes: [], relations: [] },
  });
  assert.equal(graphOff.graphEnabled, false);
  assert.equal(graphOff.nodeExtract.enabled, false);

  // Defaults and missing kb mirror Vue createDefaultUIState.
  const base = defaultUploadConfirmUIState();
  assert.equal(base.graphEnabled, false);
  assert.deepEqual(base.nodeExtract, { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '' });
  assert.equal(uploadConfirmStateFromKb(null).graphEnabled, false);
});

test('graph override payload matches Vue buildProcessOverrides', () => {
  const state: UploadConfirmUIState = {
    ...defaultUploadConfirmUIState(),
    graphEnabled: true,
    nodeExtract: {
      enabled: true,
      text: 'sample text',
      tags: ['Author', 'Alias'],
      nodes: [{ name: 'Verona', attributes: ['City in Italy'] }],
      relations: [{ node1: 'Verona', node2: 'Romeo', type: 'Setting' }],
      customInstructions: 'focus',
    },
  };
  const overrides = buildUploadConfirmOverrides(state);
  // Vue: graph_enabled = nodeExtract.enabled && graphEnabled.
  assert.equal(overrides.graph_enabled, true);
  assert.deepEqual(overrides.extract_config, {
    enabled: true,
    text: 'sample text',
    tags: ['Author', 'Alias'],
    nodes: [{ name: 'Verona', attributes: ['City in Italy'] }],
    relations: [{ node1: 'Verona', node2: 'Romeo', type: 'Setting' }],
    custom_instructions: 'focus',
  });

  // Extraction toggle off wins even with the kb flag set.
  const disabled = buildUploadConfirmOverrides({ ...state, nodeExtract: { ...state.nodeExtract, enabled: false } });
  assert.equal(disabled.graph_enabled, false);
  assert.equal(disabled.extract_config?.enabled, false);

  // Default state keeps both keys present but off (Vue always emits them).
  const base = buildUploadConfirmOverrides(defaultUploadConfirmUIState());
  assert.equal(base.graph_enabled, false);
  assert.deepEqual(base.extract_config, { enabled: false, text: '', tags: [], nodes: [], relations: [], custom_instructions: '' });
});

test('reparse overrides apply extract_config and clamp enabled by graph_enabled', () => {
  const base = uploadConfirmStateFromKb({
    indexing_strategy: { graph_enabled: true },
    extract_config: { enabled: true, text: 'kb text', tags: ['T'], nodes: [], relations: [] },
  });
  const seeded = applyUploadOverrides(base, {
    extract_config: {
      enabled: true,
      text: 'stored text',
      tags: ['Author'],
      nodes: [{ name: 'Romeo', attributes: [] }],
      relations: [{ node1: 'Romeo', node2: 'Juliet', type: 'Loves' }],
      custom_instructions: 're',
    },
    graph_enabled: true,
  });
  assert.equal(seeded.graphEnabled, true);
  assert.equal(seeded.nodeExtract.enabled, true);
  assert.equal(seeded.nodeExtract.text, 'stored text');

  // Vue applyOverridesToState L1240-1241: graph_enabled=false clamps extract enabled.
  const clamped = applyUploadOverrides(base, { extract_config: { enabled: true }, graph_enabled: false });
  assert.equal(clamped.graphEnabled, false);
  assert.equal(clamped.nodeExtract.enabled, false);

  // graph_enabled alone can re-arm the flag without extract data.
  const rearmed = applyUploadOverrides(base, { graph_enabled: true });
  assert.equal(rearmed.graphEnabled, true);
});

test('graph default example matches the Vue GraphSettings fixture', () => {
  assert.equal(GRAPH_EXTRACT_DEFAULT_EXAMPLE.tags.join(','), 'Author,Alias');
  assert.equal(GRAPH_EXTRACT_DEFAULT_EXAMPLE.nodes.length, 4);
  assert.equal(GRAPH_EXTRACT_DEFAULT_EXAMPLE.relations.length, 3);
  assert.match(GRAPH_EXTRACT_DEFAULT_EXAMPLE.text, /Romeo and Juliet/);
});

// --- Byte-backed upload progress (Vue uploadKnowledgeFile + KnowledgeBaseList panel) --

test('uploadProgressPercent mirrors the Vue byte math and clamps', () => {
  // frontend/src/api/system/index.ts:1117 — Math.round((loaded * 100) / total).
  assert.equal(uploadProgressPercent({ loaded: 0, total: 1000 }), 0);
  assert.equal(uploadProgressPercent({ loaded: 333, total: 1000 }), 33);
  assert.equal(uploadProgressPercent({ loaded: 1500, total: 1000 }), 100);
  // KnowledgeBaseList.vue:1601 clampProgress guards unknown totals.
  assert.equal(uploadProgressPercent({ loaded: 12, total: 0 }), 0);
});

test('pipeline forwards byte-backed progress into per-file state', async () => {
  const snapshots: Array<Array<{ name: string; status: string; progress: number }>> = [];
  const final = await runUploadPipeline({
    entries: toUploadEntries([entry('a.pdf'), entry('b.pdf')]),
    upload: async (_item, _tagIds, _signal, onProgress) => {
      onProgress?.(25);
      onProgress?.(50);
      onProgress?.(140); // clamped to 100 like Vue clampProgress
    },
    onStateChange: (states) => snapshots.push(states.map((state) => ({
      name: state.entry.name,
      status: state.status,
      progress: state.progress ?? 0,
    }))),
  });
  const whileUploading = snapshots.find((snapshot) => snapshot[0]!.status === 'uploading' && snapshot[0]!.progress === 50);
  assert.ok(whileUploading, 'per-file percent is emitted while uploading');
  assert.deepEqual(final.map((state) => ({ status: state.status, progress: state.progress })), [
    { status: 'done', progress: 100 },
    { status: 'done', progress: 100 },
  ]);
});

test('batchUploadProgress averages per-file progress like the Vue upload panel', () => {
  // KnowledgeBaseList.vue:1586-1595 — mean of per-task progress, rounded, clamped.
  const states = [
    { entry: entryWithSize('a.pdf', 1), status: 'done' as const, progress: 100 },
    { entry: entryWithSize('b.pdf', 1), status: 'uploading' as const, progress: 40 },
    { entry: entryWithSize('c.pdf', 1), status: 'pending' as const, progress: 0 },
  ];
  assert.equal(batchUploadProgress(states), Math.round((100 + 40 + 0) / 3));
  assert.equal(batchUploadProgress([]), 0);
});
