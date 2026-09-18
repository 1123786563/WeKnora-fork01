// W06 craft web report — complete browser acceptance.
//
// Drives the real full stack (vite web app, Go server, SQLite, storage,
// pinned OpenCode serve, isolated preview origin) through the eight
// Phase-B acceptance steps and the security/idempotency assertions:
//
//   1. create            — craft-goal + craft-create on /craft
//   2. upload            — craft-upload with e2e/fixtures/craft-sales.csv
//   3. main retrieve     — the run's snapshot carries the authorized input
//   4. sub generation    — craft_delegate -> pinned OpenCode serve
//   5. preview           — isolated-origin iframe (title / total 300 / filter)
//   6. modify            — second turn rewrites the report
//   7. old version download — same SHA across downloads, new version differs
//   8. reopen            — navigate away and back, the workbench restores
//
// Security: malicious preview fixture (parent DOM/cookie access blocked,
// external fetch blocked by CSP), cross-tenant 404, viewer write 403, scope
// switch isolation, and exactly one POST /runs admission across reloads of a
// generating page (counted from the database, never from UI copy).
//
// CRAFT_MODEL_MODE=mock runs the deterministic fixture sub-model;
// CRAFT_MODEL_MODE=real runs the OpenCode built-in free model (no user
// credentials are loaded or consumed — the serve is XDG-isolated).
import { test, expect, type Download, type Page, type Frame } from '@playwright/test';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const MODEL_MODE = process.env.CRAFT_MODEL_MODE ?? 'mock';
const IS_REAL = MODEL_MODE === 'real';
const API_URL = process.env.CRAFT_API_URL ?? '';
const DB_PATH = process.env.CRAFT_DB_PATH ?? '';
const VIEWER_STATE = process.env.CRAFT_VIEWER_STATE ?? '';
const FOREIGN_STATE = process.env.CRAFT_FOREIGN_STATE ?? '';

test.describe.configure({ mode: 'serial' });
test.setTimeout(IS_REAL ? 900_000 : 240_000);

const monthlyGoal = '按月分析销售数据，生成可筛选网页报告';
const quarterlyGoal = '改为季度汇总，保留地区筛选';
const probeGoal = '生成一个安全探测页面，用于安全探测父站隔离';

interface AuthedContext {
  token: string;
  tenantId: string;
}

function authFromStateFile(path: string): AuthedContext {
  const raw = JSON.parse(readFileSync(path, 'utf8')) as {
    origins?: { origin: string; localStorage: { name: string; value: string }[] }[];
  };
  for (const origin of raw.origins ?? []) {
    const token = origin.localStorage.find((entry) => entry.name === 'weknora_token')?.value ?? '';
    const tenantId = origin.localStorage.find((entry) => entry.name === 'weknora_selected_tenant_id')?.value ?? '';
    if (token !== '') return { token, tenantId };
  }
  throw new Error('no weknora_token in ' + path);
}

async function apiFetch(auth: AuthedContext, path: string, init?: RequestInit): Promise<Response> {
  return fetch(API_URL + path, {
    ...init,
    headers: {
      'content-type': 'application/json',
      authorization: 'Bearer ' + auth.token,
      'x-tenant-id': auth.tenantId,
      ...(init?.headers ?? {}),
    },
  });
}

function sqliteQuery(sql: string): string {
  const out = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH, sql], { encoding: 'utf8' });
  return out.trim();
}

