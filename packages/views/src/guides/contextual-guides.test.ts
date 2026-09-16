// Catalog + persistence + trigger-routing tests for the contextual guide
// port. Vue baseline (read-only): frontend/src/config/contextualGuides.ts,
// KbCreateContextualGuide.vue, AgentCreateContextualGuide.vue,
// TenantModelsGuide.vue and the contextualGuide block of the five locales.
import assert from 'node:assert/strict';
import test from 'node:test';

import {
  AGENT_EDITOR_FOCUS_SECTION_EVENT,
  CONTEXTUAL_GUIDE_ALSO_COMPLETE_TOURS,
  CONTEXTUAL_GUIDE_OPEN_DELAY_MS,
  CONTEXTUAL_GUIDE_STEPS,
  CONTEXTUAL_GUIDE_TOUR_IDS,
  CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS,
  KB_EDITOR_FOCUS_SECTION_EVENT,
  agentCreateGuideSteps,
  contextualGuideMessage,
  contextualGuideStepPrefix,
  consumePendingContextualGuide,
  focusAgentEditorSection,
  focusKbEditorSection,
  isContextualGuideDone,
  isGlobalUserGuideDone,
  kbCreateGuideSteps,
  markContextualGuideDone,
  openContextualGuide,
  resolveContextualGuideSteps,
  shouldArmKbDetailGuideOnEntry,
  shouldOpenContextualGuide,
} from './contextual-guides.ts';
import { CONTEXTUAL_GUIDE_MESSAGES } from './contextual-guide-messages.ts';
import type { ContextualGuideStep } from './contextual-guides.ts';

function makeStorage(initial: Record<string, string> = {}) {
  const map = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => (map.has(key) ? map.get(key)! : null),
    setItem: (key: string, value: string) => { map.set(key, value); },
    removeItem: (key: string) => { map.delete(key); },
    map,
  };
}

test('tour ids and storage keys replay the Vue contextualGuides.ts literals', () => {
  assert.deepEqual([...CONTEXTUAL_GUIDE_TOUR_IDS], ['kbList', 'kbCreate', 'kbDetail', 'chat', 'tenantModels', 'agentList', 'agentCreate']);
  assert.deepEqual(CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS, {
    kbList: 'weknora:contextual-guide-kb-list:v2',
    kbCreate: 'weknora:contextual-guide-kb-create:v3',
    kbDetail: 'weknora:contextual-guide-kb-detail:v1',
    chat: 'weknora:contextual-guide-chat:v1',
    tenantModels: 'weknora:contextual-guide-tenant-models:v1',
    agentList: 'weknora:contextual-guide-agent-list:v1',
    agentCreate: 'weknora:contextual-guide-agent-create:v1',
  });
});

test('focus events keep the exact Vue event names', () => {
  assert.equal(KB_EDITOR_FOCUS_SECTION_EVENT, 'weknora:kb-editor-focus-section');
  assert.equal(AGENT_EDITOR_FOCUS_SECTION_EVENT, 'weknora:agent-editor-focus-section');
});

test('open delays and alsoCompleteTours match the Vue config', () => {
  assert.deepEqual(CONTEXTUAL_GUIDE_OPEN_DELAY_MS, {
    kbList: 500, kbCreate: 450, kbDetail: 600, chat: 800, tenantModels: 500, agentList: 500, agentCreate: 450,
  });
  assert.deepEqual(CONTEXTUAL_GUIDE_ALSO_COMPLETE_TOURS, { kbCreate: ['kbList'], agentCreate: ['agentList'] });
});

