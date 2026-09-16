#!/usr/bin/env node
// Backfill missing locale keys into @weknora/i18n from the authoritative Vue
// locale tables (frontend/src/i18n/locales/*.ts), byte-exact.
//
// Reusable pattern (see merge-settings-keys.mjs for the merge-into-existing-file
// variant): this script EMITS new generated modules under packages/i18n/src/
// generated/ and refuses to shadow a key that already exists in the shared
// bundle with a different value.
//
// Run: pnpm exec tsx scripts/parity/backfill-i18n-keys.mjs
// Slice: 2026-09-13 i18n backfill (font / tenant members / agent integrations /
// shell session list / kb list extras).

const LOCALES = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'];

function flatten(obj, prefix = '', out = {}) {
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? prefix + '.' + k : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flatten(v, path, out);
    else out[path] = String(v);
  }
  return out;
}

const vueMaps = {};
for (const locale of LOCALES) {
  const mod = await import('../../frontend/src/i18n/locales/' + locale + '.ts');
  const def = mod.default ?? mod;
  vueMaps[locale] = flatten(def);
}
console.log('zh-CN flattened keys:', Object.keys(vueMaps['zh-CN']).length);

// ---- Block definitions -----------------------------------------------------
// PAGER: the Vue files carry NO pager keys (TenantMembers.vue renders TDesign's
// built-in t-pagination locale, see
// frontend/node_modules/tdesign-vue-next/esm/locale/zh_CN.js -> pagination).
// Values mirror the React panel's local fallbacks byte-identically
// (apps/web/src/settings/TenantMembersPanel.tsx LOCAL_FALLBACKS). Two key-name
// sets are emitted: the slice-spec names (tenantMember.pager.*) AND the names
// the panel actually queries (tenantMembersPanel.pager.*) so the panel's
// fallbacks are shadowed once the bundle ships. ja/ko/ru carry the en-US copy,
// matching the panel's current en-US fallthrough (byte-identical rendering).

const PAGER_VALUES = {
  'zh-CN': {
    'tenantMember.pager.totalItems': '共 {total} 条数据',
    'tenantMember.pager.itemsPerPage': '{size} 条/页',
    'tenantMember.pager.jumpTo': '跳至',
    'tenantMember.pager.pageUnit': '页',
    'tenantMembersPanel.pager.total': '共 {total} 条数据',
    'tenantMembersPanel.pager.sizePerPage': '{size} 条/页',
    'tenantMembersPanel.pager.jumper': '跳至',
    'tenantMembersPanel.pager.pageUnit': '页',
  },
  'en-US': {
    'tenantMember.pager.totalItems': 'Total {total} items',
    'tenantMember.pager.itemsPerPage': '{size} / page',
    'tenantMember.pager.jumpTo': 'Go to',
    'tenantMember.pager.pageUnit': 'page',
    'tenantMembersPanel.pager.total': 'Total {total} items',
    'tenantMembersPanel.pager.sizePerPage': '{size} / page',
    'tenantMembersPanel.pager.jumper': 'Go to',
    'tenantMembersPanel.pager.pageUnit': 'page',
  },
};
for (const locale of ['ja-JP', 'ko-KR', 'ru-RU']) PAGER_VALUES[locale] = PAGER_VALUES['en-US'];

const RBAC_ACTIONS = ['member_added', 'member_removed', 'member_role_changed', 'member_left', 'access_denied', 'invitation_sent', 'invitation_accepted', 'invitation_declined', 'invitation_revoked', 'invitation_expired'];

