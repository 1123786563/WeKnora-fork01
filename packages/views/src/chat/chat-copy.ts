/*
 * zh-CN chat copy for the React chat rendering layer.
 *
 * Ported byte-exact from the authoritative Vue locale
 * (frontend/src/i18n/locales/zh-CN.ts). packages/i18n has no chat.* domain yet
 * (agent.copy / agent.addToKnowledgeBase exist, chat.* do not), and packages/i18n
 * is outside this slice's write scope, so the strings live here behind the keys
 * they must move to. TODO(migration): replace with formatMessage(locale, key)
 * once the chat domain is generated in @weknora/i18n.
 */

export const CHAT_COPY = {
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
  /** menu.newSession (chat header fallback + new-chat rows) */
  newSession: '新会话',
  /** sidebar list */
  sidebarTitle: '会话列表',
  newChat: '新对话',
  searchSessions: '搜索会话',
  sourceLabel: '来源',
  groupLabel: '分组',
  groupAll: '全部',
  groupByDate: '按日期',
  loadingSessions: '会话加载中…',
  loadingMessages: '消息加载中…',
  loadingHistory: '历史消息加载中…',
  loadOlder: '加载更多历史',
  untitledChat: '未命名会话',
  send: '发送',
  stopGeneration: '停止生成',
  composerPlaceholder: '直接向模型提问',
  /** input.normalMode — built-in quick answer agent display name */
  quickAnswer: '快速问答',
  uploadAttachment: '上传附件',
  mentionKnowledge: '知识库',
  modelChip: '对话模型',
  /** chat.conversationTime.* */
  today: '今天',
  yesterday: '昨天',
  /** header actions (chatHeader.*) */
  moreActions: '更多操作',
  pin: '置顶',
  unpin: '取消置顶',
  renameSession: '重命名会话',
  clearMessages: '清空消息',
  deleteSession: '删除会话',
  /** answer toolbar (agent.* exists in @weknora/i18n; duplicated here until the chat domain lands) */
  copy: '复制',
  copied: '已复制',
  addToKnowledgeBase: '收藏进知识库',
  requestInfo: '请求信息',
  /** artifacts */
  artifacts: '产物',
  artifactsPending: '产物生成中…',
  preview: '预览',
  download: '下载',
  available: '可用',
  expired: '已过期',
  /** pending user bubble */
  sending: '发送中…',
  sendFailed: '发送失败',
  retry: '重试',
  /** steer composer (input.*) */
  steerCurrent: '补充当前任务',
  steerQueued: '完成后发送',
  /** sandbox panel (chat.sandbox.*) */
  sandboxPanelTitle: '沙箱可视化',
  openSandboxPanel: '打开沙箱面板',
  close: '关闭',
  startTerminal: '启动终端',
  terminalInput: '终端输入',
  sendInput: '发送输入',
  closeTerminal: '断开终端',
  /** thinking & tool extras (AgentStreamDisplay analog) */
  thinkingAndTools: '思考与工具',
  /** stream strip */
  streamStatus: '状态',
  /** agent picker */
  disabledAgentSuffix: '不可用',
  /** suggestions */
  suggestedRefresh: '换一批',
  dismiss: '收起',
  /** session list time group labels (time.*) */
  groupPinned: '已置顶',
  groupToday: '今天',
  groupYesterday: '昨天',
  groupLast7Days: '近7天',
  groupLast30Days: '近30天',
  groupOlder: '更早',
  /** pagination (common.*) */
  previous: '上一页',
  next: '下一页',
  pageOf: '第 {page} 页 / 共 {total} 页',
} as const;

export type ChatCopyKey = keyof typeof CHAT_COPY;

/** formatMessage-style {name} interpolation for the couple of templated labels. */
export function chatCopy(key: ChatCopyKey, values: Record<string, string | number> = {}): string {
  return CHAT_COPY[key].replace(/\{(\w+)\}/g, (_match, name: string) => String(values[name] ?? `{${name}}`));
}

/** Sidebar session group label for the keys produced by sessionGroups(). */
export function sessionGroupLabel(key: string): string {
  switch (key) {
    case 'pinned': return CHAT_COPY.groupPinned;
    case 'today': return CHAT_COPY.groupToday;
    case 'yesterday': return CHAT_COPY.groupYesterday;
    case 'last7Days': return CHAT_COPY.groupLast7Days;
    case 'last30Days': return CHAT_COPY.groupLast30Days;
    case 'older': return CHAT_COPY.groupOlder;
    default: return key;
  }
}

/** Conversation date separator labels (chat.conversationTime.*) for formatConversationTimestampLabel. */
export const CONVERSATION_TIME_LABELS = {
  today: CHAT_COPY.today,
  yesterday: CHAT_COPY.yesterday,
  thisYear: (model: { month: number; day: number }) => `${model.month}月${model.day}日`,
  otherYear: (model: { year: number; month: number; day: number }) => `${model.year}年${model.month}月${model.day}日`,
};
