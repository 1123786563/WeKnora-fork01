import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { ChatComposer, ChatPage, resolveChatCopy } from '@weknora/views';
import { SessionSidebarList } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

test('header rename uses the localized inline editor contract instead of a browser prompt', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  // Vue ChatHeader.vue 同构迁移后，rename 编辑器住在 apps/web 的 tdesign header 里。
  const headerSource = readFileSync(new URL('./chat-header.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(routeSource, /window\.prompt\(/);
  assert.match(headerSource, /chat-header__edit-input/);
  assert.match(headerSource, /renameSubmittingRef/);
  assert.match(headerSource, /requestAnimationFrame/);
  assert.match(headerSource, /\.select\(\)/);
  assert.match(headerSource, /chat-header__edit-error/);
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const copy = resolveChatCopy(locale);
    assert.ok(copy.renameTitle && copy.renameTitlePlaceholder && copy.renameConfirm && copy.renameCancel);
  }
});

test('chat view keeps destructive session actions behind the Vue confirmation state', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  const viewSource = readFileSync(new URL('../../../../packages/views/src/chat/page.tsx', import.meta.url), 'utf8');
  const sidebarSource = readFileSync(new URL('../../../../packages/views/src/chat/session-sidebar.tsx', import.meta.url), 'utf8');
  // Vue ChatHeader.vue：clear/delete 走同一弹层的 .chat-header-confirm 二次确认。
  const headerSource = readFileSync(new URL('./chat-header.tsx', import.meta.url), 'utf8');
  assert.match(headerSource, /menuMode/);
  assert.match(headerSource, /'clear'/);
  assert.match(headerSource, /'delete'/);
  assert.match(headerSource, /chat-header-confirm__btn is-danger/);
  assert.match(routeSource, /onClearSession=\{clearMessages\}/);
  assert.match(routeSource, /setStreamState\(\(current\) => \(\{ \.\.\.current, phase: 'error'/);
  assert.match(routeSource, /setStreamState\(\(current\) => \(\{ \.\.\.current, phase: 'stopped'/);
  assert.match(sidebarSource, /clearConfirmBody/);
  assert.match(sidebarSource, /deleteConfirmBody/);
  assert.match(sidebarSource, /sessionDangerAction/);
  assert.match(sidebarSource, /role="dialog"/);
});

test('streaming steer composer renders one localized task label', () => {
  const viewSource = readFileSync(new URL('../../../../packages/views/src/chat/page.tsx', import.meta.url), 'utf8');
  const labels = viewSource.match(/\{copy\.steerCurrent\}/g) ?? [];
  assert.equal(labels.length, 1);
});

test('streaming non-steer sessions keep the stop action visible with a stale draft', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    draft: 'draft typed before the reply started',
    disabled: true,
    streaming: true,
    onDraftChange: () => undefined,
    onSubmit: () => undefined,
    onStop: () => undefined,
    copy: resolveChatCopy('zh-CN'),
  }));

  // Vue Input-field: isReplying && (!canSteer || !query.trim()) => stop.
  assert.match(html, /class="wk-chat-stop/);
  assert.doesNotMatch(html, /type="submit"/);
});

const baseProps = {
  sessions: [{ id: 'session-1', title: 'Chat', is_pinned: false }],
  selectedSessionId: 'session-1',
  messages: [],
  draft: '',
  // Pin the Vue-baseline locale: Node resolves navigator.language ('en-US'),
  // but these assertions verify the zh-CN copy byte-exact.
  locale: 'zh-CN',
  onSelectSession: () => undefined,
  onCreateSession: () => undefined,
  onDraftChange: () => undefined,
  send: async () => undefined,
};

test('chat page exposes the selected agent and server-disabled state at the chat entry', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    sessions: [{ id: 'session-1', title: 'Chat', is_pinned: true }],
    agents: [{ id: 'agent-1', name: 'Research' }, { id: 'agent-2', name: 'Disabled', disabled: true }],
    selectedAgentId: 'agent-1',
    onAgentChange: () => undefined,
    toolApprovals: [{ pendingId: 'approval-1', toolName: 'search_docs', status: 'pending' }],
    oauthApprovals: [{ pendingId: 'oauth-1', serviceId: 'mcp-1', serviceName: 'Docs MCP', toolName: 'search', status: 'pending' }],
    onResolveToolApproval: async () => undefined,
    onAuthorizeOAuth: async () => undefined,
    onCancelOAuth: async () => undefined,
    onSteer: async () => undefined,
    onStopStream: () => undefined,
    onRenameSession: async () => undefined,
    onToggleSessionPin: async () => undefined,
    onDeleteSession: async () => undefined,
    terminal: { status: 'ready', output: '$ ls' },
    terminalOpen: true,
    onOpenTerminal: async () => undefined,
    onTerminalInput: async () => undefined,
    onCloseTerminal: () => undefined,
    stream: {
      phase: 'streaming',
      thinking: 'checking sources',
      references: [{ title: 'Guide <safe>' }],
      toolCalls: [{ id: 'tool-1', name: 'search_docs', status: 'completed', result: { html: '<not markup>', api_key: 'secret' } }],
    },
  }));

  // Agent selector lives in the composer control bar (Vue agent-mode chip):
  // a button trigger (upstream AgentSelector) carrying the current agent
  // name; the grouped panel itself mounts on click.
  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /aria-haspopup="dialog"/);
  assert.match(html, /Research/);
  assert.doesNotMatch(html, /Disabled/);
  assert.match(html, /同意/);
  assert.match(html, /去授权 Docs MCP/);
  assert.match(html, /补充当前任务/);
  // Vue main-face contract: the SSE `thinking` field never renders a fold on
  // the main face (it only feeds the agent timeline); the live thinking
  // indicator is deepThink, driven by `<think>` tags in the streamed answer.
  assert.doesNotMatch(html, /checking sources/);
  // 2026-09-19 live-round contract: streamed references stay behind the
  // collapsed 检索完成 summary (Vue ChatReferencesDrawer) — no completed
  // assistant message exists here, so the panel never renders in this view.
  assert.doesNotMatch(html, /Guide &lt;safe&gt;/);
  assert.match(html, /search_docs/);
  assert.match(html, /&lt;not markup&gt;/);
  assert.match(html, /redacted/);
  // Session menu items (Vue ChatHeader menu; zh aligns with menu.renameSession
  // 修改标题 / chatHeader.deleteSession 删除对话).
  // Vue ChatHeader.vue t-popup destroyOnClose：菜单项仅开层时渲染（ix-chat-header-menu 扫描覆盖）。
  assert.match(html, /aria-label=\"更多对话操作\"/);
  // Sandbox drawer opened via terminalOpen: connected terminal surface.
  assert.match(html, /沙箱可视化/);
  assert.match(html, /ls/);
  assert.match(html, /终端输入/);
  assert.match(html, /id="wk-chat-draft"[^>]*disabled=""/);
});

