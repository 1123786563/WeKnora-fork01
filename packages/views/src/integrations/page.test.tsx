import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import { INTEGRATION_SECTIONS } from './registry.ts';
import { integrationSectionCopy, imPlatformLabels, imPlatformOrder, unresolvedCopyKeys } from './view.ts';

test('external integration registry entries expose actionable guide urls', () => {
  for (const section of INTEGRATION_SECTIONS.filter((item) => item.external)) {
    assert.match(section.externalUrl ?? '', /^https:\/\//);
  }
});

// Vue baseline: frontend/src/views/integrations/IntegrationSettingsSection.vue
// renders one .section per tab with an h2 heading, a .section-description
// paragraph (IM adds the 查看接入文档 doc link), then the channel panel from
// frontend/src/components/IMChannelPanel.vue / AgentEmbedChannelPanel.vue.

test('im tab section copy mirrors the Vue zh-CN anatomy', () => {
  const copy = integrationSectionCopy('im', 'zh-CN');
  assert.equal(copy.heading, 'IM 集成');
  assert.match(copy.description, /将智能体接入即时通讯平台/);
  assert.match(copy.description, /企业微信/);
  assert.equal(copy.docLinkLabel, '查看接入文档');
  assert.match(copy.docUrl ?? '', /^https:\/\/github\.com\/Tencent\/WeKnora/);
  assert.equal(copy.channelsTitle, 'IM 渠道');
  assert.equal(copy.addTileLabel, '添加渠道');
  assert.equal(copy.emptyText, '暂无 IM 渠道');
  assert.equal(copy.disabledLabel, '已停用');
  assert.equal(copy.unnamedLabel, '未命名渠道');
  assert.match(copy.deleteConfirm, /删除该渠道/);
});

test('embed tab section copy mirrors the Vue zh-CN anatomy', () => {
  const copy = integrationSectionCopy('embed', 'zh-CN');
  assert.equal(copy.heading, '网页嵌入');
  assert.match(copy.description, /将智能体嵌入到您的网页/);
  assert.equal(copy.docLinkLabel, undefined);
  assert.equal(copy.docUrl, undefined);
  assert.equal(copy.channelsTitle, '嵌入渠道');
  assert.equal(copy.addTileLabel, '新建嵌入渠道');
  assert.equal(copy.emptyText, '暂无嵌入渠道');
  assert.equal(copy.disabledLabel, '已停用');
  assert.match(copy.deleteConfirm, /嵌入渠道/);
});

test('api tab section copy resolves from the shared integrations domain', () => {
  const copy = integrationSectionCopy('api', 'zh-CN');
  assert.equal(copy.heading, 'API 集成');
  assert.match(copy.description, /REST API/);
  assert.equal(copy.docLinkLabel, undefined);
});

test('external tabs reuse the shared landing hero copy', () => {
  const cli = integrationSectionCopy('cli', 'zh-CN');
  assert.equal(cli.heading, 'WeKnora CLI');
  assert.ok(cli.description.length > 0);
  const chrome = integrationSectionCopy('chrome', 'zh-CN');
  assert.equal(chrome.heading, '知识管理助手');
  const claw = integrationSectionCopy('claw', 'zh-CN');
  assert.equal(claw.heading, 'WeKnora Skill');
});

test('external landing pages keep the Vue landing layout contract', () => {
  const source = readFileSync(new URL('./page.tsx', import.meta.url), 'utf8');
  // S7：utilities 平移至 apps/web/src/integrations/views-integrations-u.css（wk-vi-168/wk-vi-111）
  const css = readFileSync(new URL('../../../../apps/web/src/integrations/views-integrations-u.css', import.meta.url), 'utf8');
  assert.match(source, /className=\{\'integration-landing wk-vi-168/);
  assert.match(source, /landing-hero/);
  assert.match(css, /grid-template-columns: minmax\(0,1fr\) minmax\(300px,380px\)/);
  assert.match(css, /@media \(min-width: 821px\)/);
  assert.match(source, /integrations\.chrome\.scenarios\.\$\{key\}/);
  assert.match(source, /integrations\.chrome\.storeMeta/);
  assert.match(source, /integrations\.claw\.ecosystemNote/);
  assert.match(source, /aria-label=\{t\(key\)\}/);
});

test('channel cards preserve keyboard activation and a compact permission checkbox', () => {
  const source = readFileSync(new URL('./page.tsx', import.meta.url), 'utf8');
  assert.match(source, /role=\{onOpenCard \? 'button' : undefined\}/);
  assert.match(source, /event\.key !== 'Enter' && event\.key !== ' '/);
  assert.match(source, /className="wk-vi-78 wk-vi-accent-primary" type="checkbox"/);
  assert.doesNotMatch(source, /className="box-border w-full max-w-\[420px\].*type="checkbox"/);
});

test('integration mutations are explicitly gated by the Vue admin boundary', () => {
  const source = readFileSync(new URL('./page.tsx', import.meta.url), 'utf8');
  assert.match(source, /canEdit = true/);
  assert.match(source, /canEdit && tab === 'im' && actions\.onToggleIm/);
  assert.match(source, /canEdit && \(actions\.onDeleteEmbed \|\| actions\.onDeleteIm\)/);
  assert.match(source, /\{canEdit \? <button type="button" className=\{CHANNEL_CARD_ADD_CLASS\}/);
  assert.match(source, /canSubmit=\{canEdit && Boolean\(actions\.onCreateEmbed \|\| actions\.onUpdateEmbed\)\}/);
});

test('im and embed section copy resolves for all five locales without leaking raw keys', () => {
  const locales = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;
  for (const locale of locales) {
    for (const tab of ['im', 'embed', 'api', 'cli', 'chrome', 'claw'] as const) {
      const copy = integrationSectionCopy(tab, locale);
      assert.ok(copy.heading.length > 0, tab + ' heading for ' + locale);
      assert.ok(copy.description.length > 0, tab + ' description for ' + locale);
    }
    const leaked = unresolvedCopyKeys(['im', 'embed'], locale);
    assert.deepEqual(leaked, [], 'unresolved keys for ' + locale);
  }
});

test('im platform labels cover the Vue platform list with zh-CN names', () => {
  const labels = imPlatformLabels('zh-CN');
  assert.deepEqual(Object.keys(labels).sort(), imPlatformOrder().map((key) => key).sort());
  assert.equal(labels.feishu, '飞书');
  assert.equal(labels.wecom, '企业微信');
  assert.equal(labels.dingtalk, '钉钉');
  assert.equal(labels.yunzhijia, '云之家');
  assert.equal(labels.lark, 'Lark（飞书国际版）');
  const english = imPlatformLabels('en-US');
  assert.equal(english.feishu, 'Feishu');
  assert.equal(english.wecom, 'WeCom');
});

// 跨任务转交 T08-OCR2-F5：plugins tab 内容面板必须有挂载点。packages/views
// 不能反向 import apps/web 的 PluginsPanel，接线走 view 层插槽——page.tsx 需
// 提供 pluginsSlot prop 并在 tab === 'plugins' 时渲染它。
test('plugins tab body mounts through the view-layer plugins slot', () => {
  const source = readFileSync(new URL('./page.tsx', import.meta.url), 'utf8');
  assert.match(source, /pluginsSlot/, 'IntegrationsPageProps must expose a pluginsSlot');
  assert.match(source, /tab === 'plugins'/, "a render branch keyed on tab === 'plugins' must exist");
});
