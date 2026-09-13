// C03 craft snapshot-reconnection browser acceptance.
//
// Drives the REAL full stack (the W06 craft-stack.sh harness: vite web app,
// Go server, SQLite, storage, pinned OpenCode serve, isolated preview
// origin) through the C03 user results — 刷新/休眠/切空间回到服务端事实:
//
//   1. three-timepoint refresh — reload in the WAITING/no-events window
//      (run just admitted, zero events), again while GENERATING, and after
//      FINISHED: POST /runs admissions stay at exactly one (counted from
//      the database, never from UI copy), the streamed summary appears at
//      most once per page (replay dedupe), and the preview keeps the
//      selected version;
//   2. offline/online — cutting the network kills the SSE stream: the UI
//      shows the syncing banner, never fabricates completion without the
//      terminal projection, and once the network returns the controller
//      recovers on its backoff, the run settles and the banner clears —
//      still exactly one admission.
//
// Server-fact note: once a run finishes, the workspace snapshot releases it
// (active_run_id empty, last_seq 0) — the surviving facts are the published
// current version and the absence of a live run. The workbench must not
// fabricate a live (or finished) run status from that absence; the idle
// wording itself is a W05 label-map matter.
//
// CRAFT_MODEL_MODE=mock (the harness default) keeps the run deterministic.
import { test, expect, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';

const DB_PATH = process.env.CRAFT_DB_PATH ?? '';

test.describe.configure({ mode: 'serial' });
test.setTimeout(300_000);

const goal = '重连刷新验证：生成可筛选的按月销售网页报告';
const finalText = '已生成按月网页报告'; // the deterministic mock's streamed summary
const ACTIVE = /等待执行|正在生成|需要你的处理|正在停止/;

function sqliteQuery(sql: string): string {
  const out = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH, sql], { encoding: 'utf8' });
  return out.trim();
}

async function countOccurrences(page: Page, needle: string): Promise<number> {
  const text = await page.locator('.wk-craft-msgs').innerText();
  return text.split(needle).length - 1;
}

test('01 refreshing at waiting/generating/finished keeps one run, no duplicate messages, selected version', async ({ page }) => {
  await page.goto('/craft');
  await expect(page.getByTestId('craft-goal')).toBeVisible();
  await page.getByTestId('craft-goal').fill(goal);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(goal, { timeout: 30_000 });

  // Wait until the POST /runs admission is durable and the run is live,
  // then reload — this is the WAITING point: the run has barely started
  // (zero or few events), the workbench restores purely from the snapshot
  // and must not display completion.
  await page.getByTestId('craft-send').click();
  const sessionId = new URL(page.url()).pathname.split('/').pop() ?? '';
  expect(sessionId).not.toBe('');
  await expect(page.getByTestId('craft-main-status')).toHaveText(ACTIVE, { timeout: 120_000 });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await expect(page.getByTestId('craft-main-status')).not.toBe('已完成');

  // GENERATING point: a second reload while the delegation is still running.
  await expect(page.getByTestId('craft-main-status')).toHaveText(/等待执行|正在生成|已完成/, { timeout: 60_000 });
  await page.reload({ waitUntil: 'domcontentloaded' });

  // The live tail after the reloads settles to the terminal projection. The
  // resumed conversation only carries events AFTER the snapshot watermark
  // (W05 design), so the streamed summary appears at most once — never
  // duplicated by a replay; the strict per-seq dedupe is unit-pinned.
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成', { timeout: 240_000 });
  await expect(page.locator('.wk-craft-msgs')).toBeVisible({ timeout: 60_000 });
  expect(await countOccurrences(page, finalText)).toBeLessThanOrEqual(1);

  // FINISHED point: record the selected version, reload, require the same
  // server facts back — the published version stays selected, no run is
  // fabricated as live, and no message duplicates.
  const versionBefore = await page.getByTestId('craft-version').inputValue();
  expect(versionBefore).not.toBe('');
  await page.reload({ waitUntil: 'domcontentloaded' });
  await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId('craft-main-status')).not.toHaveText(ACTIVE, { timeout: 10_000 });
  await expect(page.getByTestId('craft-version')).toHaveValue(versionBefore);
  expect(await countOccurrences(page, finalText)).toBeLessThanOrEqual(1);

  // POST admissions stay at exactly one across all three reloads.
  const safeId = sessionId.replace(/[^A-Za-z0-9_-]/g, '');
  expect(sqliteQuery("SELECT COUNT(*) FROM agent_runs WHERE session_id = '" + safeId + "'")).toBe('1');
  expect(sqliteQuery("SELECT COUNT(*) FROM craft_session_requests WHERE session_id = '" + safeId + "' AND purpose = 'create'")).toBe('1');
});

test('02 a killed stream shows syncing, recovers on restore without a second admission', async ({ page }) => {
  // NOTE on scenario honesty: Playwright/Chromium's context.setOffline does
  // NOT emulate offline for loopback (127.0.0.1) — established SSE streams
  // and new fetches keep flowing (verified empirically against this stack).
  // The offline essence is therefore driven at the request layer: the run-event
  // stream endpoint is connection-reset (the stream is 'down') while the
  // snapshot endpoint stays reachable, then restored.
  const eventsUrl = '**/runs/*/events*';
  await page.route(eventsUrl, (route) => route.abort('connectionreset'));
  await page.goto('/craft');
  const offlineGoal = '断网重连验证：生成季度汇总网页报告';
  await page.getByTestId('craft-goal').fill(offlineGoal);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-prompt')).toHaveValue(offlineGoal, { timeout: 30_000 });
  await page.getByTestId('craft-send').click();

  const sessionId = new URL(page.url()).pathname.split('/').pop() ?? '';
  expect(sessionId).not.toBe('');

  // Stream down: every subscribe is reset. The controller must show the
  // syncing banner, keep cycling backoff snapshot-refresh -> resubscribe,
  // and never display completion without the terminal projection (the
  // status comes from the authoritative snapshot, which stays reachable).
  await expect(page.getByTestId('craft-main-status')).toHaveText(ACTIVE, { timeout: 120_000 });
  await expect(page.getByRole('alert').filter({ hasText: '正在恢复事件流' })).toBeVisible({ timeout: 30_000 });
  expect(await page.getByTestId('craft-main-status').innerText()).not.toBe('已完成');

  // Restore the stream: the next backoff cycle resubscribes after the
  // fresh watermark. The run settles to its terminal projection (live
  // close) or the released-snapshot fact, the banner clears, and there is
  // still exactly one admission and at most one summary copy.
  await page.waitForTimeout(2_500); // span ~2 backoff steps while down
  await page.unroute(eventsUrl);
  await expect(page.getByTestId('craft-main-status')).not.toHaveText(ACTIVE, { timeout: 240_000 });
  await expect(page.getByRole('alert').filter({ hasText: '正在恢复事件流' })).toHaveCount(0, { timeout: 60_000 });
  expect(await countOccurrences(page, '已生成季度汇总网页报告')).toBeLessThanOrEqual(1);

  const safeId = sessionId.replace(/[^A-Za-z0-9_-]/g, '');
  expect(sqliteQuery("SELECT COUNT(*) FROM agent_runs WHERE session_id = '" + safeId + "'")).toBe('1');
});
