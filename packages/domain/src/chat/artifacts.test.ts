import assert from 'node:assert/strict';
import test from 'node:test';

import {
  artifactDownloadPath,
  isArtifactExpired,
  normalizeArtifactMetadata,
  resolveArtifactReference,
  safeArtifactFileName,
} from './artifacts.ts';

const handle = 'resource://abcdefghijklmnopqrstuv';

test('keeps a public resource handle without exposing provider or sandbox paths', () => {
  const artifact = normalizeArtifactMetadata({
    index: 2,
    handle,
    url: 'local://tenant/private/report.pdf',
    source_path: '/workspace/output/private/report.pdf',
    file_name: 'report.pdf',
    file_type: '.pdf',
    file_size: 42,
    version: 'v3',
    expires_at: '2026-09-11T00:00:00Z',
  });

  assert.deepEqual(artifact, {
    index: 2,
    handle,
    fileName: 'report.pdf',
    fileType: '.pdf',
    fileSize: 42,
    version: 'v3',
    expiresAt: '2026-09-11T00:00:00.000Z',
  });
  assert.equal('url' in (artifact ?? {}), false);
  assert.equal('sourcePath' in (artifact ?? {}), false);
});

test('keeps the public name while preventing path traversal and control characters in local sinks', () => {
  assert.equal(safeArtifactFileName('../private\\report\u0000.pdf'), '.._private_report.pdf');
  assert.equal(safeArtifactFileName(' '), 'artifact');
});

test('rejects internal URLs as handles and resolves only public handles or sandbox names', () => {
  const artifact = normalizeArtifactMetadata({
    index: 0,
    url: 's3://private-bucket/report.csv',
    file_name: 'report.csv',
  });
  assert.deepEqual(artifact, { index: 0, fileName: 'report.csv' });
  assert.equal(resolveArtifactReference('s3://private-bucket/report.csv', [artifact!]), null);
  assert.equal(resolveArtifactReference('sandbox:report.csv', [artifact!])?.index, 0);

  const handled = normalizeArtifactMetadata({ index: 1, handle, file_name: 'chart.png' });
  assert.equal(resolveArtifactReference(handle, [handled!])?.fileName, 'chart.png');
});

test('builds only the authenticated index route and evaluates expiration deterministically', () => {
  assert.equal(
    artifactDownloadPath('session / 1', 'message/2', 3),
    '/api/v1/sessions/session%20%2F%201/messages/message%2F2/artifacts/3/download',
  );
  assert.throws(() => artifactDownloadPath('session', 'message', -1), /artifact index/);

  const artifact = normalizeArtifactMetadata({
    index: 0,
    file_name: 'expired.txt',
    expires_at: '2026-09-10T09:00:00Z',
  });
  assert.equal(isArtifactExpired(artifact!, new Date('2026-09-10T09:00:00Z')), true);
  assert.equal(isArtifactExpired({ index: 1, fileName: 'durable.txt' }), false);
});
