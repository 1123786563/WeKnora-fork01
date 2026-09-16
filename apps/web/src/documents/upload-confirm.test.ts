import test from 'node:test';
import assert from 'node:assert/strict';
import { getUploadConfirmButtonOrder, getUploadConfirmDefaultSection, getUploadConfirmSections, getUploadConfirmSourceItems, isUploadConfirmDismissible, requestUploadConfirmClose, validateUploadConfirm, type UploadConfirmSection } from './upload-confirm.ts';

const validBase = {
  mode: 'file' as const,
  files: [{ name: 'notes.md' }],
  urls: [],
  multimodalEnabled: false,
  multimodalModelId: '',
  asrEnabled: false,
  asrModelId: '',
};

test('routes an image validation failure to multimodal section', () => {
  const result = validateUploadConfirm({ ...validBase, files: [{ name: 'diagram.png' }] });
  assert.equal(result.valid, false);
  assert.deepEqual(result.issues, ['multimodal']);
  assert.equal(result.firstIssueSection, 'multimodal');
  assert.equal(getUploadConfirmDefaultSection({ ...validBase, files: [{ name: 'diagram.png' }] }), 'multimodal');
});

test('keeps parser as the initial section for reparse mode', () => {
  assert.equal(getUploadConfirmDefaultSection({ ...validBase, mode: 'reparse', files: [] }), 'parser');
});

test('does not treat an enabled media option without matching media as a blocking issue', () => {
  const result = validateUploadConfirm({ ...validBase, multimodalEnabled: true });
  assert.equal(result.valid, true);
  assert.deepEqual(result.issues, [] satisfies UploadConfirmSection[]);
});

test('rejects empty file and manual submissions', () => {
  assert.equal(validateUploadConfirm({ ...validBase, files: [] }).valid, false);
  assert.equal(validateUploadConfirm({ ...validBase, mode: 'manual', files: [], manualContent: '  ' }).valid, false);
});

test('keeps the Vue section set and omits tags while reparsing', () => {
  assert.deepEqual(getUploadConfirmSections('file'), ['tags', 'parser', 'chunking', 'multimodal', 'asr', 'question']);
  assert.deepEqual(getUploadConfirmSections('manual'), ['tags', 'parser', 'chunking', 'multimodal', 'asr', 'question']);
  assert.deepEqual(getUploadConfirmSections('reparse'), ['parser', 'chunking', 'multimodal', 'asr', 'question']);
});

test('reports every media setup issue in Vue navigation order', () => {
  const result = validateUploadConfirm({ ...validBase, files: [{ name: 'scan.png' }, { name: 'meeting.m4a' }] });
  assert.deepEqual(result.issues, ['multimodal', 'asr']);
  assert.equal(result.firstIssueSection, 'multimodal');
});

test('detects media in URL paths without treating query strings as extensions', () => {
  const result = validateUploadConfirm({ ...validBase, files: [], urls: ['https://example.test/recording.M4A?download=1'] });
  assert.deepEqual(result.issues, ['asr']);
  assert.equal(result.valid, false);
});

test('keeps the dialog dismissible until submission starts', () => {
  assert.equal(isUploadConfirmDismissible(false), true);
  assert.equal(isUploadConfirmDismissible(true), false);
});

test('keeps Vue source preview order and exposes removable source items', () => {
  assert.deepEqual(getUploadConfirmSourceItems({
    files: [{ name: 'notes.md' }, { name: 'diagram.png' }],
    urls: ['https://example.test/doc'],
  }), [
    { kind: 'url', index: 0, label: 'https://example.test/doc', meta: 'URL' },
    { kind: 'file', index: 0, label: 'notes.md', meta: 'File' },
    { kind: 'file', index: 1, label: 'diagram.png', meta: 'File' },
  ]);
});

test('keeps cancel before confirm in every Vue dialog mode', () => {
  assert.deepEqual(getUploadConfirmButtonOrder(), ['cancel', 'confirm']);
});

test('keeps every close entry point closed while submission is in flight', () => {
  let cancellations = 0;
  requestUploadConfirmClose(true, () => { cancellations += 1; });
  assert.equal(cancellations, 0);
  requestUploadConfirmClose(false, () => { cancellations += 1; });
  assert.equal(cancellations, 1);
  assert.equal(isUploadConfirmDismissible(false), true);
  assert.equal(isUploadConfirmDismissible(true), false);
});
