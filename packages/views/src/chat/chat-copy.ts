/*
 * Chat copy for the React rendering layer, keyed by the active UI locale.
 *
 * Values are ported byte-exact from the authoritative Vue locales
 * (frontend/src/i18n/locales/{zh-CN,en-US,ja-JP,ko-KR,ru-RU}.ts) for every key
 * that exists there — the Vue source path is annotated on each zh-CN entry.
 * Keys the Vue locale files do not define (React-side affordances with no Vue
 * counterpart) keep their zh-CN string in all locales and are annotated
 * zh-only; they are recorded as a gap in
 * docs/migrations/react/evidence/vue-react-parity/2026-09-13-i18n-backfill.md.
 *
 * The chat-domain subset of these values is also generated into
 * @weknora/i18n (packages/i18n/src/generated/chat.ts). packages/views cannot
 * depend on @weknora/i18n (see integrations/messages.ts), so this local table
 * mirrors it byte-exact. Components resolve a table with resolveChatCopy()
 * (or take the resolved table via the copy prop); CHAT_COPY remains the
 * zh-CN default so existing consumers keep working.
 */

export const CHAT_COPY_LOCALES = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;

export type ChatCopyLocale = (typeof CHAT_COPY_LOCALES)[number];

/** The zh-CN table doubles as the key inventory for the other locales. */
const CHAT_COPY_ZH = {
/** chat.suggestedQuestions */
suggestedQuestions: '你可以这样问我',
/** chat.refreshSuggestedQuestions */
refreshSuggestedQuestions: '换一批',
/** chat.followUpQuestions */
followUpQuestions: '继续问',
/** chat.followUpQuestionsLoading */
followUpQuestionsLoading: '加载推荐问题',
/** chat.thinkingAlt */
thinkingAlt: '正在思考',
/** chat.fallbackHint */
fallbackHint: '未从知识库中检索到相关内容，以上为模型直接回答',
/** createChat.title */
createChatTitle: 'Hi，我是 WeKnora，让你的知识触手可及',
/** menu.newSession */
newSession: '新会话',
/** menu.myChats */
sidebarTitle: '我的对话',
/** menu.newChat */
newChat: '新对话',
/** menu.search */
searchSessions: '搜索',
/** knowledgeBase.columnSource */
sourceLabel: '来源',
/** embed.unknownLink */
unknownLink: '未知链接',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
groupLabel: '分组',
/** common.all */
groupAll: '全部',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
groupByDate: '按日期',
/** common.loading */
loadingSessions: '加载中...',
/** common.loading */
loadingMessages: '加载中...',
/** common.loading */
loadingHistory: '加载中...',
/** common.loadMore */
loadOlder: '加载更多',
/** menu.newSession */
untitledChat: '新会话',
/** input.send */
send: '发送',
/** input.stopGeneration */
stopGeneration: '停止生成',
/** input.placeholder */
composerPlaceholder: '直接向模型提问',
/** input.normalMode */
quickAnswer: '快速问答',
/** agent.selectAgent */
selectAgent: '选择智能体',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
uploadAttachment: '上传附件',
/** input.knowledgeBase */
mentionKnowledge: '知识库',
mentionNoResults: '暂无匹配知识库',
mentionNoAvailable: '暂无可用知识库',
/** chat.noRelatedChunks */
noRelatedChunks: '没有找到相关片段',
/** chat.knowledgeBaseCount */
knowledgeBaseCount: '共 {count} 个知识库',
artifactPreviewBack: '返回列表',
artifactPreviewDownload: '下载',
artifactPreviewLoading: '加载预览中…',
artifactPreviewDownloadOnly: '下载后查看',
artifactPreviewPdf: 'PDF 预览',
artifactPreviewImage: '图片预览',
artifactPreviewMarkdown: 'Markdown 预览',
artifactPreviewText: '文本预览',
referencesTitle: '引用来源',
referenceWebSources: '网页来源',
referenceDocuments: '文档',
referenceToolResults: '工具结果',
referenceChunks: '引用片段',
approvalTitle: '工具调用需审批',
approvalViewArgs: '查看参数',
approvalApprove: '同意',
approvalReject: '拒绝',
approvalResolved: '已处理',
oauthTitle: 'MCP 授权',
oauthTool: '工具',
oauthAuthorize: '去授权',
oauthCancel: '取消',
oauthAuthorized: '已授权',
chatActionsTitle: '操作',
/** input.agentMissingSummaryModel */
modelChip: '对话模型',
attachmentProcessing: '解析中',
steerAttachmentsBlocked: '请先移除附件，再补充当前任务',
/** time.today */
today: '今天',
/** time.yesterday */
yesterday: '昨天',
/** chat.conversationTime.today */
conversationTimeToday: '今天 {time}',
/** chat.conversationTime.yesterday */
conversationTimeYesterday: '昨天 {time}',
/** chat.conversationTime.thisYear */
conversationTimeThisYear: '{month}月{day}日 {time}',
/** chat.conversationTime.otherYear */
conversationTimeOtherYear: '{year}年{month}月{day}日 {time}',
/** chatHeader.moreActions */
moreActions: '更多对话操作',
/** menu.pin */
pin: '置顶',
/** menu.unpin */
unpin: '取消置顶',
/** menu.renameSession */
renameSession: '修改标题',
renameTitle: '修改标题',
renameTitlePlaceholder: '输入会话标题',
renameConfirm: '保存',
renameCancel: '取消',
renameSaving: '保存中…',
renameTitleRequired: '标题不能为空',
renameTitleFailed: '修改标题失败',
/** menu.clearMessages */
clearMessages: '清空消息',
/** upload.deleteRecord (Vue menu.vue session-row delete option) */
deleteRecord: '删除记录',
/** chatHeader.deleteSession */
deleteSession: '删除对话',
/** chatHeader.deleteConfirmBody */
deleteConfirmBody: '确认删除当前对话？删除后将无法恢复。',
/** knowledgeList.loadFailed */
knowledgeBasesLoadFailed: '加载知识库失败',
/** agent.copy */
copy: '复制',
/** common.copied */
copied: '已复制',
/** agent.addToKnowledgeBase */
addToKnowledgeBase: '添加到知识库',
/** chat.requestInfoTitle */
requestInfo: '请求信息',
/** chat.sandbox.tabArtifacts */
artifacts: '产物',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
artifactsPending: '产物生成中…',
/** agent.artifactDrawer.preview */
preview: '预览',
/** common.download */
download: '下载',
/** settings.storage.available */
available: '可用',
/** tenantInvitation.status.expired */
expired: '已过期',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
sending: '发送中…',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
sendFailed: '发送失败',
/** common.retry */
retry: '重试',
/** input.steerCurrent */
steerCurrent: '补充当前任务',
/** input.steerAfter */
steerQueued: '完成后发送',
/** chat.sandbox.panelTitle */
sandboxPanelTitle: '沙箱可视化',
/** chatHeader.toggleSandboxPanel */
openSandboxPanel: '沙箱终端',
/** common.close */
close: '关闭',
/** chat.sandbox.start */
startTerminal: '启动终端',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
terminalInput: '终端输入',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
sendInput: '发送输入',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
closeTerminal: '断开终端',
/** zh-only (no Vue locale source; see 2026-09-13-i18n-backfill.md) */
thinkingAndTools: '思考与工具',
/** knowledgeBase.columnStatus */
streamStatus: '状态',
/** agentStream.grepResults.titleMatch */
grepTitleMatch: '标题匹配',
/** agentStream.grepResults.faqEntry */
grepFaqEntry: 'FAQ 条目',
/** chat.chunkIdLabel */
chunkIdLabel: '片段ID:',
/** chat.documentIdLabel */
documentIdLabel: '文档ID:',
/** chat.positionLabel */
positionLabel: '位置:',
/** agentStream.mcp.status.unavailable */
disabledAgentSuffix: '不可用',
/** chat.rawTextLabel */
rawTextLabel: '原始文本',
/** chat.webFetchPartialContent */
webFetchPartialContent: '部分页面内容',
/** chat.webFetchSummaryFailed */
webFetchSummaryFailed: '摘要失败',
/** chat.summaryLabel */
summaryLabel: '总结',
/** chat.fullContentLabel */
fullContentLabel: '完整内容',
/** chat.contentLengthLabelSimple */
contentLengthLabelSimple: '内容长度:',
/** chat.lengthChars */
lengthChars: '{value} 字',
/** chat.refreshSuggestedQuestions */
suggestedRefresh: '换一批',
/** common.collapse */
dismiss: '收起',
/** time.pinned */
groupPinned: '已置顶',
/** time.today */
groupToday: '今天',
/** time.yesterday */
groupYesterday: '昨天',
/** time.last7Days */
groupLast7Days: '近7天',
/** time.last30Days */
groupLast30Days: '近30天',
/** time.earlier */
groupOlder: '更早',
/** @weknora/i18n common.* bundle (no Vue key) */
previous: '上一页',
/** @weknora/i18n common.* bundle (no Vue key) */
next: '下一步',
/** @weknora/i18n common.* bundle (no Vue key) */
pageOf: '第 {page} 页 / 共 {total} 页',
batchManage: '批量管理',
batchCancel: '取消',
batchSelectAll: '全选会话',
batchDelete: '删除所选 ({count})',
batchDeleteConfirm: '确认删除选中的 {count} 个会话？删除后将无法恢复。',
batchDeleteError: '批量删除失败：{message}',
batchRetry: '重试',
batchDeleteBusy: '删除中…',
batchSelectSession: '选择会话 {title}',
sourceSelectLabel: '会话来源',
};

