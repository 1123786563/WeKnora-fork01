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
// React-side value sources: keys the Vue files do NOT carry (no Vue baseline).
// The script owns these values verbatim; Vue presence/shadow guards are
// skipped for them. PAGER: the Vue files carry NO pager keys
// (TenantMembers.vue renders TDesign's built-in t-pagination locale, see
// frontend/node_modules/tdesign-vue-next/esm/locale/zh_CN.js -> pagination).
// Values mirror the React panel's local fallbacks byte-identically
// (apps/web/src/settings/TenantMembersPanel.tsx LOCAL_FALLBACKS). Two key-name
// sets are emitted: the slice-spec names (tenantMember.pager.*) AND the names
// the panel actually queries (tenantMembersPanel.pager.*) so the panel's
// fallbacks are shadowed once the bundle ships. ja/ko/ru carry the en-US copy,
// matching the panel's current en-US fallthrough (byte-identical rendering).
// PERSONALIZATION: Octop MBTI persona section (apps/web/src/agents/
// PersonaSection.tsx) — React-only feature, real ja/ko/ru translations.

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

const PERSONALIZATION_VALUES = {
  'zh-CN': {
    'agentEditor.personalization.title': '个性化',
    'agentEditor.personalization.desc': '为智能体选择 MBTI 人格类型并补充表达风格，塑造其回复的语气与性格',
    'agentEditor.personalization.mbtiLabel': '人格类型',
    'agentEditor.personalization.mbtiDesc': '选择一种 16 型人格，选「无」则保持默认回复风格',
    'agentEditor.personalization.noneLabel': '无',
    'agentEditor.personalization.noneDesc': '不设置人格，保持默认回复风格',
    'agentEditor.personalization.dimensionsLabel': '维度倾向',
    'agentEditor.personalization.styleLabel': '补充风格',
    'agentEditor.personalization.styleDesc': '可选。补充语气、口头禅等要求，与所选人格一起生效',
    'agentEditor.personalization.stylePlaceholder': '例如：语气轻松一些，多用类比解释复杂概念',
    'agentEditor.personalization.takeTest': '做个测试',
    'agentEditor.personalization.takeTestHint': '测试功能即将开放',
    'agentEditor.personalization.loadFailed': '人格类型加载失败',
    'agentEditor.personalization.unknownType': '当前保存的类型 {code} 不在类型列表中',
  },
  'en-US': {
    'agentEditor.personalization.title': 'Personalization',
    'agentEditor.personalization.desc': 'Pick an MBTI persona and add style notes to shape how the agent speaks',
    'agentEditor.personalization.mbtiLabel': 'Persona type',
    'agentEditor.personalization.mbtiDesc': 'Choose one of the 16 types, or None to keep the default reply style',
    'agentEditor.personalization.noneLabel': 'None',
    'agentEditor.personalization.noneDesc': 'No persona; keep the default reply style',
    'agentEditor.personalization.dimensionsLabel': 'Dimension tendencies',
    'agentEditor.personalization.styleLabel': 'Style notes',
    'agentEditor.personalization.styleDesc': 'Optional. Extra tone or wording requirements applied along with the selected persona',
    'agentEditor.personalization.stylePlaceholder': 'e.g. Keep a relaxed tone and use analogies for complex ideas',
    'agentEditor.personalization.takeTest': 'Take the test',
    'agentEditor.personalization.takeTestHint': 'The test is coming soon',
    'agentEditor.personalization.loadFailed': 'Failed to load persona types',
    'agentEditor.personalization.unknownType': 'Saved type {code} is not in the catalog',
  },
  'ja-JP': {
    'agentEditor.personalization.title': 'パーソナライズ',
    'agentEditor.personalization.desc': 'MBTIパーソナリティを選び、スタイルメモを追加してエージェントの話し方を調整します',
    'agentEditor.personalization.mbtiLabel': 'パーソナリティタイプ',
    'agentEditor.personalization.mbtiDesc': '16タイプから選択します。「なし」ならデフォルトの応答スタイルを維持します',
    'agentEditor.personalization.noneLabel': 'なし',
    'agentEditor.personalization.noneDesc': 'パーソナリティを設定せず、デフォルトの応答スタイルを維持します',
    'agentEditor.personalization.dimensionsLabel': '次元の傾向',
    'agentEditor.personalization.styleLabel': 'スタイルメモ',
    'agentEditor.personalization.styleDesc': '任意。話し方や口癖などの要件を追加し、選択したパーソナリティと合わせて適用されます',
    'agentEditor.personalization.stylePlaceholder': '例：口調はリラックスさせ、複雑な概念はたとえ話で説明する',
    'agentEditor.personalization.takeTest': 'テストを受ける',
    'agentEditor.personalization.takeTestHint': 'テスト機能は近日公開です',
    'agentEditor.personalization.loadFailed': 'パーソナリティタイプの読み込みに失敗しました',
    'agentEditor.personalization.unknownType': '保存済みのタイプ {code} は一覧にありません',
  },
  'ko-KR': {
    'agentEditor.personalization.title': '개인화',
    'agentEditor.personalization.desc': 'MBTI 성격 유형을 선택하고 스타일 메모를 추가해 에이전트의 말투를 조정합니다',
    'agentEditor.personalization.mbtiLabel': '성격 유형',
    'agentEditor.personalization.mbtiDesc': '16가지 유형 중 선택하거나 «없음»을 선택해 기본 응답 스타일을 유지합니다',
    'agentEditor.personalization.noneLabel': '없음',
    'agentEditor.personalization.noneDesc': '성격을 설정하지 않고 기본 응답 스타일을 유지합니다',
    'agentEditor.personalization.dimensionsLabel': '차원 성향',
    'agentEditor.personalization.styleLabel': '스타일 메모',
    'agentEditor.personalization.styleDesc': '선택 사항. 말투·버릇 등 요구 사항을 추가하면 선택한 성격과 함께 적용됩니다',
    'agentEditor.personalization.stylePlaceholder': '예: 편안한 말투로, 복잡한 개념은 비유로 설명',
    'agentEditor.personalization.takeTest': '테스트하기',
    'agentEditor.personalization.takeTestHint': '테스트 기능이 곧 제공됩니다',
    'agentEditor.personalization.loadFailed': '성격 유형을 불러오지 못했습니다',
    'agentEditor.personalization.unknownType': '저장된 유형 {code}이(가) 목록에 없습니다',
  },
  'ru-RU': {
    'agentEditor.personalization.title': 'Персонализация',
    'agentEditor.personalization.desc': 'Выберите тип личности MBTI и добавьте заметки о стиле, чтобы задать манеру речи агента',
    'agentEditor.personalization.mbtiLabel': 'Тип личности',
    'agentEditor.personalization.mbtiDesc': 'Выберите один из 16 типов или «Нет», чтобы сохранить стандартный стиль ответов',
    'agentEditor.personalization.noneLabel': 'Нет',
    'agentEditor.personalization.noneDesc': 'Без типа личности; стандартный стиль ответов',
    'agentEditor.personalization.dimensionsLabel': 'Склонности по осям',
    'agentEditor.personalization.styleLabel': 'Заметки о стиле',
    'agentEditor.personalization.styleDesc': 'Необязательно. Дополнительные требования к тону и формулировкам применяются вместе с выбранной личностью',
    'agentEditor.personalization.stylePlaceholder': 'напр. Непринуждённый тон, сложные понятия объяснять через аналогии',
    'agentEditor.personalization.takeTest': 'Пройти тест',
    'agentEditor.personalization.takeTestHint': 'Тест скоро будет доступен',
    'agentEditor.personalization.loadFailed': 'Не удалось загрузить типы личности',
    'agentEditor.personalization.unknownType': 'Сохранённый тип {code} отсутствует в каталоге',
  },
};

