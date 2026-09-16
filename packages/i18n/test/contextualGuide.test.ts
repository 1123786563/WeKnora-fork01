import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, contextualGuideMessages, supportedLocales, type Locale } from '../src/index.ts';
import zhVue from '../../../frontend/src/i18n/locales/zh-CN.ts';
import enVue from '../../../frontend/src/i18n/locales/en-US.ts';
import jaVue from '../../../frontend/src/i18n/locales/ja-JP.ts';
import koVue from '../../../frontend/src/i18n/locales/ko-KR.ts';
import ruVue from '../../../frontend/src/i18n/locales/ru-RU.ts';
import { CONTEXTUAL_GUIDE_MESSAGES } from '../../../packages/views/src/guides/contextual-guide-messages.ts';

// contextualGuide block (packages/i18n/src/generated/contextualGuide.ts) —
// byte-exact port of the Vue contextual-guide copy (frontend/src/i18n/locales/
// *.ts, contextualGuide block: zh-CN.ts:6689, en-US.ts:97, ja-JP.ts:97,
// ko-KR.ts:6687, ru-RU.ts:6687 = 89 keys x 5 locales = 445 values). The keys
// cover every string the Vue guides render: SpotlightGuide chrome
// (stepOf/skip/prev/next/done/interactHint), step title/desc pairs for the 7
// tours registered in frontend/src/config/contextualGuides.ts (kbList,
// kbCreate, kbDetail, chat, tenantModels, agentList, agentCreate — tenantModels
// with steps + stepsAgent variants and needChatModelFirst), exactly what
// SpotlightGuide.vue + the *_ContextualGuide.vue wrappers consume. No
// component-only inline copy exists: every consumed key resolves from the
// locale files, so this suite has no vue-component-source entries.
//
// packages/views/src/guides/contextual-guide-messages.ts keeps the same values
// as a local table (views cannot depend on the @weknora/i18n package name);
// this suite also byte-pins that local table to the shared bundle so the two
// copies cannot drift (R007 precedent: fix both sides, Vue is authoritative).
// Test-only relative import — the views runtime layering is untouched.

const VUE_BASELINE: Record<Locale, Record<string, unknown>> = {
  'zh-CN': zhVue,
  'en-US': enVue,
  'ja-JP': jaVue,
  'ko-KR': koVue,
  'ru-RU': ruVue,
};

const STEP_TOURS: Record<string, string[]> = {
  agentCreate: ['agentType', 'knowledge', 'mode', 'model', 'multimodal', 'name', 'navKnowledge', 'navModel', 'navMultimodal', 'navTools', 'navWebsearch', 'submit'],
  agentList: ['create'],
  chat: ['done', 'input', 'kb', 'send'],
  kbCreate: ['chunking', 'embedding', 'faq', 'indexing', 'llm', 'multimodalToggle', 'multimodalVllm', 'name', 'navModels', 'navMultimodal', 'parser', 'storage', 'submit', 'type'],
  kbDetail: ['done', 'intro', 'upload'],
  kbList: ['create'],
};

const EXPECTED_LEAF_KEYS = [
  'done', 'interactHint', 'next', 'prev', 'skip', 'stepOf',
  ...Object.entries(STEP_TOURS).flatMap(([tour, steps]) => steps.flatMap((step) => [`${tour}.steps.${step}.desc`, `${tour}.steps.${step}.title`])),
  'tenantModels.needChatModelFirst',
  ...['addModel', 'done', 'intro'].flatMap((step) => [`tenantModels.steps.${step}.desc`, `tenantModels.steps.${step}.title`]),
  ...['addModel', 'done', 'intro'].flatMap((step) => [`tenantModels.stepsAgent.${step}.desc`, `tenantModels.stepsAgent.${step}.title`]),
].map((leaf) => `contextualGuide.${leaf}`).sort();

function vueGet(obj: Record<string, unknown>, path: string): unknown {
  let current: unknown = obj;
  for (const part of path.split('.')) {
    if (current == null || typeof current !== 'object') return undefined;
    current = (current as Record<string, unknown>)[part];
  }
  return current;
}

test('contextualGuide carries exactly the 89 Vue keys in every locale', () => {
  assert.equal(EXPECTED_LEAF_KEYS.length, 89, 'inventory size (89 keys x 5 locales = 445 values)');
  for (const locale of supportedLocales) {
    const keys = Object.keys(contextualGuideMessages[locale]).sort();
    assert.equal(keys.length, 89, `contextualGuide key count for ${locale}`);
    assert.deepEqual(keys, EXPECTED_LEAF_KEYS, `key set drift for ${locale}`);
  }
});

test('contextualGuide values are byte-exact against the Vue locale baseline', () => {
  for (const locale of supportedLocales) {
    for (const [key, value] of Object.entries(contextualGuideMessages[locale])) {
      const fromVue = vueGet(VUE_BASELINE[locale], key);
      assert.equal(typeof fromVue, 'string', `Vue baseline lacks ${key} for ${locale}`);
      assert.equal(value, fromVue, `value drift for ${key} (${locale})`);
    }
  }
});

test('contextualGuide keys are merged into the shared messages bundle with no locale drift', () => {
  const localeKeySets = supportedLocales.map((locale) => Object.keys(messages[locale]).filter((key) => key.startsWith('contextualGuide.')));
  for (const keySet of localeKeySets) assert.equal(keySet.length, 89, 'merged bundle carries the full block');
  for (let i = 1; i < localeKeySets.length; i += 1) {
    assert.deepEqual([...localeKeySets[i]].sort(), [...localeKeySets[0]].sort(), `merged bundle drift for ${supportedLocales[i]}`);
  }
});

test('contextualGuide copy resolves through formatMessage with vue-i18n syntax preserved', () => {
  assert.equal(formatMessage('zh-CN', 'contextualGuide.stepOf', { current: 2, total: 7 }), '2 / 7');
  assert.equal(formatMessage('zh-CN', 'contextualGuide.done'), '知道了');
  assert.equal(formatMessage('en-US', 'contextualGuide.done'), 'Got it');
  assert.equal(formatMessage('ja-JP', 'contextualGuide.done'), 'OK');
  assert.equal(formatMessage('ko-KR', 'contextualGuide.done'), '확인');
  assert.equal(formatMessage('ru-RU', 'contextualGuide.done'), 'Понятно');
  assert.equal(formatMessage('zh-CN', 'contextualGuide.interactHint'), '请直接点击高亮区域继续');
  // The literal vue-i18n escape {'@'} must survive verbatim in the shared
  // bundle (renderContextualGuideMessage resolves it, like the message compiler).
  assert.equal(formatMessage('en-US', 'contextualGuide.chat.steps.kb.desc', {}), CONTEXTUAL_GUIDE_MESSAGES['en-US']['contextualGuide.chat.steps.kb.desc']);
  assert.ok(formatMessage('zh-CN', 'contextualGuide.chat.steps.kb.desc').includes(`{'@'}`));
});

test('views local table stays byte-identical to the shared contextualGuide bundle', () => {
  for (const locale of supportedLocales) {
    assert.deepEqual(CONTEXTUAL_GUIDE_MESSAGES[locale], contextualGuideMessages[locale], `local table drift for ${locale}`);
  }
});
