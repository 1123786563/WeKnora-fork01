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
  paletteShortcutDigit,
  pushRecentQuery,
  RECENT_QUERIES_LIMIT,
  recentQueriesStorageKey,
  shortcutDigitFor,
  visibleCommands,
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

// ─── Palette-scoped ⌘1-9 (N003 deferred item, Vue GlobalCommandPalette.vue:508-520) ───

test('paletteShortcutDigit mirrors the Vue dialog guard: Cmd/Ctrl + bare digit 1-9', () => {
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: false, key: '1' }), 1);
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: false, key: '9' }), 9);
  assert.equal(paletteShortcutDigit({ metaKey: false, ctrlKey: true, key: '5' }), 5);
  // Vue checks (metaKey||ctrlKey) then string-ranges e.key; digits take
  // precedence over ⌘Enter because a digit key can never be Enter.
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: true, key: '3' }), 3);
});

test('paletteShortcutDigit rejects non-digit or unmodified keys like Vue', () => {
  // Plain typing in the input must never jump rows.
  assert.equal(paletteShortcutDigit({ metaKey: false, ctrlKey: false, key: '1' }), undefined);
  // Shift+digit produces punctuation on US layouts (e.key = '!'), so Vue's
  // e.key range check naturally excludes it; Alt chords behave likewise.
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: false, key: '!' }), undefined);
  // Non-digit keys with ⌘ held (⌘K toggle, ⌘↵, arrows…).
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: false, key: 'k' }), undefined);
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: false, key: 'Enter' }), undefined);
  // 0 is outside Vue's '1'..'9' range.
  assert.equal(paletteShortcutDigit({ metaKey: true, ctrlKey: false, key: '0' }), undefined);
});

test('decideGlobalShortcutAction stays digit-free: ⌘1-9 never act app-wide', () => {
  // Regression pin (Round N+3 ruling): the rejected slice bound ⌘1 globally.
  // The window-level handler must keep returning 'none' for digits — they
  // are handled only inside the open palette dialog.
  assert.equal(decideGlobalShortcutAction({ metaKey: true, ctrlKey: false, key: '1' }, { open: false, isEditingTarget: false }), 'none');
  assert.equal(decideGlobalShortcutAction({ metaKey: true, ctrlKey: false, key: '1' }, { open: true, isEditingTarget: false }), 'none');
  assert.equal(decideGlobalShortcutAction({ metaKey: false, ctrlKey: true, key: '9' }, { open: true, isEditingTarget: true }), 'none');
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

test('visibleCommands hides "Open agents" without the agents capability', () => {
  const visible = visibleCommands(COMMANDS, { canOpenAgents: false, canOpenOrganizations: true });
  assert.deepEqual(visible.map((c) => c.id), ['new-chat', 'open-kb-list', 'open-organizations', 'open-settings']);
});

test('visibleCommands hides "Open shared spaces" without admin+organizations access', () => {
  const visible = visibleCommands(COMMANDS, { canOpenAgents: true, canOpenOrganizations: false });
  assert.deepEqual(visible.map((c) => c.id), ['new-chat', 'open-kb-list', 'open-agents', 'open-settings']);
});

test('visibleCommands keeps every command when both capabilities are granted', () => {
  assert.equal(visibleCommands(COMMANDS, { canOpenAgents: true, canOpenOrganizations: true }).length, 5);
});