export type ChatCopyKey = keyof typeof CHAT_COPY_ZH;

export type ChatCopyTable = Record<ChatCopyKey, string>;

/** All five locale tables; key sets are identical by construction. */
const CHAT_COPY_TABLES: Record<ChatCopyLocale, ChatCopyTable> = {
  'zh-CN': CHAT_COPY_ZH,
  'en-US': {
  suggestedQuestions: 'You can ask me',
  refreshSuggestedQuestions: 'More',
  followUpQuestions: 'Keep asking',
  followUpQuestionsLoading: 'Loading suggested questions',
  thinkingAlt: 'Thinking in progress',
  fallbackHint: 'No relevant content found in knowledge base. Above is a direct response from the model.',
  createChatTitle: 'Hi, I am WeKnora — your knowledge, within reach',
  newSession: 'New Chat',
  sidebarTitle: 'My chats',
  newChat: 'New Chat',
  searchSessions: 'Search',
  sourceLabel: 'Source',
  unknownLink: 'Unknown link',
  groupLabel: '分组',
  groupAll: 'All',
  groupByDate: '按日期',
  loadingSessions: 'Loading...',
  loadingMessages: 'Loading...',
  loadingHistory: 'Loading...',
  loadOlder: 'Load more',
  untitledChat: 'New Chat',
  send: 'Send',
  stopGeneration: 'Stop Generation',
  composerPlaceholder: 'Ask questions directly to the model',
  quickAnswer: 'Quick Answer',
  selectAgent: 'Select Agent',
  uploadAttachment: '上传附件',
  mentionKnowledge: 'Knowledge Base',
  mentionNoResults: 'No matching knowledge bases',
  mentionNoAvailable: 'No knowledge bases available',
  noRelatedChunks: 'No related chunks found',
  knowledgeBaseCount: '{count} knowledge bases',
  artifactPreviewBack: 'Back to list',
  artifactPreviewDownload: 'Download',
  artifactPreviewLoading: 'Loading preview…',
  artifactPreviewDownloadOnly: 'Download to view',
  artifactPreviewPdf: 'PDF preview',
  artifactPreviewImage: 'Image preview',
  artifactPreviewMarkdown: 'Markdown preview',
  artifactPreviewText: 'Text preview',
  referencesTitle: 'References',
  referenceWebSources: 'Web sources',
  referenceDocuments: 'Documents',
  referenceToolResults: 'Tool results',
  referenceChunks: 'Reference chunks',
  approvalTitle: 'Tool approval required',
  approvalViewArgs: 'View arguments',
  approvalApprove: 'Approve',
  approvalReject: 'Reject',
  approvalResolved: 'Resolved',
  oauthTitle: 'MCP authorization',
  oauthTool: 'Tool',
  oauthAuthorize: 'Authorize',
  oauthCancel: 'Cancel',
  oauthAuthorized: 'Authorized',
  chatActionsTitle: 'Actions',
  modelChip: 'Chat model',
  attachmentProcessing: 'Parsing…',
  steerAttachmentsBlocked: 'Remove attachments before steering the current task.',
  today: 'Today',
  yesterday: 'Yesterday',
  conversationTimeToday: 'Today {time}',
  conversationTimeYesterday: 'Yesterday {time}',
  conversationTimeThisYear: '{month}/{day} {time}',
  conversationTimeOtherYear: '{month}/{day}/{year} {time}',
  moreActions: 'More conversation actions',
  pin: 'Pin',
  unpin: 'Unpin',
  renameSession: 'Rename',
renameTitle: 'Rename conversation',
renameTitlePlaceholder: 'Enter a conversation title',
renameConfirm: 'Save',
renameCancel: 'Cancel',
renameSaving: 'Saving…',
renameTitleRequired: 'Title is required',
renameTitleFailed: 'Unable to rename conversation',
  clearMessages: 'Clear Messages',
  deleteRecord: 'Delete Record',
  deleteSession: 'Delete Conversation',
  deleteConfirmBody: 'Delete this conversation? This cannot be undone.',
  knowledgeBasesLoadFailed: 'Failed to load knowledge bases',
  copy: 'Copy',
  copied: 'Copied',
  addToKnowledgeBase: 'Add to Knowledge Base',
  requestInfo: 'Request info',
  artifacts: 'Files',
  artifactsPending: '产物生成中…',
  preview: 'Preview',
  download: 'Download',
  available: 'Available',
  expired: 'Expired',
  sending: '发送中…',
  sendFailed: '发送失败',
  retry: 'Retry',
  steerCurrent: 'Supplement current task',
  steerQueued: 'Send after completion',
  sandboxPanelTitle: 'Sandbox',
  openSandboxPanel: 'Sandbox terminal',
  close: 'Close',
  startTerminal: 'Start terminal',
  terminalInput: '终端输入',
  sendInput: '发送输入',
  closeTerminal: '断开终端',
  thinkingAndTools: '思考与工具',
  streamStatus: 'Status',
  grepTitleMatch: 'title',
  grepFaqEntry: 'FAQ entry',
  chunkIdLabel: 'Chunk ID:',
  documentIdLabel: 'Document ID:',
  positionLabel: 'Position:',
  disabledAgentSuffix: 'Unavailable',
  rawTextLabel: 'Raw text',
  webFetchPartialContent: 'Partial page',
  webFetchSummaryFailed: 'Summary failed',
  summaryLabel: 'Summary',
  fullContentLabel: 'Full content',
  contentLengthLabelSimple: 'Content length:',
  lengthChars: '{value} characters',
  suggestedRefresh: 'More',
  dismiss: 'Collapse',
  groupPinned: 'Pinned',
  groupToday: 'Today',
  groupYesterday: 'Yesterday',
  groupLast7Days: 'Last 7 Days',
  groupLast30Days: 'Last 30 Days',
  groupOlder: 'Earlier',
  previous: 'Previous',
  next: 'Next',
  pageOf: 'Page {page} of {total}',
  batchManage: 'Batch manage',
  batchCancel: 'Cancel',
  batchSelectAll: 'Select all chats',
  batchDelete: 'Delete selected ({count})',
  batchDeleteConfirm: 'Delete the selected {count} chats? This cannot be undone.',
  batchDeleteError: 'Batch delete failed: {message}',
  batchRetry: 'Retry',
  batchDeleteBusy: 'Deleting…',
  batchSelectSession: 'Select chat {title}',
  sourceSelectLabel: 'Chat source',
  },
  'ja-JP': {
  suggestedQuestions: 'こんな質問ができます',
  refreshSuggestedQuestions: '別の候補',
  followUpQuestions: '続けて質問',
  followUpQuestionsLoading: '質問候補を読み込み中',
  thinkingAlt: '思考中',
  fallbackHint: 'ナレッジベースから関連する内容が見つかりませんでした。上記はモデルの直接回答です。',
  createChatTitle: 'こんにちは、WeKnoraです。あなたのナレッジを、すぐそばに',
  newSession: '新しいチャット',
  sidebarTitle: 'マイチャット',
  newChat: '新しいチャット',
  searchSessions: '検索',
  sourceLabel: '取得元',
  unknownLink: '不明なリンク',
  groupLabel: '分组',
  groupAll: 'すべて',
  groupByDate: '按日期',
  loadingSessions: '読み込み中...',
  loadingMessages: '読み込み中...',
  loadingHistory: '読み込み中...',
  loadOlder: 'さらに読み込む',
  untitledChat: '新しいチャット',
  send: '送信',
  stopGeneration: '生成を停止',
  composerPlaceholder: 'モデルに直接質問できます',
  quickAnswer: 'クイック回答',
  selectAgent: 'エージェントを選択',
  uploadAttachment: '上传附件',
  mentionKnowledge: 'ナレッジベース',
  mentionNoResults: '一致するナレッジベースがありません',
  mentionNoAvailable: '利用可能なナレッジベースがありません',
  noRelatedChunks: '関連するチャンクが見つかりません',
  knowledgeBaseCount: '{count}件のナレッジベース',
  artifactPreviewBack: 'リストに戻る',
  artifactPreviewDownload: 'ダウンロード',
  artifactPreviewLoading: 'プレビューを読み込み中…',
  artifactPreviewDownloadOnly: 'ダウンロードして表示',
  artifactPreviewPdf: 'PDFプレビュー',
  artifactPreviewImage: '画像プレビュー',
  artifactPreviewMarkdown: 'Markdownプレビュー',
  artifactPreviewText: 'テキストプレビュー',
  referencesTitle: '引用元',
  referenceWebSources: 'ウェブソース',
  referenceDocuments: 'ドキュメント',
  referenceToolResults: 'ツール結果',
  referenceChunks: '引用チャンク',
  approvalTitle: 'ツール呼び出しの承認',
  approvalViewArgs: '引数を表示',
  approvalApprove: '承認',
  approvalReject: '拒否',
  approvalResolved: '処理済み',
  oauthTitle: 'MCP 認証',
  oauthTool: 'ツール',
  oauthAuthorize: '認証する',
  oauthCancel: 'キャンセル',
  oauthAuthorized: '認証済み',
  chatActionsTitle: '操作',
  modelChip: 'チャットモデル',
  attachmentProcessing: '解析中…',
  steerAttachmentsBlocked: '添付ファイルを削除してから、現在のタスクを補足してください。',
  today: '今日',
  yesterday: '昨日',
  conversationTimeToday: '今日{time}',
  conversationTimeYesterday: '昨日{time}',
  conversationTimeThisYear: '{month}/{day} {time}',
  conversationTimeOtherYear: '{year}/{month}/{day} {time}',
  moreActions: 'その他の会話操作',
  pin: 'ピン留め',
  unpin: 'ピン留めを解除',
  renameSession: 'タイトルを変更',
renameTitle: '会話のタイトルを変更',
renameTitlePlaceholder: '会話のタイトルを入力',
renameConfirm: '保存',
renameCancel: 'キャンセル',
renameSaving: '保存中…',
renameTitleRequired: 'タイトルを入力してください',
renameTitleFailed: 'タイトルを変更できません',
  clearMessages: 'メッセージをクリア',
  deleteRecord: '記録を削除',
  deleteSession: '会話を削除',
  deleteConfirmBody: 'この会話を削除しますか？この操作は取り消せません。',
  knowledgeBasesLoadFailed: 'ナレッジベースの読み込みに失敗しました',
  copy: 'コピー',
  copied: 'コピーしました',
  addToKnowledgeBase: 'ナレッジベースに追加',
  requestInfo: 'リクエスト情報',
  artifacts: 'ファイル',
  artifactsPending: '产物生成中…',
  preview: 'プレビュー',
  download: 'ダウンロード',
  available: '利用可能',
  expired: '期限切れ',
  sending: '发送中…',
  sendFailed: '发送失败',
  retry: '再試行',
  steerCurrent: '現在のタスクに追加',
  steerQueued: '完了後に送信',
  sandboxPanelTitle: 'サンドボックス',
  openSandboxPanel: 'サンドボックスターミナル',
  close: '閉じる',
  startTerminal: 'ターミナルを起動',
  terminalInput: '终端输入',
  sendInput: '发送输入',
  closeTerminal: '断开终端',
  thinkingAndTools: '思考与工具',
  streamStatus: 'ステータス',
  grepTitleMatch: 'タイトル一致',
  grepFaqEntry: 'FAQ項目',
  chunkIdLabel: 'チャンクID:',
  documentIdLabel: 'ドキュメントID:',
  positionLabel: '位置:',
  disabledAgentSuffix: '利用不可',
  rawTextLabel: '元テキスト',
  webFetchPartialContent: 'ページの一部',
  webFetchSummaryFailed: '要約失敗',
  summaryLabel: '要約',
  fullContentLabel: '全文',
  contentLengthLabelSimple: 'コンテンツ長:',
  lengthChars: '{value}文字',
  suggestedRefresh: '別の候補',
  dismiss: '折りたたむ',
  groupPinned: 'ピン留め',
  groupToday: '今日',
  groupYesterday: '昨日',
  groupLast7Days: '過去7日間',
  groupLast30Days: '過去30日間',
  groupOlder: 'それ以前',
  previous: '前へ',
  next: '次へ',
  pageOf: '{total} ページ中 {page} ページ目',
  batchManage: '一括管理',
  batchCancel: 'キャンセル',
  batchSelectAll: '会話をすべて選択',
  batchDelete: '選択を削除 ({count})',
  batchDeleteConfirm: '選択した {count} 件の会話を削除しますか？元に戻せません。',
  batchDeleteError: '一括削除に失敗しました：{message}',
  batchRetry: '再試行',
  batchDeleteBusy: '削除中…',
  batchSelectSession: '会話 {title} を選択',
  sourceSelectLabel: '会話のソース',
  },
  'ko-KR': {
  suggestedQuestions: '이렇게 물어보세요',
  refreshSuggestedQuestions: '다른 질문',
  followUpQuestions: '이어서 질문',
  followUpQuestionsLoading: '추천 질문 로딩 중',
  thinkingAlt: '생각 중',
  fallbackHint: '지식 베이스에서 관련 내용을 찾지 못했습니다. 위는 모델의 직접 응답입니다.',
  createChatTitle: '안녕하세요, WeKnora입니다 — 당신의 지식을 손끝에',
  newSession: '새 세션',
  sidebarTitle: '내 대화',
  newChat: '새 대화',
  searchSessions: '검색',
  sourceLabel: '소스',
  unknownLink: '알 수 없는 링크',
  groupLabel: '分组',
  groupAll: '전체',
  groupByDate: '按日期',
  loadingSessions: '로딩 중...',
  loadingMessages: '로딩 중...',
  loadingHistory: '로딩 중...',
  loadOlder: '더 보기',
  untitledChat: '새 세션',
  send: '전송',
  stopGeneration: '생성 중지',
  composerPlaceholder: '모델에 직접 질문',
  quickAnswer: '일반 모드',
  selectAgent: '에이전트 선택',
  uploadAttachment: '上传附件',
  mentionKnowledge: '지식베이스',
  mentionNoResults: '일치하는 지식베이스가 없습니다',
  mentionNoAvailable: '사용 가능한 지식베이스가 없습니다',
  noRelatedChunks: '관련 청크를 찾을 수 없습니다',
  knowledgeBaseCount: '총 {count}개 지식베이스',
  artifactPreviewBack: '목록으로 돌아가기',
  artifactPreviewDownload: '다운로드',
  artifactPreviewLoading: '미리보기 로드 중…',
  artifactPreviewDownloadOnly: '다운로드하여 보기',
  artifactPreviewPdf: 'PDF 미리보기',
  artifactPreviewImage: '이미지 미리보기',
  artifactPreviewMarkdown: 'Markdown 미리보기',
  artifactPreviewText: '텍스트 미리보기',
  referencesTitle: '참조 출처',
  referenceWebSources: '웹 소스',
  referenceDocuments: '문서',
  referenceToolResults: '도구 결과',
  referenceChunks: '참조 청크',
  approvalTitle: '도구 호출 승인 필요',
  approvalViewArgs: '매개변수 보기',
  approvalApprove: '승인',
  approvalReject: '거부',
  approvalResolved: '처리됨',
  oauthTitle: 'MCP 인증',
  oauthTool: '도구',
  oauthAuthorize: '승인하기',
  oauthCancel: '취소',
  oauthAuthorized: '인증됨',
  chatActionsTitle: '작업',
  modelChip: '대화 모델',
  attachmentProcessing: '분석 중…',
  steerAttachmentsBlocked: '첨부 파일을 제거한 후 현재 작업을 보완하세요.',
  today: '오늘',
  yesterday: '어제',
  conversationTimeToday: '오늘 {time}',
  conversationTimeYesterday: '어제 {time}',
  conversationTimeThisYear: '{month}월 {day}일 {time}',
  conversationTimeOtherYear: '{year}년 {month}월 {day}일 {time}',
  moreActions: '대화 추가 작업',
  pin: '고정',
  unpin: '고정 해제',
  renameSession: '제목 수정',
renameTitle: '대화 제목 수정',
renameTitlePlaceholder: '대화 제목 입력',
renameConfirm: '저장',
renameCancel: '취소',
renameSaving: '저장 중…',
renameTitleRequired: '제목을 입력하세요',
renameTitleFailed: '대화 제목을 수정할 수 없습니다',
  clearMessages: '메시지 지우기',
  deleteRecord: '기록 삭제',
  deleteSession: '대화 삭제',
  deleteConfirmBody: '현재 대화를 삭제할까요? 삭제 후에는 복구할 수 없습니다.',
  knowledgeBasesLoadFailed: '지식베이스 로드 실패',
  copy: '복사',
  copied: '복사됨',
  addToKnowledgeBase: '지식베이스에 추가',
  requestInfo: 'Request info',
  artifacts: '파일',
  artifactsPending: '产物生成中…',
  preview: '미리보기',
  download: '다운로드',
  available: '사용 가능',
  expired: '만료됨',
  sending: '发送中…',
  sendFailed: '发送失败',
  retry: '재시도',
  steerCurrent: '현재 작업에 추가',
  steerQueued: '완료 후 보내기',
  sandboxPanelTitle: '샌드박스',
  openSandboxPanel: '샌드박스 터미널',
  close: '닫기',
  startTerminal: '터미널 시작',
  terminalInput: '终端输入',
  sendInput: '发送输入',
  closeTerminal: '断开终端',
  thinkingAndTools: '思考与工具',
  streamStatus: '상태',
  grepTitleMatch: '제목',
  grepFaqEntry: 'FAQ entry',
  chunkIdLabel: '청크 ID:',
  documentIdLabel: '문서 ID:',
  positionLabel: '위치:',
  disabledAgentSuffix: '사용 불가',
  rawTextLabel: '원본 텍스트',
  webFetchPartialContent: '페이지 일부',
  webFetchSummaryFailed: '요약 실패',
  summaryLabel: '요약',
  fullContentLabel: '전체 내용',
  contentLengthLabelSimple: '내용 길이:',
  lengthChars: '{value}자',
  suggestedRefresh: '다른 질문',
  dismiss: '접기',
  groupPinned: '고정됨',
  groupToday: '오늘',
  groupYesterday: '어제',
  groupLast7Days: '최근 7일',
  groupLast30Days: '최근 30일',
  groupOlder: '이전',
  previous: '이전',
  next: '다음',
  pageOf: '{total} 페이지 중 {page} 페이지',
  batchManage: '일괄 관리',
  batchCancel: '취소',
  batchSelectAll: '대화 모두 선택',
  batchDelete: '선택 삭제 ({count})',
  batchDeleteConfirm: '선택한 {count}개의 대화를 삭제할까요? 되돌릴 수 없습니다.',
  batchDeleteError: '일괄 삭제 실패: {message}',
  batchRetry: '재시도',
  batchDeleteBusy: '삭제 중…',
  batchSelectSession: '대화 {title} 선택',
  sourceSelectLabel: '대화 출처',
  },
  'ru-RU': {
  suggestedQuestions: 'Вы можете спросить меня',
  refreshSuggestedQuestions: 'Ещё',
  followUpQuestions: 'Спрашивайте дальше',
  followUpQuestionsLoading: 'Загрузка рекомендуемых вопросов',
  thinkingAlt: 'Обдумывание...',
  fallbackHint: 'В базе знаний не найдено релевантного содержимого. Выше представлен прямой ответ модели.',
  createChatTitle: 'Привет, я WeKnora — ваши знания всегда под рукой',
  newSession: 'Новый диалог',
  sidebarTitle: 'Мои чаты',
  newChat: 'Новый диалог',
  searchSessions: 'Поиск',
  sourceLabel: 'Источник',
  unknownLink: 'Неизвестная ссылка',
  groupLabel: '分组',
  groupAll: 'Все',
  groupByDate: '按日期',
  loadingSessions: 'Загрузка...',
  loadingMessages: 'Загрузка...',
  loadingHistory: 'Загрузка...',
  loadOlder: 'Загрузить ещё',
  untitledChat: 'Новый диалог',
  send: 'Отправить',
  stopGeneration: 'Остановить генерацию',
  composerPlaceholder: 'Задайте вопрос напрямую модели',
  quickAnswer: 'Быстрый ответ',
  selectAgent: 'Select Agent',
  uploadAttachment: '上传附件',
  mentionKnowledge: 'База знаний',
  mentionNoResults: 'Подходящих баз знаний нет',
  mentionNoAvailable: 'Нет доступных баз знаний',
  noRelatedChunks: 'Связанные фрагменты не найдены',
  knowledgeBaseCount: '{count} баз знаний',
  artifactPreviewBack: 'Вернуться к списку',
  artifactPreviewDownload: 'Скачать',
  artifactPreviewLoading: 'Загрузка предпросмотра…',
  artifactPreviewDownloadOnly: 'Скачайте для просмотра',
  artifactPreviewPdf: 'Предпросмотр PDF',
  artifactPreviewImage: 'Предпросмотр изображения',
  artifactPreviewMarkdown: 'Предпросмотр Markdown',
  artifactPreviewText: 'Предпросмотр текста',
  referencesTitle: 'Источники',
  referenceWebSources: 'Веб-источники',
  referenceDocuments: 'Документы',
  referenceToolResults: 'Результаты инструментов',
  referenceChunks: 'Фрагменты источников',
  approvalTitle: 'Требуется подтверждение инструмента',
  approvalViewArgs: 'Показать аргументы',
  approvalApprove: 'Разрешить',
  approvalReject: 'Отклонить',
  approvalResolved: 'Обработано',
  oauthTitle: 'Авторизация MCP',
  oauthTool: 'Инструмент',
  oauthAuthorize: 'Авторизовать',
  oauthCancel: 'Отмена',
  oauthAuthorized: 'Авторизовано',
  chatActionsTitle: 'Действия',
  modelChip: 'Модель беседы',
  attachmentProcessing: 'Обработка…',
  steerAttachmentsBlocked: 'Удалите вложения перед дополнением текущей задачи.',
  today: 'Сегодня',
  yesterday: 'Вчера',
  conversationTimeToday: 'Сегодня {time}',
  conversationTimeYesterday: 'Вчера {time}',
  conversationTimeThisYear: '{day}.{month} {time}',
  conversationTimeOtherYear: '{day}.{month}.{year} {time}',
  moreActions: 'Другие действия с диалогом',
  pin: 'Закрепить',
  unpin: 'Открепить',
  renameSession: 'Переименовать',
renameTitle: 'Переименовать диалог',
renameTitlePlaceholder: 'Введите название диалога',
renameConfirm: 'Сохранить',
renameCancel: 'Отмена',
renameSaving: 'Сохранение…',
renameTitleRequired: 'Введите название',
renameTitleFailed: 'Не удалось переименовать диалог',
  clearMessages: 'Очистить сообщения',
  deleteRecord: 'Удалить запись',
  deleteSession: 'Удалить диалог',
  deleteConfirmBody: 'Удалить этот диалог? Это действие нельзя отменить.',
  knowledgeBasesLoadFailed: 'Не удалось загрузить базы знаний',
  copy: 'Копировать',
  copied: 'Скопировано',
  addToKnowledgeBase: 'Добавить в базу знаний',
  requestInfo: 'Request info',
  artifacts: 'Файлы',
  artifactsPending: '产物生成中…',
  preview: 'Предпросмотр',
  download: 'Скачать',
  available: 'Доступно',
  expired: 'Expired',
  sending: '发送中…',
  sendFailed: '发送失败',
  retry: 'Повторить',
  steerCurrent: 'Дополнить текущую задачу',
  steerQueued: 'Отправить после завершения',
  sandboxPanelTitle: 'Песочница',
  openSandboxPanel: 'Терминал песочницы',
  close: 'Закрыть',
  startTerminal: 'Запустить терминал',
  terminalInput: '终端输入',
  sendInput: '发送输入',
  closeTerminal: '断开终端',
  thinkingAndTools: '思考与工具',
  streamStatus: 'Статус',
  grepTitleMatch: 'заголовок',
  grepFaqEntry: 'FAQ entry',
  chunkIdLabel: 'ID фрагмента:',
  documentIdLabel: 'ID документа:',
  positionLabel: 'Позиция:',
  disabledAgentSuffix: 'Недоступно',
  rawTextLabel: 'Исходный текст',
  webFetchPartialContent: 'Часть страницы',
  webFetchSummaryFailed: 'Summary failed',
  summaryLabel: 'Сводка',
  fullContentLabel: 'Полный текст',
  contentLengthLabelSimple: 'Длина содержимого:',
  lengthChars: '{value} символов',
  suggestedRefresh: 'Ещё',
  dismiss: 'Свернуть',
  groupPinned: 'Закреплено',
  groupToday: 'Сегодня',
  groupYesterday: 'Вчера',
  groupLast7Days: 'Последние 7 дней',
  groupLast30Days: 'Последние 30 дней',
  groupOlder: 'Ранее',
  previous: 'Назад',
  next: 'Далее',
  pageOf: 'Страница {page} из {total}',
  batchManage: 'Массовое управление',
  batchCancel: 'Отмена',
  batchSelectAll: 'Выбрать все чаты',
  batchDelete: 'Удалить выбранные ({count})',
  batchDeleteConfirm: 'Удалить выбранные чаты ({count})? Отменить действие нельзя.',
  batchDeleteError: 'Ошибка массового удаления: {message}',
  batchRetry: 'Повторить',
  batchDeleteBusy: 'Удаление…',
  batchSelectSession: 'Выбрать чат {title}',
  sourceSelectLabel: 'Источник чатов',
  },
};