test('sandbox terminal stays hidden until the header toggle opens the drawer', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    terminal: { status: 'idle', output: '' },
    onOpenTerminal: async () => undefined,
    onTerminalInput: async () => undefined,
    onCloseTerminal: () => undefined,
  }));
  // Vue SandboxSidePanel: no terminal surface until toggled.
  assert.doesNotMatch(html, /wk-chat-sandbox-drawer/);
  assert.doesNotMatch(html, /启动终端/);
  // Header mirror toggle (Vue sandbox-header-toggle; chatHeader.toggleSandboxPanel).
  assert.match(html, /aria-label="沙箱终端"/);
  assert.match(html, /aria-expanded="false"/);
});

test('opened drawer with an unstarted terminal shows the start action', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    terminal: { status: 'idle', output: '' },
    terminalOpen: true,
    onOpenTerminal: async () => undefined,
    onTerminalInput: async () => undefined,
    onCloseTerminal: () => undefined,
  }));
  assert.match(html, /wk-chat-sandbox-drawer/);
  assert.match(html, /沙箱可视化/);
  assert.match(html, /启动终端/);
});

test('chat page renders an expandable args editor on the pending tool approval card', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    toolApprovals: [{
      pendingId: 'approval-1',
      toolName: 'search_docs',
      status: 'pending',
      arguments: { query: 'install guide', limit: 5 },
    }],
    onResolveToolApproval: async () => undefined,
  }));
  // The expandable editor prefills the textarea with the original arguments.
  assert.match(html, /查看参数/);
  assert.match(html, /wk-chat-approval-args-input/);
  assert.match(html, /&quot;query&quot;: &quot;install guide&quot;/);
  assert.match(html, /&quot;limit&quot;: 5/);
  // Approve/Reject are the resolution actions; no error is shown for valid args.
  assert.match(html, /同意/);
  assert.match(html, /拒绝/);
  assert.doesNotMatch(html, /wk-chat-approval-error/);
});

