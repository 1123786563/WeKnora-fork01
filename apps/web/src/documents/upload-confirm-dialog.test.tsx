import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import type { ParserEngineInfo } from '@weknora/api-client';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css') || specifier.endsWith('.svg')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const {
  UploadConfirmSections,
  UploadDestinationPicker,
  UploadFilesPanel,
  UploadGraphSettings,
  UploadSectionNav,
  UploadSourceDropdown,
  stageNoticeClass,
  clampGraphTextareaHeight,
  GraphTagsField,
  GraphSwitch,
  GraphRelationSelect,
  moveGraphRelationOption,
  UploadSingleSelect,
  uploadConfirmValidationFailure,
  canCloseUploadConfirmDialog,
} = await import('./KnowledgeDocumentsPage.tsx');
const {
  uploadConfirmT,
  uploadConfirmMessage,
  defaultUploadConfirmUIState,
  defaultUploadConfirmSection,
  uploadConfirmStateFromKb,
  graphSectionAvailable,
  sectionAfterGraphAvailabilityChange,
  toUploadEntries,
  uploadSectionStatus,
} = await import('./upload-pipeline.ts');

const ct = uploadConfirmT('zh-CN');
const noop = () => {};

// --- Submit validation and close lifecycle ----------------------------------------

test('media validation identifies the Vue configuration section to reveal before submission', () => {
  // This catches a regression where a missing required model only emits an
  // error message, leaving the invalid controls hidden in another section.
  assert.equal(typeof uploadConfirmValidationFailure, 'function');
  const state = defaultUploadConfirmUIState();
  assert.deepEqual(uploadConfirmValidationFailure({
    state,
    hasImages: true,
    hasAudio: false,
  }), {
    section: 'multimodal',
    messageKey: 'uploadConfirm.vlmModelRequired',
    patch: { multimodalEnabled: true },
  });
  assert.deepEqual(uploadConfirmValidationFailure({
    state,
    hasImages: false,
    hasAudio: true,
  }), {
    section: 'asr',
    messageKey: 'uploadConfirm.asrModelRequired',
    patch: { asrEnabled: true },
  });
});

test('upload confirmation cannot be dismissed while its request is in flight', () => {
  // This catches a cancellation path that clears staged files while the upload
  // pipeline is still using them.
  assert.equal(typeof canCloseUploadConfirmDialog, 'function');
  assert.equal(canCloseUploadConfirmDialog(false), true);
  assert.equal(canCloseUploadConfirmDialog(true), false);
});