/** zh-CN table kept as the default for consumers that have no locale yet. */
export const CHAT_COPY: ChatCopyTable = CHAT_COPY_ZH;

export function isChatCopyLocale(value: string | null | undefined): value is ChatCopyLocale {
  return value != null && (CHAT_COPY_LOCALES as readonly string[]).includes(value);
}

/** Resolved chat copy for a locale; unknown locales fall back to zh-CN. */
export function resolveChatCopy(locale?: string | null): ChatCopyTable {
  return isChatCopyLocale(locale) ? CHAT_COPY_TABLES[locale] : CHAT_COPY_ZH;
}

/**
 * App locale convention: the language switch stores localStorage['locale']
 * (GeneralPreferencesPanel), otherwise navigator.language resolves like
 * App.tsx resolveLocale(); zh-CN remains the final fallback so Node runtimes
 * and storage-less embeds keep today's rendering.
 */
export function resolveChatLocale(): ChatCopyLocale {
  try {
    const stored = typeof window !== 'undefined' ? window.localStorage?.getItem('locale') : null;
    if (isChatCopyLocale(stored)) return stored;
  } catch {
    // Storage can be unavailable (sandboxed iframes); fall through.
  }
  const language = typeof navigator !== 'undefined' ? navigator.language : '';
  if (isChatCopyLocale(language)) return language;
  const base = language.split('-')[0] ?? '';
  const baseMatch = CHAT_COPY_LOCALES.find((locale) => locale.split('-')[0] === base);
  return baseMatch ?? 'zh-CN';
}

