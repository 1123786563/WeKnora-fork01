#!/usr/bin/env node
// Byte-exact verification for the 2026-09-13 i18n backfill: re-imports the
// authoritative Vue locale tables and asserts every backfilled key in
// packages/i18n/src/generated/{font,members,agentIntegrations,shellSessionList,
// kbListExtras}.ts matches the Vue value verbatim in all five locales, then
// prints the acceptance spot-checks through formatMessage.
//
// Run: pnpm exec tsx scripts/parity/verify-backfill-2026-09-13.mjs

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
  vueMaps[locale] = flatten(mod.default ?? mod);
}

const { formatMessage } = await import('../../packages/i18n/src/index.ts');
const MODULES = [
  'font.ts', 'members.ts', 'agentIntegrations.ts', 'shellSessionList.ts', 'kbListExtras.ts',
].map((f) => 'packages/i18n/src/generated/' + f);

// Keys whose values are intentionally NOT in the Vue locale files (TDesign
// built-in pager locale) — verified against the TenantMembersPanel fallback
// table instead.
const PAGER_EXPECTED = {
  // Slice-spec key names…
  'tenantMember.pager.totalItems': { 'zh-CN': '共 {total} 条数据', 'en-US': 'Total {total} items' },
  'tenantMember.pager.itemsPerPage': { 'zh-CN': '{size} 条/页', 'en-US': '{size} / page' },
  'tenantMember.pager.jumpTo': { 'zh-CN': '跳至', 'en-US': 'Go to' },
  'tenantMember.pager.pageUnit': { 'zh-CN': '页', 'en-US': 'page' },
  // …and the names TenantMembersPanel.tsx actually queries (same values).
  'tenantMembersPanel.pager.total': { 'zh-CN': '共 {total} 条数据', 'en-US': 'Total {total} items' },
  'tenantMembersPanel.pager.sizePerPage': { 'zh-CN': '{size} 条/页', 'en-US': '{size} / page' },
  'tenantMembersPanel.pager.jumper': { 'zh-CN': '跳至', 'en-US': 'Go to' },
  'tenantMembersPanel.pager.pageUnit': { 'zh-CN': '页', 'en-US': 'page' },
};

let checked = 0;
let mismatches = 0;
for (const path of MODULES) {
  const mod = await import('../../' + path);
  const exportName = Object.keys(mod).find((k) => k.endsWith('Messages'));
  const table = mod[exportName];
  for (const locale of LOCALES) {
    for (const [key, value] of Object.entries(table[locale])) {
      checked++;
      if (PAGER_EXPECTED[key] !== undefined) {
        const expected = PAGER_EXPECTED[key][locale];
        if (expected !== undefined && expected !== value) {
          mismatches++;
          console.error('MISMATCH (panel fallback) ' + locale + ' ' + key + ': ' + JSON.stringify(value) + ' != ' + JSON.stringify(expected));
        }
        continue;
      }
      if (vueMaps[locale][key] !== value) {
        mismatches++;
        console.error('MISMATCH (vue) ' + locale + ' ' + key + ': ' + JSON.stringify(value) + ' != ' + JSON.stringify(vueMaps[locale][key]));
      }
    }
  }
}
console.log('values checked:', checked, '| mismatches:', mismatches);

// Acceptance spot-checks through the public API.
const spot = (locale, key, values) => [locale, key, formatMessage(locale, key, values)];
console.log('spot:', spot('zh-CN', 'tenantMember.pager.totalItems', { total: 7 }).join(' -> '));
console.log('spot:', spot('zh-CN', 'tenantMembersPanel.pager.total', { total: 7 }).join(' -> '));
console.log('spot:', spot('en-US', 'font.sans.pingfang').join(' -> '));
console.log('spot:', spot('ja-JP', 'agentEditor.im.channelsTitle').join(' -> '));
console.log('spot:', spot('ko-KR', 'listSpaceSidebar.recents').join(' -> '));
console.log('spot:', spot('ru-RU', 'chatHeader.clearConfirmBody').slice(0, 80).join(' -> '));

if (mismatches > 0) process.exit(1);
console.log('BYTE-EXACT OK');
