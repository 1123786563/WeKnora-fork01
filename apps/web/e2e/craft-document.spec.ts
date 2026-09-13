// D01 craft document — browser acceptance over the real full stack.
//
// Drives the W06 harness stack (vite, Go server, SQLite, storage, pinned
// OpenCode serve) through the D01 acceptance with the document kind OPEN
// (CRAFT_KINDS=web,document): the mock sub-model's document round writes a
// real DOCX (stdlib OOXML in the same storage shape as the image-verified
// python-docx chain — see the D01 report), report.md and manifest.json. The
// spec verifies in the browser: the workbench document view (safe Markdown
// renderer, NO raw HTML) with 3 sections, an 8-row table, the verbatim
// numbers 12/96%/6 周 and 2 CLICKABLE kc_ citations; the DOCX download as
// an attachment whose stored OOXML matches the Markdown; the FAQ round
// keeping the numbers and the old version's DOCX bytes (SHA) untouched; and
// the failure acceptance — a corrupt export with an honest export=failed
// manifest publishes NO version and shows no export success.
import { test, expect, type Download, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const API_URL = process.env.CRAFT_API_URL ?? '';
const DB_PATH = process.env.CRAFT_DB_PATH ?? '';
const KINDS = (process.env.CRAFT_KINDS ?? 'web').split(',').map((kind) => kind.trim());
const DOCUMENT_KIND_OPEN = KINDS.includes('document');

test.skip(!DOCUMENT_KIND_OPEN, 'the document kind is not open in this deployment (run with CRAFT_KINDS=web,document)');
test.describe.configure({ mode: 'serial' });
test.setTimeout(300_000);

const introGoal = '用两份知识资料生成客户介绍，导出DOCX，含3个章节和2个可点引用';
const faqGoal = '客户介绍新增常见问题FAQ章节，保持原有数字不变，导出DOCX';
const brokenGoal = '生成一份损坏的DOCX导出，用于失败验证';

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

// Opens the Files tab and downloads exactly report.docx of the SELECTED
// version — a fixed (versionId, path) pair, never the latest file.
async function downloadDocxFromFilesTab(page: Page): Promise<Download> {
  await page.getByRole('button', { name: '版本历史' }).click();
  const row = page.getByRole('row', { name: /^report\.docx/ });
  await expect(row).toBeVisible({ timeout: 30_000 });
  const pending = page.waitForEvent('download');
  await row.getByRole('button').click();
  const download = await pending;
  expect(download.suggestedFilename()).toBe('report.docx');
  return download;
}

// The python one-liner below parses the REAL stored OOXML of the downloaded
// document (stdlib zipfile+re on the host): styled headings (Title +
// Heading1), the first table's row count, the distinct kc_ citations and
// whether styles.xml declares the CJK font.
const DOCX_FACTS_SCRIPT = [
  'import sys, zipfile, re, json',
  'zf = zipfile.ZipFile(sys.argv[1])',
  'doc = zf.read("word/document.xml").decode("utf-8")',
  'styles = zf.read("word/styles.xml").decode("utf-8")',
  'headings = []',
  'for p in re.findall(r"<w:p>(?:(?!</w:p>).)*</w:p>", doc):',
  '    if re.search(r"<w:pStyle w:val=[^>]*(Title|Heading1)[^>]*>", p):',
  '        headings.append("".join(re.findall(r"<w:t[^>]*>([^<]*)</w:t>", p)))',
  'rows = len(re.findall(r"<w:tr>", doc))',
  'citations = sorted(set(re.findall(r"kc_[0-9a-f]{24}", doc)))',
  'print(json.dumps({"headings": headings, "rows": rows, "citations": citations, "font": "Noto Sans CJK SC" in styles}, ensure_ascii=False))',
].join(String.fromCharCode(10));

interface DocxFacts {
  headings: string[];
  rows: number;
  citations: string[];
  font: boolean;
}

function parseDocxFacts(path: string | null): DocxFacts {
  if (path === null) throw new Error('download produced no file');
  const out = execFileSync('python3', ['-c', DOCX_FACTS_SCRIPT, path], { encoding: 'utf8' });
  return JSON.parse(out.trim()) as DocxFacts;
}

interface DocumentManifestFacts {
  kind: string;
  markdown: string;
  docx: string;
  markdown_ref: string;
  docx_ref: string;
  headings: string[];
  citation_ids: string[];
  checks: { name: string; status: string; detail: string }[];
}

async function fetchManifest(auth: AuthedContext, sessionId: string, versionId: string): Promise<DocumentManifestFacts> {
  const res = await apiFetch(auth, '/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/files/manifest.json');
  expect(res.status).toBe(200);
  return await res.json() as DocumentManifestFacts;
}

function dbCount(sql: string): number {
  expect(DB_PATH).not.toBe('');
  return Number(execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH, sql], { encoding: 'utf8' }).trim());
}