/** formatMessage-style {name} interpolation against a resolved table. */
export function formatChatCopy(table: ChatCopyTable, key: ChatCopyKey, values: Record<string, string | number> = {}): string {
  return table[key].replace(/\{(\w+)\}/g, (_match, name: string) => String(values[name] ?? `{${name}}`));
}

/** zh-CN-bound interpolation kept for the legacy module-level consumers. */
export function chatCopy(key: ChatCopyKey, values: Record<string, string | number> = {}): string {
  return formatChatCopy(CHAT_COPY, key, values);
}

/** Sidebar session group label for the keys produced by sessionGroups(). */
export function sessionGroupLabel(copy: ChatCopyTable, key: string): string {
  switch (key) {
    case 'pinned': return copy.groupPinned;
    case 'today': return copy.groupToday;
    case 'yesterday': return copy.groupYesterday;
    case 'last7Days': return copy.groupLast7Days;
    case 'last30Days': return copy.groupLast30Days;
    case 'older': return copy.groupOlder;
    default: return key;
  }
}

/**
 * Conversation date separator labels (chat.conversationTime.*) for
 * formatConversationTimestampLabel: the {time} placeholder is handled by the
 * caller, so the thisYear/otherYear formatters strip it from the template.
 */
export function conversationTimeLabels(copy: ChatCopyTable) {
  const dayLabel = (template: ChatCopyKey, model: { year: number; month: number; day: number }): string =>
    formatChatCopy(copy, template, { ...model, time: '' }).trim();
  return {
    today: copy.today,
    yesterday: copy.yesterday,
    thisYear: (model: { year: number; month: number; day: number }): string => dayLabel('conversationTimeThisYear', model),
    otherYear: (model: { year: number; month: number; day: number }): string => dayLabel('conversationTimeOtherYear', model),
  };
}

/** zh-CN conversation labels kept for the legacy module-level consumers. */
export const CONVERSATION_TIME_LABELS = conversationTimeLabels(CHAT_COPY);
