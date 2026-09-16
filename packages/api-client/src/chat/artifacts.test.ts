import assert from 'node:assert/strict';
import test from 'node:test';

import { createChatArtifactsApi } from './artifacts.ts';

test('lists artifact metadata without exposing the server storage path', async () => {
  const api = createChatArtifactsApi(
    async () => ({ success: true, data: [{ index: 0, file_name: '报告.pdf', file_type: 'application/pdf', file_size: 12, source_path: '/workspace/output/报告.pdf' }] }),
    async () => { throw new Error('unexpected binary request'); },
  );

  assert.deepEqual(await api.message('session/1', 'message/1'), [{
    index: 0,
    fileName: '报告.pdf',
    fileType: 'application/pdf',
    fileSize: 12,
  }]);
});

test('rejects an unsuccessful or malformed artifact list envelope', async () => {
  const api = createChatArtifactsApi(
    async () => ({ success: false, data: [] }),
    async () => { throw new Error('unexpected binary request'); },
  );

  await assert.rejects(() => api.session('session-1'), /Invalid artifact list/);
});

test('downloads only through the authenticated message artifact route', async () => {
  let path = '';
  const api = createChatArtifactsApi(
    async () => ({ success: true, data: [] }),
    async (input) => {
      path = input.path;
      return { body: new ArrayBuffer(2), contentType: 'application/pdf', headers: {} };
    },
  );

  const response = await api.download('session/1', 'message/1', 0);
  assert.equal(path, '/api/v1/sessions/session%2F1/messages/message%2F1/artifacts/0/download');
  assert.equal(response.contentType, 'application/pdf');
});
