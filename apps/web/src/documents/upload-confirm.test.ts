import test from 'node:test';
import assert from 'node:assert/strict';
import { getUploadConfirmDefaultSection, validateUploadConfirm, type UploadConfirmSection } from './upload-confirm.ts';

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
  const result = validateUploadConfirm({
    ...validBase,
    files: [{ name: 'diagram.png' }],
    multimodalEnabled: false,
  });

  assert.equal(result.valid, false);
  assert.deepEqual(result.issues, ['multimodal']);
  assert.equal(result.firstIssueSection, 'multimodal');
  assert.equal(getUploadConfirmDefaultSection({ ...validBase, files: [{ name: 'diagram.png' }] }), 'multimodal');
});

test('keeps parser as the initial section for reparse mode', () => {
  assert.equal(getUploadConfirmDefaultSection({ ...validBase, mode: 'reparse', files: [] }), 'parser');
});

test('does not treat an enabled media option without matching media as a blocking issue', () => {
  const result = validateUploadConfirm({
    ...validBase,
    multimodalEnabled: true,
    multimodalModelId: '',
  });

  assert.equal(result.valid, true);
  assert.deepEqual(result.issues, [] satisfies UploadConfirmSection[]);
});

test('rejects empty file and manual submissions', () => {
  assert.equal(validateUploadConfirm({ ...validBase, files: [] }).valid, false);
  assert.equal(validateUploadConfirm({ ...validBase, mode: 'manual', files: [], manualContent: '  ' }).valid, false);
});