const BLOCKS = [
  {
    module: 'font.ts',
    exportName: 'fontMessages',
    provenance: "font block (settings GeneralPreferences font dropdowns; Vue GeneralSettings.vue uses t('font.sans.<key>') / t('font.mono.<key>')).",
    keys: [
      'font.sans.system', 'font.sans.pingfang', 'font.sans.georgia', 'font.sans.yahei',
      'font.sans.times', 'font.sans.noto-cjk', 'font.sans.dejavu-serif', 'font.sans.sans-serif',
      'font.mono.system', 'font.mono.menlo', 'font.mono.monaco', 'font.mono.consolas',
      'font.mono.cascadia', 'font.mono.dejavu-mono', 'font.mono.liberation-mono', 'font.mono.monospace',
    ],
  },
  {
    module: 'members.ts',
    exportName: 'memberMessages',
    provenance: 'tenantInvitation.status / tenantMember.permissions / tenantMember.audit blocks (settings TenantMembers surface) + pager copy mirrored from the React panel fallbacks (no Vue keys: TDesign built-in locale).',
    keys: [
      'tenantInvitation.status.pending', 'tenantInvitation.status.accepted', 'tenantInvitation.status.declined',
      'tenantInvitation.status.revoked', 'tenantInvitation.status.expired',
      'tenantInvitation.copied', 'tenantInvitation.copyFailed',
      'tenantMember.permissions.manageMembers', 'tenantMember.permissions.manageTenantConfig',
      'tenantMember.permissions.manageInfra', 'tenantMember.permissions.createOwnKB', 'tenantMember.permissions.readAll',
      ...RBAC_ACTIONS.map((s) => 'tenantMember.audit.action.rbac.' + s),
      'tenantMember.audit.outcome.success', 'tenantMember.audit.outcome.denied',
      ...Object.keys(PAGER_VALUES['zh-CN']),
    ],
  },
  {
    module: 'agentIntegrations.ts',
    exportName: 'agentIntegrationsMessages',
    provenance: 'agentEditor.im / agentEditor.embed / embedPublish blocks (integrations surface).',
    keys: [
      'agentEditor.im.description', 'agentEditor.im.docLink', 'agentEditor.im.channelsTitle', 'agentEditor.im.addChannel',
      'agentEditor.im.empty', 'agentEditor.im.disabled', 'agentEditor.im.unnamed', 'agentEditor.im.deleteConfirm',
      'agentEditor.im.enabled', 'agentEditor.im.platform', 'agentEditor.im.channelName', 'agentEditor.im.channelNamePlaceholder',
      'agentEditor.im.channelNameDefaultHint', 'agentEditor.im.sectionCredentials',
      'agentEditor.im.feishu', 'agentEditor.im.lark', 'agentEditor.im.slack', 'agentEditor.im.telegram',
      'agentEditor.im.dingtalk', 'agentEditor.im.mattermost', 'agentEditor.im.wecom', 'agentEditor.im.wechat',
      'agentEditor.im.qqbot', 'agentEditor.im.yunzhijia',
      'agentEditor.embed.title', 'agentEditor.embed.description',
      'embedPublish.create', 'embedPublish.channelsTitle', 'embedPublish.disabled', 'embedPublish.empty',
      'embedPublish.deleteConfirm', 'embedPublish.name', 'embedPublish.namePlaceholder', 'embedPublish.nameDefaultHint',
      'embedPublish.defaultChannelName', 'embedPublish.allowedOrigins', 'embedPublish.originsPlaceholder',
      'embedPublish.created', 'embedPublish.resetKeyTitle',
    ],
  },
  {
    module: 'shellSessionList.ts',
    exportName: 'shellSessionListMessages',
    provenance: 'chatHeader (confirm bodies + delete action) / time (session list date groups) / listSpaceSidebar (agents page rail) blocks. menu.deleteSession requested by the slice does NOT exist in the Vue files; the Vue-canonical name is chatHeader.deleteSession.',
    keys: [
      'chatHeader.clearConfirmBody', 'chatHeader.deleteConfirmBody', 'chatHeader.deleteSession',
      'time.today', 'time.yesterday', 'time.last7Days', 'time.last30Days', 'time.earlier', 'time.pinned',
      'listSpaceSidebar.all', 'listSpaceSidebar.workspace', 'listSpaceSidebar.spaces',
      'listSpaceSidebar.favorites', 'listSpaceSidebar.recents',
    ],
  },
  {
    module: 'kbListExtras.ts',
    exportName: 'kbListExtrasMessages',
    provenance: 'common.noMoreData (FAQ/KB list end-of-data hint). knowledgeBase.infoCard.{myRole,chunkCount,hitCount} requested by the slice do NOT exist in the Vue files (closest: knowledgeBase.accessInfo.myRole / knowledgeBase.chunkCount) and were NOT invented.',
    keys: ['common.noMoreData'],
  },
];

// ---- Guards ----------------------------------------------------------------
const { messages } = await import('../../packages/i18n/src/index.ts');

function placeholders(value) {
  return [...value.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');
}

const problems = [];
for (const block of BLOCKS) {
  for (const key of block.keys) {
    if (key in PAGER_VALUES['zh-CN']) {
      for (const locale of LOCALES) {
        if (PAGER_VALUES[locale][key] === undefined) problems.push('PAGER INCOMPLETE ' + locale + ': ' + key);
      }
      continue;
    }
    const missing = LOCALES.filter((l) => vueMaps[l][key] === undefined);
    if (missing.length) { problems.push('NOT IN VUE [' + missing.join(',') + ']: ' + key); continue; }
    // Cross-locale placeholder differences exist in the Vue source itself
    // (embedPublish.nameDefaultHint: zh/ko have no {agent}); byte-exact is the
    // contract, so these are warnings, not failures.
    const expected = placeholders(vueMaps['zh-CN'][key]);
    for (const locale of LOCALES) {
      if (placeholders(vueMaps[locale][key]) !== expected) console.warn('note: placeholder set differs vs zh-CN (' + locale + '): ' + key);
    }
    for (const locale of LOCALES) {
      const existing = messages[locale][key];
      if (existing !== undefined && existing !== vueMaps[locale][key]) {
        problems.push('SHADOWS BUNDLE WITH DIFFERENT VALUE ' + locale + ': ' + key + ' existing=' + JSON.stringify(existing));
      }
    }
  }
}
if (problems.length) {
  console.error('PROBLEMS:\n' + problems.join('\n'));
  process.exit(1);
}

// ---- Emit ------------------------------------------------------------------
function renderModule(block) {
  const lines = [
    '// AUTO-PORTED (scripts/parity/backfill-i18n-keys.mjs) from',
    '// frontend/src/i18n/locales/*.ts -> ' + block.provenance,
    '// Keys are flattened with dot separators; values are byte-exact ports.',
    "import type { Locale } from '../index.ts';",
    '',
    'export const ' + block.exportName + ': Record<Locale, Record<string, string>> = {',
  ];
  for (const locale of LOCALES) {
    const entries = block.keys.map((key) => {
      const value = key in PAGER_VALUES['zh-CN'] ? PAGER_VALUES[locale][key] : vueMaps[locale][key];
      return JSON.stringify(key) + ':' + JSON.stringify(value);
    });
    lines.push('  ' + JSON.stringify(locale) + ': {' + entries.join(',') + '},');
  }
  lines.push('};');
  return lines.join('\n') + '\n';
}

const { writeFileSync } = await import('node:fs');
for (const block of BLOCKS) {
  const path = 'packages/i18n/src/generated/' + block.module;
  writeFileSync(path, renderModule(block));
  console.log('wrote', path, '(' + block.keys.length + ' keys x ' + LOCALES.length + ' locales)');
}
console.log('OK: ' + BLOCKS.reduce((n, b) => n + b.keys.length, 0) + ' distinct keys backfilled');