test('static catalogs reuse the Vue selectors, placements and flags verbatim', () => {
  assert.deepEqual(CONTEXTUAL_GUIDE_STEPS.kbList, [{
    key: 'create',
    target: '.empty-state-btn[data-guide="kb-list-create"], [data-guide="kb-list-create"]',
    placement: 'bottom',
    interact: true,
  }]);
  assert.deepEqual(CONTEXTUAL_GUIDE_STEPS.kbDetail.map((step) => [step.key, step.target, step.placement, step.optional]), [
    ['intro', undefined, undefined, undefined],
    ['upload', '[data-guide="kb-detail-add-doc"]', 'bottom', undefined],
    ['done', undefined, undefined, undefined],
  ]);
  assert.deepEqual(CONTEXTUAL_GUIDE_STEPS.chat.map((step) => [step.key, step.target, step.placement, step.optional]), [
    ['kb', '[data-guide="chat-kb-mention"]', 'top', true],
    ['input', '[data-guide="chat-input"]', 'top', undefined],
    ['send', '[data-guide="chat-send"]', 'top', undefined],
    ['done', undefined, undefined, undefined],
  ]);
  assert.deepEqual(CONTEXTUAL_GUIDE_STEPS.agentList, [{
    key: 'create',
    target: '.empty-state-btn[data-guide="agent-list-create"], [data-guide="agent-list-create"]',
    placement: 'bottom',
    interact: true,
  }]);
  assert.deepEqual(CONTEXTUAL_GUIDE_STEPS.tenantModels.map((step) => [step.key, step.target, step.placement, step.action?.kind ?? null]), [
    ['intro', undefined, undefined, null],
    ['addModel', '[data-guide="settings-add-model"], [data-guide="settings-models"]', 'left', 'open-models-settings'],
    ['done', undefined, undefined, null],
  ]);
});

test('kbCreate steps assemble like KbCreateContextualGuide.vue (document KB with embedding)', () => {
  const steps = kbCreateGuideSteps({ isFaq: false, needsEmbedding: true });
  assert.deepEqual(steps.map((step) => step.key), [
    'type', 'name', 'indexing', 'navModels', 'llm', 'embedding',
    'parser', 'chunking', 'storage', 'navMultimodal', 'multimodalToggle', 'multimodalVllm', 'submit',
  ]);
  const byKey = new Map(steps.map((step) => [step.key, step]));
  assert.equal(byKey.get('type')?.target, '[data-guide="kb-create-type"]');
  assert.deepEqual(byKey.get('type')?.action, { kind: 'focus-kb-editor-section', section: 'basic' });
  assert.equal(byKey.get('indexing')?.optional, undefined, 'indexing step is not optional in Vue');
  assert.deepEqual(byKey.get('navModels')?.action, { kind: 'focus-kb-editor-section', section: 'models' });
  assert.equal(byKey.get('embedding')?.optional, true);
  assert.deepEqual(byKey.get('parser')?.action, { kind: 'focus-kb-editor-section', section: 'parser' });
  assert.deepEqual(byKey.get('submit')?.action, { kind: 'focus-kb-editor-section', section: 'basic' });
  assert.equal(byKey.get('submit')?.placement, 'top');
});

test('kbCreate steps drop indexing/multimodal chain for FAQ and add the faq nav instead', () => {
  const steps = kbCreateGuideSteps({ isFaq: true });
  assert.deepEqual(steps.map((step) => step.key), [
    'type', 'name', 'navModels', 'llm', 'faq', 'submit',
  ]);
  const byKey = new Map(steps.map((step) => [step.key, step]));
  assert.equal(byKey.get('faq')?.target, '[data-guide="kb-editor-nav-faq"]');
  assert.deepEqual(byKey.get('faq')?.action, { kind: 'focus-kb-editor-section', section: 'faq' });
  assert.equal(byKey.get('faq')?.optional, true);
});

test('agentCreate steps assemble like AgentCreateContextualGuide.vue (navTools only in agent mode)', () => {
  const normal = agentCreateGuideSteps({ isAgentMode: false });
  assert.deepEqual(normal.map((step) => step.key), [
    'mode', 'agentType', 'name', 'navModel', 'model', 'navKnowledge', 'knowledge',
    'navWebsearch', 'navMultimodal', 'multimodal', 'submit',
  ]);
  const byKey = new Map(normal.map((step) => [step.key, step]));
  assert.deepEqual(byKey.get('navModel')?.action, { kind: 'focus-agent-editor-section', section: 'model' });
  assert.equal(byKey.get('agentType')?.optional, true);
  assert.equal(byKey.get('submit')?.interact, true);
  assert.equal(byKey.get('submit')?.placement, 'top');

  const agentMode = agentCreateGuideSteps({ isAgentMode: true });
  assert.equal(agentMode.find((step) => step.key === 'navTools')?.target, '[data-guide="agent-editor-nav-tools"]');
  assert.deepEqual(agentMode.find((step) => step.key === 'navTools')?.action, { kind: 'focus-agent-editor-section', section: 'tools' });
  assert.deepEqual(
    agentMode.filter((step) => (step as ContextualGuideStep).optional).map((step) => step.key),
    ['agentType', 'navWebsearch', 'navMultimodal', 'multimodal', 'navTools'],
  );
});

