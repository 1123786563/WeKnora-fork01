/**
 * R490 B3 (R489 N2) — the 添加到知识库 answer-toolbar target.
 * R491 P2 — deep sub-features ported from the Vue manual editor.
 * panel-matrix Batch-1 根因 B（px-chat-addtokb）：Vue uiStore.openManualEditor 打开
 * 的是 SettingDrawer（t-drawer--right，760px，拖宽把手）内的 manual-knowledge-editor；
 * 本组件自起初误以居中 wk-bookmark-dialog 实现——本轮按 Vue 事实源同构重写为
 * tdesign-react 右抽屉 + 手写编辑器 DOM/样式（manual-knowledge-editor.vue +
 * settings/SettingDrawer.vue 逐块平移，样式入 chat.td.css §17）。
 *
 * Vue 行为面（botmsg.vue handleAddToKnowledge）：formatManualTitle(userQuery) /
 * buildManualMarkdown(userQuery, content) 预填，status 'draft'，KB 选择仅文档型，
 * 暂存草稿 POST /knowledge-bases/:id/knowledge/manual（status draft）。发布流程的
 * process_config 上传确认仍属 documents 域缺口（R491 P2 注记保留）。
 */
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Button, Drawer, Input, Select, Tag, Textarea, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import type { WeKnoraClient } from '@weknora/api-client';
import { formatMessage, type Locale } from '@weknora/i18n';
import type { ChatCopyTable } from '@weknora/views/chat/chat-copy';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import './chat-u.css';
import './bookmark-drawer.css';

/** Vue formatManualTitle (chatMessageShared.ts:69-78): collapse whitespace,
 *  truncate at 40 chars with '...', fall back to the session-excerpt label. */
export function formatManualBookmarkTitle(question: string | undefined, excerptLabel: string): string {
  if (!question) return excerptLabel;
  const condensed = question.replace(/\s+/g, ' ').trim();
  if (!condensed) return excerptLabel;
  return condensed.length > 40 ? `${condensed.slice(0, 40)}...` : condensed;
}

/** Vue buildManualMarkdown (chatMessageShared.ts:80-83): the trimmed answer
 *  with the no-answer placeholder fallback. */
export function buildManualBookmarkContent(answer: string, noAnswerLabel: string): string {
  return answer?.trim() || noAnswerLabel;
}

/** manualEditor strings used by this drawer, ported byte-exact from the Vue
 *  locales (frontend/src/i18n/locales/*.ts manualEditor section — the keys
 *  are not in @weknora/i18n yet, so they live here until backfilled). */
export interface ManualEditorStrings {
  publish: string;
  publishedToast: string;
  contentTooShortWarning: string;
  enterContentWarning: string;
  editLabel: string;
  previewLabel: string;
  previewEmpty: string;
  /* drawer chrome / sections / form */
  titleCreate: string;
  description: string;
  sectionBasic: string;
  sectionContent: string;
  titleLabel: string;
  titlePlaceholder: string;
  kbLabel: string;
  kbPlaceholder: string;
  noDocumentKnowledgeBases: string;
  contentPlaceholder: string;
  draftTag: string;
  cancel: string;
  saveDraft: string;
  selectKbWarning: string;
  enterTitleWarning: string;
  /* toolbar tooltips (hover-only, not in screenshots) */
  toolbarBold: string;
  toolbarItalic: string;
  toolbarStrike: string;
  toolbarInlineCode: string;
  toolbarHeading1: string;
  toolbarHeading2: string;
  toolbarHeading3: string;
  toolbarBulletList: string;
  toolbarOrderedList: string;
  toolbarTaskList: string;
  toolbarBlockquote: string;
  toolbarCodeBlock: string;
  toolbarLink: string;
  toolbarImage: string;
  toolbarTable: string;
  toolbarHorizontalRule: string;
}

