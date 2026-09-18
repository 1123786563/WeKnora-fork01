import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { AgentSelectorPanel, agentNotReadyLabels, resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
});

const copy = resolveChatCopy('zh-CN');
const anchorRect = new dom.window.DOMRect(40, 500, 180, 28);

const MODELS = [
  { id: 'chat-1', type: 'KnowledgeQA' },
  { id: 'rerank-1', type: 'Rerank' },
];

const AGENTS = [
  { id: 'builtin-quick-answer', name: '快速问答', is_builtin: true, config: { agent_mode: 'quick-answer', model_id: 'chat-1', kb_selection_mode: 'all' } },
  { id: 'builtin-smart-reasoning', name: '智能推理', is_builtin: true, config: { agent_mode: 'smart-reasoning', model_id: 'chat-1', kb_selection_mode: 'all', rerank_model_id: 'rerank-1', multi_turn_enabled: true } },
  { id: 'builtin-wiki', name: '维基问答', is_builtin: true, config: { agent_mode: 'smart-reasoning', kb_selection_mode: 'all' } },
  { id: 'own-1', name: 'Parity 检索问答', description: '知识库问答智能体', config: { agent_mode: 'quick-answer', model_id: 'chat-1' } },
];

test('agent selector groups builtin and custom agents with a manage entry', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const r = createRoot(container);
  root = r;
  const events: string[] = [];
  await act(async () => r.render(<AgentSelectorPanel
    copy={copy}
    currentAgentId="builtin-quick-answer"
    agents={AGENTS}
    models={MODELS}
    anchorRect={anchorRect}
    onSelect={() => events.push('select')}
    onNotReady={() => events.push('not-ready')}
    onManage={() => events.push('manage')}
    onConfigureAgent={() => events.push('configure')}
    onClose={() => events.push('close')}
  />));
  const text = document.body.textContent ?? '';
  assert.match(text, /内置智能体/);
  assert.match(text, /自定义智能体/);
  assert.match(text, /快速问答/);
  assert.match(text, /Parity 检索问答/);
  const manage = [...document.body.querySelectorAll('button')].find((b) => b.textContent === '+管理');
  assert.ok(manage, 'manage entry present');
  await act(async () => manage?.click());
  assert.ok(events.includes('manage') && events.includes('close'), 'manage closes the panel');
});

test('ready agent selection fires select; not-ready agent is blocked with localized labels', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const r = createRoot(container);
  root = r;
  const selected: string[] = [];
  const blocked: Array<{ id: string; labels: string[] }> = [];
  await act(async () => r.render(<AgentSelectorPanel
    copy={copy}
    currentAgentId=""
    agents={AGENTS}
    models={MODELS}
    anchorRect={anchorRect}
    onSelect={(id) => selected.push(id)}
    onNotReady={(agent, labels) => blocked.push({ id: agent.id, labels })}
    onManage={() => undefined}
    onConfigureAgent={() => undefined}
    onClose={() => undefined}
  />));
  const option = (id: string) => document.body.querySelector<HTMLButtonElement>(`[data-agent-id="${id}"]`);
  await act(async () => option('builtin-smart-reasoning')?.click());
  await act(async () => option('builtin-wiki')?.click());
  assert.deepEqual(selected, ['builtin-smart-reasoning'], 'ready agent selected');
  assert.equal(blocked.length, 1, 'not-ready agent blocked');
  assert.equal(blocked[0]?.id, 'builtin-wiki');
  assert.deepEqual(blocked[0]?.labels, ['对话模型', '重排模型'], 'missing items labelled in the UI locale');
});

test('hover detail card shows capability badges and a configure jump for not-ready agents', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const r = createRoot(container);
  root = r;
  const configured: string[] = [];
  await act(async () => r.render(<AgentSelectorPanel
    copy={copy}
    currentAgentId="builtin-smart-reasoning"
    agents={AGENTS}
    models={MODELS}
    anchorRect={anchorRect}
    onSelect={() => undefined}
    onNotReady={() => undefined}
    onManage={() => undefined}
    onConfigureAgent={(agent) => configured.push(agent.id)}
    onClose={() => undefined}
  />));
  const row = document.body.querySelector<HTMLButtonElement>('[data-agent-id="builtin-smart-reasoning"]');
  assert.ok(row);
  await act(async () => { row?.dispatchEvent(new dom.window.MouseEvent('mouseover', { bubbles: true, relatedTarget: document.body })); });
  const card = document.body.querySelector('[data-agent-detail="builtin-smart-reasoning"]');
  assert.ok(card, 'detail card renders on hover');
  const cardText = card.textContent ?? '';
  assert.match(cardText, /当前/);
  assert.match(cardText, /智能推理/);
  assert.match(cardText, /可访问全部知识库/);
  assert.match(cardText, /多轮对话/);
  assert.match(cardText, /网络搜索/);
  assert.match(cardText, /图片上传/);

  await act(async () => { row?.dispatchEvent(new dom.window.MouseEvent('mouseout', { bubbles: true, relatedTarget: document.body })); });
  await act(async () => new Promise((resolve) => setTimeout(resolve, 460)));
  assert.equal(document.body.querySelector('[data-agent-detail="builtin-smart-reasoning"]'), null, 'detail card hides after the leave delay');

  const wikiRow = document.body.querySelector<HTMLButtonElement>('[data-agent-id="builtin-wiki"]');
  assert.ok(wikiRow);
  await act(async () => { wikiRow?.dispatchEvent(new dom.window.MouseEvent('mouseover', { bubbles: true, relatedTarget: document.body })); });
  const wikiCard = document.body.querySelector('[data-agent-detail="builtin-wiki"]');
  assert.ok(wikiCard);
  assert.match(wikiCard.textContent ?? '', /待配置/);
  const jump = [...(wikiCard as HTMLElement).querySelectorAll('button')].find((b) => b.textContent?.includes('去配置'));
  assert.ok(jump, 'configure jump present for not-ready agents');
  await act(async () => jump?.click());
  assert.deepEqual(configured, ['builtin-wiki']);
});

test('not-ready labels map reason keys to localized copy', () => {
  assert.deepEqual(agentNotReadyLabels(copy, ['summary_model', 'rerank_model']), ['对话模型', '重排模型']);
});