test('resolveContextualGuideSteps routes dynamic tours and keeps static ones', () => {
  assert.equal(resolveContextualGuideSteps('kbList'), CONTEXTUAL_GUIDE_STEPS.kbList);
  assert.equal(resolveContextualGuideSteps('chat')[0]?.key, 'kb');
  assert.equal(resolveContextualGuideSteps('kbCreate', { isFaq: true }).length, 6);
  assert.equal(resolveContextualGuideSteps('agentCreate', {}).length, 11);
});

test('step copy prefix switches to stepsAgent for the agent variant (TenantModelsGuide.vue)', () => {
  assert.equal(contextualGuideStepPrefix('tenantModels'), 'contextualGuide.tenantModels.steps');
  assert.equal(contextualGuideStepPrefix('tenantModels', { variant: 'agent' }), 'contextualGuide.tenantModels.stepsAgent');
  assert.equal(contextualGuideStepPrefix('kbList'), 'contextualGuide.kbList.steps');
});

test('(dismissal) marking done writes the Vue key and cascades alsoCompleteTours', () => {
  const kbStorage = makeStorage();
  markContextualGuideDone(kbStorage, 'kbCreate');
  assert.equal(kbStorage.map.get('weknora:contextual-guide-kb-create:v3'), '1');
  assert.equal(kbStorage.map.get('weknora:contextual-guide-kb-list:v2'), '1', 'kbCreate completes kbList');
  assert.equal([...kbStorage.map.keys()].length, 2, 'no unrelated keys are written');

  const agentStorage = makeStorage();
  markContextualGuideDone(agentStorage, 'agentCreate');
  assert.equal(agentStorage.map.get('weknora:contextual-guide-agent-list:v1'), '1', 'agentCreate completes agentList');

  const plain = makeStorage();
  markContextualGuideDone(plain, 'chat');
  assert.equal(plain.map.get('weknora:contextual-guide-chat:v1'), '1');
  assert.equal(plain.map.size, 1, 'chat has no cascade');

  assert.equal(isContextualGuideDone(makeStorage({ 'weknora:contextual-guide-chat:v1': '1' }), 'chat'), true);
  assert.equal(isContextualGuideDone(makeStorage({ 'weknora:contextual-guide-chat:v1': '0' }), 'chat'), false);
  assert.equal(isGlobalUserGuideDone(makeStorage({ 'weknora:new-user-guide-done:v1': '1' })), true);
});

test('(gating) shouldOpenContextualGuide mirrors the Vue tryOpen guard', () => {
  const fresh = makeStorage();
  assert.equal(shouldOpenContextualGuide(fresh, 'kbList', true), false, 'global welcome tour must finish first');
  const globalDone = makeStorage({ 'weknora:new-user-guide-done:v1': '1' });
  assert.equal(shouldOpenContextualGuide(globalDone, 'kbList', true), true);
  assert.equal(shouldOpenContextualGuide(globalDone, 'kbList', false), false, 'when=false blocks the open');
  markContextualGuideDone(globalDone, 'kbList');
  assert.equal(shouldOpenContextualGuide(globalDone, 'kbList', true), false, 'dismissed tours never reopen');
});