const MANUAL_EDITOR_STRINGS: Record<Locale, ManualEditorStrings> = {
  'zh-CN': {
    publish: '发布入库',
    publishedToast: '知识已发布并开始索引',
    contentTooShortWarning: '内容过短，建议补充更多信息后再发布',
    enterContentWarning: '请输入知识内容',
    editLabel: '返回编辑',
    previewLabel: '预览内容',
    previewEmpty: '暂无内容',
    titleCreate: '在线编辑 Markdown 知识',
    description: '使用 Markdown 编写知识内容，支持实时预览',
    sectionBasic: '基本信息',
    sectionContent: '知识内容',
    titleLabel: '知识标题',
    titlePlaceholder: '请输入标题',
    kbLabel: '目标知识库',
    kbPlaceholder: '请选择知识库',
    noDocumentKnowledgeBases: '暂无可用的文档型知识库，请先创建一个文档型知识库',
    contentPlaceholder: '支持 Markdown 语法，可使用 # 标题、列表、代码块等',
    draftTag: '当前状态：草稿',
    cancel: '取消',
    saveDraft: '暂存草稿',
    selectKbWarning: '请选择目标知识库',
    enterTitleWarning: '请输入知识标题',
    toolbarBold: '加粗', toolbarItalic: '斜体', toolbarStrike: '删除线', toolbarInlineCode: '行内代码',
    toolbarHeading1: '一级标题', toolbarHeading2: '二级标题', toolbarHeading3: '三级标题',
    toolbarBulletList: '无序列表', toolbarOrderedList: '有序列表', toolbarTaskList: '任务列表',
    toolbarBlockquote: '引用', toolbarCodeBlock: '代码块', toolbarLink: '插入链接',
    toolbarImage: '插入图片', toolbarTable: '插入表格', toolbarHorizontalRule: '分割线',
  },
  'en-US': {
    publish: 'Publish',
    publishedToast: 'Knowledge published and indexing started',
    contentTooShortWarning: 'Content is too short. Please add more information before publishing',
    enterContentWarning: 'Please enter knowledge content',
    editLabel: 'Back to edit',
    previewLabel: 'Preview content',
    previewEmpty: 'No content yet',
    titleCreate: 'Create Markdown Knowledge',
    description: 'Write knowledge in Markdown with live preview',
    sectionBasic: 'Basic Info',
    sectionContent: 'Content',
    titleLabel: 'Title',
    titlePlaceholder: 'Enter title',
    kbLabel: 'Target knowledge base',
    kbPlaceholder: 'Select a knowledge base',
    noDocumentKnowledgeBases: 'No document-type knowledge bases available. Please create one first',
    contentPlaceholder: 'Markdown supported: # headings, lists, code blocks, and more',
    draftTag: 'Status: Draft',
    cancel: 'Cancel',
    saveDraft: 'Save draft',
    selectKbWarning: 'Please select a target knowledge base',
    enterTitleWarning: 'Please enter a knowledge title',
    toolbarBold: 'Bold', toolbarItalic: 'Italic', toolbarStrike: 'Strikethrough', toolbarInlineCode: 'Inline code',
    toolbarHeading1: 'Heading 1', toolbarHeading2: 'Heading 2', toolbarHeading3: 'Heading 3',
    toolbarBulletList: 'Bullet list', toolbarOrderedList: 'Numbered list', toolbarTaskList: 'Task list',
    toolbarBlockquote: 'Blockquote', toolbarCodeBlock: 'Code block', toolbarLink: 'Insert link',
    toolbarImage: 'Insert image', toolbarTable: 'Insert table', toolbarHorizontalRule: 'Horizontal rule',
  },
  'ja-JP': {
    publish: '公開',
    publishedToast: 'ナレッジを公開し、インデックス作成を開始しました',
    contentTooShortWarning: '内容が短すぎます。公開する前に情報を追加してください',
    enterContentWarning: 'ナレッジの内容を入力してください',
    editLabel: '編集に戻る',
    previewLabel: '内容をプレビュー',
    previewEmpty: 'まだ内容がありません',
    titleCreate: 'Markdownナレッジを作成',
    description: 'Markdownでナレッジを記述し、リアルタイムでプレビューできます',
    sectionBasic: '基本情報',
    sectionContent: '内容',
    titleLabel: 'ナレッジタイトル',
    titlePlaceholder: 'タイトルを入力',
    kbLabel: '対象ナレッジベース',
    kbPlaceholder: 'ナレッジベースを選択',
    noDocumentKnowledgeBases: '利用可能なドキュメント型ナレッジベースがありません。先に作成してください',
    contentPlaceholder: 'Markdown構文対応：# 見出し、リスト、コードブロックなど',
    draftTag: '現在の状態：下書き',
    cancel: 'キャンセル',
    saveDraft: '下書き保存',
    selectKbWarning: '対象ナレッジベースを選択してください',
    enterTitleWarning: 'ナレッジタイトルを入力してください',
    toolbarBold: '太字', toolbarItalic: '斜体', toolbarStrike: '取り消し線', toolbarInlineCode: 'インラインコード',
    toolbarHeading1: '見出し1', toolbarHeading2: '見出し2', toolbarHeading3: '見出し3',
    toolbarBulletList: '箇条書きリスト', toolbarOrderedList: '番号付きリスト', toolbarTaskList: 'タスクリスト',
    toolbarBlockquote: '引用', toolbarCodeBlock: 'コードブロック', toolbarLink: 'リンクを挿入',
    toolbarImage: '画像を挿入', toolbarTable: '表を挿入', toolbarHorizontalRule: '水平線',
  },
  'ko-KR': {
    publish: '게시하기',
    publishedToast: '지식이 게시되고 인덱싱이 시작되었습니다',
    contentTooShortWarning: '내용이 너무 짧습니다. 더 많은 정보를 추가한 후 게시하는 것을 권장합니다',
    enterContentWarning: '지식 내용을 입력해주세요',
    editLabel: '편집으로 돌아가기',
    previewLabel: '내용 미리보기',
    previewEmpty: '내용 없음',
    titleCreate: '온라인 Markdown 지식 편집',
    description: 'Markdown으로 지식을 작성하고 실시간 미리보기 지원',
    sectionBasic: '기본 정보',
    sectionContent: '지식 내용',
    titleLabel: '지식 제목',
    titlePlaceholder: '제목을 입력해주세요',
    kbLabel: '대상 지식베이스',
    kbPlaceholder: '지식베이스를 선택해주세요',
    noDocumentKnowledgeBases: '사용 가능한 문서형 지식베이스가 없습니다. 먼저 문서형 지식베이스를 생성해주세요',
    contentPlaceholder: 'Markdown 구문을 지원합니다. # 제목, 목록, 코드 블록 등을 사용할 수 있습니다',
    draftTag: '현재 상태: 임시 저장',
    cancel: '취소',
    saveDraft: '임시 저장',
    selectKbWarning: '대상 지식베이스를 선택해주세요',
    enterTitleWarning: '지식 제목을 입력해주세요',
    toolbarBold: '굵게', toolbarItalic: '기울임', toolbarStrike: '취소선', toolbarInlineCode: '인라인 코드',
    toolbarHeading1: '제목 1', toolbarHeading2: '제목 2', toolbarHeading3: '제목 3',
    toolbarBulletList: '글머리 기호 목록', toolbarOrderedList: '번호 매기기 목록', toolbarTaskList: '작업 목록',
    toolbarBlockquote: '인용구', toolbarCodeBlock: '코드 블록', toolbarLink: '링크 삽입',
    toolbarImage: '이미지 삽입', toolbarTable: '표 삽입', toolbarHorizontalRule: '가로선',
  },
  'ru-RU': {
    publish: 'Опубликовать',
    publishedToast: 'Знание опубликовано и начата индексация',
    contentTooShortWarning: 'Контент слишком короткий. Добавьте больше информации перед публикацией',
    enterContentWarning: 'Введите содержимое знания',
    editLabel: 'Вернуться к редактированию',
    previewLabel: 'Предпросмотр',
    previewEmpty: 'Пока нет содержимого',
    titleCreate: 'Создать Markdown-знание',
    description: 'Пишите знания в Markdown с предпросмотром в реальном времени',
    sectionBasic: 'Основная информация',
    sectionContent: 'Содержимое',
    titleLabel: 'Заголовок знания',
    titlePlaceholder: 'Введите заголовок',
    kbLabel: 'Целевая база знаний',
    kbPlaceholder: 'Выберите базу знаний',
    noDocumentKnowledgeBases: 'Нет доступных баз знаний типа «документ». Сначала создайте одну',
    contentPlaceholder: 'Поддерживается Markdown: # заголовки, списки, блоки кода и т.д.',
    draftTag: 'Статус: Черновик',
    cancel: 'Отмена',
    saveDraft: 'Сохранить черновик',
    selectKbWarning: 'Пожалуйста, выберите целевую базу знаний',
    enterTitleWarning: 'Введите заголовок знания',
    toolbarBold: 'Жирный', toolbarItalic: 'Курсив', toolbarStrike: 'Зачёркнутый', toolbarInlineCode: 'Встроенный код',
    toolbarHeading1: 'Заголовок 1', toolbarHeading2: 'Заголовок 2', toolbarHeading3: 'Заголовок 3',
    toolbarBulletList: 'Маркированный список', toolbarOrderedList: 'Нумерованный список', toolbarTaskList: 'Список задач',
    toolbarBlockquote: 'Цитата', toolbarCodeBlock: 'Блок кода', toolbarLink: 'Вставить ссылку',
    toolbarImage: 'Вставить изображение', toolbarTable: 'Вставить таблицу', toolbarHorizontalRule: 'Горизонтальная линия',
  },
};

