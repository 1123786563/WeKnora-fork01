import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { TaskOffice } from '@weknora/mobile-core';
import { createAuthorizedTaskEntry, probeNativeTaskCapabilities } from './task-entry.ts';

test('MobileTaskOffice_PublicSeam_RejectsCrossTenantAndRestoresSameTask', async () => {
  const opened: Array<{ taskId: string; runId: string }> = [];
  const detail = {
    hydrate: async () => ({ taskId: 'task-allowed', runId: 'run-1' }),
    view: () => undefined,
    updates: () => () => undefined,
    resync: async () => ({ taskId: 'task-allowed', runId: 'run-1' }),
    act: async () => ({ outcome: 'accepted' }),
    flushQueuedIntents: async () => undefined,
    close: () => undefined,
  };
  const office = {
    async tasks(input: { limit?: number }) {
      assert.deepEqual(input, { limit: 30 });
      return { items: [{ taskId: 'task-allowed', runId: 'run-1', title: 'Existing', runStatus: 'running', attention: 'none', updatedAt: 'now' }], duplicateRunIds: [] };
    },
    open(input: { taskId: string; runId: string }) { opened.push(input); return detail; },
  } as unknown as Pick<TaskOffice, 'open' | 'tasks'>;

  let access: { surface: 'deployment-login' | 'authorized'; hasScopeLease: boolean; office?: Pick<TaskOffice, 'open' | 'tasks'> } = { surface: 'deployment-login', hasScopeLease: false, office };
  const entry = createAuthorizedTaskEntry({ current: () => access });
  assert.throws(() => entry.openExisting({ taskId: 'task-foreign', runId: 'run-x' }), /authorized scope/i);
  await assert.rejects(entry.listExisting(), /authorized scope/i);
  assert.deepEqual(opened, [], 'unauthenticated/cross-tenant route must not enter Task Office');

  access = { surface: 'authorized', hasScopeLease: true, office };
  const page = await entry.listExisting();
  assert.equal(page.items[0]?.taskId, 'task-allowed');
  const handle = entry.openExisting({ taskId: 'task-allowed', runId: 'run-1' });
  assert.deepEqual(await handle.hydrate(), { taskId: 'task-allowed', runId: 'run-1' });
  assert.deepEqual(opened, [{ taskId: 'task-allowed', runId: 'run-1' }]);
  assert.equal('start' in entry, false, 'the entry exposes existing Task reads only');

  access = { surface: 'authorized', hasScopeLease: false, office };
  assert.throws(() => entry.openExisting({ taskId: 'task-allowed', runId: 'run-1' }), /authorized scope/i);
});

test('ExpoTaskOffice_NativeBoundary_ReportsCapabilityAdaptersExplicitly', () => {
  const result = probeNativeTaskCapabilities({
    navigation: true,
    fileSelection: false,
    fileDownload: true,
    systemShare: true,
    notifications: false,
    secureStorage: true,
  });
  assert.deepEqual(result.fileSelection, { status: 'unavailable', reason: 'native adapter is not installed' });
  assert.deepEqual(result.notifications, { status: 'unavailable', reason: 'native adapter is not installed' });
  assert.equal(result.navigation.status, 'available');
  assert.equal(result.fileDownload.status, 'available');
  assert.equal(result.systemShare.status, 'available');
  assert.equal(result.secureStorage.status, 'available');
});

test('ExpoTaskOffice_NativeBoundary_UsesReactNativeTDesignTokensAndInteractions', async () => {
  const source = await readFile(resolve(import.meta.dirname, 'TaskEntryScreen.tsx'), 'utf8');
  assert.match(source, /nativeTokens/);
  assert.match(source, /nativeTokens\.colors\[dark \? 'dark' : 'light'\]/);
  assert.match(source, /spacing\[.?20/);
  assert.match(source, /Pressable/);
  assert.match(source, /accessibilityRole/);
  assert.match(source, /onPress/);
});

test('ExpoTaskOffice_NativeBoundary_NoMobileWebComponentImports', async () => {
  const source = await readFile(resolve(import.meta.dirname, 'TaskEntryScreen.tsx'), 'utf8');
  assert.doesNotMatch(source, /from\s+['"][^'"]*(?:apps\/web|packages\/views|tdesign-mobile-react)[^'"]*['"]/i);
  assert.match(source, /from ['"]react-native['"]/);
});
