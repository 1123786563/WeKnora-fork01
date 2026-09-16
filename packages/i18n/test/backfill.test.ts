import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';
import { fontMessages } from '../src/generated/font.ts';
import { memberMessages } from '../src/generated/members.ts';
import { agentIntegrationsMessages } from '../src/generated/agentIntegrations.ts';
import { shellSessionListMessages } from '../src/generated/shellSessionList.ts';
import { kbListExtrasMessages } from '../src/generated/kbListExtras.ts';

// 2026-09-13 i18n backfill: keys four parity slices reported missing from the
// shared bundle, byte-exact from frontend/src/i18n/locales/*.ts (pager copy
// mirrored byte-identically from the TenantMembersPanel fallback table; the
// Vue file renders TDesign's built-in pagination locale instead).
const MODULES: Array<[string, Record<string, Record<string, string>>]> = [
  ['font', fontMessages],
  ['members', memberMessages],
  ['agentIntegrations', agentIntegrationsMessages],
  ['shellSessionList', shellSessionListMessages],
  ['kbListExtras', kbListExtrasMessages],
];

test('backfill blocks carry identical key sets across all five locales', () => {
  for (const [name, mod] of MODULES) {
    const keySets = supportedLocales.map((locale) => Object.keys(mod[locale] ?? {}).sort());
    for (const locale of supportedLocales) assert.ok(mod[locale], name + ' missing locale ' + locale);
    for (let i = 1; i < keySets.length; i++) {
      assert.deepEqual(keySets[i], keySets[0], name + ' key set mismatch');
    }
    assert.ok(keySets[0].length > 0, name + ' is empty');
  }
});

test('every backfilled key resolves via formatMessage in every locale', () => {
  for (const [, mod] of MODULES) {
    for (const key of Object.keys(mod['zh-CN']!)) {
      for (const locale of supportedLocales) {
        assert.notEqual(formatMessage(locale, key), key, locale + ' does not resolve ' + key);
        assert.equal(messages[locale][key], mod[locale]![key], locale + ' merged bundle value drifted for ' + key);
      }
    }
  }
});

// Spot checks: zh-CN / en-US values hardcoded from the Vue locale files
// (frontend/src/i18n/locales/{zh-CN,en-US}.ts) — byte-exact.
test('font block spot checks match the Vue values', () => {
  assert.equal(messages['zh-CN']['font.sans.system'], '系统默认');
  assert.equal(messages['zh-CN']['font.sans.pingfang'], '苹方 PingFang SC');
  assert.equal(messages['en-US']['font.sans.times'], 'Times New Roman (Serif)');
  assert.equal(messages['en-US']['font.mono.cascadia'], 'Cascadia Code');
});

test('members block spot checks match the Vue values and the panel pager copy', () => {
  assert.equal(messages['zh-CN']['tenantInvitation.status.pending'], '待接受');
  assert.equal(messages['en-US']['tenantInvitation.copied'], 'Copied to clipboard');
  assert.equal(messages['zh-CN']['tenantMember.permissions.manageInfra'], '配置模型 / 向量库 / IM 通道');
  assert.equal(messages['zh-CN']['tenantMember.audit.action.rbac.invitation_expired'], '邀请过期');
  assert.equal(messages['en-US']['tenantMember.audit.outcome.denied'], 'Denied');
  // Pager copy (TDesign locale in Vue; mirrored from TenantMembersPanel fallbacks).
  assert.equal(formatMessage('zh-CN', 'tenantMember.pager.totalItems', { total: 12 }), '共 12 条数据');
  assert.equal(messages['en-US']['tenantMembersPanel.pager.total'], 'Total {total} items');
  assert.equal(messages['ja-JP']['tenantMember.pager.jumpTo'], 'Go to');
});

test('agentIntegrations block spot checks match the Vue values', () => {
  assert.equal(messages['zh-CN']['agentEditor.im.sectionCredentials'], '平台凭证');
  assert.equal(messages['zh-CN']['agentEditor.embed.title'], '网页嵌入');
  assert.equal(messages['zh-CN']['embedPublish.created'], '嵌入渠道已创建');
  assert.equal(messages['en-US']['embedPublish.resetKeyTitle'], 'Reset channel key');
});

test('shellSessionList block spot checks match the Vue values', () => {
  // Same literal apps/web/src/platform/PlatformShell.tsx hardcodes today.
  assert.equal(messages['zh-CN']['chatHeader.clearConfirmBody'], '确认清空当前对话的全部消息？对话本身会保留，此操作无法恢复。');
  assert.equal(messages['zh-CN']['time.pinned'], '已置顶');
  assert.equal(messages['en-US']['time.today'], 'Today');
  assert.equal(messages['zh-CN']['listSpaceSidebar.recents'], '最近');
  assert.equal(messages['en-US']['listSpaceSidebar.recents'], 'Recent');
});

test('kbListExtras spot checks match the Vue values', () => {
  assert.equal(messages['zh-CN']['common.noMoreData'], '已加载全部内容');
  assert.equal(messages['en-US']['common.noMoreData'], 'All content loaded');
});
