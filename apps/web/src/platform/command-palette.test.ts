import assert from 'node:assert/strict';
import test from 'node:test';
import {
  clearRecentQueries,
  COMMANDS,
  consumeCmdkParam,
  decideGlobalShortcutAction,
  filterCommands,
  loadRecentQueries,
  nextSelectedIndex,
  pushRecentQuery,
  RECENT_QUERIES_LIMIT,
  recentQueriesStorageKey,
  shortcutDigitFor,
  type KeyValueStorage,
} from './command-palette.ts';

const t = (key: string): string => {
  const labels: Record<string, string> = {
    'commandPalette.quick.newChat': 'New conversation',
    'commandPalette.quick.knowledgeBases': 'Open knowledge bases',
    'commandPalette.quick.agents': 'Open agents',
    'commandPalette.quick.organizations': 'Open shared spaces',
    'commandPalette.quick.settings': 'Open settings',
  };
  return labels[key] ?? key;
};

test('the command catalogue mirrors Vue quick actions (minus the unported product tour)', () => {
  assert.equal(COMMANDS.length, 5);
  assert.deepEqual(COMMANDS.map((c) => c.id), [
    'new-chat', 'open-kb-list', 'open-agents', 'open-organizations', 'open-settings',
  ]);
  assert.deepEqual(COMMANDS.map((c) => c.path), [
    '/platform/creatChat', '/platform/knowledge-bases', '/platform/agents', '/platform/organizations', '/platform/settings',
  ]);
});

test('filterCommands returns everything for a blank query', () => {
  assert.equal(filterCommands(COMMANDS, '', t).length, 5);
  assert.equal(filterCommands(COMMANDS, '   ', t).length, 5);
});

test('filterCommands matches localized label text case-insensitively', () => {
  const matches = filterCommands(COMMANDS, 'KNOWLEDGE', t);
  assert.deepEqual(matches.map((c) => c.id), ['open-kb-list']);
});

test('filterCommands matches non-English keywords not present in the label', () => {
  const matches = filterCommands(COMMANDS, '智能体', t);
  assert.deepEqual(matches.map((c) => c.id), ['open-agents']);
});

test('filterCommands returns no results for an unmatched query', () => {
  assert.deepEqual(filterCommands(COMMANDS, 'zzz-nope', t), []);
});

function createMemoryStorage(): KeyValueStorage {
  const store = new Map<string, string>();
  return {
    getItem: (key) => store.get(key) ?? null,
    setItem: (key, value) => { store.set(key, value); },
    removeItem: (key) => { store.delete(key); },
  };
}

test('recentQueriesStorageKey scopes by user and tenant, falling back to anon/none', () => {
  assert.equal(recentQueriesStorageKey('user-1', 'tenant-a'), 'weknora_cmdk_recent:user-1:tenant-a');
  assert.equal(recentQueriesStorageKey(null, null), 'weknora_cmdk_recent:anon:none');
  assert.equal(recentQueriesStorageKey(undefined, 7), 'weknora_cmdk_recent:anon:7');
});

test('pushRecentQuery dedupes, orders most-recent-first, and caps at the limit', () => {
  const storage = createMemoryStorage();
  const key = recentQueriesStorageKey('user-1', 'tenant-a');
  pushRecentQuery(storage, key, 'alpha');
  pushRecentQuery(storage, key, 'beta');
  pushRecentQuery(storage, key, 'alpha');
  const result = pushRecentQuery(storage, key, 'gamma');
  assert.deepEqual(result, ['gamma', 'alpha', 'beta']);
  assert.deepEqual(loadRecentQueries(storage, key), ['gamma', 'alpha', 'beta']);
});

test('pushRecentQuery ignores blank input and enforces RECENT_QUERIES_LIMIT', () => {
  const storage = createMemoryStorage();
  const key = recentQueriesStorageKey('user-1', 'tenant-a');
  assert.deepEqual(pushRecentQuery(storage, key, '   '), []);
  for (const q of ['q1', 'q2', 'q3', 'q4', 'q5']) pushRecentQuery(storage, key, q);
  const result = loadRecentQueries(storage, key);
  assert.equal(result.length, RECENT_QUERIES_LIMIT);
  assert.deepEqual(result, ['q5', 'q4', 'q3', 'q2']);
});

test('clearRecentQueries removes the stored entry', () => {
  const storage = createMemoryStorage();
  const key = recentQueriesStorageKey('user-1', 'tenant-a');
  pushRecentQuery(storage, key, 'alpha');
  clearRecentQueries(storage, key);
  assert.deepEqual(loadRecentQueries(storage, key), []);
});

test('loadRecentQueries tolerates malformed storage instead of throwing', () => {
  const storage = createMemoryStorage();
  const key = recentQueriesStorageKey('user-1', 'tenant-a');
  storage.setItem(key, 'not-json');
  assert.deepEqual(loadRecentQueries(storage, key), []);
});

test('nextSelectedIndex wraps forward and backward', () => {
  assert.equal(nextSelectedIndex(0, 1, 3), 1);
  assert.equal(nextSelectedIndex(2, 1, 3), 0);
  assert.equal(nextSelectedIndex(0, -1, 3), 2);
  assert.equal(nextSelectedIndex(0, -1, 0), -1);
});

test('shortcutDigitFor exposes ⌘1-9 only for the first nine flat items', () => {
  assert.equal(shortcutDigitFor(0), 1);
  assert.equal(shortcutDigitFor(8), 9);
  assert.equal(shortcutDigitFor(9), undefined);
  assert.equal(shortcutDigitFor(-1), undefined);
});

test('decideGlobalShortcutAction toggles on Cmd/Ctrl+K even while editing', () => {
  assert.equal(decideGlobalShortcutAction({ metaKey: true, ctrlKey: false, key: 'k' }, { open: false, isEditingTarget: true }), 'toggle');
  assert.equal(decideGlobalShortcutAction({ metaKey: false, ctrlKey: true, key: 'K' }, { open: true, isEditingTarget: false }), 'toggle');
});

test('decideGlobalShortcutAction opens on bare "/" only when closed and not editing', () => {
  assert.equal(decideGlobalShortcutAction({ metaKey: false, ctrlKey: false, key: '/' }, { open: false, isEditingTarget: false }), 'open');
  assert.equal(decideGlobalShortcutAction({ metaKey: false, ctrlKey: false, key: '/' }, { open: false, isEditingTarget: true }), 'none');
  assert.equal(decideGlobalShortcutAction({ metaKey: false, ctrlKey: false, key: '/' }, { open: true, isEditingTarget: false }), 'none');
});

test('decideGlobalShortcutAction ignores unrelated keys', () => {
  assert.equal(decideGlobalShortcutAction({ metaKey: false, ctrlKey: false, key: 'a' }, { open: false, isEditingTarget: false }), 'none');
});

test('consumeCmdkParam extracts the query and strips it from the search string', () => {
  assert.deepEqual(consumeCmdkParam('?cmdk=hello'), { query: 'hello', remainingSearch: '' });
  assert.deepEqual(consumeCmdkParam('?cmdk=hello&scope=mine'), { query: 'hello', remainingSearch: '?scope=mine' });
  assert.deepEqual(consumeCmdkParam('?cmdk='), { query: '', remainingSearch: '' });
});

test('consumeCmdkParam is a no-op when there is no cmdk param', () => {
  assert.deepEqual(consumeCmdkParam('?scope=mine'), { query: null, remainingSearch: '?scope=mine' });
  assert.deepEqual(consumeCmdkParam(''), { query: null, remainingSearch: '' });
});
