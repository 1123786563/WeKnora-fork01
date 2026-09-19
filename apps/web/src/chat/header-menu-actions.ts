/*
 * R483 D16 — chat header ⋯ menu utility block, ported from the Vue baseline:
 *
 * - frontend/src/components/ChatHeader.vue:61-78 — the four items between
 *   修改标题 and 清空消息: 复制会话 ID / 复制对话链接 / 复制为 Markdown /
 *   在新窗口中打开 (with dividers around the block);
 * - frontend/src/utils/clipboard.ts — copyToClipboard: async Clipboard API
 *   first, hidden textarea + document.execCommand('copy') fallback for
 *   non-secure origins or denied permission;
 * - frontend/src/utils/sessionMarkdown.ts — collectAllSessionMessages
 *   (backward paging via before_time, id dedupe, time+role sort) and
 *   buildSessionMarkdown (title header, session id + export-time quote,
 *   user/assistant sections, attachments and deduped reference links,
 *   <kb/>/<web/> citation tags stripped);
 * - frontend/src/api/chat/index.ts getMessageList — the page fetch maps the
 *   Vue created_at param onto the same before_time query the React
 *   sessions.messages client uses.
 *
 * Copy values are byte-exact from the Vue locale chatHeader blocks; they live
 * here (host domain) so the shared chat-copy table stays untouched while the
 * i18n lane works in parallel.
 */

export type HeaderUtilityLocale = 'zh-CN' | 'en-US' | 'ja-JP' | 'ko-KR' | 'ru-RU';

export interface HeaderUtilityCopyTable {
  copySessionId: string;
  copyLink: string;
  copyMarkdown: string;
  openNewWindow: string;
  sessionIdCopied: string;
  linkCopied: string;
  copyFailed: string;
  markdownCopied: string;
  markdownCopyFailed: string;
  markdown: {
    sessionId: string;
    exportedAt: string;
    user: string;
    assistant: string;
    attachments: string;
    references: string;
  };
}

const HEADER_UTILITY_COPY: Record<HeaderUtilityLocale, HeaderUtilityCopyTable> = {
  'zh-CN': {
    copySessionId: '复制会话 ID',
    copyLink: '复制对话链接',
    copyMarkdown: '复制为 Markdown',
    openNewWindow: '在新窗口中打开',
    sessionIdCopied: '会话 ID 已复制',
    linkCopied: '对话链接已复制',
    copyFailed: '复制失败，请检查浏览器剪贴板权限',
    markdownCopied: '完整对话已复制为 Markdown',
    markdownCopyFailed: '复制 Markdown 失败，请稍后重试',
    markdown: {
      sessionId: '会话 ID',
      exportedAt: '导出时间',
      user: '用户',
      assistant: '助手',
      attachments: '附件',
      references: '引用',
    },
  },
  'en-US': {
    copySessionId: 'Copy Session ID',
    copyLink: 'Copy Conversation Link',
    copyMarkdown: 'Copy as Markdown',
    openNewWindow: 'Open in New Window',
    sessionIdCopied: 'Session ID copied',
    linkCopied: 'Conversation link copied',
    copyFailed: 'Copy failed. Check your browser clipboard permission.',
    markdownCopied: 'Full conversation copied as Markdown',
    markdownCopyFailed: 'Failed to copy Markdown. Please try again.',
    markdown: {
      sessionId: 'Session ID',
      exportedAt: 'Exported at',
      user: 'User',
      assistant: 'Assistant',
      attachments: 'Attachments',
      references: 'References',
    },
  },
  'ja-JP': {
    copySessionId: 'セッションIDをコピー',
    copyLink: '会話リンクをコピー',
    copyMarkdown: 'Markdownとしてコピー',
    openNewWindow: '新しいウィンドウで開く',
    sessionIdCopied: 'セッションIDをコピーしました',
    linkCopied: '会話リンクをコピーしました',
    copyFailed: 'コピーに失敗しました。ブラウザのクリップボード権限を確認してください。',
    markdownCopied: '会話全体をMarkdownとしてコピーしました',
    markdownCopyFailed: 'Markdownのコピーに失敗しました。再試行してください。',
    markdown: {
      sessionId: 'セッションID',
      exportedAt: 'エクスポート日時',
      user: 'ユーザ',
      assistant: 'アシスタント',
      attachments: '添付ファイル',
      references: '出典',
    },
  },
  'ko-KR': {
    copySessionId: '세션 ID 복사',
    copyLink: '대화 링크 복사',
    copyMarkdown: 'Markdown으로 복사',
    openNewWindow: '새 창에서 열기',
    sessionIdCopied: '세션 ID가 복사되었습니다',
    linkCopied: '대화 링크가 복사되었습니다',
    copyFailed: '복사하지 못했습니다. 브라우저 클립보드 권한을 확인하세요.',
    markdownCopied: '전체 대화가 Markdown으로 복사되었습니다',
    markdownCopyFailed: 'Markdown 복사에 실패했습니다. 다시 시도하세요.',
    markdown: {
      sessionId: '세션 ID',
      exportedAt: '내보낸 시간',
      user: '사용자',
      assistant: '어시스턴트',
      attachments: '첨부 파일',
      references: '참조',
    },
  },
  'ru-RU': {
    copySessionId: 'Копировать ID сессии',
    copyLink: 'Копировать ссылку на диалог',
    copyMarkdown: 'Копировать как Markdown',
    openNewWindow: 'Открыть в новом окне',
    sessionIdCopied: 'ID сессии скопирован',
    linkCopied: 'Ссылка на диалог скопирована',
    copyFailed: 'Не удалось скопировать. Проверьте разрешение браузера на доступ к буферу обмена.',
    markdownCopied: 'Весь диалог скопирован как Markdown',
    markdownCopyFailed: 'Не удалось скопировать Markdown. Повторите попытку.',
    markdown: {
      sessionId: 'ID сессии',
      exportedAt: 'Время экспорта',
      user: 'Пользователь',
      assistant: 'Ассистент',
      attachments: 'Вложения',
      references: 'Источники',
    },
  },
};