/** Resolve the ported manualEditor strings for a locale (zh-CN fallback). */
export function manualEditorStrings(locale: Locale | undefined): ManualEditorStrings {
  return MANUAL_EDITOR_STRINGS[locale ?? 'zh-CN'] ?? MANUAL_EDITOR_STRINGS['zh-CN'];
}

interface BookmarkTagOption {
  id: string;
  name: string;
}

export interface BookmarkAnswerDialogProps {
  client: WeKnoraClient;
  copy: ChatCopyTable;
  open: boolean;
  initialTitle: string;
  initialContent: string;
  /** UI locale for the manualEditor/uploadConfirm strings (zh-CN default). */
  locale?: Locale;
  onClose: () => void;
  /** Fired after a successful save with the action's status; the host toasts
   * bookmarkDraftSaved (draft) or publishedToast (publish). */
  onSaved?: (status: 'draft' | 'publish') => void;
}

/* ---------- SettingDrawer.vue 宽度状态（右抽屉拖宽） ---------- */

const DRAWER_MIN_WIDTH = 560;
const DRAWER_MAX_WIDTH = 1280;
const DRAWER_STORAGE_KEY = 'setting-drawer:width:manual-markdown-editor';

function clampDrawerWidth(n: number): number {
  const cap = Math.min(DRAWER_MAX_WIDTH, typeof window === 'undefined' ? DRAWER_MAX_WIDTH : window.innerWidth);
  const floor = Math.min(DRAWER_MIN_WIDTH, cap);
  return Math.max(floor, Math.min(cap, Math.round(n)));
}

