import assert from 'node:assert/strict';
import test from 'node:test';

import { formatMessage, messages, supportedLocales } from '../src/index.ts';

// R483 F2 (R482 B3-D2): the vue-i18n literal escape syntax ({'@'}) was
// copied verbatim from frontend/src/i18n/locales/*.ts into locale values.
// vue-i18n's message compiler renders {'@'} as a bare '@' (the escape
// exists because a bare '@' would open a linked message), but the React
// formatMessage has no escape syntax, so the sequence leaked into the UI
// verbatim (skills add dialog "粘贴 …或 {'@'}owner/slug", agent editor
// retrieve-only-when-mentioned copy, claw landing ecosystem note/meta,
// organization settings sharedAgentsKbHint, tenant member invite
// placeholder). These keys must therefore carry the RESOLVED form in the
// React bundle — the value Vue renders, not the value Vue stores.
//
// Deliberate exception: the contextualGuide.* block stays byte-exact
// against the Vue locale files (pinned by contextualGuide.test.ts) because
// its only consumers render through contextualGuideMessage() in
// packages/views/src/guides/contextual-guides.ts, which resolves the
// escapes like the vue-i18n message compiler does.

const AT_ESCAPE = "{'@'}";

const RESOLVED_AT_KEYS = [
  'settings.sandbox.skillSourcePlaceholder',
  'settings.sandbox.skillSourceSectionHint',
  'tenantMember.add.emailPlaceholder',
  'agent.editor.retrieveKBOnlyWhenMentioned',
  'agent.editor.retrieveKBOnlyWhenMentionedDesc',
  'integrations.claw.ecosystemNote',
  'integrations.claw.hubMeta',
  'organization.settings.sharedAgentsKbHint',
];

test('keys consumed via formatMessage render the at sign instead of the vue-i18n escape', () => {
  for (const locale of supportedLocales) {
    for (const key of RESOLVED_AT_KEYS) {
      const rendered = formatMessage(locale, key, { size: 256 });
      assert.ok(!rendered.includes(AT_ESCAPE), `${locale} ${key} still renders the literal escape: ${rendered}`);
    }
  }
  // Where the Vue baseline carries the escape, the resolved form must show @.
  assert.ok(formatMessage('zh-CN', 'agent.editor.retrieveKBOnlyWhenMentioned').includes('@'));
  assert.ok(formatMessage('en-US', 'agent.editor.retrieveKBOnlyWhenMentionedDesc').includes('@'));
  assert.ok(formatMessage('ja-JP', 'agent.editor.retrieveKBOnlyWhenMentionedDesc').includes('@'));
  assert.ok(formatMessage('ko-KR', 'agent.editor.retrieveKBOnlyWhenMentionedDesc').includes('@'));
  assert.ok(formatMessage('ru-RU', 'agent.editor.retrieveKBOnlyWhenMentionedDesc').includes('@'));
  assert.ok(formatMessage('en-US', 'integrations.claw.ecosystemNote').includes('@lyingbug/weknora'));
});

test('the leaked zh-CN / en-US copies match the Vue-rendered baseline', () => {
  // Vue zh-CN.ts:5664/5667/327 + en-US.ts:1525/6880 rendered through the
  // vue-i18n message compiler ({'@'} -> @, {size} interpolated).
  assert.equal(formatMessage('zh-CN', 'settings.sandbox.skillSourceSectionHint', { size: 256 }), '粘贴 ClawHub、GitHub 或 SkillHub 链接，或 @owner/slug。压缩包不超过 256 MB。');
  assert.equal(formatMessage('zh-CN', 'settings.sandbox.skillSourcePlaceholder'), 'ClawHub 用 @owner/slug，GitHub / SkillHub 请粘贴完整链接');
  assert.equal(formatMessage('zh-CN', 'tenantMember.add.emailPlaceholder'), 'invitee@example.com');
  assert.equal(formatMessage('en-US', 'settings.sandbox.skillSourcePlaceholder'), 'ClawHub: @owner/slug. GitHub/SkillHub: paste the full URL');
  assert.equal(formatMessage('en-US', 'integrations.claw.hubMeta'), 'ClawHub · @lyingbug/weknora · MIT-0');
  assert.equal(formatMessage('zh-CN', 'organization.settings.sharedAgentsKbHint').includes('可 @ 使用'), true);
});

test('no key outside the byte-exact contextualGuide block carries a raw vue-i18n literal escape', () => {
  for (const locale of supportedLocales) {
    for (const [key, value] of Object.entries(messages[locale])) {
      if (key.startsWith('contextualGuide.')) continue;
      assert.ok(!value.includes(AT_ESCAPE), `${locale} ${key} carries the raw escape: ${value}`);
    }
  }
});
