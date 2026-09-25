import assert from 'node:assert/strict';
import test from 'node:test';

import { INTEGRATION_SECTIONS, integrationKeyFromQuery, integrationSection } from './registry.ts';
import { integrationSectionCopy } from './view.ts';
import { integrationsT } from './messages.ts';

test('registers every external integration entry without an empty navigation target', () => {
  assert.deepEqual(INTEGRATION_SECTIONS.map((item) => item.key), ['im', 'embed', 'api', 'cli', 'chrome', 'claw', 'plugins']);
  for (const item of INTEGRATION_SECTIONS) {
    assert.ok(item.viewId.length > 0);
    assert.ok(item.operations.length > 0);
  }
});

test('keeps API ownership and embed/IM capabilities explicit', () => {
  assert.equal(integrationSection('api')?.minRole, 'owner');
  assert.equal(integrationSection('embed')?.apiDomain, 'channels');
  assert.equal(integrationSection('im')?.apiDomain, 'im');
  assert.equal(integrationSection('chrome')?.external, true);
  assert.ok(integrationSection('embed')?.operations.includes('manage'));
});

test('registers the plugins discovery section for every member (T08)', () => {
  const plugins = integrationSection('plugins');
  assert.equal(plugins?.viewId, 'PluginDiscoverPanel');
  assert.equal(plugins?.apiDomain, null);
  assert.equal(plugins?.minRole, 'viewer');
  assert.equal(plugins?.external, false);
  assert.ok(plugins?.operations.includes('manage'));
  assert.equal(integrationKeyFromQuery('?section=plugins'), 'plugins', 'the plugins section resolves from the query alias');
});

// T08-OCR1-F1/F2：plugins 注册后集成页 tab 按钮（page.tsx 遍历 INTEGRATION_SECTIONS
// 渲染 t('integrations.tabs.' + key)）与 tab 内容 heading/description
// （integrationSectionCopy）都是即时消费者——三 key 必须经 integrationsT 的
// FALLBACK_STRINGS 回退层解析，任何 locale 都不得泄漏裸 key。
test('plugins section copy resolves through the fallback layer for all five locales (no raw key leak)', () => {
  const locales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;
  for (const locale of locales) {
    const tabLabel = integrationsT(locale, 'integrations.tabs.plugins');
    assert.notEqual(tabLabel, 'integrations.tabs.plugins', `integrations.tabs.plugins must resolve for ${locale}`);
    const copy = integrationSectionCopy('plugins', locale);
    assert.notEqual(copy.heading, 'integrations.plugins.title', `heading must resolve for ${locale}`);
    assert.notEqual(copy.description, 'integrations.plugins.subtitle', `description must resolve for ${locale}`);
    assert.ok(tabLabel.trim().length > 0, `tab label is non-empty for ${locale}`);
    assert.ok(copy.heading.trim().length > 0, `heading is non-empty for ${locale}`);
    assert.ok(copy.description.trim().length > 0, `description is non-empty for ${locale}`);
  }
});

test('maps legacy integration section and tab query aliases to a concrete tab', () => {
  assert.equal(integrationKeyFromQuery('?tab=cli'), 'cli');
  assert.equal(integrationKeyFromQuery('?section=integration-api'), 'api');
  assert.equal(integrationKeyFromQuery('?section=api'), 'api');
  assert.equal(integrationKeyFromQuery('?section=integrations'), 'im');
  assert.equal(integrationKeyFromQuery('?section=integrations&tab=embed'), 'embed');
  assert.equal(integrationKeyFromQuery('?section=unknown'), 'embed');
});