test('chat page hides the args editor for resolved tool approvals', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    toolApprovals: [{ pendingId: 'approval-1', toolName: 'search_docs', status: 'resolved', decision: 'approve' }],
    onResolveToolApproval: async () => undefined,
  }));
  assert.match(html, /已处理: approve/);
  assert.doesNotMatch(html, /wk-chat-approval-args-input/);
  assert.doesNotMatch(html, /查看参数/);
});

test('chat page hides the steer composer and stop button when idle', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    onSteer: async () => undefined,
    onStopStream: () => undefined,
    stream: { phase: 'idle', thinking: '', references: [], toolCalls: [] },
  }));
  assert.doesNotMatch(html, /补充当前任务/);
  assert.doesNotMatch(html, /class="wk-chat-stop"/);
});

test('chat page exposes KB mention controls in the streaming steer composer', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    mentionOptions: [{ id: 'kb-1', name: '产品文档', type: 'kb' }],
    mentionedItems: [{ id: 'kb-2', name: 'FAQ', type: 'kb', kbType: 'faq' }],
    onMentionOpen: () => undefined,
    onMentionSelect: () => undefined,
    onMentionRemove: () => undefined,
    onSteer: async () => undefined,
    stream: { phase: 'streaming', thinking: '', references: [], toolCalls: [] },
  }));
  assert.match(html, /id="wk-chat-steer-mention"/);
  assert.match(html, /FAQ/);
  assert.match(html, /aria-expanded="false"/);
});

test('chat page shows the artifacts-pending indicator only while streaming', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    stream: { phase: 'streaming', thinking: '', references: [], toolCalls: [], artifactsPending: true },
  }));
  assert.match(html, /产物生成中/);
  assert.match(html, /状态: streaming/);
});

test('new-conversation view renders the agent picker and the agent suggested questions', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }, { id: 'agent-2', name: 'Disabled agent', disabled: true }],
    selectedAgentId: 'agent-1',
    onAgentChange: () => undefined,
    starterQuestions: ['What is WeKnora?', 'How do I upload files?'],
    onStarterQuestionClick: () => undefined,
    locale: 'zh-CN',
  }));
  // Agent picker (Vue AgentSelector semantics): button trigger shows the
  // current agent; disabled agents stay out of the picker panel.
  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /Research/);
  assert.doesNotMatch(html, /Disabled agent/);
  // Vue creatChat empty state: welcome heading + suggested question cards.
  assert.match(html, /wk-chat-starters/);
  assert.match(html, /Hi，我是 WeKnora，让你的知识触手可及/);
  assert.match(html, /你可以这样问我/);
  assert.match(html, /What is WeKnora?/);
  assert.match(html, /How do I upload files?/);
  // Vue composer anatomy: agent chip (current agent label), model chip, send.
  assert.match(html, /wk-chat-agent-chip/);
  assert.match(html, /wk-chat-model-chip/);
  assert.match(html, /aria-label="发送"/);
});

test('creatChat empty state centers the welcome cluster like Vue dialogue-wrap (no message scroll)', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    locale: 'zh-CN',
  }));
  // Vue creatChat.vue .dialogue-wrap centers the welcome+composer cluster;
  // the empty view must not render the (flex:1) message scroll that would
  // push the composer to the bottom.
  assert.match(html, /wk-chat-conversation--empty/);
  assert.doesNotMatch(html, /wk-chat-message-scroll/);
  assert.match(html, /wk-chat-composer/);
});

test('new-conversation view keeps the welcome heading but no question cards without suggestions', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    starterQuestions: [],
    locale: 'zh-CN',
  }));
  // Vue creatChat.vue always shows the welcome heading; the cards stay absent.
  assert.match(html, /Hi，我是 WeKnora，让你的知识触手可及/);
  assert.doesNotMatch(html, /你可以这样问我/);
  assert.doesNotMatch(html, /wk-chat-starter-card/);
});