// Keys owned by a React-side source instead of the Vue locale tables.
const REACT_SIDE_SOURCES = [PAGER_VALUES, PERSONALIZATION_VALUES];
const reactSideSource = (key) => REACT_SIDE_SOURCES.find((source) => key in source['zh-CN']);

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
    provenance: 'agentEditor.im / agentEditor.embed / embedPublish blocks (integrations surface) + agentEditor.personalization block (Octop MBTI persona, React-only — script-owned values, see PERSONALIZATION_VALUES).',
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
      'agentEditor.personalization.title', 'agentEditor.personalization.desc',
      'agentEditor.personalization.mbtiLabel', 'agentEditor.personalization.mbtiDesc',
      'agentEditor.personalization.noneLabel', 'agentEditor.personalization.noneDesc',
      'agentEditor.personalization.dimensionsLabel',
      'agentEditor.personalization.styleLabel', 'agentEditor.personalization.styleDesc', 'agentEditor.personalization.stylePlaceholder',
      'agentEditor.personalization.takeTest', 'agentEditor.personalization.takeTestHint',
      'agentEditor.personalization.loadFailed', 'agentEditor.personalization.unknownType',
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
    const source = reactSideSource(key);
    if (source) {
      for (const locale of LOCALES) {
        if (source[locale][key] === undefined) problems.push('REACT-SIDE INCOMPLETE ' + locale + ': ' + key);
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
      const source = reactSideSource(key);
      const value = source ? source[locale][key] : vueMaps[locale][key];
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
