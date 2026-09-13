import assert from 'node:assert/strict';
import test from 'node:test';

import { buildCLIConnectCommand } from './cli.ts';

// Ported from frontend/src/views/integrations/cliIntegration.ts: the CLI
// appends /api/v1 itself, so the connect command keeps reverse-proxy path
// prefixes but strips the api suffix, and quotes the host for POSIX shells.

test('connect command strips the api suffix but keeps proxy path prefixes', () => {
  assert.match(
    buildCLIConnectCommand('https://example.com/api/v1', 'https://example.com'),
    /weknora profile add weknora --host 'https:\/\/example\.com' --use/,
  );
  assert.match(
    buildCLIConnectCommand('https://proxy.example.com/weknora/api/v1', 'https://proxy.example.com'),
    /host 'https:\/\/proxy\.example\.com\/weknora'/,
  );
});

test('connect command falls back to the placeholder host when no base url resolves', () => {
  assert.match(buildCLIConnectCommand('', ''), /host 'https:\/\/your-server\.com'/);
  // Same as the Vue baseline: relative paths resolve against the origin.
  assert.match(buildCLIConnectCommand('', 'http://localhost:5181'), /host 'http:\/\/localhost:5181'/);
  assert.match(buildCLIConnectCommand('not a url', ''), /host 'https:\/\/your-server\.com'/);
});

test('connect command quotes the host and ends with an auth login step', () => {
  const command = buildCLIConnectCommand('http://127.0.0.1:8080/api/v1', 'http://127.0.0.1:5181');
  assert.match(command, /--host 'http:\/\/127\.0\.0\.1:8080'/);
  assert.match(command, /&&\nweknora auth login$/);
});