test('welcome and starters are hidden once a session is open', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    starterQuestions: ['What is WeKnora?'],
  }));
  assert.doesNotMatch(html, /wk-chat-starters/);
  assert.doesNotMatch(html, /Hi，我是 WeKnora，让你的知识触手可及/);
});

test('new-conversation view renders the starter-questions skeleton while loading', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    agents: [{ id: 'agent-1', name: 'Research' }],
    selectedAgentId: 'agent-1',
    starterQuestions: [],
    starterQuestionsLoading: true,
  }));
  assert.match(html, /wk-chat-starters--loading/);
  assert.match(html, /aria-busy="true"/);
  assert.match(html, /wk-chat-starter-skeleton/);
});

test('new-conversation starters keep Vue refresh control and existing cards during refresh', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    starterQuestions: ['保留这条问题'],
    starterQuestionsLoading: true,
    onRefreshStarterQuestions: () => undefined,
    locale: 'zh-CN',
  }));

  // Vue creatChat keeps the old cards when sqLoading is true and questions
  // already exist; only the refresh affordance becomes disabled/spinning.
  assert.match(html, /aria-label="换一批"/);
  assert.match(html, /title="换一批"/);
  assert.match(html, /wk-chat-starter-refresh[^>]*disabled=""/);
  assert.match(html, /保留这条问题/);
  assert.doesNotMatch(html, /wk-chat-starter-skeleton/);
});

test('chat page passes the typing indicator while streaming without assistant content', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: '', references: [], toolCalls: [] },
  }));
  assert.match(html, /wk-chat-typing/);
});

test('chat page hides the typing indicator once thinking or tool calls arrive', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: 'checking', references: [], toolCalls: [] },
  }));
  assert.doesNotMatch(html, /wk-chat-typing/);
});

/*
 * Vue main-face streaming thinking indicator (deepThink.vue + processStreamChunk):
 * an open `<think>` tag in the accumulated answer switches the live block to
 * 「思考中...」 with the reasoning text streamed inline; the answer stays hidden
 * and the global typing dots are replaced by the deepThink block (botmsg shows
 * the message, so shouldShowGlobalTypingIndicator is false).
 */
test('chat page shows the deepThink live indicator while an open think tag streams', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: '', answer: '<think>先分解问题\n再检索', references: [], toolCalls: [] },
  }));
  // Live header: pulsing status + chat.thinking 「思考中...」(deepThink thinking-text).
  assert.match(html, /wk-chat-live-think/);
  assert.match(html, /role="status"[^>]*>[\s\S]*?思考中\.\.\./);
  assert.match(html, /先分解问题/);
  // The think tag never leaks into the visible answer area.
  assert.doesNotMatch(html, /&lt;think&gt;/);
  // Vue deepThink replaces the typing dots once the block streams.
  assert.doesNotMatch(html, /wk-chat-typing/);
});

test('chat page folds the live indicator to 已深度思考 once the think tag closes', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: '', answer: '<think>推理过程</think>最终答案正文', references: [], toolCalls: [] },
  }));
  // Folded header: chat.deepThoughtCompleted, auto-collapsed like deepThink's watcher.
  assert.match(html, /wk-chat-live-think/);
  assert.match(html, /已深度思考/);
  assert.doesNotMatch(html, /<details open/);
  assert.match(html, /推理过程/);
  assert.doesNotMatch(html, /&lt;think&gt;/);
});

test('chat page renders plain streamed answers without any live thinking block', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'u1', session_id: 's', role: 'user', content: 'hello' }],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    stream: { phase: 'streaming', thinking: '', answer: '正文开始流出', references: [], toolCalls: [] },
  }));
  assert.doesNotMatch(html, /wk-chat-live-think/);
  assert.doesNotMatch(html, /思考中/);
  assert.doesNotMatch(html, /已深度思考/);
});