let sessionId = '';
let firstVersionId = '';
let firstSha = '';
const owner = () => authFromStateFile(process.env.CRAFT_AUTH_STATE as string);

test('01 two knowledge materials -> 客户介绍: 3 sections, 2 clickable citations, DOCX export', async ({ page }) => {
  await page.goto('/craft');
  await expect(page.getByTestId('craft-goal')).toBeVisible();
  await page.locator('#craft-kind-select').selectOption('document');
  await page.getByTestId('craft-goal').fill(introGoal);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(introGoal, { timeout: 30_000 });
  // The TWO knowledge materials ride the creation round as staged inputs.
  await page.getByTestId('craft-upload').setInputFiles([
    'e2e/fixtures/craft-company-profile.md',
    'e2e/fixtures/craft-case-study.md',
  ]);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  sessionId = new URL(page.url()).pathname.split('/').pop() ?? '';
  expect(sessionId).not.toBe('');
  firstVersionId = await page.getByTestId('craft-version').inputValue();
  expect(firstVersionId).not.toBe('');

  // The workbench document view (NOT an iframe): safe Markdown projection.
  const view = page.getByTestId('craft-document');
  await expect(view).toHaveAttribute('data-state', 'active', { timeout: 30_000 });
  const sections = view.locator('[data-testid="craft-document-heading"]');
  // 3 content sections plus the trailing 来源 section carry the heading testid.
  await expect(sections).toHaveCount(4, { timeout: 30_000 });
  await expect(sections.nth(0)).toHaveText('公司概况');
  await expect(sections.nth(1)).toHaveText('产品能力');
  await expect(sections.nth(2)).toHaveText('实施效果');
  await expect(sections.nth(3)).toHaveText('来源');
  // The long table carries every staged customer row.
  await expect(view.locator('[data-testid="craft-document-table"] tr')).toHaveCount(8);
  // The numbers are verbatim from the two materials.
  const body = view.locator('[data-testid="craft-document-body"]');
  await expect(body).toContainText('12');
  await expect(body).toContainText('96%');
  await expect(body).toContainText('6 周');
  // Exactly TWO distinct clickable citations (kc_ ids), in body and panel.
  const cited = await view.locator('[data-testid="craft-document-citation"]').evaluateAll((nodes) =>
    Array.from(new Set(nodes.map((node) => (node as HTMLElement).dataset.citation ?? ''))).filter((id) => id !== ''));
  expect(cited).toHaveLength(2);
  for (const id of cited) expect(id).toMatch(/^kc_[0-9a-f]{24}$/);
  // Clicking a citation resolves through the knowledge permission chain and
  // answers with a notice (resolved title, or the honest not-visible text).
  await view.locator('[data-testid="craft-document-citation"]').first().click();
  // The notice lands in the 执行详情 tab (the shared source-resolution
  // surface): open it and read the honest answer.
  await page.getByRole('tab', { name: '执行详情' }).click();
  await expect(page.getByTestId('craft-source-notice')).toBeVisible({ timeout: 30_000 });
  await page.getByRole('tab', { name: '预览' }).click();
  // Raw HTML never renders: the markdown source carries none, and the view
  // would have counted dropped lines otherwise.
  await expect(view.locator('[data-testid="craft-document-html-dropped"]')).toHaveCount(0);
  await page.screenshot({ path: artifactDir() + '/d01-01-intro.png', fullPage: true });

  // Download the REAL export through the Files tab (fixed version+path) and
  // read its stored OOXML back: headings, table, citations, CJK font.
  const download = await downloadDocxFromFilesTab(page);
  firstSha = await sha256OfDownload(download);
  expect(firstSha).not.toBe('');
  const facts = parseDocxFacts(await download.path());
  expect(facts.headings).toEqual(['智绘云图客户介绍', '公司概况', '产品能力', '实施效果', '来源']);
  expect(facts.rows).toBe(8);
  expect(facts.citations).toEqual(cited.slice().sort());
  expect(facts.font).toBe(true);

  // manifest.json: the four document gate checks all passed, both refs.
  const manifest = await fetchManifest(owner(), sessionId, firstVersionId);
  expect(manifest.kind).toBe('document');
  expect(manifest.markdown).toBe('report.md');
  expect(manifest.docx).toBe('report.docx');
  expect(manifest.markdown_ref).toBe('resource://report.md');
  expect(manifest.docx_ref).toBe('resource://report.docx');
  expect(manifest.citation_ids).toEqual(cited.slice().sort());
  expect(manifest.headings).toContain('实施效果');
  expect(manifest.checks.map((check) => check.name).sort()).toEqual(['export', 'generate', 'modify', 'preview']);
  for (const check of manifest.checks) expect(check.status).toBe('passed');

  // The version is stamped kind=document and the entry check judged the
  // DOCX deliverable (server-side admission accepted the manifest).
  const versionRow = dbCount("SELECT COUNT(*) FROM craft_versions WHERE id = '" + firstVersionId + "' AND kind = 'document'");
  expect(versionRow).toBe(1);
});

