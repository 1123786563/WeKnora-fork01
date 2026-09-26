import assert from 'node:assert/strict';
import test from 'node:test';

import { integrationSettingsQuery, integrationTabForSection, normalizeIntegrationSettingsSection } from './settings-route.ts';
import { SETTINGS_SECTIONS } from '../settings/registry.ts';

// OCR 终局第 2 轮 f27：设置分区裸 key 'plugins'（PluginSettings 管理面板）
// 与 integrations registry 的 tab key 'plugins'（PluginDiscoverPanel）撞名。
// normalizeIntegrationSettingsSection 此前对 INTEGRATION_SECTIONS 中的裸 key
// 一律加 integration- 前缀——SettingsPage 的 requestedSection（初始挂载与
// popstate 均经 integrationSettingsQuery）把 ?section=plugins 归一化为
// integration-plugins，深链/popstate 落到成员发现 tab，管理员插件设置面板
// 无任何可用深链。修复后：已注册 SETTINGS_SECTIONS 的裸 key 保持原样
//（settings 路由的 ?section= 唯一合法含义就是设置分区），integrations
// 专属裸 key 仍归一化为带前缀的规范 tab URL。
test('normalize keeps every registered settings-section key verbatim (deep links survive)', () => {
  for (const section of SETTINGS_SECTIONS) {
    assert.equal(normalizeIntegrationSettingsSection(section.key), section.key,
      `the settings-section key ${section.key} must survive normalization — prefixing it hijacks the panel's deep link`);
  }
  // 撞名主角单列：?section=plugins 深链必须回到管理员插件设置面板。
  assert.equal(normalizeIntegrationSettingsSection('plugins'), 'plugins');
  assert.equal(integrationSettingsQuery('?section=plugins').get('section'), 'plugins');
});

test('integration-only bare keys still normalize to the prefixed tab URL', () => {
  assert.equal(normalizeIntegrationSettingsSection('im'), 'integration-im');
  assert.equal(normalizeIntegrationSettingsSection('unknown-section'), 'unknown-section');
  assert.equal(integrationSettingsQuery('?section=im').get('section'), 'integration-im');
  // 裸 'plugins' 不再被 integrationTabForSection 命中（渲染不被劫持到
  // IntegrationsRoutePage）；带前缀的规范 tab URL 语义不变。
  assert.equal(integrationTabForSection('plugins'), undefined);
  assert.equal(integrationTabForSection('integration-plugins'), 'plugins');
});