function initialDrawerWidth(): number {
  if (typeof window === 'undefined') return 760;
  try {
    const raw = window.localStorage.getItem(DRAWER_STORAGE_KEY);
    const n = Number(raw);
    if (raw && Number.isFinite(n) && n > 0) return clampDrawerWidth(n);
  } catch { /* storage unavailable */ }
  return 760;
}

/* ---------- Vue manual-knowledge-editor.vue 工具栏文本操作 ---------- */

type TextOp = { text: string; selectionStart: number; selectionEnd: number };

function applyTextOp(current: string, op: TextOp): { next: string; start: number; end: number } {
  return { next: current.slice(0, op.selectionStart) + op.text + current.slice(op.selectionEnd), start: op.selectionStart, end: op.selectionStart + op.text.length };
}

/** Vue wrapSelection 系（选区包裹/行前缀插入），选区来自编辑 textarea。 */
function runToolbarAction(action: string, textarea: HTMLTextAreaElement, setContent: (v: string) => void, strings: ManualEditorStrings): void {
  const value = textarea.value;
  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const sel = value.slice(start, end);
  const lineStart = value.lastIndexOf('\n', start - 1) + 1;
  const lineEndRaw = value.indexOf('\n', end);
  const lineEnd = lineEndRaw === -1 ? value.length : lineEndRaw;
  const line = value.slice(lineStart, lineEnd);

  const wrap = (before: string, after: string, placeholder: string): TextOp => {
    const inner = sel || placeholder;
    return { text: before + inner + after, selectionStart: start, selectionEnd: end };
  };
  const prefixLine = (prefix: string): TextOp => ({ text: prefix + line, selectionStart: lineStart, selectionEnd: lineEnd });
  const insertBlock = (block: string): TextOp => {
    const pad = lineStart === lineEnd && line === '' ? '' : '\n\n';
    const text = pad + block;
    return { text, selectionStart: lineEnd, selectionEnd: lineEnd };
  };

  let op: TextOp | null = null;
  switch (action) {
    case 'bold': op = wrap('**', '**', strings.toolbarBold); break;
    case 'italic': op = wrap('*', '*', strings.toolbarItalic); break;
    case 'strike': op = wrap('~~', '~~', strings.toolbarStrike); break;
    case 'inline-code': op = wrap('`', '`', strings.toolbarInlineCode); break;
    case 'h1': op = prefixLine('# '); break;
    case 'h2': op = prefixLine('## '); break;
    case 'h3': op = prefixLine('### '); break;
    case 'ul': op = prefixLine('- '); break;
    case 'ol': op = prefixLine('1. '); break;
    case 'task': op = prefixLine('- [ ] '); break;
    case 'quote': op = prefixLine('> '); break;
    case 'codeblock': op = insertBlock('```\n\n```'); break;
    case 'link': {
      const inner = sel || strings.toolbarLink;
      const text = `[${inner}](https://)`;
      op = { text, selectionStart: start, selectionEnd: end };
      break;
    }
    case 'image': {
      const inner = sel || strings.toolbarImage;
      const text = `![${inner}](https://)`;
      op = { text, selectionStart: start, selectionEnd: end };
      break;
    }
    case 'table': op = insertBlock('| 列1 | 列2 |\n| --- | --- |\n| 内容 | 内容 |'); break;
    case 'hr': op = insertBlock('---'); break;
    default: return;
  }
  if (!op) return;
  const { next, start: s, end: e } = applyTextOp(value, op);
  setContent(next);
  requestAnimationFrame(() => {
    textarea.focus();
    textarea.setSelectionRange(s, e);
  });
}