export function headerUtilityCopy(locale: string): HeaderUtilityCopyTable {
  return HEADER_UTILITY_COPY[locale as HeaderUtilityLocale] ?? HEADER_UTILITY_COPY['zh-CN'];
}

/** Vue ChatHeader.vue currentSessionLink: the URL minus query and hash. */
export function currentSessionLink(url: URL): string {
  const link = new URL(url.toString());
  link.search = '';
  link.hash = '';
  return link.toString();
}

/** Vue clipboard.ts copyToClipboard: async API first, execCommand fallback. */
export async function copyTextToClipboard(text: string): Promise<boolean> {
  if (!text) return false;
  const clipboard = (navigator as { clipboard?: { writeText?(value: string): Promise<void> } }).clipboard;
  if (clipboard && typeof clipboard.writeText === 'function') {
    try {
      await clipboard.writeText(text);
      return true;
    } catch {
      // Fall through to the legacy path (permission denied).
    }
  }
  try {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.top = '0';
    textArea.style.left = '0';
    textArea.style.opacity = '0';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(textArea);
    return ok;
  } catch {
    return false;
  }
}

export interface SessionExportAttachment {
  file_name?: string;
}

export interface SessionExportReference {
  knowledge_title?: string;
  knowledge_filename?: string;
  knowledge_source?: string;
  chunk_type?: string;
  metadata?: Record<string, string>;
}

export interface SessionExportMessage {
  id?: string;
  role?: string;
  content?: string;
  created_at?: string;
  attachments?: SessionExportAttachment[];
  knowledge_references?: SessionExportReference[];
}

export type SessionMessagePageFetcher = (beforeTime: string, limit: number) => Promise<SessionExportMessage[]>;

function roleOrder(role?: string): number {
  if (role === 'user') return 0;
  if (role === 'assistant') return 1;
  return 2;
}

/** Vue sessionMarkdown.ts collectAllSessionMessages (page, dedupe, sort). */
export async function collectAllSessionMessages(
  fetchPage: SessionMessagePageFetcher,
  pageSize = 100,
  maxPages = 500,
): Promise<SessionExportMessage[]> {
  const pages: SessionExportMessage[][] = [];
  let beforeTime = '';

  for (let pageIndex = 0; pageIndex < maxPages; pageIndex += 1) {
    const page = await fetchPage(beforeTime, pageSize);
    if (!Array.isArray(page) || page.length === 0) break;
    pages.unshift(page);

    const oldestTime = page[0]?.created_at || '';
    if (page.length < pageSize || !oldestTime || oldestTime === beforeTime) break;
    beforeTime = oldestTime;
  }

  const seen = new Set<string>();
  return pages
    .flat()
    .filter((message) => {
      const key = message.id
        || `${message.role || ''}:${message.created_at || ''}:${message.content || ''}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    })
    .sort((a, b) => {
      const timeCompare = String(a.created_at || '').localeCompare(String(b.created_at || ''));
      return timeCompare || roleOrder(a.role) - roleOrder(b.role);
    });
}

/** Vue citationMarkdown.ts KB_WEB_TAG_RE — self-closing/unclosed kb/web tags. */
const KB_WEB_TAG_RE = /<(?:kb|web)\b[^>]*?\s*\/?>/g;

function cleanMessageContent(content?: string): string {
  return String(content || '')
    .replace(KB_WEB_TAG_RE, '')
    .replace(/[ \t]+\n/g, '\n')
    .trim();
}

function markdownListText(value: string): string {
  return value.replace(/\s+/g, ' ').replace(/([\\`*_[\]<>])/g, '\\$1').trim();
}

function isHttpUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function referenceLine(reference: SessionExportReference): string {
  const title = reference.knowledge_title
    || reference.knowledge_filename
    || reference.metadata?.title
    || reference.knowledge_source
    || '';
  if (!title) return '';

  const source = reference.metadata?.url || reference.knowledge_source || '';
  const safeTitle = markdownListText(title);
  return isHttpUrl(source) ? `- [${safeTitle}](${source})` : `- ${safeTitle}`;
}

/** Vue sessionMarkdown.ts buildSessionMarkdown. */
export function buildSessionMarkdown(options: {
  sessionId: string;
  title: string;
  messages: SessionExportMessage[];
  labels: HeaderUtilityCopyTable['markdown'];
  exportedAt?: string;
}): string {
  const { sessionId, labels } = options;
  const title = options.title.replace(/\s+/g, ' ').trim() || sessionId;
  const exportedAt = options.exportedAt || new Date().toISOString();
  const blocks = [
    `# ${title.replace(/^#+\s*/, '')}`,
    `> ${labels.sessionId}: ${sessionId}  `,
    `> ${labels.exportedAt}: ${exportedAt}`,
  ];

  for (const message of options.messages) {
    if (message.role !== 'user' && message.role !== 'assistant') continue;
    const content = cleanMessageContent(message.content);
    const attachments = (message.attachments || [])
      .map((attachment) => attachment.file_name?.trim() || '')
      .filter(Boolean);
    const references = [...new Set(
      (message.knowledge_references || []).map(referenceLine).filter(Boolean),
    )];
    if (!content && attachments.length === 0 && references.length === 0) continue;

    blocks.push(`## ${message.role === 'user' ? labels.user : labels.assistant}`);
    if (content) blocks.push(content);
    if (attachments.length > 0) {
      blocks.push(`### ${labels.attachments}`, ...attachments.map((name) => `- ${markdownListText(name)}`));
    }
    if (references.length > 0) {
      blocks.push(`### ${labels.references}`, ...references);
    }
  }

  return `${blocks.join('\n\n')}\n`;
}

export interface HeaderUtilityItem {
  id: string;
  label: string;
  onActivate(): void;
}

/**
 * Builds the four Vue header utility actions in the Vue order:
 * copySessionId, copyLink, copyMarkdown, openNewWindow (ChatHeader.vue
 * handleMenuClick). Toasts mirror the Vue MessagePlugin feedback; failures
 * fall back to the localized copy-failure / markdown-failure messages.
 */
export function buildHeaderUtilityItems(input: {
  locale: string;
  sessionId: string;
  sessionTitle: string;
  currentUrl: URL;
  loadMessagesPage: SessionMessagePageFetcher;
  toast(message: string): void;
  openWindow(url: string): void;
  now?(): Date;
}): HeaderUtilityItem[] {
  const copy = headerUtilityCopy(input.locale);
  const link = currentSessionLink(input.currentUrl);
  const copyWithFeedback = async (text: string, success: string, failure: string): Promise<void> => {
    const ok = await copyTextToClipboard(text);
    input.toast(ok ? success : failure);
  };

  return [
    {
      id: 'copySessionId',
      label: copy.copySessionId,
      onActivate: () => { void copyWithFeedback(input.sessionId, copy.sessionIdCopied, copy.copyFailed); },
    },
    {
      id: 'copyLink',
      label: copy.copyLink,
      onActivate: () => { void copyWithFeedback(link, copy.linkCopied, copy.copyFailed); },
    },
    {
      id: 'copyMarkdown',
      label: copy.copyMarkdown,
      onActivate: () => {
        void (async () => {
          try {
            const messages = await collectAllSessionMessages(input.loadMessagesPage);
            const markdown = buildSessionMarkdown({
              sessionId: input.sessionId,
              title: input.sessionTitle,
              messages,
              labels: copy.markdown,
              exportedAt: (input.now ? input.now() : new Date()).toISOString(),
            });
            await copyWithFeedback(markdown, copy.markdownCopied, copy.markdownCopyFailed);
          } catch {
            input.toast(copy.markdownCopyFailed);
          }
        })();
      },
    },
    {
      id: 'openNewWindow',
      label: copy.openNewWindow,
      onActivate: () => { input.openWindow(link); },
    },
  ];
}