test('upload confirmation stays mounted while closed like the Vue v-show host', () => {
  const source = readFileSync(new URL('./KnowledgeDocumentsPage.tsx', import.meta.url), 'utf8');
  assert.match(source, /<Dialog\s+open=\{uploadDialogOpen\}/);
  assert.doesNotMatch(source, /\{uploadDialogOpen \? \(/);
});

test('upload confirmation footer keeps the Vue cancel-then-confirm action order', () => {
  const source = readFileSync(new URL('./KnowledgeDocumentsPage.tsx', import.meta.url), 'utf8');
  const footerStart = source.indexOf('className="wk-upload-confirm-footer');
  const footer = source.slice(footerStart, source.indexOf('</div>', footerStart));
  const cancelIndex = footer.indexOf('ct("uploadConfirm.cancel")');
  const confirmIndex = footer.indexOf('confirmButtonText');

  assert.ok(footerStart >= 0, 'upload confirmation footer remains present');
  assert.ok(cancelIndex >= 0, 'cancel action remains in the footer');
  assert.ok(confirmIndex >= 0, 'confirm action remains in the footer');
  assert.ok(cancelIndex < confirmIndex, 'Vue renders Cancel before Confirm');
});

// --- Destination picker (Vue FolderPickerMenu parity) ---------------------------

test('destination picker renders root, depth-indented folders and per-row add affordances', () => {
  const html = renderToStaticMarkup(React.createElement(UploadDestinationPicker, {
    options: [
      { path: 'docs', name: 'docs', depth: 0 },
      { path: 'docs/spec', name: 'spec', depth: 1 },
    ],
    currentPath: 'docs',
    creatingUnder: null,
    newFolderName: '',
    duplicateWarning: false,
    labels: {
      pickerLabel: '更改上传位置',
      rootRow: '根目录',
      newFolderPlaceholder: '输入新目录名称',
      newFolderAddRoot: '在根目录下新建子目录',
      newFolderAddUnder: (folder) => `在「${folder}」下新建子目录`,
      duplicate: '该目录已存在',
    },
    onChoose: noop,
    onStartCreate: noop,
    onCancelCreate: noop,
    onNewFolderNameChange: noop,
    onCommitNewFolder: noop,
  }));
  assert.match(html, /根目录/);
  assert.match(html, /docs/);
  assert.match(html, /在根目录下新建子目录/);
  assert.match(html, /在「docs」下新建子目录/);
  // Depth indentation and the current-destination check are both rendered.
  assert.match(html, /padding:0 8px 0 10px/);
  assert.match(html, /padding:0 8px 0 22px/);
  assert.match(html, /aria-current="true"/);
  assert.match(html, /data-folder-path="docs[/]spec"/);
  assert.doesNotMatch(html, /该目录已存在/);
});

test('destination picker shows the inline create row under its parent and the duplicate warning', () => {
  const html = renderToStaticMarkup(React.createElement(UploadDestinationPicker, {
    options: [{ path: 'docs', name: 'docs', depth: 0 }],
    currentPath: '',
    creatingUnder: 'docs',
    newFolderName: 'sp',
    duplicateWarning: true,
    labels: {
      pickerLabel: '更改上传位置',
      rootRow: '根目录',
      newFolderPlaceholder: '输入新目录名称',
      newFolderAddRoot: '在根目录下新建子目录',
      newFolderAddUnder: (folder) => `在「${folder}」下新建子目录`,
      duplicate: '该目录已存在',
    },
    onChoose: noop,
    onStartCreate: noop,
    onCancelCreate: noop,
    onNewFolderNameChange: noop,
    onCommitNewFolder: noop,
  }));
  assert.match(html, /value="sp"/);
  assert.match(html, /placeholder="输入新目录名称"/);
  assert.match(html, /role="alert"/);
  assert.match(html, /该目录已存在/);
});

// --- Files panel ----------------------------------------------------------------

test('files panel lists staged URLs and files with relative dirs, sizes and removal', () => {
  const file = new File(['x'], 'README.md');
  Object.defineProperty(file, 'webkitRelativePath', { value: 'docs/README.md' });
  const entries = [...toUploadEntries([new File(['x'], 'spec.pdf')]), { file, name: 'README.md', size: 512 }];
  const html = renderToStaticMarkup(React.createElement(UploadFilesPanel, {
    mode: 'file',
    entries,
    urls: ['https://example.com/spec.pdf'],
    uploadStates: [{ entry: entries[0], status: 'uploading' }],
    uploading: false,
    labels: {
      urlItemLabel: 'URL',
      remove: '移除',
      noItems: '请至少添加一个文件或 URL',
      manualCharCount: (count) => `${count} 个字符`,
      reparseSource: '待重新解析文档',
      reparseHint: '将沿用上次解析的配置，可在此调整',
      statusLabel: (status) => status === 'uploading' ? '进行中' : '等待中',
    },
    onRemoveUrl: noop,
    onRemoveEntry: noop,
  }));
  assert.match(html, /https:[/][/]example[.]com[/]spec[.]pdf/);
  assert.match(html, /spec.pdf/);
  assert.match(html, /README.md/);
  assert.match(html, /docs/);
  assert.match(html, /512 B/);
  assert.match(html, /进行中/);
  assert.match(html, /移除/);
});

test('files panel shows the manual preview counts and the reparse source hint', () => {
  const manual = renderToStaticMarkup(React.createElement(UploadFilesPanel, {
    mode: 'manual',
    entries: [],
    urls: [],
    uploadStates: [],
    manualTitle: '旅行指南',
    manualCharCount: 42,
    uploading: false,
    labels: {
      urlItemLabel: 'URL', remove: '移除', noItems: '',
      manualCharCount: (count) => `${count} 个字符`,
      reparseSource: '待重新解析文档',
      reparseHint: '将沿用上次解析的配置，可在此调整',
      statusLabel: () => '',
    },
    onRemoveUrl: noop,
    onRemoveEntry: noop,
  }));
  assert.match(manual, /旅行指南/);
  assert.match(manual, /42 个字符/);

  const reparse = renderToStaticMarkup(React.createElement(UploadFilesPanel, {
    mode: 'reparse',
    entries: [],
    urls: [],
    uploadStates: [],
    reparseFileName: 'annual-report.pdf',
    uploading: false,
    labels: {
      urlItemLabel: 'URL', remove: '移除', noItems: '',
      manualCharCount: (count) => `${count} 个字符`,
      reparseSource: '待重新解析文档',
      reparseHint: '将沿用上次解析的配置，可在此调整',
      statusLabel: () => '',
    },
    onRemoveUrl: noop,
    onRemoveEntry: noop,
  }));
  assert.match(reparse, /annual-report.pdf/);
  assert.match(reparse, /将沿用上次解析的配置，可在此调整/);
  const anonymous = renderToStaticMarkup(React.createElement(UploadFilesPanel, {
    mode: 'reparse',
    entries: [],
    urls: [],
    uploadStates: [],
    uploading: false,
    labels: {
      urlItemLabel: 'URL', remove: '移除', noItems: '',
      manualCharCount: (count) => `${count} 个字符`,
      reparseSource: '待重新解析文档',
      reparseHint: '将沿用上次解析的配置，可在此调整',
      statusLabel: () => '',
    },
    onRemoveUrl: noop,
    onRemoveEntry: noop,
  }));
  assert.match(anonymous, /待重新解析文档/);
});

test('files panel empty state asks for at least one file or URL', () => {
  const html = renderToStaticMarkup(React.createElement(UploadFilesPanel, {
    mode: 'file',
    entries: [],
    urls: [],
    uploadStates: [],
    uploading: false,
    labels: {
      urlItemLabel: 'URL', remove: '移除',
      noItems: '请至少添加一个文件或 URL',
      manualCharCount: (count) => `${count} 个字符`,
      reparseSource: '', reparseHint: '', statusLabel: () => '',
    },
    onRemoveUrl: noop,
    onRemoveEntry: noop,
  }));
  assert.match(html, /请至少添加一个文件或 URL/);
});

// --- Section nav ------------------------------------------------------------------

test('section nav shows per-section status text with issue markers', () => {
  const html = renderToStaticMarkup(React.createElement(UploadSectionNav, {
    items: [
      { key: 'tags', label: '文档标签', status: '2 个标签', statusTitle: '2 个标签', issue: false },
      { key: 'parser', label: '解析引擎', status: '扫描件解析', statusTitle: '扫描件解析', issue: false },
      { key: 'multimodal', label: '多模态', status: '待配置', statusTitle: '待配置', tone: 'error', issue: true },
    ],
    navLabel: '解析配置导航',
    onSelect: noop,
  }));
  assert.match(html, /文档标签/);
  assert.match(html, /2 个标签/);
  assert.match(html, /扫描件解析/);
  assert.match(html, /待配置/);
  assert.match(html, /aria-label="解析配置导航"/);
  assert.match(html, /wk-upload-nav-dot/);
});

// --- Config sections (PDF presentation, chunking more options, question) ----------

function sectionsHtml(overrides?: { state?: Partial<ReturnType<typeof defaultUploadConfirmUIState>>; hasPdf?: boolean; moreOpen?: boolean; locale?: 'zh-CN' | 'en-US'; parserEngines?: ParserEngineInfo[]; parserLoading?: boolean }) {
  const locale = overrides?.locale ?? 'zh-CN';
  return renderToStaticMarkup(React.createElement(UploadConfirmSections, {
    state: { ...defaultUploadConfirmUIState(), ...overrides?.state },
    update: noop,
    hasPdf: overrides?.hasPdf ?? true,
    multimodalIssue: false,
    asrIssue: false,
    parserEngines: overrides?.parserEngines ?? [{ Name: 'mineru', Description: '', FileTypes: ['pdf'], Available: true }],
    parserLoading: overrides?.parserLoading,
    vllmModels: [],
    asrModels: [],
    moreOpen: overrides?.moreOpen ?? false,
    onToggleMore: noop,
    t: uploadConfirmT(locale),
  }));
}

test('parser section distinguishes Vue loading from an empty engine result', () => {
  const html = sectionsHtml({ parserLoading: true, parserEngines: [] });
  assert.match(html, /role="status"/);
  assert.match(html, /加载中/);
  assert.doesNotMatch(html, /没有检测到可用的解析引擎/);
});

test('scanned-PDF override renders with its label and hint only for PDF batches', () => {
  const withPdf = sectionsHtml({ hasPdf: true });
  assert.match(withPdf, /按扫描件解析 PDF/);
  assert.match(withPdf, /适用于网页打印、扫描件、图片型 PDF/);
  assert.match(withPdf, /wk-pdf-force-scanned/);
  const withoutPdf = sectionsHtml({ hasPdf: false });
  assert.doesNotMatch(withoutPdf, /按扫描件解析 PDF/);
  assert.doesNotMatch(withoutPdf, /wk-pdf-force-scanned/);
});

test('upload boolean settings use project switch semantics', () => {
  const html = sectionsHtml({ hasPdf: true, moreOpen: true, state: { enableParentChild: true, multimodalEnabled: true, asrEnabled: true } });
  assert.ok((html.match(/role="switch"/g) ?? []).length >= 5);
  assert.match(html, /wk-upload-switch/);
});

test('active section keeps Vue v-show sections mounted but hides inactive panels', () => {
  const html = sectionsHtml({});
  assert.match(html, /data-section="parser"/);
  const active = renderToStaticMarkup(React.createElement(UploadConfirmSections, {
    state: defaultUploadConfirmUIState(),
    update: noop,
    activeSection: 'chunking',
    hasPdf: true,
    multimodalIssue: false,
    asrIssue: false,
    parserEngines: [{ Name: 'mineru', Description: '', FileTypes: ['pdf'], Available: true }],
    vllmModels: [],
    asrModels: [],
    moreOpen: false,
    onToggleMore: noop,
    t: uploadConfirmT('zh-CN'),
  }));
  assert.match(active, /data-section="parser" style="display:none"/);
  assert.match(active, /data-section="chunking"/);
  assert.doesNotMatch(active, /data-section="chunking" style="display:none"/);
});

test('chunking more options stay collapsed until the toggle opens them', () => {
  const collapsed = sectionsHtml({ moreOpen: false });
  assert.match(collapsed, /更多处理选项/);
  assert.match(collapsed, /aria-expanded="false"/);
  assert.doesNotMatch(collapsed, /父子分块/);
  const open = sectionsHtml({ moreOpen: true });
  assert.match(open, /aria-expanded="true"/);
  assert.match(open, /分隔符/);
  assert.match(open, /每块 Token 上限/);
  assert.match(open, /父子分块/);
});

test('chunking strategy uses a Vue-shaped single select instead of native select', () => {
  const html = renderToStaticMarkup(React.createElement(UploadConfirmSections, {
    state: defaultUploadConfirmUIState(), update: noop, hasPdf: false, multimodalIssue: false, asrIssue: false,
    parserEngines: [], vllmModels: [], asrModels: [], moreOpen: false, onToggleMore: noop, t: uploadConfirmT('zh-CN'),
  }));
  assert.match(html, /wk-upload-chunk-strategy-select/);
  assert.match(html, /role="combobox"/);
});

test('parser engine rules use the same project single-select surface', () => {
  const html = sectionsHtml({});
  assert.match(html, /wk-upload-parser-group/);
  assert.match(html, /wk-upload-parser-select/);
  assert.match(html, /role="combobox"/);
});

test('parser families expose Vue no-engine warning after loading', () => {
  const html = sectionsHtml({ parserEngines: [{ Name: 'disabled', Description: '', FileTypes: ['pdf'], Available: false }] });
  assert.match(html, /wk-upload-parser-warning/);
  assert.match(html, /未检测到解析引擎|settings\.parser\.noEngineDetected/);
});

test('Excel parser group preserves Vue first-row-header control', () => {
  const html = sectionsHtml({
    parserEngines: [{ Name: 'builtin', Description: '', FileTypes: ['xlsx', 'xls'], Available: true }],
    state: { parserRules: [{ file_types: ['xlsx', 'xls'], engine: 'builtin' }] },
  });
  assert.match(html, /Excel/);
  // i18n sweep (70e9f061) replaced the hardcoded 首行为表头 copy with the
  // kbSettings key value 将首行作为列标题; the control itself is unchanged.
  assert.match(html, /xlsxFirstRowAsHeader|将首行作为列标题/);
});

test('multimodal and ASR model/language fields use project selectors', () => {
  const html = renderToStaticMarkup(React.createElement(UploadConfirmSections, {
    state: { ...defaultUploadConfirmUIState(), multimodalEnabled: true, asrEnabled: true }, update: noop, hasPdf: false, multimodalIssue: false, asrIssue: false,
    parserEngines: [], vllmModels: [{ id: 'vlm-1', name: 'vlm' } as never], asrModels: [{ id: 'asr-1', name: 'asr' } as never], moreOpen: false, onToggleMore: noop, t: uploadConfirmT('zh-CN'),
  }));
  assert.equal((html.match(/wk-upload-model-select/g) ?? []).length, 3);
  assert.match(html, /自动跟随文档语言/);
  assert.ok((html.match(/wk-upload-setting-row/g) ?? []).length >= 8);
  assert.match(html, /用于音频中语音转文本的识别模型/);
});

test('ASR language input exposes Vue clearable behavior', () => {
  const html = sectionsHtml({ state: { asrEnabled: true, asrLanguage: 'zh' } });
  assert.match(html, /wk-upload-clearable-input/);
  assert.match(html, /清除音频语言提示/);
});

test('chunking advanced fields use Vue-shaped filterable multi-selects', () => {
  const html = sectionsHtml({ moreOpen: true });
  assert.equal((html.match(/wk-upload-multi-select"/g) ?? []).length, 2);
  assert.match(html, /aria-multiselectable|role="combobox"/);
  assert.equal((html.match(/wk-upload-number-input--wide/g) ?? []).length, 5);
});

test('question generation section exposes the count and instructions controls', () => {
  const html = sectionsHtml({ state: { questionEnabled: true, questionCount: 4, questionInstructions: '面向新员工' } });
  assert.match(html, /问题生成/);
  assert.match(html, /value="4"/);
  assert.match(html, /面向新员工/);
  assert.match(html, /wk-question-enabled/);
  assert.match(html, /role="switch"/);
  assert.match(html, /wk-upload-number-input/);
  assert.match(html, /减少生成问题数量/);
  assert.match(html, /增加生成问题数量/);
  assert.match(html, /aria-valuemin="1"/);
  assert.match(html, /aria-valuemax="10"/);
  assert.match(html, /wk-upload-section-header/);
});

// --- Question generation silent-skip hint (R478) ----------------------------------
// React-first mitigation for a shared Vue+React UX defect: the backend gates
// question generation on kb.NeedsEmbeddingModel() (vector_enabled ||
// keyword_enabled, indexing_strategy.go L34), silently skipping wiki-only KBs.
// Vue's UploadConfirmDialog question section has no linkage at all, so React
// adds an inline hint ahead of any Vue-side fix. This is a superset
// mitigation, NOT a parity claim.

const wikiOnlyKb = {
  id: 'kb-wiki-only',
  name: 'wiki-only',
  chunking_config: {},
  question_generation_config: { enabled: true, question_count: 3 },
  indexing_strategy: { vector_enabled: false, keyword_enabled: false, wiki_enabled: true, graph_enabled: false },
};

const vectorKb = {
  id: 'kb-vector',
  name: 'vector',
  chunking_config: {},
  question_generation_config: { enabled: true, question_count: 3 },
  indexing_strategy: { vector_enabled: true, keyword_enabled: true, wiki_enabled: false, graph_enabled: false },
};

test('question generation shows the silent-skip hint for vector/keyword-off KBs without blocking the switch', () => {
  const html = sectionsHtml({ state: uploadConfirmStateFromKb(wikiOnlyKb) });
  assert.match(html, /wk-question-skip-hint/);
  assert.match(html, /role="note"/);
  assert.match(html, /当前知识库未开启向量\/关键词检索/);
  // Non-blocking mitigation: the enabled switch stays operable and the
  // payload keeps the user's choice (backend behavior is unchanged).
  const switchTag = html.match(/<button[^>]*id="wk-question-enabled"[^>]*>/)?.[0] ?? '';
  assert.match(switchTag, /role="switch"/);
  assert.doesNotMatch(switchTag, /\bdisabled\b/);
});

test('question generation renders no silent-skip hint once vector or keyword search is on', () => {
  const html = sectionsHtml({ state: uploadConfirmStateFromKb(vectorKb) });
  assert.doesNotMatch(html, /wk-question-skip-hint/);
  assert.doesNotMatch(html, /当前知识库未开启向量\/关键词检索/);
  // Unknown indexing strategy (KB not loaded yet) must stay hint-free.
  const unknown = sectionsHtml({ state: uploadConfirmStateFromKb({ name: 'no-strategy' }) });
  assert.doesNotMatch(unknown, /wk-question-skip-hint/);
});

test('question generation silent-skip hint carries complete copy for every locale', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const text = uploadConfirmT(locale)('uploadConfirm.questionGeneration.skippedHint');
    assert.notEqual(text, 'uploadConfirm.questionGeneration.skippedHint', `${locale} must resolve real copy`);
    assert.ok(text.trim().length > 0, `${locale} copy must be non-empty`);
  }
  const en = sectionsHtml({ state: uploadConfirmStateFromKb(wikiOnlyKb), locale: 'en-US' });
  assert.match(en, /vector and keyword search disabled/);
  assert.doesNotMatch(en, /当前知识库未开启向量\/关键词检索/);
});

test('config sections localize through the dialog copy table in other locales', () => {
  const en = sectionsHtml({ hasPdf: true, locale: 'en-US' });
  assert.match(en, /Force scanned PDF parsing/);
  assert.doesNotMatch(en, /按扫描件解析 PDF/);
  const chunkingOpen = renderToStaticMarkup(React.createElement(UploadConfirmSections, {
    state: { ...defaultUploadConfirmUIState(), enableParentChild: true },
    update: noop,
    hasPdf: false,
    multimodalIssue: false,
    asrIssue: false,
    parserEngines: [],
    vllmModels: [],
    asrModels: [],
    moreOpen: true,
    onToggleMore: noop,
    t: uploadConfirmT('en-US'),
  }));
  assert.match(chunkingOpen, /Parent-Child/);
});

// --- Graph section (Vue GraphSettings parity, N007) -------------------------------

const enabledGraphExtract = {
  enabled: true,
  text: 'Romeo loves Juliet.',
  tags: ['Author', 'Alias'],
  nodes: [
    { name: 'Romeo and Juliet', attributes: ['A tragedy', 'Set in Verona'] },
    { name: 'William Shakespeare', attributes: ['English playwright'] },
  ],
  relations: [{ node1: 'Romeo and Juliet', node2: 'William Shakespeare', type: 'Author' }],
  customInstructions: '重点提取人物',
};

function graphHtml(overrides?: {
  graphExtract?: Partial<typeof enabledGraphExtract>;
  graphDatabaseOn?: boolean;
  llmModelId?: string;
  canRunExtract?: boolean;
  locale?: 'zh-CN' | 'en-US';
}) {
  return renderToStaticMarkup(React.createElement(UploadGraphSettings, {
    graphExtract: { ...enabledGraphExtract, ...overrides?.graphExtract },
    graphDatabaseOn: overrides?.graphDatabaseOn ?? true,
    llmModelId: overrides?.llmModelId ?? 'llm-1',
    canRunExtract: overrides?.canRunExtract ?? true,
    onChange: noop,
    t: uploadConfirmT(overrides?.locale ?? 'zh-CN'),
  }));
}

test('graph section renders the Vue GraphSettings form when enabled', () => {
  const html = graphHtml();
  assert.match(html, /知识图谱配置/);
  assert.match(html, /配置实体-关系提取功能/);
  assert.match(html, /启用实体关系提取/);
  assert.match(html, /额外提取要求/);
  assert.match(html, /重点提取人物/);
  assert.match(html, /关系类型/);
  assert.match(html, /示例文本/);
  assert.match(html, /Romeo loves Juliet[.]/);
  assert.match(html, /实体列表/);
  assert.match(html, /value="Romeo and Juliet"/);
  assert.match(html, /value="A tragedy"/);
  assert.match(html, /添加属性/);
  assert.match(html, /管理实体/);
  assert.match(html, /添加实体/);
  assert.match(html, /关系列表/);
  assert.match(html, /添加关系/);
  assert.match(html, /提取操作/);
  assert.match(html, /开始提取/);
  assert.match(html, /默认示例/);
  assert.match(html, /清除示例/);
});

test('graph section hides config rows until enabled and warns when the graph database is off', () => {
  const disabled = graphHtml({ graphExtract: { enabled: false, text: '', tags: [], nodes: [], relations: [], customInstructions: '保留指令' } });
  assert.match(disabled, /启用实体关系提取/);
  // Turning extraction off clears the example data but keeps custom instructions (Vue handleEnabledChange).
  assert.doesNotMatch(disabled, /示例文本/);
  assert.doesNotMatch(disabled, /关系类型/);
  assert.doesNotMatch(disabled, /添加实体/);
  const dbOff = graphHtml({ graphDatabaseOn: false });
  assert.match(dbOff, /知识图谱数据库未启用，实体关系提取功能将无法使用/);
  const en = graphHtml({ locale: 'en-US' });
  assert.match(en, /Enable Entity-Relationship Extraction/);
  assert.match(en, /Relationship Types/);
});

test('graph section extraction actions gate on admin role and the summary LLM model', () => {
  const admin = graphHtml();
  assert.match(admin, /生成随机标签/);
  assert.match(admin, /生成随机文本/);
  const nonAdmin = graphHtml({ canRunExtract: false });
  assert.doesNotMatch(nonAdmin, /生成随机标签/);
  assert.doesNotMatch(nonAdmin, /开始提取/);
  const noLlm = graphHtml({ llmModelId: '' });
  assert.match(noLlm, /disabled/);
});

test('config panel gates the graph slot on section availability like Vue v-if', () => {
  const slot = React.createElement('p', { key: 'g' }, 'GRAPH-SLOT');
  const base = {
    state: defaultUploadConfirmUIState(),
    update: noop,
    hasPdf: false,
    multimodalIssue: false,
    asrIssue: false,
    parserEngines: [],
    vllmModels: [],
    asrModels: [],
    moreOpen: false,
    onToggleMore: noop,
    t: ct,
    graphSettings: slot,
  };
  const available = renderToStaticMarkup(React.createElement(UploadConfirmSections, { ...base, graphAvailable: true }));
  assert.match(available, /data-section="graph"/);
  assert.match(available, /wk-upload-section-graph/);
  assert.match(available, /GRAPH-SLOT/);
  const unavailable = renderToStaticMarkup(React.createElement(UploadConfirmSections, { ...base, graphAvailable: false }));
  assert.doesNotMatch(unavailable, /GRAPH-SLOT/);
});

// --- URL list (Vue localUrls parity) -----------------------------------------------

test('files panel renders multiple staged URL rows each with removal', () => {
  const html = renderToStaticMarkup(React.createElement(UploadFilesPanel, {
    mode: 'file',
    entries: [],
    urls: ['https://example.com/a.html', 'https://example.com/b.pdf'],
    uploadStates: [],
    uploading: false,
    labels: {
      urlItemLabel: 'URL', remove: '移除', noItems: '',
      manualCharCount: (count: number) => count + ' 个字符',
      reparseSource: '', reparseHint: '', statusLabel: () => '',
    },
    onRemoveUrl: noop,
    onRemoveEntry: noop,
  }));
  assert.match(html, /example[.]com[/]a[.]html/);
  assert.match(html, /example[.]com[/]b[.]pdf/);
  assert.equal((html.match(/aria-label="移除"/g) ?? []).length, 2);
});

// --- Add-source dropdown (Vue KbUploadSourceDropdown parity) ------------------------

test('add-source control is a dropdown menu with file, folder and URL entries', () => {
  const dropdownProps = {
    tooltip: '继续添加',
    items: [
      { key: 'file' as const, label: '上传文档' },
      { key: 'folder' as const, label: '上传文件夹' },
      { key: 'url' as const, label: '导入网页' },
    ],
    onToggle: noop,
    onSelect: noop,
    onFiles: noop,
  };
  const html = renderToStaticMarkup(React.createElement(UploadSourceDropdown, { ...dropdownProps, open: true }));
  assert.match(html, /aria-label="继续添加"/);
  assert.match(html, /<svg[^>]*viewBox="0 0 24 24"/);
  assert.doesNotMatch(html, /＋/);
  assert.match(html, /上传文档/);
  assert.match(html, /上传文件夹/);
  assert.match(html, /导入网页/);
  assert.match(html, /type="file"/);
  assert.match(html, /webkitdirectory/);
  const closed = renderToStaticMarkup(React.createElement(UploadSourceDropdown, { ...dropdownProps, open: false }));
  assert.doesNotMatch(closed, /上传文件夹/);
});

test('the page toolbar dropdown carries the kbDetail guide anchor like Vue data-guide', () => {
  // Vue KbUploadSourceDropdown trigger (KnowledgeBase.vue:2613):
  // data-guide="kb-detail-add-doc" marks the upload entry for the spotlight.
  const dropdownProps = {
    tooltip: '添加文档',
    items: [{ key: 'file' as const, label: '上传文档' }],
    onToggle: noop,
    onSelect: noop,
    onFiles: noop,
  };
  const anchored = renderToStaticMarkup(React.createElement(UploadSourceDropdown, { ...dropdownProps, open: false, guideTarget: 'kb-detail-add-doc' }));
  assert.match(anchored, /data-guide="kb-detail-add-doc"/);
  const anonymous = renderToStaticMarkup(React.createElement(UploadSourceDropdown, { ...dropdownProps, open: false }));
  assert.doesNotMatch(anonymous, /data-guide=/, 'the dialog "continue add" instance stays unmarked');

  // Wiring contract: exactly one instance (the page toolbar) passes guideTarget.
  const source = readFileSync(new URL('./KnowledgeDocumentsPage.tsx', import.meta.url), 'utf8');
  assert.equal((source.match(/guideTarget=/g) ?? []).length, 1, 'only the toolbar instance is anchored');
  assert.match(source, /guideTarget="kb-detail-add-doc"/);
});

// --- Nav graph item + section availability fallback ---------------------------------

test('section nav renders the graph entry and availability falls back like Vue watch', () => {
  const html = renderToStaticMarkup(React.createElement(UploadSectionNav, {
    items: [
      { key: 'tags', label: '文档标签', status: '未设置', statusTitle: '未设置', tone: 'muted', issue: false },
      { key: 'graph', label: '知识图谱', status: '2 个', statusTitle: '2 个', issue: false },
    ],
    navLabel: '解析配置导航',
    onSelect: noop,
  }));
  assert.match(html, /知识图谱/);
  assert.match(html, /2 个/);

  // Vue getDefaultSection: reparse -> parser, issue -> that section, else tags.
  assert.equal(defaultUploadConfirmSection({ mode: 'reparse', multimodalIssue: false, asrIssue: false }), 'parser');
  assert.equal(defaultUploadConfirmSection({ mode: 'file', multimodalIssue: true, asrIssue: false }), 'multimodal');
  assert.equal(defaultUploadConfirmSection({ mode: 'file', multimodalIssue: false, asrIssue: true }), 'asr');
  assert.equal(defaultUploadConfirmSection({ mode: 'manual', multimodalIssue: false, asrIssue: false }), 'tags');

  // Vue watch(isGraphSectionAvailable): leaving graph resets to the default section.
  assert.equal(sectionAfterGraphAvailabilityChange('graph', false, 'tags'), 'tags');
  assert.equal(sectionAfterGraphAvailabilityChange('graph', true, 'tags'), 'graph');
  assert.equal(sectionAfterGraphAvailabilityChange('chunking', false, 'tags'), 'chunking');

  // Vue isGraphSectionAvailable: engine set (and not "Not Enabled") AND kb graph flag.
  assert.equal(graphSectionAvailable({ systemInfo: { graph_database_engine: 'neo4j' }, graphEnabled: true }), true);
  assert.equal(graphSectionAvailable({ systemInfo: { graph_database_engine: 'Not Enabled' }, graphEnabled: true }), false);
  assert.equal(graphSectionAvailable({ systemInfo: {}, graphEnabled: true }), false);
  assert.equal(graphSectionAvailable({ systemInfo: { graph_database_engine: 'neo4j' }, graphEnabled: false }), false);

  // Vue getSectionNavStatus('graph').
  const state = { ...defaultUploadConfirmUIState(), graphEnabled: true };
  const navInput = { selectedTagCount: 0, hasPdf: false, hasImages: false, hasAudio: false };
  assert.equal(uploadSectionStatus('graph', { state: { ...state, nodeExtract: { ...state.nodeExtract, enabled: false } }, ...navInput })?.key, 'uploadConfirm.statusOff');
  const withTags = uploadSectionStatus('graph', { state: { ...state, nodeExtract: { ...state.nodeExtract, enabled: true, tags: ['Author', 'Alias'] } }, ...navInput });
  assert.equal(withTags?.key, 'uploadConfirm.summaryGraphTagsValue');
  assert.equal(withTags?.values?.count, 2);
  assert.equal(uploadSectionStatus('graph', { state: { ...state, nodeExtract: { ...state.nodeExtract, enabled: true } }, ...navInput })?.key, 'uploadConfirm.statusOn');
});

// --- Graph copy table ----------------------------------------------------------------

test('graph section copy resolves five locales from the ported Vue table', () => {
  assert.equal(uploadConfirmMessage('zh-CN', 'graphSettings.title'), '知识图谱配置');
  assert.equal(uploadConfirmMessage('en-US', 'graphSettings.title'), 'Knowledge Graph Configuration');
  assert.equal(uploadConfirmMessage('ja-JP', 'graphSettings.enableLabel'), 'エンティティとリレーションの抽出を有効化');
  assert.equal(uploadConfirmMessage('ko-KR', 'graphSettings.addEntity'), '엔티티 추가');
  assert.equal(uploadConfirmMessage('ru-RU', 'graphSettings.addRelation'), 'Добавить отношение');
  assert.equal(uploadConfirmMessage('zh-CN', 'uploadConfirm.summaryGraphTagsValue', { count: 3 }), '3 个');
  assert.equal(uploadConfirmMessage('en-US', 'uploadConfirm.summaryGraphTagsValue', { count: 3 }), '3');
  assert.equal(uploadConfirmMessage('ko-KR', 'uploadConfirm.summaryGraphTagsValue', { count: 3 }), '3개');
  // Add-source dropdown labels missing from shared i18n are ported byte-exact.
  assert.equal(uploadConfirmMessage('zh-CN', 'upload.uploadDocument'), '上传文档');
  assert.equal(uploadConfirmMessage('en-US', 'upload.uploadFolder'), 'Upload Folder');
  assert.equal(uploadConfirmMessage('zh-CN', 'common.confirm'), '确认');
});

test('graph feedback keeps Vue success, warning, and error message semantics', () => {
  assert.equal(stageNoticeClass('success'), 'wk-documents-toast success');
  assert.equal(stageNoticeClass('warning'), 'wk-documents-toast warning');
  assert.equal(stageNoticeClass('error'), 'wk-documents-toast error');
});

test('graph textareas use the Vue autosize row bounds', () => {
  assert.equal(clampGraphTextareaHeight(20, 20, 8, 3, 8), 68);
  assert.equal(clampGraphTextareaHeight(400, 20, 8, 3, 8), 168);
  assert.equal(clampGraphTextareaHeight(400, 20, 8, 6, 12), 248);
});

test('graph tags use a Vue-like creatable multi-value field instead of native multi-select', () => {
  const markup = renderToStaticMarkup(React.createElement(GraphTagsField, {
    tags: ['Author', 'Alias'],
    onChange: noop,
    placeholder: '请输入关系类型',
    ariaLabel: '关系类型',
  }));
  assert.doesNotMatch(markup, /<select/);
  assert.match(markup, /role="listbox"/);
  assert.match(markup, /Author/);
  assert.match(markup, /Alias/);
});

test('graph enabled control uses an accessible switch surface', () => {
  const markup = renderToStaticMarkup(React.createElement(GraphSwitch, {
    id: 'graph-enabled-test', checked: true, onChange: noop, labelId: 'graph-enabled-label',
  }));
  assert.match(markup, /role="switch"/);
  assert.match(markup, /aria-checked="true"/);
  assert.doesNotMatch(markup, /type="checkbox"/);
});

test('graph sample text exposes the Vue word-limit counter', () => {
  const markup = renderToStaticMarkup(React.createElement(UploadGraphSettings, {
    graphExtract: { ...enabledGraphExtract, text: 'abc' },
    graphDatabaseOn: true,
    llmModelId: 'llm-1',
    canRunExtract: false,
    onChange: noop,
    t: ct,
  }));
  assert.match(markup, /wk-graph-text-limit/);
  assert.match(markup, />3\/5000</);
});

test('graph relation selectors expose filterable Vue-like comboboxes', () => {
  const markup = renderToStaticMarkup(React.createElement(GraphRelationSelect, {
    value: 'Author', options: ['Author', 'Alias'], placeholder: '选择关系类型', ariaLabel: '选择关系类型',
    creatable: true, clearable: true, onChange: noop,
  }));
  assert.doesNotMatch(markup, /<select/);
  assert.match(markup, /role="combobox"/);
  assert.match(markup, /Author/);
  assert.match(markup, /清除选择关系类型/);
});

test('graph relation selector arrow navigation clamps to available options', () => {
  assert.equal(moveGraphRelationOption(0, 'down', 2), 1);
  assert.equal(moveGraphRelationOption(1, 'down', 2), 1);
  assert.equal(moveGraphRelationOption(0, 'up', 2), 0);
  assert.equal(moveGraphRelationOption(1, 'up', 2), 0);
  assert.equal(moveGraphRelationOption(0, 'down', 0), 0);
});

// --- Upload progress mask (Vue upload-mask.vue parity + percent) -------------------

const { UploadProgressMask } = await import('./KnowledgeDocumentsPage.tsx');

test('upload progress mask renders the percent text and bar like the Vue mask', () => {
  const html = renderToStaticMarkup(React.createElement(UploadProgressMask, { percent: 42 }));
  assert.match(html, /Uploading 42%/);
  assert.match(html, /role="status"/);
  assert.match(html, /role="progressbar"/);
  assert.match(html, /aria-valuenow="42"/);
  assert.match(html, /width:42%/);
});

test('upload progress mask clamps out-of-range percents', () => {
  const html = renderToStaticMarkup(React.createElement(UploadProgressMask, { percent: 140 }));
  assert.match(html, /Uploading 100%/);
  assert.match(html, /aria-valuenow="100"/);
});