// Vue KnowledgeBase.vue:339-345 showKbDetailContextualGuide — detail-entry
// arm condition for the empty-KB welcome tour (tab-independent).
test('(trigger) shouldArmKbDetailGuideOnEntry replays the Vue kbDetail when-condition', () => {
  const base = { knowledgeBaseId: 'kb-1', kbType: 'document', canEdit: true, documentsLoading: false, documentCount: 0 };
  assert.equal(shouldArmKbDetailGuideOnEntry(base), true, 'editable non-FAQ empty KB arms on entry');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, kbType: 'faq' }), false, 'FAQ libraries never see the tour');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, kbType: 'FAQ' }), false, 'type comparison is case-insensitive');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, kbType: null }), true, 'missing type behaves like the Vue || fallback');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, canEdit: false }), false, 'viewers never get the upload tour');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, documentsLoading: true }), false, 'loading lists do not arm');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, documentCount: 3 }), false, 'non-empty KBs do not arm');
  assert.equal(shouldArmKbDetailGuideOnEntry({ ...base, knowledgeBaseId: '' }), false, 'falsy kbId never arms');
});

test('(routing) openContextualGuide records the pending intent and dispatches the event', () => {
  const events: Array<{ type: string; detail: unknown }> = [];
  const sessionMap = new Map<string, string>();
  const fakeWindow = {
    dispatchEvent: (event: Event) => {
      events.push({ type: event.type, detail: (event as CustomEvent).detail });
      return true;
    },
    sessionStorage: {
      getItem: (key: string) => (sessionMap.has(key) ? sessionMap.get(key)! : null),
      setItem: (key: string, value: string) => { sessionMap.set(key, value); },
      removeItem: (key: string) => { sessionMap.delete(key); },
    },
  } as unknown as Pick<Window, 'dispatchEvent' | 'sessionStorage'>;
  openContextualGuide('kbDetail', fakeWindow as Pick<Window, 'dispatchEvent' | 'sessionStorage'> ? { isFaq: false } : undefined, fakeWindow);
  const pending = consumePendingContextualGuide(fakeWindow.sessionStorage as Storage);
  assert.deepEqual(pending, { tour: 'kbDetail', options: { isFaq: false } });
  assert.deepEqual(events.map((event) => event.type), ['weknora:open-contextual-guide']);
  assert.deepEqual((events[0]?.detail as { tour: string }).tour, 'kbDetail');
  assert.equal(consumePendingContextualGuide(fakeWindow.sessionStorage as Storage), null, 'intent is one-shot');
});

test('(routing) consumePendingContextualGuide survives a full-page navigation hand-off and drops corrupt entries', () => {
  const sessionMap = new Map<string, string>([['weknora:contextual-guide-pending:v1', JSON.stringify({ tour: 'tenantModels', options: { variant: 'agent' } })]]);
  const storage = {
    getItem: (key: string) => (sessionMap.has(key) ? sessionMap.get(key)! : null),
    setItem: (key: string, value: string) => { sessionMap.set(key, value); },
    removeItem: (key: string) => { sessionMap.delete(key); },
  } as unknown as Storage;
  assert.deepEqual(consumePendingContextualGuide(storage), { tour: 'tenantModels', options: { variant: 'agent' } });

  const corrupt = new Map<string, string>([['weknora:contextual-guide-pending:v1', 'not-json']]);
  const corruptStorage = {
    getItem: (key: string) => corrupt.get(key) ?? null,
    removeItem: (key: string) => { corrupt.delete(key); },
  } as unknown as Storage;
  assert.equal(consumePendingContextualGuide(corruptStorage), null);
});