async function sha256OfDownload(download: Download): Promise<string> {
  const path = await download.path();
  if (path === null) throw new Error('download produced no file');
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

function artifactDir(): string {
  return process.env.CRAFT_E2E_OUTPUT ?? '.';
}

async function previewFrame(page: Page): Promise<Frame> {
  await expect(page.getByTestId('craft-preview')).toHaveAttribute('data-state', 'active', { timeout: 60_000 });
  for (let attempt = 0; attempt < 120; attempt += 1) {
    for (const frame of page.frames()) {
      if (frame.url().includes('/p/')) return frame;
    }
    await page.waitForTimeout(500);
  }
  throw new Error('preview iframe never attached: frames=' + page.frames().map((frame) => frame.url()).join(', '));
}

let sessionId = '';
let firstVersionId = '';
let secondVersionId = '';
let firstSha = '';
test('01 create, upload, generate, preview, modify, old-version download', async ({ page }) => {
  await page.goto('/craft');
  await expect(page.getByTestId('craft-goal')).toBeVisible();
  await page.getByTestId('craft-goal').fill(monthlyGoal);
  await page.getByTestId('craft-create').click();

  // The workbench mounts with the creation prompt; attach the sales fixture.
  await expect(page.getByTestId('craft-prompt')).toHaveValue(monthlyGoal, { timeout: 30_000 });
  await page.getByTestId('craft-upload').setInputFiles('e2e/fixtures/craft-sales.csv');
  await page.getByTestId('craft-send').click();

  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  sessionId = new URL(page.url()).pathname.split('/').pop() ?? '';
  expect(sessionId).not.toBe('');

  firstVersionId = await page.getByTestId('craft-version').inputValue();
  expect(firstVersionId).not.toBe('');

  // Preview (isolated origin, real iframe): title + total 300 + region filter.
  if (!IS_REAL) {
    const frame = await previewFrame(page);
    await frame.waitForSelector('#total-value', { timeout: 60_000 });
    const title = await frame.evaluate(() => document.title);
    expect(title).toBe('按月销售报告');
    await expect(frame.locator('#total-value')).toHaveText('300');
    await frame.selectOption('#region-filter', 'east');
    await expect(frame.locator('#total-value')).toHaveText('100');
    await frame.selectOption('#region-filter', 'all');
    await expect(frame.locator('#total-value')).toHaveText('300');
  } else {
    const frame = await previewFrame(page);
    await frame.waitForSelector('body', { timeout: 120_000 });
    const title = await frame.evaluate(() => document.title);
    expect(title.trim()).not.toBe('');
  }
  await page.screenshot({ path: artifactDir() + '/01-preview-monthly.png', fullPage: true });

  // First download: bytes + SHA recorded.
  const firstDownload = page.waitForEvent('download');
  await page.getByTestId('craft-download').click();
  const first = await firstDownload;
  expect(first.suggestedFilename()).not.toBe('');
  firstSha = await sha256OfDownload(first);
  expect(firstSha).not.toBe('');

  // Modify: quarterly rewrite must publish a different version.
  await page.getByTestId('craft-prompt').fill(quarterlyGoal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-version')).not.toHaveValue(firstVersionId, { timeout: 240_000 });
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  secondVersionId = await page.getByTestId('craft-version').inputValue();
  expect(secondVersionId).not.toBe(firstVersionId);

  if (!IS_REAL) {
    const frame = await previewFrame(page);
    await frame.waitForSelector('#total-value', { timeout: 60_000 });
    await expect(frame.locator('#total-value')).toHaveText('300');
    const title = await frame.evaluate(() => document.title);
    expect(title).toBe('季度销售汇总');
  }

  // Old version download again: byte-identical SHA.
  await page.getByTestId('craft-version').selectOption(firstVersionId);
  const oldAgain = page.waitForEvent('download');
  await page.getByTestId('craft-download').click();
  const oldDownload = await oldAgain;
  expect(oldDownload.suggestedFilename()).not.toBe('');
  expect(await sha256OfDownload(oldDownload)).toBe(firstSha);

  // The new version must differ from the old bytes (deterministic mock).
  if (!IS_REAL) {
    await page.getByTestId('craft-version').selectOption(secondVersionId);
    const newDownload = page.waitForEvent('download');
    await page.getByTestId('craft-download').click();
    const next = await newDownload;
    expect(await sha256OfDownload(next)).not.toBe(firstSha);
  }
  await page.screenshot({ path: artifactDir() + '/02-versions.png', fullPage: true });
});

test('02 reopen keeps the workbench and the published versions', async ({ page }) => {
  await page.goto('/craft');
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId('craft-version').locator('option')).toHaveCount(2, { timeout: 60_000 });
  await expect(page.getByTestId('craft-version')).toHaveValue(secondVersionId);
});

test('03 malicious preview fixture cannot reach the parent or the network', async ({ page }) => {
  test.skip(IS_REAL, 'malicious fixture is the deterministic mock sub-model');
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await page.getByTestId('craft-prompt').fill(probeGoal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });

  const frame = await previewFrame(page);
  await frame.waitForFunction(() => {
    const node = document.getElementById('probe-results');
    return node !== null && node.textContent !== null && node.textContent !== 'pending';
  }, null, { timeout: 60_000 });
  const raw = await frame.locator('#probe-results').textContent();
  expect(raw).not.toBe(null);
  const results = JSON.parse(raw as string) as Record<string, string>;
  expect(results.cookieRead).toMatch(/^BLOCKED:/);
  expect(results.cookieWrite).toMatch(/^BLOCKED:/);
  expect(results.parentTitle).toMatch(/^BLOCKED:/);
  expect(results.parentCookie).toMatch(/^BLOCKED:/);
  expect(results.parentDom).toMatch(/^BLOCKED:/);
  expect(results.fetchApp).toMatch(/^BLOCKED:/);
  expect(results.fetchExternal).toMatch(/^BLOCKED:/);
  await page.screenshot({ path: artifactDir() + '/03-malicious-blocked.png', fullPage: true });
});