test('message list renders the Vue anatomy: date separators, user pill, plain assistant text with icon row', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [
      { id: 'u1', session_id: 's', role: 'user', content: 'hello', created_at: new Date(2024, 2, 5, 9, 0, 0).toISOString() },
      { id: 'a1', session_id: 's', role: 'assistant', content: 'The answer is 42', is_completed: true, created_at: new Date(2024, 2, 5, 9, 1, 0).toISOString() },
      { id: 'u2', session_id: 's', role: 'user', content: 'next day', created_at: new Date(2024, 2, 6, 9, 0, 0).toISOString() },
    ],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
  }));
  // Conversation date separator (Vue MessageTimestamp, chat.conversationTime.thisYear).
  assert.match(html, /wk-chat-timestamp/);
  assert.match(html, /3月5日/);
  assert.match(html, /3月6日/);
  // User pill: right-aligned bubble, no role label/avatar.
  assert.match(html, /wk-chat-message-row--user/);
  assert.doesNotMatch(html, /wk-chat-message-role/);
  assert.doesNotMatch(html, /wk-chat-avatar/);
  // Assistant: icon row with 复制/收藏 (Vue answer-toolbar; agent.addToKnowledgeBase).
  assert.match(html, /wk-chat-answer-toolbar/);
  assert.match(html, /aria-label="复制"/);
  assert.match(html, /aria-label="添加到知识库"/);
  assert.match(html, /<button[^>]*class="[^"]*wk-chat-bookmark[^>]*aria-label="添加到知识库"[^>]*disabled="[^"]*"[^>]*aria-disabled="true"/);
  assert.match(html, /The answer is 42/);
  // Scroll-to-bottom only appears after the user scrolls up (client-only).
  assert.doesNotMatch(html, /wk-chat-scroll-bottom/);
});

test('artifact rows expose the Vue drawer entry while keeping protected actions host-owned', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    messages: [{
      id: 'assistant-1',
      session_id: 'session-1',
      role: 'assistant',
      content: 'generated files',
      artifacts: [{ index: 0, file_name: 'report.md', file_type: 'text/markdown', file_size: 2048, created_at: '2026-09-08T04:05:00Z' }],
    }],
    onArtifactDownload: async () => undefined,
    onArtifactPreview: async () => ({ body: '# report', contentType: 'text/markdown' }),
    locale: 'zh-CN',
  }));
  assert.match(html, /wk-chat-artifacts-open/);
  assert.match(html, /report\.md/);
  assert.match(html, /预览/);
  // The drawer is interaction-owned and must not be rendered before an
  // artifact action opens it during SSR.
  assert.doesNotMatch(html, /wk-chat-artifact-drawer-overlay/);
});

test('session sidebar renders time-group headers, full titles, and the hover ⋯ menu', () => {
  // Vue menu.vue：会话侧栏由平台 shell 渲染（chat/index.vue 无自有侧栏），
  // 分组列表直接经 SessionSidebarList 断言（ChatPage 会话视图不再重复侧栏）。
  const html = renderToStaticMarkup(React.createElement(SessionSidebarList, {
    sessions: [
      { id: 'session-1', title: '修复后首发截图', is_pinned: false },
      { id: 'session-2', title: 'creatChat首发修复验证', is_pinned: false },
    ],
    selectedSessionId: 'session-1',
    onSelect: () => undefined,
    onRename: async () => undefined,
    onTogglePin: async () => undefined,
    onDelete: async () => undefined,
    copy: resolveChatCopy('zh-CN'),
    groups: [
      { key: 'yesterday', label: 'yesterday', items: [{ id: 'session-1', title: '修复后首发截图', is_pinned: false }] },
      { key: 'older', label: 'older', items: [{ id: 'session-2', title: 'creatChat首发修复验证', is_pinned: false }] },
    ],
  }));
  assert.match(html, /昨天/);
  assert.match(html, /更早/);
  assert.match(html, /修复后首发截图/);
  assert.match(html, /creatChat首发修复验证/);
  assert.match(html, /aria-current=\"page\"/);
  assert.match(html, /修改标题/);
});

// --- R016 model chip (Vue Input-field.vue model-display parity) ---

const composerBase = {
  draft: '',
  onDraftChange: () => undefined,
  onSubmit: () => undefined,
};

test('composer chip renders the real model name and the context spec span like Vue', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('zh-CN'),
    modelLabel: 'mock-stream-model',
    modelContext: '200K',
  }));
  // Vue model-selector-trigger: name span + compact ctx suffix (200K/1M).
  assert.match(html, /wk-chat-model-name[^>]*>mock-stream-model</);
  assert.match(html, /wk-chat-model-ctx[^>]*>200K</);
  // The chip's accessible label carries the resolved model, not the placeholder.
  assert.match(html, /aria-label="mock-stream-model"/);
});

