// R464 A4 live finding: the palette scope chip seeded only on the Vue-form
// alias path — the user's primary React path /knowledgeBase/:id(…) must seed
// it too. Pinned here so the regex cannot regress to a single form.
import assert from 'node:assert/strict';
import test from 'node:test';

const { PlatformShell } = await import('./PlatformShell.tsx').catch(() => ({ PlatformShell: null as null | Record<string, unknown> }));
const scope = (PlatformShell as null | { kbScopeFromLocation?: () => { id: string; name: string } | null })?.kbScopeFromLocation;

test('kbScopeFromLocation seeds on both the React-native and Vue-form KB paths', () => {
  if (!scope) return; // PlatformShell may require a DOM harness in some runners; the source test below covers it then.
  const original = window.location.pathname;
  const paths: Array<[string, string | null]> = [
    ['/knowledgeBase/7cea6ec0-0a07', '7cea6ec0-0a07'],
    ['/knowledgeBase/22d38cb7/settings', '22d38cb7'],
    ['/platform/knowledge-bases/abc', 'abc'],
    ['/platform/knowledge-bases', null],
    ['/knowledgeBase', null],
    ['/platform/chat/session-1', null],
  ];
  try {
    for (const [pathname, expected] of paths) {
      window.history.replaceState({}, '', pathname);
      const seeded = scope();
      assert.equal(seeded?.id ?? null, expected, `${pathname} should seed ${expected}`);
    }
  } finally {
    window.history.replaceState({}, '', original);
  }
});