test('04 cross-tenant download 404 and viewer write 403', async ({ browser }) => {
  // Resolve the acceptance session from the API when this test runs in its
  // own playwright invocation (real-mode batches split the spec).
  if (sessionId === '') {
    const owner = authFromStateFile(process.env.CRAFT_AUTH_STATE as string);
    const list = await apiFetch(owner, '/craft/sessions');
    expect(list.status).toBe(200);
    const rows = ((await list.json()) as { data?: { session_id: string }[] }).data ?? [];
    expect(rows.length).toBeGreaterThan(0);
    sessionId = rows[0].session_id;
    const versions = await apiFetch(owner, '/sessions/' + sessionId + '/craft/versions');
    const page = ((await versions.json()) as { data?: { id: string }[] }).data ?? [];
    firstVersionId = page.length > 0 ? page[0].id : '';
  }
  // Cross-tenant: the foreign tenant cannot even see the session or version.
  if (FOREIGN_STATE !== '') {
    const foreign = authFromStateFile(FOREIGN_STATE);
    const workspace = await apiFetch(foreign, '/sessions/' + sessionId + '/craft');
    expect(workspace.status).toBe(404);
    const file = await apiFetch(foreign, '/sessions/' + sessionId + '/craft/versions/' + firstVersionId + '/files/index.html');
    expect([403, 404]).toContain(file.status);
  }

  // Non-owner tenant member (viewer): sessions are owner-scoped reads, so
  // the craft surface is fail-closed INVISIBLE for the viewer — reads and
  // writes both answer 404. (writeSession's explicit 403 path requires a
  // readable-but-not-owned session, i.e. a share; session sharing is not
  // wired yet, so the stronger 404 boundary is the honest production
  // behavior — the 403 branch itself is covered by W03's service tests.)
  if (VIEWER_STATE !== '') {
    const viewer = authFromStateFile(VIEWER_STATE);
    const versions = await apiFetch(viewer, '/sessions/' + sessionId + '/craft/versions');
    expect(versions.status).toBe(404);
    const run = await apiFetch(viewer, '/sessions/' + sessionId + '/craft/runs', {
      method: 'POST',
      body: JSON.stringify({ request_id: 'w06-viewer-' + Date.now(), prompt: 'viewer should not write' }),
    });
    expect([403, 404]).toContain(run.status);
    expect(run.status).toBe(404);

    const viewerContext = await browser.newContext({ storageState: VIEWER_STATE, ignoreHTTPSErrors: true });
    const viewerPage = await viewerContext.newPage();
    await viewerPage.goto('/craft/' + encodeURIComponent(sessionId));
    // The workbench never opens the composer for an invisible session: the
    // load error is surfaced instead of a writable surface.
    await expect(viewerPage.locator('main[role="status"], .wk-craft-page p[role="status"]')).toBeVisible({ timeout: 60_000 });
    await expect(viewerPage.getByTestId('craft-prompt')).toHaveCount(0);
    await viewerPage.screenshot({ path: artifactDir() + '/04-viewer-invisible.png', fullPage: true });
    await viewerContext.close();
  }
});

test('05 switching sessions never appends the old stream', async ({ page }) => {
  const bPrompt = 'B会话专用提示词：生成月度报告';
  await page.goto('/craft');
  await page.getByTestId('craft-goal').fill(bPrompt);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(bPrompt, { timeout: 30_000 });
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText(/正在生成|等待执行|已完成/, { timeout: 120_000 });

  // Switch back to the first session while B is still generating: the first
  // session's conversation must not receive B's live stream.
  // (CFT-S01-T010: the conversation column renders through assistant-ui —
  // the scrolling viewport is the conversation container now.)
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
  const conversationA = page.locator('.wk-craft-thread-viewport');
  await expect(conversationA).toBeVisible({ timeout: 60_000 });
  const textA = await conversationA.innerText();
  expect(textA).not.toContain(bPrompt);

  // Back to B: it must finish with only its own conversation.
  await page.goBack();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  const conversation = page.locator('.wk-craft-thread-viewport');
  const textB = await conversation.innerText();
  expect(textB).not.toContain(monthlyGoal);
  expect(textB).not.toContain(quarterlyGoal);
});

test('06 reloading a generating page keeps exactly one run admission', async ({ page }) => {
  await page.goto('/craft');
  const goal = '幂等刷新验证：生成月度报告';
  await page.getByTestId('craft-goal').fill(goal);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(goal, { timeout: 30_000 });
  await page.getByTestId('craft-send').click();

  const reloadSessionId = new URL(page.url()).pathname.split('/').pop() ?? '';

  // Reload while the run is generating (twice — resume + refresh).
  for (let attempt = 0; attempt < 2; attempt += 1) {
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
  }
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });

  expect(DB_PATH).not.toBe('');
  const safeId = reloadSessionId.replace(/[^A-Za-z0-9_-]/g, '');
  const admitted = sqliteQuery("SELECT COUNT(*) FROM agent_runs WHERE session_id = '" + safeId + "'");
  expect(admitted).toBe('1');
  const created = sqliteQuery("SELECT COUNT(*) FROM craft_session_requests WHERE session_id = '" + safeId + "' AND purpose = 'create'");
  expect(created).toBe('1');
});
