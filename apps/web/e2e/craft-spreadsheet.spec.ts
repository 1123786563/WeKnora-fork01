// D02 craft spreadsheet — browser acceptance over the real full stack.
//
// Drives the W06 harness stack (vite, Go server, SQLite, storage, pinned
// OpenCode serve, isolated preview origin) through the D02 acceptance:
// the mock sub-model's spreadsheet round generates a real XLSX (formulas
// WITH fixture-simulated cached values — the LibreOffice recalc itself is
// proven inside the craft image, see the D02 report), preview.json,
// manifest.json and the static escaped preview page. The spec verifies in
// the browser: sheet switching, row counts, the computed total, XLSX
// download, byte-stable old versions across later rounds, and the
// craft.kinds feature gate staying fail-closed for spreadsheet until the
// recalc/preview/parse evidence chain justifies enabling it.
import { test, expect, type Download, type Frame, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const API_URL = process.env.CRAFT_API_URL ?? '';
const DB_PATH = process.env.CRAFT_DB_PATH ?? '';
const KINDS = (process.env.CRAFT_KINDS ?? 'web').split(',').map((kind) => kind.trim());

test.describe.configure({ mode: 'serial' });
test.setTimeout(300_000);

const monthlyGoal = '把销售数据做成数据表格，导出XLSX，显示计算总额';
const quarterlyGoal = '改为季度汇总表格，保留XLSX导出';
const extraGoal = '表格加一行 2026-03 中区 50，更新总额';

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

// Opens the Files tab and downloads exactly report.xlsx of the SELECTED
// version — a fixed (versionId, path) pair, never the latest file.
async function downloadXlsxFromFilesTab(page: Page): Promise<Download> {
  await page.getByRole('button', { name: '版本历史' }).click();
  const row = page.locator('tr', { hasText: 'report.xlsx' });
  await expect(row).toBeVisible({ timeout: 30_000 });
  const pending = page.waitForEvent('download');
  await row.getByRole('button').click();
  const download = await pending;
  expect(download.suggestedFilename()).toBe('report.xlsx');
  return download;
}

// The python one-liner below parses the REAL stored OOXML of the downloaded
// workbook (stdlib zipfile+re on the host): sheet names, the total formula
// and its cached <v> value.
const XLSX_FACTS_SCRIPT = [
  'import sys, zipfile, re, json',
  'zf = zipfile.ZipFile(sys.argv[1])',
  'wb = zf.read("xl/workbook.xml").decode("utf-8")',
  'sheets = re.findall(' + JSON.stringify('name="([^"]+)"') + ', wb)',
  'facts = {}',
  'for name in zf.namelist():',
  '    if name.startswith("xl/worksheets/sheet"):',
  '        xml = zf.read(name).decode("utf-8")',
  '        m = re.search(' + JSON.stringify('<c r="C\\d+"[^>]*><f>([^<]+)</f><v>([^<]*)</v>') + ', xml)',
  '        if m and m.group(1).startswith("SUM"):',
  '            facts = {"formula": m.group(1), "cached": m.group(2)}',
  'print(json.dumps({"sheets": sheets, **facts}))',
].join(String.fromCharCode(10));

function parseXlsxFacts(path: string | null): { sheets: string[]; formula: string; cached: string } {
  if (path === null) throw new Error('download produced no file');
  const out = execFileSync('python3', ['-c', XLSX_FACTS_SCRIPT, path], { encoding: 'utf8' });
  return JSON.parse(out.trim()) as { sheets: string[]; formula: string; cached: string };
}

interface VersionFileFacts {
  sheets: { name: string; rows: number; columns: number; recalculated: boolean; formula_errors: string[] }[];
  totals: Record<string, string>;
  preview_totals: Record<string, string>;
  checks: { name: string; status: string }[];
}

async function fetchManifest(auth: AuthedContext, sessionId: string, versionId: string): Promise<VersionFileFacts> {
  const res = await apiFetch(auth, '/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/files/manifest.json');
  expect(res.status).toBe(200);
  return await res.json() as VersionFileFacts;
}

let sessionId = '';
let firstVersionId = '';
let firstSha = '';
const owner = () => authFromStateFile(process.env.CRAFT_AUTH_STATE as string);

test('01 create, upload, spreadsheet generate, sheet switch, XLSX download', async ({ page }) => {
  await page.goto('/craft');
  await expect(page.getByTestId('craft-goal')).toBeVisible();
  await page.getByTestId('craft-goal').fill(monthlyGoal);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(monthlyGoal, { timeout: 30_000 });
  await page.getByTestId('craft-upload').setInputFiles('e2e/fixtures/craft-sales.csv');
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  sessionId = new URL(page.url()).pathname.split('/').pop() ?? '';
  expect(sessionId).not.toBe('');
  firstVersionId = await page.getByTestId('craft-version').inputValue();
  expect(firstVersionId).not.toBe('');

  // Preview (isolated origin, real iframe): title, sheet tabs, totals.
  const frame = await previewFrame(page);
  await frame.waitForSelector('.sheet-tab', { timeout: 60_000 });
  await expect(frame.locator('.sheet-tab')).toHaveCount(2);
  const title = await frame.evaluate(() => document.title);
  expect(title).toBe('按月销售表格');
  const monthly = frame.locator('section[data-sheet="月度销售"]');
  await expect(monthly.locator('.sheet-rows')).toHaveText('4');
  await expect(monthly.locator('.sheet-total')).toHaveText('300');
  // Numeric/date cells keep their type tags in the converted preview.
  // (Row-scoped: the header row's thousands-format cell is also numeric.)
  await expect(monthly.locator('td[data-type="date"]').first()).toHaveText('2026-01-01');
  await expect(monthly.locator('tr').nth(1).locator('td[data-type="number"]').first()).toHaveText('100');
  await expect(monthly.locator('td[data-type="number"]').first()).toHaveText('1,234');

  // Sheet switching: the 汇总 sheet renders its own rows and the same total.
  await frame.locator('.sheet-tab[data-sheet="汇总"]').click();
  const summary = frame.locator('section[data-sheet="汇总"]');
  await expect(summary).toBeVisible();
  await expect(monthly).toBeHidden();
  await expect(summary.locator('.sheet-rows')).toHaveText('3');
  await expect(summary.locator('.sheet-total')).toHaveText('300');
  // The download affordance is present and points at the deliverable.
  await expect(frame.locator('a.xlsx-download')).toHaveAttribute('href', 'report.xlsx');
  await page.screenshot({ path: artifactDir() + '/d02-01-preview-monthly.png', fullPage: true });

  // Download the REAL workbook through the Files tab (fixed version+path).
  const download = await downloadXlsxFromFilesTab(page);
  firstSha = await sha256OfDownload(download);
  expect(firstSha).not.toBe('');
  const facts = parseXlsxFacts(await download.path());
  expect(facts.sheets).toEqual(['月度销售', '汇总']);
  expect(facts.formula).toBe('SUM(C2:C3)');
  expect(facts.cached).toBe('300');

  // manifest.json records the sheet facts and the three gate checks.
  const manifest = await fetchManifest(owner(), sessionId, firstVersionId);
  expect(manifest.sheets.map((sheet) => sheet.name)).toEqual(['月度销售', '汇总']);
  expect(manifest.totals['月度销售']).toBe('300');
  expect(manifest.preview_totals['汇总']).toBe('300');
  for (const check of manifest.checks) expect(check.status).toBe('passed');
  expect(manifest.checks.map((check) => check.name).sort()).toEqual(['parse', 'preview', 'recalc']);
});

test('02 quarterly round keeps 300 and the old XLSX bytes', async ({ page }) => {
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await page.getByTestId('craft-prompt').fill(quarterlyGoal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  const secondVersionId = await page.getByTestId('craft-version').inputValue();
  expect(secondVersionId).not.toBe(firstVersionId);

  const frame = await previewFrame(page);
  await frame.waitForSelector('.sheet-tab[data-sheet="季度销售"]', { timeout: 60_000 });
  const title = await frame.evaluate(() => document.title);
  expect(title).toBe('季度销售汇总表格');
  const quarterly = frame.locator('section[data-sheet="季度销售"]');
  await expect(quarterly.locator('.sheet-total')).toHaveText('300');

  // The new version's manifest still carries agreeing totals.
  const manifest = await fetchManifest(owner(), sessionId, secondVersionId);
  expect(manifest.totals['季度销售']).toBe('300');
  expect(manifest.preview_totals['季度销售']).toBe('300');

  // Old version download: the exact same bytes as round 1.
  await page.getByTestId('craft-version').selectOption(firstVersionId);
  const oldDownload = await downloadXlsxFromFilesTab(page);
  expect(await sha256OfDownload(oldDownload)).toBe(firstSha);
  const oldFacts = parseXlsxFacts(await oldDownload.path());
  expect(oldFacts.sheets).toEqual(['月度销售', '汇总']);
  expect(oldFacts.cached).toBe('300');
  await page.screenshot({ path: artifactDir() + '/d02-02-versions.png', fullPage: true });
});

test('03 adding a 50 row recalculates the total to 350', async ({ page }) => {
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await page.getByTestId('craft-prompt').fill(extraGoal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });

  const frame = await previewFrame(page);
  await frame.waitForSelector('.sheet-total', { timeout: 60_000 });
  const monthly = frame.locator('section[data-sheet="月度销售"]');
  await expect(monthly.locator('.sheet-rows')).toHaveText('5');
  await expect(monthly.locator('.sheet-total')).toHaveText('350');
  await expect(monthly.locator('tr').nth(3).locator('td[data-type="number"]').first()).toHaveText('50');

  // The downloaded workbook's formula extended and its cache says 350.
  const download = await downloadXlsxFromFilesTab(page);
  const facts = parseXlsxFacts(await download.path());
  expect(facts.formula).toBe('SUM(C2:C4)');
  expect(facts.cached).toBe('350');
  await page.screenshot({ path: artifactDir() + '/d02-03-extra-row.png', fullPage: true });
});

test('04 spreadsheet kind gate: fail-closed until the deployment opens it', async () => {
  // The deployment gate (WEKNORA_CRAFT_KINDS, CRAFT_KINDS in the stack
  // harness) owns this decision. With the kind CLOSED (the default web-
  // only gate) creating one must be refused; once the evidence chain (this
  // spec + the image acceptance + the D01 wiring: EntryPath,
  // PreviewableKind, the server-side manifest admission) is green the kind
  // opens and creation is admitted — both branches are asserted honestly.
  const res = await apiFetch(owner(), '/craft/sessions', {
    method: 'POST',
    body: JSON.stringify({ request_id: 'd02-gate-' + Date.now(), title: 'D02 gate probe', kind: 'spreadsheet' }),
  });
  expect(DB_PATH).not.toBe('');
  if (!KINDS.includes('spreadsheet')) {
    expect(res.status).toBe(503);
    const body = await res.json() as { success?: boolean; data?: { session_id?: string } };
    expect(body.success ?? true).not.toBe(true);
    expect(body.data?.session_id ?? '').toBe('');
    const count = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH,
      "SELECT COUNT(*) FROM craft_sessions WHERE kind = 'spreadsheet'"], { encoding: 'utf8' }).trim();
    expect(count).toBe('0');
    // The accepted rounds above ran as kind=web sessions (the only open kind)
    // while producing spreadsheet deliverables — sanity check from the DB.
    const web = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH,
      "SELECT COUNT(*) FROM craft_sessions WHERE kind = 'web'"], { encoding: 'utf8' }).trim();
    expect(Number(web)).toBeGreaterThanOrEqual(1);
    return;
  }
  // OPEN: the wired kind is admitted end-to-end.
  expect([200, 201]).toContain(res.status);
  const created = await res.json() as { data?: { session_id?: string } };
  expect(created.data?.session_id ?? '').not.toBe('');
  const count = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH,
    "SELECT COUNT(*) FROM craft_sessions WHERE kind = 'spreadsheet'"], { encoding: 'utf8' }).trim();
  expect(Number(count)).toBeGreaterThanOrEqual(1);
});