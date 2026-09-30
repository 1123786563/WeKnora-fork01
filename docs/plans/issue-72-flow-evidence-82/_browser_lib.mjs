// #82 Task 16 (OCR r2): the SHARED browser-leg library for the flow-evidence
// scripts. The three top-level drivers and the r5-verify legs consume this
// one copy of the login flow, the results recorder, the A-03 catch wrapper
// (a browser leg that THROWS must still emit a RESULT line and a non-zero
// exit code — never a bare stack trace with exit 0), and the A-12 order
// identity anchor (re-visits of the same order must observe the SAME ord_
// id — the frozen replay discipline).
//
// Exports: { LOGIN, login, note, runLeg, assertOrderIdentity }.
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

// requirePlaywright resolves @playwright/test from the REAL web workspace
// (never a global install); the docs/ tree has no node_modules of its own.
export function requirePlaywright() {
  const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));
  return require('@playwright/test');
}

// The login flow's frozen selector set (r2:38 — one copy; a selector change
// now propagates to every leg instead of drifting across 15 copies).
export const LOGIN = {
  emailSelector: '#auth-email',
  passwordSelector: '#auth-password',
  submitSelector: 'form[aria-label="Login form"] button[type="submit"]',
  settled: (u) => !u.pathname.startsWith('/login'),
  timeoutMs: 30000,
};

// login drives the real login form with env-REQUIRED credentials (the
// branch redline: no usable credential literals — the caller validates the
// env vars and passes them in).
export async function login(page, web, email, password) {
  await page.goto(`${web}/login`);
  await page.fill(LOGIN.emailSelector, email);
  await page.fill(LOGIN.passwordSelector, password);
  await page.locator(LOGIN.submitSelector).click();
  await page.waitForURL((u) => LOGIN.settled(u), { timeout: LOGIN.timeoutMs });
}

// note records one assertion into the caller-held results array (the RESULT
// JSON line at the end of every leg is built from it).
export function note(results, step, ok, detail) {
  results.push({ step, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
}

// assertOrderIdentity extracts the visible order id (the A-12 anchor: the
// 订单号 row) and checks it against the previously observed id — a re-visit
// that mints a DIFFERENT order means the leg accidentally re-purchased.
export function assertOrderIdentity(results, text, anchoredID, step) {
  const match = text.match(/ord_[0-9a-f]+/);
  const seen = match ? match[0] : '(no order id visible)';
  const ok = anchoredID === undefined ? Boolean(match) : seen === anchoredID;
  note(results, step ?? 'order-identity-anchor', ok,
    `order id ${seen}${anchoredID === undefined ? '' : ` (anchored ${anchoredID})`}`);
  return seen;
}

// runLeg wraps one browser leg: launch/close the browser, run the body
// (which returns its results array), catch ANY throw into a FAIL record
// (A-03 — the leg still emits RESULT and exits non-zero), print the RESULT
// line, and derive the process exit code from the records.
export async function runLeg(body) {
  const { chromium } = requirePlaywright();
  const results = [];
  const browser = await chromium.launch();
  try {
    await body(results, browser);
  } catch (err) {
    // (A-03) A thrown browser leg must not die with a bare stack trace and
    // a zero exit code: record the failure face, keep the RESULT contract.
    note(results, 'leg-error', false, String(err?.message ?? err));
  } finally {
    await browser.close().catch(() => { /* best-effort close */ });
  }
  console.log('RESULT ' + JSON.stringify(results));
  process.exitCode = results.every((r) => r.ok) ? 0 : 1;
  return results;
}

// evidencePath resolves a path under this evidence directory (optionally a
// named subdirectory), the parameterized FLOW82_EVIDENCE_DIR face.
export function evidencePath(subdir) {
  const base = fileURLToPath(new URL('.', import.meta.url));
  return subdir ? base + subdir + '/' : base;
}

// envRequired exits with usage guidance when any named env var is missing —
// the no-source-code-fallback credential posture shared by every leg.
export function envRequired(...names) {
  const missing = names.filter((n) => !process.env[n]);
  if (missing.length > 0) {
    console.error(`missing required env: ${missing.join(' / ')} (no source-code fallback)`);
    process.exit(2);
  }
  return Object.fromEntries(names.map((n) => [n, process.env[n]]));
}