interface ToolbarButtonDef {
  key: string;
  icon: string;
  tooltip: keyof ManualEditorStrings;
  action: string;
}

const TOOLBAR_GROUPS: Array<{ key: string; buttons: ToolbarButtonDef[] }> = [
  {
    key: 'format',
    buttons: [
      { key: 'bold', icon: 'textformat-bold', tooltip: 'toolbarBold', action: 'bold' },
      { key: 'italic', icon: 'textformat-italic', tooltip: 'toolbarItalic', action: 'italic' },
      { key: 'strike', icon: 'textformat-strikethrough', tooltip: 'toolbarStrike', action: 'strike' },
      { key: 'inline-code', icon: 'code', tooltip: 'toolbarInlineCode', action: 'inline-code' },
    ],
  },
  {
    key: 'heading',
    buttons: [
      { key: 'h1', icon: 'numbers-1', tooltip: 'toolbarHeading1', action: 'h1' },
      { key: 'h2', icon: 'numbers-2', tooltip: 'toolbarHeading2', action: 'h2' },
      { key: 'h3', icon: 'numbers-3', tooltip: 'toolbarHeading3', action: 'h3' },
    ],
  },
  {
    key: 'list',
    buttons: [
      { key: 'ul', icon: 'view-list', tooltip: 'toolbarBulletList', action: 'ul' },
      { key: 'ol', icon: 'list-numbered', tooltip: 'toolbarOrderedList', action: 'ol' },
      { key: 'task', icon: 'check-rectangle', tooltip: 'toolbarTaskList', action: 'task' },
      { key: 'quote', icon: 'quote', tooltip: 'toolbarBlockquote', action: 'quote' },
    ],
  },
  {
    key: 'insert',
    buttons: [
      { key: 'codeblock', icon: 'code-1', tooltip: 'toolbarCodeBlock', action: 'codeblock' },
      { key: 'link', icon: 'link', tooltip: 'toolbarLink', action: 'link' },
      { key: 'image', icon: 'image', tooltip: 'toolbarImage', action: 'image' },
      { key: 'table', icon: 'table', tooltip: 'toolbarTable', action: 'table' },
      { key: 'hr', icon: 'component-divider-horizontal', tooltip: 'toolbarHorizontalRule', action: 'hr' },
    ],
  },
];

