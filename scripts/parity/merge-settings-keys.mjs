#!/usr/bin/env node
// Resolve real dotted key paths from nested Vue locale modules, then merge into settings.ts.
import { readFileSync, writeFileSync } from 'node:fs';

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

// leaf-suffix resolver: find full dotted path ending with the semantic suffix
function resolve(suffix) {
  const paths = Object.keys(vueMaps['zh-CN']).filter((p) => p === suffix || p.endsWith('.' + suffix));
  return paths.length === 1 ? paths[0] : paths.length === 0 ? null : 'AMBIGUOUS:' + paths.join('|');
}

// Prefix-based wholesale extraction: every Vue key under these prefixes that is
// missing from the shared bundle gets migrated (settings/model/sandbox/skills surfaces).
const PREFIXES = ['model.', 'modelSettings.', 'settings.sandbox.', 'settings.skills.', 'settings.weknoraCloud.', 'uploadConfirm.', 'knowledgeStages.'];
const EXTRA = ['common.remove', 'common.copied', 'common.on', 'common.confirmDelete', 'common.yes', 'common.no'];
// MCP settings dialog/metadata copy (R024/R043-R046/N016): every key the Vue
// drawer and its tool directory reference, so the React port stays byte-exact.
const MCP = [
  'mcpMetadata.description', 'mcpMetadata.parameters', 'mcpMetadata.fullSchema', 'mcpMetadata.fetch', 'mcpMetadata.refresh',
  'mcpServiceDialog.oauthAuthorization', 'mcpServiceDialog.oauthAuthorized', 'mcpServiceDialog.oauthUnauthorized',
];
const prefixKeys = Object.keys(vueMaps['zh-CN']).filter((p) => PREFIXES.some((pre) => p.startsWith(pre)));

const SEMANTIC = [
  ...prefixKeys,
  ...EXTRA,
  ...MCP,
  // uploadConfirm (top-level object)
  ...['title','titleManual','titleReparse','parseConfig','configNav','navParserDefault','navParserCustomized','moreOptions','summaryParentChildShort','summaryParserForceScanned','summaryQuestionCountValue','navChunkingSummary','statusOn','statusOff','notSet','summaryNoTags','summaryTagsCount','confirm','cancel','tabTags','tagsDescription','tagsPlaceholder','tagsEmpty','tagsLoadFailed','noItems','urlItemLabel','urlAdded','urlDuplicate','statusNeedsSetup','multimodalSetupHint','asrSetupHint','vlmModelRequired','asrModelRequired','vlmModelSelectRequired','asrModelSelectRequired','continueAdd','destinationLabel','destinationChange','filesAdded','filesAllDuplicate','confirmManual','confirmReparse','reparseSource','reparseHint','manualCharCount'].map((k) => 'uploadConfirm.' + k),
  'uploadConfirm.pdfForceScanned.label', 'uploadConfirm.pdfForceScanned.description',
  'common.remove', 'common.copied', 'common.on', 'common.confirmDelete',
  'settings.sandbox.backends.docker', 'settings.sandbox.backends.cube', 'settings.sandbox.backends.e2b',
  'settings.sandbox.skillUploadAccepted', 'settings.sandbox.skillDisableHint',
  'settings.sandbox.skillRemoveInProgress', 'settings.sandbox.skillRemoveWaiting', 'settings.sandbox.skillRemoveDone',
  'settings.sandbox.skillRemoveStage.accepted', 'settings.sandbox.skillRemoveStage.sandbox_ready', 'settings.sandbox.skillRemoveStage.removed', 'settings.sandbox.skillRemoveStage.done', 'settings.sandbox.skillRemoveStage.failed',
  'settings.sandbox.skillTranscript', 'settings.sandbox.skillTranscriptLiveHint', 'settings.sandbox.skillTranscriptTitle',
  'settings.sandbox.skillLoadFailed', 'settings.sandbox.skillToggleFailed', 'settings.sandbox.skillDeleteAccepted',
  'settings.sandbox.skillRetry', 'settings.sandbox.skillRetryHint', 'settings.sandbox.skillRetryAccepted', 'settings.sandbox.skillRetryFailed',
  'settings.sandbox.skillStop', 'settings.sandbox.skillStopHint', 'settings.sandbox.skillStopAccepted', 'settings.sandbox.skillStopFailed',
  'settings.sandbox.skillFilesTitle', 'settings.sandbox.skillFilesEmpty', 'settings.sandbox.skillFilesLoadFailed', 'settings.sandbox.skillFilesFileLoadFailed', 'settings.sandbox.skillFilesBinary', 'settings.sandbox.skillFilesTruncated', 'settings.sandbox.skillFilesSelectHint', 'settings.sandbox.skillFilesPreview', 'settings.sandbox.skillFilesSource',
  'settings.sandbox.skillEnabled', 'settings.sandbox.skillDisabled',
  'settings.sandbox.skillEnv.toggle', 'settings.sandbox.skillEnv.workspaceTitle', 'settings.sandbox.skillEnv.workspaceHint', 'settings.sandbox.skillEnv.required', 'settings.sandbox.skillEnv.isSet', 'settings.sandbox.skillEnv.notSet', 'settings.sandbox.skillEnv.placeholderSet', 'settings.sandbox.skillEnv.placeholderUnset', 'settings.sandbox.skillEnv.save', 'settings.sandbox.skillEnv.saveSuccess', 'settings.sandbox.skillEnv.saveFailed', 'settings.sandbox.skillEnv.clear', 'settings.sandbox.skillEnv.clearConfirm', 'settings.sandbox.skillEnv.clearSuccess', 'settings.sandbox.skillEnv.valueTooLong',
  'settings.skills.manageEnable', 'settings.skills.manageUninstall', 'settings.skills.manageUninstallConfirm',
];

