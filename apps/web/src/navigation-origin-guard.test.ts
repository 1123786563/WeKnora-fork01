// R447-A3 sentinel: the React web app must never navigate the top-level
// browsing context cross-origin from first-party source. During the R446/R447
// dual-frontend alignment (React dev :5181, Vue reference dev :5180) a
// cross-source jump was observed from the settings page; the app-side audit
// (apps/web/src + shared packages) found every top-level navigation is a
// same-origin relative path — the jump itself came from an external browser
// driver sharing the tab. This guard keeps that app-side invariant true: any
// future hardcoded absolute-URL top-level navigation (e.g. a "fall back to the
// other frontend" idle redirect) turns this test red at CI time.
//
// Scope mirrors the R447 write boundary: externally-owned dirs (documents,
// knowledge, knowledge-settings, platform, embed) are audited read-only and
// reported here but cannot be modified by general agents; they are asserted
// separately so a violation inside them is still surfaced.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const WEB_SRC = new URL('.', import.meta.url).pathname;

// Directories owned by other workstreams (read-only for this guard). Still
// scanned; failures are reported as "needs owner handoff" notes.
const EXTERNALLY_OWNED = new Set(['documents', 'knowledge', 'knowledge-settings', 'platform', 'embed']);

const NAVIGATION_PATTERNS: Array<{ label: string; re: RegExp }> = [
  // window.location.assign('https://...') / location.href = 'https://...'
  { label: 'location.assign', re: /location\s*\.\s*assign\s*\(\s*(['"`])\s*(https?:)?\/\// },
  { label: 'location.replace', re: /location\s*\.\s*replace\s*\(\s*(['"`])\s*(https?:)?\/\// },
  { label: 'location.href assignment', re: /location\s*\.\s*href\s*=\s*(['"`])\s*(https?:)?\/\// },
  // window.open('https://...', '_self'|'_top') navigates the same tab.
  { label: 'window.open self', re: /window\s*\.\s*open\s*\(\s*(['"`])[^'"`]*['"`]\s*,\s*(['"`])(_self|_top)\2/ },
];

function listFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const s = statSync(full);
    if (s.isDirectory()) {
      if (entry === 'node_modules' || entry.startsWith('.')) continue;
      out.push(...listFiles(full));
    } else if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry)) {
      out.push(full);
    }
  }
  return out;
}

test('no hardcoded cross-origin top-level navigation in first-party web source', () => {
  const violations: string[] = [];
  const externalViolations: string[] = [];
  for (const file of listFiles(WEB_SRC)) {
    const rel = relative(WEB_SRC, file);
    const topLevel = rel.split('/')[0]!;
    const source = readFileSync(file, 'utf8');
    const lines = source.split('\n');
    for (const { label, re } of NAVIGATION_PATTERNS) {
      lines.forEach((line, index) => {
        if (!re.test(line)) return;
        const entry = `${rel}:${index + 1}: ${label}: ${line.trim().slice(0, 160)}`;
        // The auth OIDC start legitimately receives an absolute authorization_url
        // from the backend at runtime; only literal absolute URLs are violations.
        if (EXTERNALLY_OWNED.has(topLevel)) externalViolations.push(entry);
        else violations.push(entry);
      });
    }
  }
  assert.deepEqual(
    violations,
    [],
    `Cross-origin top-level navigation literals found in general source (R447-A3 sentinel):\n${violations.join('\n')}`,
  );
  assert.deepEqual(
    externalViolations,
    [],
    `Cross-origin top-level navigation literals found in externally-owned dirs (report to owners):\n${externalViolations.join('\n')}`,
  );
});

test('session-failure relogin path lands on a same-origin relative path', async () => {
  const { reloginAfterRefreshFailure } = await import('./auth/relogin.ts');
  const assigned: string[] = [];
  await reloginAfterRefreshFailure({
    clearSession: () => undefined,
    pathname: '/knowledgeBase/kb-1/settings',
    assign: (url) => assigned.push(url),
  });
  assert.deepEqual(assigned, ['/login']);
  for (const url of assigned) {
    assert.ok(url.startsWith('/'), `relogin must assign a relative same-origin path, got ${url}`);
    assert.ok(!url.startsWith('//'), `protocol-relative URL would escape the origin: ${url}`);
  }
});
