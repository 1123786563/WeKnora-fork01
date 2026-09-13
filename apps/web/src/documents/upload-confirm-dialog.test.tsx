import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);
(globalThis as typeof globalThis & { React: typeof React }).React = React;

const {
  UploadConfirmSections,
  UploadDestinationPicker,
  UploadFilesPanel,
  UploadSectionNav,
} = await import('./KnowledgeDocumentsPage.tsx');
const { uploadConfirmT, defaultUploadConfirmUIState, toUploadEntries } = await import('./upload-pipeline.ts');

const ct = uploadConfirmT('zh-CN');
const noop = () => {};

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

function sectionsHtml(overrides?: { state?: Partial<ReturnType<typeof defaultUploadConfirmUIState>>; hasPdf?: boolean; moreOpen?: boolean; locale?: 'zh-CN' | 'en-US' }) {
  const locale = overrides?.locale ?? 'zh-CN';
  return renderToStaticMarkup(React.createElement(UploadConfirmSections, {
    state: { ...defaultUploadConfirmUIState(), ...overrides?.state },
    update: noop,
    hasPdf: overrides?.hasPdf ?? true,
    multimodalIssue: false,
    asrIssue: false,
    parserEngines: [{ Name: 'mineru', Description: '', FileTypes: ['pdf'], Available: true }],
    vllmModels: [],
    asrModels: [],
    moreOpen: overrides?.moreOpen ?? false,
    onToggleMore: noop,
    t: uploadConfirmT(locale),
  }));
}

test('scanned-PDF override renders with its label and hint only for PDF batches', () => {
  const withPdf = sectionsHtml({ hasPdf: true });
  assert.match(withPdf, /按扫描件解析 PDF/);
  assert.match(withPdf, /适用于网页打印、扫描件、图片型 PDF/);
  assert.match(withPdf, /wk-pdf-force-scanned/);
  const withoutPdf = sectionsHtml({ hasPdf: false });
  assert.doesNotMatch(withoutPdf, /按扫描件解析 PDF/);
  assert.doesNotMatch(withoutPdf, /wk-pdf-force-scanned/);
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

test('question generation section exposes the count and instructions controls', () => {
  const html = sectionsHtml({ state: { questionEnabled: true, questionCount: 4, questionInstructions: '面向新员工' } });
  assert.match(html, /问题生成/);
  assert.match(html, /value="4"/);
  assert.match(html, /面向新员工/);
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