const resolved = {};
const problems = [];
for (const sem of SEMANTIC) {
  const path = vueMaps['zh-CN'][sem] !== undefined ? sem : resolve(sem);
  if (path === null) { problems.push('NOT FOUND: ' + sem); continue; }
  if (typeof path === 'string' && path.startsWith('AMBIGUOUS:')) { problems.push(path); continue; }
  const missing = LOCALES.filter((l) => vueMaps[l][path] === undefined);
  if (missing.length) { problems.push('MISSING IN ' + missing.join(',') + ': ' + path); continue; }
  resolved[path] = Object.fromEntries(LOCALES.map((l) => [l, vueMaps[l][path]]));
}
if (problems.length) console.log('PROBLEMS:\n' + problems.join('\n'));
console.log('resolved keys:', Object.keys(resolved).length);

// merge — each locale map is one physical line:   "xx-XX": {...},
const path = 'packages/i18n/src/settings.ts';
const lines = readFileSync(path, 'utf8').split('\n');
let added = 0;
for (let i = 0; i < lines.length; i++) {
  // locale maps open on their own line:   "xx-XX": {...   (closing brace sits on the NEXT line)
  const m = /^  "([a-zA-Z-]+)": \{(.*)$/.exec(lines[i]);
  if (!m) continue;
  const locale = m[1];
  if (!LOCALES.includes(locale)) continue;
  let body = m[2].trim();
  if (body.endsWith(',')) body = body.slice(0, -1);
  const obj = JSON.parse('{' + body + '}');
  for (const [key, perLocale] of Object.entries(resolved)) {
    if (!(key in obj)) { obj[key] = perLocale[locale]; added++; }
  }
  const entries = Object.entries(obj).map(([k, v]) => JSON.stringify(k) + ':' + JSON.stringify(v));
  lines[i] = '  ' + JSON.stringify(locale) + ': {' + entries.join(',');
}
writeFileSync(path, lines.join('\n'));
console.log('added entries:', added, '(keys x locales)');