export function BookmarkAnswerDialog({ client, copy, open, initialTitle, initialContent, locale = 'zh-CN', onClose, onSaved }: BookmarkAnswerDialogProps): ReactNode {
  const strings = manualEditorStrings(locale);
  const [title, setTitle] = useState(initialTitle);
  const [content, setContent] = useState(initialContent);
  const [kbOptions, setKbOptions] = useState<Array<{ id: string; name: string }>>([]);
  const [kbId, setKbId] = useState('');
  const [loadState, setLoadState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [view, setView] = useState<'edit' | 'preview'>('edit');
  const [saving, setSaving] = useState(false);
  const [warning, setWarning] = useState('');
  const [error, setError] = useState('');
  const [drawerWidth, setDrawerWidth] = useState(() => initialDrawerWidth());
  const [resizing, setResizing] = useState(false);
  const resizingRef = useRef(false);

  useEffect(() => {
    if (!open) return;
    setTitle(initialTitle);
    setContent(initialContent);
    setKbId('');
    setView('edit');
    setWarning('');
    setError('');
    setDrawerWidth(initialDrawerWidth());
  }, [open, initialTitle, initialContent]);

  useEffect(() => {
    if (!open) return;
    let active = true;
    setLoadState('loading');
    // The Vue manual editor lists document-type KBs only (manual-knowledge-
    // editor.vue), so faq KBs are filtered out here too.
    void client.knowledgeBases.list()
      .then((bases) => {
        if (!active) return;
        const options = bases.filter((base) => base.type !== 'faq').map((base) => ({ id: base.id, name: base.name }));
        setKbOptions(options);
        // Vue manual-knowledge-editor.vue initialize (471-483): create mode
        // without a preset kbId preselects the first KB, so the select shows
        // the KB name instead of the placeholder (px-chat-addtokb parity).
        setKbId((prev) => prev || options[0]?.id || '');
        setLoadState('ready');
      })
      .catch(() => {
        if (!active) return;
        setKbOptions([]);
        setLoadState('error');
      });
    return () => { active = false; };
  }, [client, open]);

  const previewHtml = useMemo(
    () => (view === 'preview' && content.trim() ? renderChatMarkdown(content) : ''),
    [view, content],
  );

  if (!open) return null;

  /* Vue manual-knowledge-editor.vue validateForm (600-618): KB → title →
   * content, then the publish-only minimum length (>= 10 trimmed chars). */
  function validate(targetStatus: 'draft' | 'publish'): boolean {
    if (!kbId) {
      setWarning(strings.selectKbWarning);
      return false;
    }
    if (!title.trim()) {
      setWarning(strings.enterTitleWarning);
      return false;
    }
    if (!content.trim()) {
      setWarning(strings.enterContentWarning);
      return false;
    }
    if (targetStatus === 'publish' && content.trim().length < 10) {
      setWarning(strings.contentTooShortWarning);
      return false;
    }
    return true;
  }

  async function save(targetStatus: 'draft' | 'publish') {
    if (saving || !validate(targetStatus)) {
      return;
    }
    setSaving(true);
    setWarning('');
    setError('');
    try {
      await client.knowledgeBases.documents.createManual(kbId, {
        title: title.trim(),
        content,
        status: targetStatus,
        // Vue create-from-chat carries no tag ids (upload-confirm owns tags).
        tag_ids: [],
      });
      onSaved?.(targetStatus);
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : copy.operationFailed);
    } finally {
      setSaving(false);
    }
  }

  /** Vue SettingDrawer onResizeStart/Move/End —— 左缘把手拖宽 + localStorage 持久化。 */
  function startResize(e: React.MouseEvent) {
    e.preventDefault();
    if (resizingRef.current) return;
    resizingRef.current = true;
    setResizing(true);
    const startX = e.clientX;
    const startWidth = drawerWidth;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    const onMove = (move: MouseEvent) => {
      setDrawerWidth(clampDrawerWidth(startWidth + (startX - move.clientX)));
    };
    const onUp = () => {
      document.removeEventListener('pointermove', onMove);
      document.removeEventListener('pointerup', onUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      resizingRef.current = false;
      setResizing(false);
      try {
        window.localStorage.setItem(DRAWER_STORAGE_KEY, String(clampDrawerWidth(startWidth)));
      } catch { /* storage unavailable */ }
    };
    document.addEventListener('pointermove', onMove);
    document.addEventListener('pointerup', onUp);
  }

  const kbEmpty = loadState === 'ready' && kbOptions.length === 0;

  const header = (
    <div className="setting-drawer__header-block">
      <div className="setting-drawer__header">
        <div className="setting-drawer__header-icon"><TIcon name="edit-1" /></div>
        <div className="setting-drawer__header-text">
          <div className="setting-drawer__title">{strings.titleCreate}</div>
          <div className="setting-drawer__subtitle">{strings.description}</div>
        </div>
      </div>
    </div>
  );

  const footer = (
    <div className="setting-drawer__footer">
      <div className="setting-drawer__footer-left">
        <div className="manual-editor-footer-meta">
          <Tag size="small" theme="warning" variant="light">{strings.draftTag}</Tag>
        </div>
      </div>
      <div className="setting-drawer__footer-right">
        <div className="manual-editor-footer-actions">
          <Button theme="default" variant="outline" className="manual-editor-cancel-btn" disabled={saving} onClick={onClose}>
            {strings.cancel}
          </Button>
          <Button theme="default" variant="outline" loading={saving} onClick={() => void save('draft')}>
            {strings.saveDraft}
          </Button>
          <Button theme="primary" loading={saving} onClick={() => void save('publish')}>
            {strings.publish}
          </Button>
        </div>
      </div>
    </div>
  );

  return (
    <>
      {/* Vue SettingDrawer resize-handle（teleport body，fixed 定位于抽屉左缘） */}
      <div
        className={'setting-drawer-resize-handle' + (resizing ? ' setting-drawer-resize-handle--active' : '')}
        style={{ right: `${drawerWidth}px`, ['--setting-drawer-travel' as string]: `${drawerWidth}px` }}
        role="separator"
        aria-orientation="vertical"
        onMouseDown={(e) => startResize(e)}
      >
        <div className="setting-drawer-resize-line" />
      </div>
      <Drawer
        visible={open}
        placement="right"
        size={`${drawerWidth}px`}
        header={header}
        footer={footer}
        closeBtn={false}
        zIndex={2500}
        destroyOnClose
        className={'setting-drawer' + (resizing ? ' setting-drawer--resizing' : '')}
        onClose={onClose}
      >
        <div className="setting-drawer__body">
          <div className="manual-editor">
            <section className="setting-drawer__section">
              <h4 className="setting-drawer__section-title">{strings.sectionBasic}</h4>
              <div className="form-item">
                <label className="form-label required">{strings.titleLabel}</label>
                <Input
                  value={title}
                  maxlength={100}
                  showLimitNumber
                  placeholder={strings.titlePlaceholder}
                  onChange={(value) => setTitle(String(value ?? ''))}
                />
              </div>
              <div className="form-item">
                <label className="form-label required">{strings.kbLabel}</label>
                <div className="kb-row">
                  <Select
                    value={kbId}
                    loading={loadState === 'loading'}
                    options={kbOptions.map((option) => ({ value: option.id, label: option.name }))}
                    placeholder={strings.kbPlaceholder}
                    popupProps={{ zIndex: 2600 }}
                    onChange={(value) => { setKbId(String(value ?? '')); setWarning(''); }}
                  />
                </div>
              </div>
            </section>

            <section className="setting-drawer__section editor-section">
              <h4 className="setting-drawer__section-title">{strings.sectionContent}</h4>
              <div className="editor-area">
                <div className="editor-toolbar">
                  <div className="editor-toolbar__format">
                    {TOOLBAR_GROUPS.map((group, groupIndex) => (
                      <span key={group.key} style={{ display: 'contents' }}>
                        <div className="toolbar-group">
                          {group.buttons.map((btn) => (
                            <Tooltip key={btn.key} content={strings[btn.tooltip]} placement="top">
                              <button
                                type="button"
                                className={`toolbar-btn btn-${btn.key}`}
                                onMouseDown={(e) => e.preventDefault()}
                                onClick={() => {
                                  const textarea = document.querySelector('.editor-textarea textarea');
                                  if (textarea instanceof HTMLTextAreaElement) runToolbarAction(btn.action, textarea, setContent, strings);
                                }}
                              >
                                <TIcon name={btn.icon} size="18px" />
                              </button>
                            </Tooltip>
                          ))}
                        </div>
                        {groupIndex < TOOLBAR_GROUPS.length - 1 ? <div className="toolbar-divider" /> : null}
                      </span>
                    ))}
                  </div>
                  <div className="editor-toolbar__view">
                    <Button
                      variant="text"
                      theme="primary"
                      size="small"
                      className={'toggle-view-btn' + (view === 'preview' ? ' is-preview' : '')}
                      disabled={saving}
                      onClick={() => setView((prev) => (prev === 'edit' ? 'preview' : 'edit'))}
                      icon={<TIcon name={view === 'preview' ? 'edit-1' : 'browse'} />}
                    >
                      {view === 'preview' ? strings.editLabel : strings.previewLabel}
                    </Button>
                  </div>
                </div>

                <div className="editor-pane" style={{ display: view === 'edit' ? undefined : 'none' }}>
                  <Textarea
                    value={content}
                    placeholder={strings.contentPlaceholder}
                    className="editor-textarea"
                    onChange={(value) => setContent(String(value ?? ''))}
                  />
                </div>
                <div className="editor-pane editor-pane--preview" style={{ display: view === 'preview' ? undefined : 'none' }}>
                  <div
                    className="preview-container"
                    dangerouslySetInnerHTML={{ __html: previewHtml || `<p class="empty-preview">${strings.previewEmpty}</p>` }}
                  />
                </div>
              </div>
            </section>

            {kbEmpty ? <p className="manual-editor__hint">{strings.noDocumentKnowledgeBases}</p> : null}
            {loadState === 'error' ? <p role="alert" className="manual-editor__hint">{strings.noDocumentKnowledgeBases}</p> : null}
            {warning ? <p role="alert" className="manual-editor__warning">{warning}</p> : null}
            {error ? <p role="alert" className="manual-editor__warning">{error}</p> : null}
          </div>
        </div>
      </Drawer>
    </>
  );
}
