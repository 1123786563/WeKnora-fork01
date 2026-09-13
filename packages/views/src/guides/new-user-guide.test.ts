import assert from 'node:assert/strict';
import test from 'node:test';

import {
  GLOBAL_USER_GUIDE_KEY,
  OPEN_NEW_USER_GUIDE_EVENT,
  guideMessage,
  isNewUserGuideDone,
  markNewUserGuideDone,
  shouldAutoOpenNewUserGuide,
} from './new-user-guide.ts';
import { NEW_USER_GUIDE_MESSAGES, NEW_USER_GUIDE_STEPS } from './steps.ts';

// localStorage stub mirroring the subset the guide touches (Vue writes the
// literal '1' on finish/dismiss and treats only '1' as done).
function makeStorage(initial: Record<string, string> = {}) {
  const map = new Map(Object.entries(initial));
  const storage = {
    getItem: (key: string) => (map.has(key) ? map.get(key)! : null),
    setItem: (key: string, value: string) => {
      storage.calls.set.push([key, value]);
      map.set(key, value);
    },
    calls: { set: [] as Array<[string, string]> },
  };
  return storage;
}

test('storage key matches the Vue literal from contextualGuides.ts', () => {
  assert.equal(GLOBAL_USER_GUIDE_KEY, 'weknora:new-user-guide-done:v1');
  assert.equal(OPEN_NEW_USER_GUIDE_EVENT, 'weknora:open-new-user-guide');
});

test('(a) guide does not auto-open once the key is set to 1', () => {
  assert.equal(shouldAutoOpenNewUserGuide(makeStorage({ [GLOBAL_USER_GUIDE_KEY]: '1' })), false);
  assert.equal(isNewUserGuideDone(makeStorage({ [GLOBAL_USER_GUIDE_KEY]: '1' })), true);
});

test('(b) guide auto-opens when the key is absent or any value other than 1', () => {
  assert.equal(shouldAutoOpenNewUserGuide(makeStorage()), true);
  assert.equal(shouldAutoOpenNewUserGuide(makeStorage({ [GLOBAL_USER_GUIDE_KEY]: '0' })), true);
  assert.equal(isNewUserGuideDone(makeStorage()), false);
});

test('(d/e) marking done writes the literal 1 under the Vue key exactly once per call', () => {
  const storage = makeStorage();
  markNewUserGuideDone(storage);
  assert.deepEqual(storage.calls.set, [[GLOBAL_USER_GUIDE_KEY, '1']]);
  assert.equal(storage.getItem(GLOBAL_USER_GUIDE_KEY), '1');
  assert.equal(shouldAutoOpenNewUserGuide(storage), false);
});

test('step catalog replays the Vue NewUserGuide 7 steps in order', () => {
  assert.deepEqual(NEW_USER_GUIDE_STEPS.map((step) => step.key), [
    'welcome', 'knowledge', 'agents', 'chat', 'settings', 'models', 'done',
  ]);
  const byKey = new Map(NEW_USER_GUIDE_STEPS.map((step) => [step.key, step]));
  // Centered-card steps carry no target (Vue: welcome and done).
  assert.equal(byKey.get('welcome')?.target, undefined);
  assert.equal(byKey.get('done')?.target, undefined);
  // Anchored steps reuse the Vue selectors and placements verbatim.
  assert.equal(byKey.get('knowledge')?.target, '[data-guide="nav-knowledge-bases"]');
  assert.equal(byKey.get('knowledge')?.placement, 'right');
  assert.equal(byKey.get('agents')?.target, '[data-guide="nav-agents"]');
  assert.equal(byKey.get('agents')?.placement, 'right');
  assert.equal(byKey.get('agents')?.optional, true);
  assert.equal(byKey.get('chat')?.target, '[data-guide="nav-creatChat"]');
  assert.equal(byKey.get('chat')?.placement, 'right');
  assert.equal(byKey.get('settings')?.target, '[data-guide="user-menu"]');
  assert.equal(byKey.get('settings')?.placement, 'right');
  assert.equal(byKey.get('models')?.target, '[data-guide="settings-add-model"], [data-guide="settings-models"]');
  assert.equal(byKey.get('models')?.placement, 'left');
});

test('step catalog copy is complete for all five supported locales', () => {
  const expectedKeys = [
    'newUserGuide.stepOf', 'newUserGuide.skip', 'newUserGuide.prev', 'newUserGuide.next', 'newUserGuide.done', 'newUserGuide.reopen',
    ...NEW_USER_GUIDE_STEPS.flatMap((step) => [`newUserGuide.steps.${step.key}.title`, `newUserGuide.steps.${step.key}.desc`]),
  ].sort();
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    assert.deepEqual(Object.keys(NEW_USER_GUIDE_MESSAGES[locale]).sort(), expectedKeys, `locale ${locale} copy incomplete`);
  }
});

test('welcome copy is byte-identical to the Vue locale sources', () => {
  assert.equal(guideMessage('zh-CN', 'newUserGuide.steps.welcome.title'), '欢迎使用 WeKnora');
  assert.equal(guideMessage('zh-CN', 'newUserGuide.skip'), '跳过引导');
  assert.equal(guideMessage('zh-CN', 'newUserGuide.next'), '下一步');
  assert.equal(guideMessage('zh-CN', 'newUserGuide.done'), '完成');
  assert.equal(guideMessage('en-US', 'newUserGuide.steps.welcome.title'), 'Welcome to WeKnora');
  assert.equal(guideMessage('ja-JP', 'newUserGuide.skip'), 'スキップ');
  assert.equal(guideMessage('ko-KR', 'newUserGuide.skip'), '건너뛰기');
  assert.equal(guideMessage('ru-RU', 'newUserGuide.skip'), 'Пропустить');
});

test('guideMessage interpolates the stepOf pattern like vue-i18n named params', () => {
  assert.equal(guideMessage('zh-CN', 'newUserGuide.stepOf', { current: 1, total: 7 }), '1 / 7');
  assert.equal(guideMessage('en-US', 'newUserGuide.stepOf', { current: 7, total: 7 }), '7 / 7');
});