test('composer model chip is exposed as a disabled control until model selection has a submit contract', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('zh-CN'),
    modelLabel: 'mock-stream-model',
  }));
  // The current React stream has no model-id selection/submit contract. Keep
  // the Vue-shaped affordance explicit and keyboard/screen-reader safe rather
  // than exposing a misleading interactive control.
  assert.match(html, /<button[^>]*class="wk-chat-model-chip[^>]*disabled=""[^>]*aria-disabled="true"[^>]*aria-label="mock-stream-model"/);
});

test('composer chip marks a defaulted context window like the Vue is-default class', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('zh-CN'),
    modelLabel: 'mock-stream-model',
    modelContext: '200K',
    modelContextIsDefault: true,
  }));
  assert.match(html, /wk-chat-model-ctx is-default[^>]*>200K</);
});

test('composer chip without a resolved model keeps the localized placeholder fallback', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('zh-CN'),
  }));
  // Views-level fallback for consumers that pass no modelLabel; the app entry
  // (ChatRoutePage) always resolves a label, mirroring Vue input.notConfigured.
  assert.match(html, /对话模型/);
});

test('chat page flows the resolved model chip label into the composer chip', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    ...baseProps,
    modelLabel: 'mock-stream-model 200K',
  }));
  assert.match(html, /wk-chat-model-chip/);
  assert.match(html, /mock-stream-model 200K/);
  assert.match(html, /aria-label="mock-stream-model 200K"/);
});

test('composer exposes a multi-file picker and truthful attachment states', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('zh-CN'),
    attachments: [
      { id: 'local-1', name: 'guide.pdf', status: 'pending' },
      { id: 'att-0', name: 'queued.txt', status: 'uploaded', attachmentId: 'att-0' },
      { id: 'att-1', name: 'parsing.txt', status: 'processing', attachmentId: 'att-1' },
      { id: 'att-2', name: 'ready.txt', status: 'ready', attachmentId: 'att-2' },
      { id: 'local-3', name: 'broken.csv', status: 'failed', error: 'Upload failed' },
    ],
    attachmentAccept: ['.pdf', '.custom'],
  }));
  assert.match(html, /type="file"/);
  assert.match(html, /accept="\.pdf,\.custom"/);
  assert.match(html, /multiple=""/);
  assert.match(html, /guide\.pdf/);
  assert.match(html, /queued\.txt/);
  assert.match(html, /parsing\.txt/);
  assert.match(html, /ready\.txt/);
  assert.match(html, /broken\.csv/);
  assert.match(html, /data-attachment-status="pending"/);
  assert.match(html, /data-attachment-status="uploaded"/);
  assert.match(html, /data-attachment-status="processing"/);
  assert.match(html, /解析中/);
  assert.match(html, /data-attachment-status="ready"/);
  assert.match(html, /data-attachment-status="failed"/);
});

test('composer exposes a KB mention listbox and selected mention chips', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('zh-CN'),
    mentionOpen: true,
    mentionOptions: [{ id: 'kb-1', name: '产品文档', type: 'kb', kbType: 'document' }],
    mentionedItems: [{ id: 'kb-2', name: 'FAQ', type: 'kb', kbType: 'faq' }],
    onMentionSelect: () => undefined,
    onMentionRemove: () => undefined,
  }));
  assert.match(html, /role="listbox"/);
  assert.match(html, /产品文档/);
  assert.match(html, /data-mention-id="kb-1"/);
  assert.match(html, /data-mention-id="kb-2"/);
  assert.match(html, /aria-label="关闭: FAQ"/);
});

test('composer localizes empty KB mention states', () => {
  const html = renderToStaticMarkup(React.createElement(ChatComposer, {
    ...composerBase,
    copy: resolveChatCopy('en-US'),
    mentionOpen: true,
    mentionOptions: [],
  }));
  // Vue MentionSelector.vue:270 空态 = emptyHint || common.noResult（无搜索框）。
  assert.match(html, /No results/);
  assert.doesNotMatch(html, /暂无可用知识库/);
});

