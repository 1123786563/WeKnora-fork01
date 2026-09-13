// D03 craft slides — browser acceptance over the real full stack.
//
// Drives the W06 harness stack (vite, Go server, SQLite, storage, pinned
// OpenCode serve, isolated preview origin) through the D03 acceptance: the
// mock sub-model's slides round writes a real 5-page deck (PPTX OOXML with
// notes + an embedded chart picture, a 5-page PDF and per-page SVG previews
// — the python-pptx + impress + pdftocairo render itself is proven inside
// the craft image, see the D03 report), preview.json, manifest.json and the
// static deck viewer. The spec verifies in the browser: per-page navigation
// and SCREENSHOTS (Chinese / chart / long title), keyboard arrows, the
// page-3-only modification (every other slide's stored XML byte-equal, page
// count stable), page-3 citations resolving to the deck's source registry
// and sources slide, byte-stable old-version PPTX/PDF downloads, and the
// craft.kinds feature gate staying fail-closed for slides until the
// render/pages/sources evidence chain justifies enabling it.
import { test, expect, type Download, type Frame, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const API_URL = process.env.CRAFT_API_URL ?? '';
const DB_PATH = process.env.CRAFT_DB_PATH ?? '';
const KINDS = (process.env.CRAFT_KINDS ?? 'web').split(',').map((kind) => kind.trim());

test.describe.configure({ mode: 'serial' });
test.setTimeout(300_000);

const proposalGoal = '把客户方案做成5页演示稿，导出PPTX，含图表与来源页';
const edit3Goal = '演示稿只修改第3页的结论为一次交付上线，其余页保持不变';

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

// Opens the Files tab and downloads exactly the requested member of the
// SELECTED version — a fixed (versionId, path) pair, never the latest file.
async function downloadFromFilesTab(page: Page, name: string): Promise<Download> {
  await page.getByRole('button', { name: '版本历史' }).click();
  const row = page.locator('tr', { hasText: name });
  await expect(row).toBeVisible({ timeout: 30_000 });
  const pending = page.waitForEvent('download');
  await row.getByRole('button').click();
  const download = await pending;
  expect(download.suggestedFilename()).toBe(name);
  return download;
}

// Parses the REAL stored OOXML of a downloaded deck (stdlib zipfile on the
// host): per-slide sha256 of the stored XML, the slide count, slide 3's
// picture/notes facts and the sources-slide text.
const PPTX_FACTS_SCRIPT = [
  'import hashlib, json, sys, zipfile',
  'zf = zipfile.ZipFile(sys.argv[1])',
  'slides = sorted(n for n in zf.namelist() if n.startswith("ppt/slides/slide") and n.endswith(".xml"))',
  'facts = {"slide_shas": {n: hashlib.sha256(zf.read(n)).hexdigest() for n in slides}, "slide_count": len(slides)}',
  'if "ppt/slides/slide3.xml" in facts["slide_shas"]:',
  '    s3 = zf.read("ppt/slides/slide3.xml").decode("utf-8")',
  '    facts["slide3_has_pic"] = "<p:pic>" in s3',
  'if "ppt/notesSlides/notesSlide3.xml" in zf.namelist():',
  '    facts["notes3"] = zf.read("ppt/notesSlides/notesSlide3.xml").decode("utf-8")',
  'if "ppt/slides/slide5.xml" in facts["slide_shas"]:',
  '    facts["sources_slide"] = zf.read("ppt/slides/slide5.xml").decode("utf-8")',
  'print(json.dumps(facts))',
].join(String.fromCharCode(10));

interface PptxFacts {
  slide_shas: Record<string, string>;
  slide_count: number;
  slide3_has_pic?: boolean;
  notes3?: string;
  sources_slide?: string;
}

function parsePptxFacts(path: string | null): PptxFacts {
  if (path === null) throw new Error('download produced no file');
  const out = execFileSync('python3', ['-c', PPTX_FACTS_SCRIPT, path], { encoding: 'utf8' });
  return JSON.parse(out.trim()) as PptxFacts;
}

function pdfPageCount(path: string | null): number {
  if (path === null) throw new Error('download produced no file');
  const raw = readFileSync(path, 'utf8');
  return (raw.match(/\/Type\s*\/Page[^s]/g) ?? []).length;
}

interface SlidesManifestFacts {
  kind: string;
  pptx_ref: string;
  page_refs: string[];
  slide_count: number;
  overflow_pages: number[];
  pages: { index: number; title: string; sources: string[]; min_font_pt: number; images: number }[];
  sources: { id: string; title: string }[];
  checks: { name: string; status: string }[];
}

async function fetchManifest(auth: AuthedContext, sessionId: string, versionId: string): Promise<SlidesManifestFacts> {
  const res = await apiFetch(auth, '/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/files/manifest.json');
  expect(res.status).toBe(200);
  return await res.json() as SlidesManifestFacts;
}

// Fetches one version-relative file through the preview capability URL the
// iframe itself was loaded from (the same authorized origin the viewer
// loads images from). Uses the browser context's request client: the
// preview origin's CSP allows image loads but no in-page fetch(), so the
// text is read via an authorized GET instead.
async function fetchVersionAsset(page: Page, frame: Frame, path: string): Promise<string> {
  const response = await page.context().request.get(new URL(path, frame.url()).toString());
  expect(response.status()).toBe(200);
  return await response.text();
}

let sessionId = '';
let firstVersionId = '';
let firstPptxSha = '';
let firstPdfSha = '';
let firstSlides: PptxFacts | null = null;
let firstSvgByText: Record<string, string> = {};
const owner = () => authFromStateFile(process.env.CRAFT_AUTH_STATE as string);

test('01 create, upload, 5-page deck, per-page screenshots + navigation + modify channel', async ({ page }) => {
  // Record the skill viewer's modify channel: the static page posts
  // craft:slides:modify to the parent; the assembly attaches the current
  // version id (integration wiring, same as EntryPath).
  await page.addInitScript(() => {
    (window as unknown as { __craftSlidesModifications: unknown[] }).__craftSlidesModifications = [];
    window.addEventListener('message', (event: MessageEvent) => {
      const data = event.data as { type?: string } | null;
      if (data !== null && data.type === 'craft:slides:modify') {
        (window as unknown as { __craftSlidesModifications: unknown[] }).__craftSlidesModifications.push(data);
      }
    });
  });

  await page.goto('/craft');
  await expect(page.getByTestId('craft-goal')).toBeVisible();
  await page.getByTestId('craft-goal').fill(proposalGoal);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(proposalGoal, { timeout: 30_000 });
  await page.getByTestId('craft-upload').setInputFiles('e2e/fixtures/craft-proposal.md');
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  sessionId = new URL(page.url()).pathname.split('/').pop() ?? '';
  expect(sessionId).not.toBe('');
  firstVersionId = await page.getByTestId('craft-version').inputValue();
  expect(firstVersionId).not.toBe('');

  const frame = await previewFrame(page);
  await frame.waitForSelector('[data-testid="slide-image"]', { timeout: 60_000 });
  await expect(frame.locator('[data-testid="slide-title"]')).toContainText('智绘云图客户方案');

  // Five rendered pages, one thumbnail each; click through every page and
  // screenshot it. The SVG each <img> loads is fetched and content-checked
  // INSIDE the authorized preview origin (Chinese text, long title on page
  // 1, the bar chart on page 3, the kc_ sources on page 5).
  const thumbs = frame.locator('[data-testid="slide-thumb"]');
  await expect(thumbs).toHaveCount(5);
  for (let n = 1; n <= 5; n += 1) {
    await frame.locator('[data-testid="slide-thumb"][data-page="' + n + '"]').click();
    await expect(frame.locator('[data-testid="slide-image"]')).toHaveAttribute('src', new RegExp('pages\\/page-' + n + '\\.svg$'), { timeout: 30_000 });
    await expect(frame.locator('[data-testid="slide-page-number"]')).toHaveText('第 ' + n + '/5 页');
    const svg = await fetchVersionAsset(page, frame, 'pages/page-' + n + '.svg');
    firstSvgByText['page-' + n] = svg;
    expect(svg).toContain('<svg');
    if (n === 1) {
      expect(svg).toContain('智绘云图客户方案');
      expect(svg).toContain('智能知识中台首期建设建议');
    }
    if (n === 2) expect(svg).toContain('客户背景');
    if (n === 3) {
      expect(svg).toContain('核心方案');
      expect(svg).toContain('季度收入');
      expect((svg.match(/<rect /g) ?? []).length).toBeGreaterThanOrEqual(5);
      expect(svg).toContain('结论：分三期交付');
    }
    if (n === 4) expect(svg).toContain('实施计划');
    if (n === 5) {
      expect(svg).toContain('kc_sales');
      expect(svg).toContain('kc_plan');
    }
    await page.screenshot({ path: artifactDir() + '/d03-01-page' + n + '.png', fullPage: true });
  }

  // Keyboard navigation: ArrowRight/ArrowLeft flip pages (from page 1).
  await frame.locator('[data-testid="slide-thumb"][data-page="1"]').click();
  await expect(frame.locator('[data-testid="slide-page-number"]')).toHaveText('第 1/5 页');
  await frame.locator('[data-testid="slide-image"]').click();
  await page.keyboard.press('ArrowRight');
  await expect(frame.locator('[data-testid="slide-page-number"]')).toHaveText('第 2/5 页');
  await page.keyboard.press('ArrowRight');
  await expect(frame.locator('[data-testid="slide-page-number"]')).toHaveText('第 3/5 页');
  await page.keyboard.press('ArrowLeft');
  await expect(frame.locator('[data-testid="slide-page-number"]')).toHaveText('第 2/5 页');

  // The modify channel: on page 3 the button carries the EXPLICIT page
  // number and posts it to the parent window.
  await frame.locator('[data-testid="slide-thumb"][data-page="3"]').click();
  await expect(frame.locator('[data-testid="slide-modify"]')).toHaveAttribute('data-page', '3');
  await frame.locator('[data-testid="slide-modify"]').click();
  // postMessage delivery to the parent's event loop is asynchronous, so the
  // recorded messages are polled, not read synchronously.
  await expect.poll(async () => await page.evaluate(
    () => (window as unknown as { __craftSlidesModifications: { page: number }[] }).__craftSlidesModifications,
  )).toEqual([{ type: 'craft:slides:modify', page: 3 }]);

  // The downloads exist in the viewer as version-relative links.
  await expect(frame.locator('[data-testid="slide-download-pptx"]')).toHaveAttribute('href', 'report.pptx');
  await expect(frame.locator('[data-testid="slide-download-pdf"]')).toHaveAttribute('href', 'report.pdf');

  // Download the REAL deck + PDF through the Files tab (fixed version+path)
  // and parse the stored OOXML: 5 slides, chart picture + cited notes on
  // page 3.
  const pptx = await downloadFromFilesTab(page, 'report.pptx');
  firstPptxSha = await sha256OfDownload(pptx);
  firstSlides = parsePptxFacts(await pptx.path());
  expect(firstSlides.slide_count).toBe(5);
  expect(firstSlides.slide3_has_pic).toBe(true);
  expect(firstSlides.notes3).toContain('kc_sales');
  expect(firstSlides.notes3).toContain('kc_market');

  const pdf = await downloadFromFilesTab(page, 'report.pdf');
  firstPdfSha = await sha256OfDownload(pdf);
  expect(pdfPageCount(await pdf.path())).toBe(5);

  // manifest.json records the slide facts, every page ref and the gate
  // checks (visual honestly not_run — human acceptance).
  const manifest = await fetchManifest(owner(), sessionId, firstVersionId);
  expect(manifest.kind).toBe('slides');
  expect(manifest.pptx_ref).toBe('resource://report.pptx');
  expect(manifest.slide_count).toBe(5);
  expect(manifest.page_refs).toHaveLength(5);
  expect(manifest.overflow_pages).toEqual([]);
  for (const check of manifest.checks) {
    if (check.name === 'visual') expect(check.status).toBe('not_run');
    else expect(check.status).toBe('passed');
  }
  expect(manifest.checks.map((check) => check.name).sort()).toEqual(['pages', 'render', 'sources', 'visual']);
});

test('02 modify ONLY page 3: other slides byte-equal, count stable, old version SHAs intact', async ({ page }) => {
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  await page.getByTestId('craft-prompt').fill(edit3Goal);
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  const secondVersionId = await page.getByTestId('craft-version').inputValue();
  expect(secondVersionId).not.toBe(firstVersionId);

  const frame = await previewFrame(page);
  await frame.waitForSelector('[data-testid="slide-image"]', { timeout: 60_000 });

  // Page count did not change; page 3 carries the new conclusion; every
  // other page's preview text is byte-identical to round 1.
  await expect(frame.locator('[data-testid="slide-thumb"]')).toHaveCount(5);
  for (let n = 1; n <= 5; n += 1) {
    const svg = await fetchVersionAsset(page, frame, 'pages/page-' + n + '.svg');
    if (n === 3) {
      expect(svg).toContain('结论：首期直接上线智能问答');
      expect(svg).not.toContain('结论：分三期交付');
    } else {
      expect(svg).toBe(firstSvgByText['page-' + n]);
    }
  }
  await frame.locator('[data-testid="slide-thumb"][data-page="3"]').click();
  await expect(frame.locator('[data-testid="slide-page-number"]')).toHaveText('第 3/5 页');
  await page.screenshot({ path: artifactDir() + '/d03-02-edit3-page3.png', fullPage: true });

  // The new deck: slides 1/2/4/5 stored XML byte-equal (sha), slide 3
  // changed, slide count still 5, notes still cite the same sources.
  const newPptx = await downloadFromFilesTab(page, 'report.pptx');
  const newSlides = parsePptxFacts(await newPptx.path());
  expect(newSlides.slide_count).toBe(5);
  const first = firstSlides as PptxFacts;
  for (const name of Object.keys(first.slide_shas)) {
    if (name === 'ppt/slides/slide3.xml') {
      expect(newSlides.slide_shas[name]).not.toBe(first.slide_shas[name]);
    } else {
      expect(newSlides.slide_shas[name]).toBe(first.slide_shas[name]);
    }
  }
  expect(newSlides.notes3).toContain('kc_sales');
  expect(newSlides.notes3).toContain('一次性交付');

  // The new version's manifest keeps the same structure.
  const manifest = await fetchManifest(owner(), sessionId, secondVersionId);
  expect(manifest.slide_count).toBe(5);
  expect(manifest.page_refs).toHaveLength(5);

  // OLD version downloads keep their exact bytes (immutability).
  await page.getByTestId('craft-version').selectOption(firstVersionId);
  const oldPptx = await downloadFromFilesTab(page, 'report.pptx');
  expect(await sha256OfDownload(oldPptx)).toBe(firstPptxSha);
  const oldPdf = await downloadFromFilesTab(page, 'report.pdf');
  expect(await sha256OfDownload(oldPdf)).toBe(firstPdfSha);
  await page.screenshot({ path: artifactDir() + '/d03-02-versions.png', fullPage: true });
});

test('03 page-3 citations resolve to the deck source registry and sources slide', async ({ page }) => {
  await page.goto('/craft/' + encodeURIComponent(sessionId));
  // The version select fills asynchronously after the workbench mounts;
  // wait for the current version before reading manifest facts.
  await expect.poll(() => page.getByTestId('craft-version').inputValue(), { timeout: 30_000 }).not.toBe('');
  const versionId = await page.getByTestId('craft-version').inputValue();
  const manifest = await fetchManifest(owner(), sessionId, versionId);
  const registry = manifest.sources.map((source) => source.id);
  expect(registry.length).toBeGreaterThanOrEqual(4);

  // Page 3's cited ids exist in the registry.
  const page3 = manifest.pages.find((p) => p.index === 3);
  expect(page3).toBeDefined();
  expect(page3?.sources.length).toBeGreaterThanOrEqual(2);
  for (const cited of page3?.sources ?? []) expect(registry).toContain(cited);

  // The citation ids exist in the REAL stored deck: the speaker notes of
  // slide 3 and the trailing sources slide.
  const pptx = await downloadFromFilesTab(page, 'report.pptx');
  const facts = parsePptxFacts(await pptx.path());
  for (const cited of page3?.sources ?? []) {
    expect(facts.notes3 ?? '').toContain(cited);
    expect(facts.sources_slide ?? '').toContain(cited);
  }
});

test('04 slides kind gate: fail-closed until the deployment opens it', async () => {
  // The deployment gate (WEKNORA_CRAFT_KINDS, CRAFT_KINDS in the stack
  // harness) owns this decision. With the kind CLOSED (the default web-
  // only gate) creating one must be refused; once the evidence chain (this
  // spec + the image acceptance + the D01 wiring: EntryPath,
  // PreviewableKind, the server-side manifest admission) is green the kind
  // opens and creation is admitted — both branches are asserted honestly.
  const res = await apiFetch(owner(), '/craft/sessions', {
    method: 'POST',
    body: JSON.stringify({ request_id: 'd03-gate-' + Date.now(), title: 'D03 gate probe', kind: 'slides' }),
  });
  expect(DB_PATH).not.toBe('');
  if (!KINDS.includes('slides')) {
    expect(res.status).toBe(503);
    const body = await res.json() as { success?: boolean; data?: { session_id?: string } };
    expect(body.success ?? true).not.toBe(true);
    expect(body.data?.session_id ?? '').toBe('');
    const count = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH,
      "SELECT COUNT(*) FROM craft_sessions WHERE kind = 'slides'"], { encoding: 'utf8' }).trim();
    expect(count).toBe('0');
    // The accepted rounds above ran as kind=web sessions (the only open kind)
    // while producing slides deliverables — sanity check from the DB.
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
    "SELECT COUNT(*) FROM craft_sessions WHERE kind = 'slides'"], { encoding: 'utf8' }).trim();
  expect(Number(count)).toBeGreaterThanOrEqual(1);
});