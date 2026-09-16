import assert from 'node:assert/strict';
import test from 'node:test';

import {
  computeSupportedFileTypes,
  computeUnsupportedFileTypes,
  dateRangeToTimeParams,
  documentsKBListPath,
  documentsKBDetailPath,
  documentsKBSettingsPath,
  isFilteringDocuments,
} from './page-chrome.ts';

// Vue parity source: frontend/src/views/knowledge/KnowledgeBase.vue:186-233
// (supportedFileTypes / unsupportedFileTypes computeds) — a file type is
// uploadable when its explicit parser_engine_rules engine is available, or,
// absent an explicit rule, when its declaring engine is available.

const engines = [
  { Name: 'basic', FileTypes: ['pdf', 'docx', 'txt', 'md'], Available: true },
  { Name: 'ocr', FileTypes: ['pdf', 'png'], Available: true },
  { Name: 'office', FileTypes: ['docm', 'odp', 'rtf', 'xlsm'], Available: false },
];

test('computeSupportedFileTypes: available engines declare their file types', () => {
  const supported = computeSupportedFileTypes(engines, []);
  assert.equal(supported.has('pdf'), true);
  assert.equal(supported.has('docx'), true);
  assert.equal(supported.has('txt'), true);
  assert.equal(supported.has('png'), true);
  assert.equal(supported.has('docm'), false, 'unavailable engine types stay unsupported');
  assert.equal(supported.has('odp'), false);
});

test('computeSupportedFileTypes: an explicit rule can lift a type onto an available engine', () => {
  const supported = computeSupportedFileTypes(engines, [{ file_types: ['docm'], engine: 'basic' }]);
  assert.equal(supported.has('docm'), true);
  // ...but only when the rule targets an engine that is actually available.
  const dead = computeSupportedFileTypes(engines, [{ file_types: ['pdf'], engine: 'office' }]);
  assert.equal(dead.has('pdf'), false);
});

test('computeSupportedFileTypes: no engines means nothing is supported', () => {
  assert.equal(computeSupportedFileTypes([], []).size, 0);
});

test('computeUnsupportedFileTypes: declared-but-unusable types, sorted (live KB set)', () => {
  const unsupported = computeUnsupportedFileTypes(engines, []);
  assert.deepEqual(unsupported, ['docm', 'odp', 'rtf', 'xlsm']);
});

test('computeUnsupportedFileTypes: empty when engines are absent or everything resolves', () => {
  assert.deepEqual(computeUnsupportedFileTypes([], []), []);
  assert.deepEqual(computeUnsupportedFileTypes([engines[0]!], []), []);
});

test('date range maps to the backend list params like Vue filterParams', () => {
  assert.deepEqual(dateRangeToTimeParams(['2026-09-13', '2026-09-14']), {
    start_time: '2026-09-13 00:00:00',
    end_time: '2026-09-14 23:59:59',
  });
  assert.deepEqual(dateRangeToTimeParams(['2026-09-13', undefined]), { start_time: '2026-09-13 00:00:00' });
  assert.deepEqual(dateRangeToTimeParams([undefined, '2026-09-14']), { end_time: '2026-09-14 23:59:59' });
  assert.deepEqual(dateRangeToTimeParams(undefined), {});
  assert.deepEqual(dateRangeToTimeParams(['', '']), {});
});

test('breadcrumb destinations mirror Vue navigation', () => {
  assert.equal(documentsKBListPath, '/platform/knowledge-bases');
  assert.equal(documentsKBDetailPath('9727d104-cde4-4d03-879f-d7e3897b69a3'), '/knowledgeBase/9727d104-cde4-4d03-879f-d7e3897b69a3');
  assert.equal(documentsKBSettingsPath('9727d104'), '/knowledgeBase/9727d104/settings');
});

test('isFilteringDocuments: search descends the folder subtree, browsing does not', () => {
  assert.equal(isFilteringDocuments({}), false);
  assert.equal(isFilteringDocuments({ keyword: '   ' }), false, 'whitespace is not a search');
  assert.equal(isFilteringDocuments({ timeRange: ['', ''] }), false);
  assert.equal(isFilteringDocuments({ keyword: 'spec' }), true);
  assert.equal(isFilteringDocuments({ tagIds: ['t1'] }), true);
  assert.equal(isFilteringDocuments({ fileType: 'pdf' }), true);
  assert.equal(isFilteringDocuments({ parseStatus: 'failed' }), true);
  assert.equal(isFilteringDocuments({ source: 'web' }), true);
  assert.equal(isFilteringDocuments({ timeRange: ['2026-01-01', ''] }), true);
});