test('copy table covers every catalog step in all five locales', () => {
  const tours: Array<{ tour: Parameters<typeof resolveContextualGuideSteps>[0]; options: Parameters<typeof resolveContextualGuideSteps>[1]; prefix: string; keys: string[] }> = [
    { tour: 'kbList', options: {}, prefix: 'contextualGuide.kbList.steps', keys: ['create'] },
    { tour: 'kbCreate', options: { isFaq: false, needsEmbedding: true }, prefix: 'contextualGuide.kbCreate.steps', keys: kbCreateGuideSteps({ isFaq: false, needsEmbedding: true }).map((s) => s.key) },
    { tour: 'kbCreate', options: { isFaq: true }, prefix: 'contextualGuide.kbCreate.steps', keys: kbCreateGuideSteps({ isFaq: true }).map((s) => s.key) },
    { tour: 'kbDetail', options: {}, prefix: 'contextualGuide.kbDetail.steps', keys: ['intro', 'upload', 'done'] },
    { tour: 'chat', options: {}, prefix: 'contextualGuide.chat.steps', keys: ['kb', 'input', 'send', 'done'] },
    { tour: 'tenantModels', options: {}, prefix: 'contextualGuide.tenantModels.steps', keys: ['intro', 'addModel', 'done'] },
    { tour: 'tenantModels', options: { variant: 'agent' }, prefix: 'contextualGuide.tenantModels.stepsAgent', keys: ['intro', 'addModel', 'done'] },
    { tour: 'agentList', options: {}, prefix: 'contextualGuide.agentList.steps', keys: ['create'] },
    { tour: 'agentCreate', options: { isAgentMode: true }, prefix: 'contextualGuide.agentCreate.steps', keys: agentCreateGuideSteps({ isAgentMode: true }).map((s) => s.key) },
  ];
  const locales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;
  for (const locale of locales) {
    const table = CONTEXTUAL_GUIDE_MESSAGES[locale];
    for (const label of ['contextualGuide.stepOf', 'contextualGuide.skip', 'contextualGuide.prev', 'contextualGuide.next', 'contextualGuide.done', 'contextualGuide.interactHint', 'contextualGuide.tenantModels.needChatModelFirst']) {
      assert.equal(typeof table[label as keyof typeof table], 'string', `${locale} missing ${label}`);
    }
    for (const { options, prefix, keys } of tours) {
      for (const key of keys) {
        assert.equal(typeof table[`${prefix}.${key}.title` as keyof typeof table], 'string', `${locale} missing ${prefix}.${key}.title`);
        assert.equal(typeof table[`${prefix}.${key}.desc` as keyof typeof table], 'string', `${locale} missing ${prefix}.${key}.desc`);
      }
    }
  }
});

test('zh-CN step copy is byte-identical to the Vue locale sources', () => {
  const table = CONTEXTUAL_GUIDE_MESSAGES['zh-CN'];
  assert.equal(table['contextualGuide.skip'], '跳过');
  assert.equal(table['contextualGuide.next'], '下一步');
  assert.equal(table['contextualGuide.done'], '知道了');
  assert.equal(table['contextualGuide.interactHint'], '请直接点击高亮区域继续');
  assert.equal(table['contextualGuide.kbList.steps.create.title'], '创建第一个知识库');
  assert.equal(table['contextualGuide.chat.steps.kb.desc'], '点击 {\'@\'} 可指定一个或多个知识库/文件，仅基于选中内容回答；不选则按当前智能体配置检索。');
  assert.equal(table['contextualGuide.kbDetail.steps.done.desc'], '文档解析入库后，可在对话中 {\'@\'} 本知识库提问，回答会附带引用来源。');
  assert.equal(table['contextualGuide.tenantModels.steps.intro.title'], '需要先配置模型');
  assert.equal(table['contextualGuide.tenantModels.stepsAgent.intro.title'], '需要先配置对话模型');
  assert.equal(table['contextualGuide.tenantModels.needChatModelFirst'], '请先添加对话模型（KnowledgeQA），再创建智能体。');
  assert.equal(CONTEXTUAL_GUIDE_MESSAGES['en-US']['contextualGuide.kbList.steps.create.title'], 'Create your first knowledge base');
});

test('contextualGuideMessage renders literals and named params like vue-i18n', () => {
  assert.equal(
    contextualGuideMessage('zh-CN', 'contextualGuide.chat.steps.kb.desc'),
    '点击 @ 可指定一个或多个知识库/文件，仅基于选中内容回答；不选则按当前智能体配置检索。',
  );
  assert.equal(contextualGuideMessage('zh-CN', 'contextualGuide.stepOf', { current: 2, total: 4 }), '2 / 4');
  assert.equal(contextualGuideMessage('zh-CN', 'contextualGuide.kbList.steps.create.title'), '创建第一个知识库');
});