test('chat route uses Vue-localized copy for destructive confirmation and KB mention load fallback', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(routeSource, /window\.confirm\(['"]Delete this conversation\?/);
  assert.doesNotMatch(routeSource, /Unable to load knowledge bases/);
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    const copy = resolveChatCopy(locale);
    assert.ok(copy.deleteConfirmBody);
    assert.ok(copy.knowledgeBasesLoadFailed);
  }
});

test('chat route feeds the raw streamed answer and strips think tags from the transient row', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  // Vue parity: the live thinking block parses the accumulated answer content,
  // so the presentation must carry it and the transient assistant row must not
  // render the raw `<think>` markup (content stays empty while thinking).
  assert.match(routeSource, /answer: streamState\.answer/);
  assert.match(routeSource, /content: splitLiveThinking\(runState\.answer\)\.answer/);
});

/*
 * R466-A2 history deepThink (Vue botmsg.vue + handleMsgList restore branch):
 * a persisted assistant answer that still contains a full `<think>…</think>`
 * block renders like the Vue history face — a folded 「已深度思考」 block with
 * the reasoning inside, and the visible message body is only the post-tag
 * answer (the tags never leak into the markdown).
 */
test('history assistant message renders the folded deepThink block with the tags stripped', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [
      { id: 'u1', session_id: 's', role: 'user', content: '解释 RAG' },
      { id: 'a1', session_id: 's', role: 'assistant', content: '<think>先拆解概念，再对比向量检索</think>RAG 是检索增强生成的缩写。', is_completed: true },
    ],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
  }));
  // deepThink.vue history mount: thinking=false → folded under 已深度思考.
  assert.match(html, /wk-chat-history-think/);
  assert.match(html, /已深度思考/);
  assert.doesNotMatch(html, /<details open/);
  // The reasoning text lives inside the foldable block.
  assert.match(html, /先拆解概念，再对比向量检索/);
  // The message body keeps only the post-tag answer.
  assert.match(html, /RAG 是检索增强生成的缩写。/);
  assert.doesNotMatch(html, /&lt;think&gt;/);
  assert.doesNotMatch(html, /&lt;\/think&gt;/);
});

test('history assistant message without think tags renders no deepThink block', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'a1', session_id: 's', role: 'assistant', content: '普通历史回答', is_completed: true }],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
  }));
  assert.doesNotMatch(html, /wk-chat-history-think/);
  assert.doesNotMatch(html, /已深度思考/);
  assert.match(html, /普通历史回答/);
});

test('an unclosed think block in history restores the live thinking presentation', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [{ id: 'a1', session_id: 's', role: 'assistant', content: '<think>中断的推理过程', is_completed: false }],
    locale: 'zh-CN',
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
  }));
  // Vue restore branch: unclosed <think> keeps thinking=true (expanded, 思考中...).
  assert.match(html, /wk-chat-history-think/);
  assert.match(html, /思考中\.\.\./);
  assert.match(html, /中断的推理过程/);
  assert.doesNotMatch(html, /&lt;think&gt;/);
});

/*
 * R483 D16 (R482 report-B2): the Vue ChatHeader ⋯ menu carries a utility block
 * between 修改标题 and 清空消息 — 复制会话 ID / 复制对话链接 / 复制为
 * Markdown / 在新窗口中打开, separated by dividers (ChatHeader.vue:61-78).
 */
test('chat header menu renders the Vue utility block between rename and clear with dividers', () => {
  // Vue ChatHeader.vue t-popup destroyOnClose：菜单项只在开层时进入 DOM
  // （ix-chat-header-menu 像素扫描覆盖开层态），静态断言改为 chat-header.tsx 源契约。
  const headerSource = readFileSync(new URL('./chat-header.tsx', import.meta.url), 'utf8');
  assert.match(headerSource, /chat-header-menu__item/);
  assert.match(headerSource, /data-menu-action=\{item\.id\}/);
  const dividers = headerSource.match(/chat-header-menu__divider/g) ?? [];
  assert.ok(dividers.length >= 2, 'the header menu must carry the two Vue dividers around the utility block');
  const order = ['onTogglePin', 'startTitleEdit', 'utilityItems.map', 'data-menu-action=\"clear\"', 'data-menu-action=\"delete\"'].map((token) => headerSource.indexOf(token));
  for (let index = 1; index < order.length; index += 1) {
    assert.ok(order[index] > order[index - 1], 'menu actions must render in the Vue order');
  }
});