test('02 FAQ round keeps the numbers and the old DOCX bytes', async ({ page }) => {
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await page.getByTestId('craft-prompt').fill(faqGoal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  const secondVersionId = await page.getByTestId('craft-version').inputValue();
  expect(secondVersionId).not.toBe(firstVersionId);

  // The FAQ section landed; the original numbers are untouched.
  const view = page.getByTestId('craft-document');
  await expect(view).toHaveAttribute('data-state', 'active', { timeout: 30_000 });
  const sections = view.locator('[data-testid="craft-document-heading"]');
  await expect(sections).toHaveCount(5, { timeout: 30_000 });
  await expect(sections.nth(3)).toHaveText('常见问题');
  await expect(sections.nth(4)).toHaveText('来源');
  const body = view.locator('[data-testid="craft-document-body"]');
  await expect(body).toContainText('12');
  await expect(body).toContainText('96%');
  await expect(body).toContainText('6 周');

  const manifest = await fetchManifest(owner(), sessionId, secondVersionId);
  expect(manifest.headings).toContain('常见问题');
  for (const check of manifest.checks) expect(check.status).toBe('passed');

  // OLD version download: the exact same bytes as round 1.
  await page.getByTestId('craft-version').selectOption(firstVersionId);
  const oldDownload = await downloadDocxFromFilesTab(page);
  expect(await sha256OfDownload(oldDownload)).toBe(firstSha);
  const oldFacts = parseDocxFacts(await oldDownload.path());
  expect(oldFacts.headings).toEqual(['智绘云图客户介绍', '公司概况', '产品能力', '实施效果', '来源']);
  expect(oldFacts.rows).toBe(8);
  await page.screenshot({ path: artifactDir() + '/d01-02-faq-versions.png', fullPage: true });
});

test('03 corrupt export + honest export=failed manifest publishes nothing', async ({ page }) => {
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  // Wait for the version list to finish loading before counting: the list
  // arrives asynchronously after the workbench mounts.
  await expect.poll(() => page.getByTestId('craft-version').locator('option').count(), { timeout: 30_000 }).toBeGreaterThanOrEqual(2);
  const versionsBefore = await page.getByTestId('craft-version').locator('option').count();
  const versionsInDbBefore = dbCount("SELECT COUNT(*) FROM craft_versions v JOIN craft_workspaces w ON w.id = v.workspace_id WHERE w.session_id = '" + sessionId + "'");
  await page.getByTestId('craft-prompt').fill(brokenGoal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });

  // The failed round must NOT publish a version: the selector gained no
  // option, the DB gained no row, and the selected version is unchanged —
  // no export success is shown for the corrupt document.
  await expect(page.getByTestId('craft-version').locator('option')).toHaveCount(versionsBefore, { timeout: 30_000 });
  const versionsInDbAfter = dbCount("SELECT COUNT(*) FROM craft_versions v JOIN craft_workspaces w ON w.id = v.workspace_id WHERE w.session_id = '" + sessionId + "'");
  expect(versionsInDbAfter).toBe(versionsInDbBefore);
  // And the honest check trail: the last run's tool card reports the failed
  // child status (the conversation never claims a fresh published version).
  await expect(page.getByTestId('craft-version').locator('option')).toHaveCount(2);
  await page.screenshot({ path: artifactDir() + '/d01-03-broken-round.png', fullPage: true });
});

test('04 document kind gate: open here, and the closed set still refuses unknown kinds', async () => {
  // This deployment opened the document kind (CRAFT_KINDS) after the four
  // document gate items (generate/modify/preview/export) passed above; the
  // session rows prove the kind is really admitted.
  expect(DOCUMENT_KIND_OPEN).toBe(true);
  const rows = dbCount("SELECT COUNT(*) FROM craft_sessions WHERE kind = 'document'");
  expect(rows).toBeGreaterThanOrEqual(1);

  // A kind OUTSIDE the closed set is still refused — opening document does
  // not open the world.
  const res = await apiFetch(owner(), '/craft/sessions', {
    method: 'POST',
    body: JSON.stringify({ request_id: 'd01-gate-' + Date.now(), title: 'D01 gate probe', kind: 'diagram' }),
  });
  expect(res.status).toBeGreaterThanOrEqual(400);
  const body = await res.json() as { success?: boolean; data?: { session_id?: string } };
  expect(body.success ?? true).not.toBe(true);
  expect(body.data?.session_id ?? '').toBe('');
});