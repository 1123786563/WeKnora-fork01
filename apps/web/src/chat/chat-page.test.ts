import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import React from 'react';

import { ChatComposer, ChatPage, resolveChatCopy } from '@weknora/views';

(globalThis as typeof globalThis & { React: typeof React }).React = React;

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

  // Agent selector lives in the composer control bar (Vue agent-mode chip).
  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /value="agent-1"/);
  assert.match(html, /Research/);
  assert.match(html, /Disabled · 不可用/);
  assert.match(html, /同意/);
  assert.match(html, /Authorize Docs MCP/);
  assert.match(html, /补充当前任务/);
  assert.match(html, /checking sources/);
  assert.match(html, /Guide &lt;safe&gt;/);
  assert.match(html, /search_docs/);
  assert.match(html, /&lt;not markup&gt;/);
  assert.match(html, /redacted/);
  // Session menu items (Vue ChatHeader menu; zh aligns with menu.renameSession
  // 修改标题 / chatHeader.deleteSession 删除对话).
  assert.match(html, /修改标题/);
  assert.match(html, /删除对话/);
  // Sandbox drawer opened via terminalOpen: connected terminal surface.
  assert.match(html, /Sandbox terminal/);
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
  assert.match(html, /Resolved: approve/);
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
  // Agent picker (Vue AgentSelector semantics) renders the available agents.
  assert.match(html, /id="wk-chat-agent"/);
  assert.match(html, /value="agent-1"/);
  assert.match(html, /Research/);
  assert.match(html, /value="agent-2"/);
  assert.match(html, /Disabled agent · 不可用/);
  // Vue creatChat empty state: welcome heading + suggested question cards.
  assert.match(html, /wk-chat-starters/);
  assert.match(html, /Hi，我是 WeKnora，让你的知识触手可及/);
  assert.match(html, /你可以这样问我/);
  assert.match(html, /What is WeKnora?/);
  assert.match(html, /How do I upload files?/);
  // Vue composer anatomy: quick-answer chip label, model chip, circular send.
  assert.match(html, /快速问答/);
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

test('message list renders the Vue anatomy: date separators, user pill, plain assistant text with icon row', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [],
    selectedSessionId: null,
    messages: [
      { id: 'u1', session_id: 's', role: 'user', content: 'hello', created_at: new Date(2024, 2, 5, 9, 0, 0).toISOString() },
      { id: 'a1', session_id: 's', role: 'assistant', content: 'The answer is 42', created_at: new Date(2024, 2, 5, 9, 1, 0).toISOString() },
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
  assert.match(html, /<button[^>]*class="wk-chat-bookmark[^>]*aria-label="添加到知识库"[^>]*aria-disabled="true"[^>]*disabled=""/);
  assert.match(html, /The answer is 42/);
  // Scroll-to-bottom only appears after the user scrolls up (client-only).
  assert.doesNotMatch(html, /wk-chat-scroll-bottom/);
});

test('session sidebar renders time-group headers, full titles, and the hover ⋯ menu', () => {
  const html = renderToStaticMarkup(React.createElement(ChatPage, {
    sessions: [
      { id: 'session-1', title: '修复后首发截图', is_pinned: false },
      { id: 'session-2', title: 'creatChat首发修复验证', is_pinned: false },
    ],
    selectedSessionId: 'session-1',
    messages: [],
    draft: '',
    onSelectSession: () => undefined,
    onCreateSession: () => undefined,
    onDraftChange: () => undefined,
    send: async () => undefined,
    onRenameSession: async () => undefined,
    onToggleSessionPin: async () => undefined,
    onDeleteSession: async () => undefined,
    locale: 'zh-CN',
    sessionGroups: [
      { key: 'yesterday', label: 'yesterday', items: [{ id: 'session-1', title: '修复后首发截图', is_pinned: false }] },
      { key: 'older', label: 'older', items: [{ id: 'session-2', title: 'creatChat首发修复验证', is_pinned: false }] },
    ],
  }));
  assert.match(html, /昨天/);
  assert.match(html, /更早/);
  assert.match(html, /修复后首发截图/);
  assert.match(html, /creatChat首发修复验证/);
  assert.match(html, /aria-current="page"/);
  assert.match(html, /修改标题/);
  assert.match(html, /aria-label="更多对话操作"/);
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
