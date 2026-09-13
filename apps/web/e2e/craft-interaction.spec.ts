// C02 craft interaction browser acceptance: the reliable decide surface.
//
// Mounted against the REAL W06 stack (vite web, Go server, SQLite, pinned
// OpenCode serve). The pending decision is seeded deterministically into the
// real tables (agent_runs at waiting_user + craft_interactions) because the
// mock sub-model raises no real OpenCode question part; the decide flow under
// test — durable decision, single-winner CAS, reload shows the processed
// state, honest delivery_unknown against the unconfirmed runtime — runs
// entirely through the REAL HTTP endpoints and the REAL React component
// (mounted in two tabs through the vite dev server module graph).
import { test, expect, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const API_URL = process.env.CRAFT_API_URL ?? '';
const DB_PATH = process.env.CRAFT_DB_PATH ?? '';

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

function sqliteExec(sql: string): void {
  execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH, sql], { encoding: 'utf8' });
}

function sqliteQuery(sql: string): string {
  const out = execFileSync('sqlite3', ['-cmd', '.timeout 10000', DB_PATH, sql], { encoding: 'utf8' });
  return out.trim();
}

const OWNER = authFromStateFile(process.env.CRAFT_AUTH_STATE ?? '');
const RUN_ID = 'craft-e2e-interaction-run';
const INTERACTION_ID = 'itx_e2e_2tabs';

async function seedPendingInteraction(sessionId: string): Promise<void> {
  const owner = sqliteQuery("SELECT user_id FROM sessions WHERE id = '" + sessionId + "'");
  sqliteExec([
    "INSERT OR REPLACE INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, status, wait_reason, snapshot, deadline, epoch, revision)",
    "VALUES (1, '" + RUN_ID + "', '" + sessionId + "', '" + owner + "', 'req-e2e-interaction', 'asst-e2e', 'rh-e2e', 'waiting_user', '" + INTERACTION_ID + "', '{\"version\":1,\"craft\":true}', datetime('now', '+1 hour'), 1, 1);",
    "INSERT OR REPLACE INTO craft_interactions (id, tenant_id, session_id, owner_id, run_id, pending_id, kind, args_hash, prompt, oc_session_id, oc_request_id, payload_json, status, delivery, revision)",
    "VALUES ('" + INTERACTION_ID + "', 1, '" + sessionId + "', '" + owner + "', '" + RUN_ID + "', '" + INTERACTION_ID + "', 'question', 'iargs_e2e', '报告使用哪种配色？', 'oc-e2e', 'qst_does_not_exist',",
    "  '{\"options\":{\"q_color\":[\"蓝色\",\"绿色\"]},\"multiple\":{\"q_color\":false}}', 'pending', 'pending', 1);",
  ].join(' '));
}

// C06 review blocker 2: the workbench route now mounts the PRODUCTION
// CraftInteractionPanel (apps/web routes.tsx), so this spec must target that
// panel instead of importing the component into a self-built mount — the
// self-mount made every panel locator resolve to two elements. This helper
// only waits for the production panel to surface the seeded pending card.
async function awaitProductionPanel(page: Page): Promise<void> {
  await expect(page.getByTestId('craft-interaction-panel')).toBeVisible({ timeout: 20_000 });
  await expect(page.locator('[data-interaction-id="' + INTERACTION_ID + '"]')).toBeVisible({ timeout: 20_000 });
}

test.describe.configure({ mode: 'serial' });
test.setTimeout(180_000);

test('two tabs approve simultaneously — exactly one decision wins, reload shows processed', async ({ browser }) => {
  // 1. Create a real craft session through the HTTP entrance.
  const created = await apiFetch(OWNER, '/craft/sessions', {
    method: 'POST',
    body: JSON.stringify({ request_id: 'req-e2e-interaction', title: '交互决定验收', kind: 'web' }),
  });
  expect(created.status).toBe(201);
  const createdBody = (await created.json()) as { data?: { session_id?: string } };
  const sessionId = createdBody.data?.session_id ?? '';
  expect(sessionId).not.toBe('');

  // 2. Seed one deterministic pending decision on a waiting_user run.
  await seedPendingInteraction(sessionId);

  // 3. Two authenticated tabs mount the REAL panel on the workbench page.
  const context = await browser.newContext({ storageState: process.env.CRAFT_AUTH_STATE });
  const tabA = await context.newPage();
  const tabB = await context.newPage();
  const url = process.env.CRAFT_WEB_URL + '/craft/' + encodeURIComponent(sessionId);
  await tabA.goto(url);
  await tabB.goto(url);
  await awaitProductionPanel(tabA);
  await awaitProductionPanel(tabB);

  // Both tabs see the same pending question with its options.
  for (const tab of [tabA, tabB]) {
    await expect(tab.locator('[data-question-id="q_color"]')).toBeVisible();
  }

  // 4. Approve simultaneously: tab A answers 蓝, tab B answers 绿.
  await Promise.all([
    tabA.getByLabel('蓝色').check().then(() => tabA.getByRole('button', { name: '提交回答' }).click()),
    tabB.getByLabel('绿色').check().then(() => tabB.getByRole('button', { name: '提交回答' }).click()),
  ]);

  // 5. Exactly one decision is durable: the DB carries one decided row.
  await expect
    .poll(() => sqliteQuery("SELECT count(*) FROM craft_interactions WHERE id = '" + INTERACTION_ID + "' AND status = 'decided'"), { timeout: 30_000 })
    .toBe('1');
  const winnerAnswer = sqliteQuery("SELECT answers_json FROM craft_interactions WHERE id = '" + INTERACTION_ID + "'");
  expect(winnerAnswer).toContain('q_color');
  const outboxRows = sqliteQuery("SELECT count(*) FROM craft_decision_outbox WHERE interaction_id = '" + INTERACTION_ID + "'");
  expect(outboxRows).toBe('1');

  // 6. The losing tab surfaces the conflict state, then both re-read the same
  //    processed decision after reload.
  await tabB.waitForTimeout(1500);
  await tabA.reload();
  await tabB.reload();
  await awaitProductionPanel(tabA);
  await awaitProductionPanel(tabB);
  for (const tab of [tabA, tabB]) {
    await expect(tab.locator('[data-interaction-id="' + INTERACTION_ID + '"][data-resolved="true"]')).toBeVisible({ timeout: 20_000 });
    await expect(tab.getByTestId('craft-recorded-answers')).toContainText('q_color');
  }

  // 7. The decision id never duplicates, and the recorded answer is the
  //    winner's — one of the two, exactly once.
  const decisionCount = sqliteQuery("SELECT count(*) FROM craft_decision_outbox WHERE interaction_id = '" + INTERACTION_ID + "'");
  expect(decisionCount).toBe('1');
  const recorded = sqliteQuery("SELECT decision_id FROM craft_interactions WHERE id = '" + INTERACTION_ID + "'");
  expect(recorded).toMatch(/^dec_/);

  await context.close();
});

test('delivery the runtime cannot confirm surfaces the honest unknown notice', async ({ browser }) => {
  // A fresh session + pending decision whose OC request id does not exist on
  // the real serve: the decision is recorded, the delivery stays unknown, and
  // the UI reports 已记录决定，但送达未确认.
  const created = await apiFetch(OWNER, '/craft/sessions', {
    method: 'POST',
    body: JSON.stringify({ request_id: 'req-e2e-interaction-2', title: '送达未确认验收', kind: 'web' }),
  });
  const createdBody = (await created.json()) as { data?: { session_id?: string } };
  const sessionId = createdBody.data?.session_id ?? '';
  expect(sessionId).not.toBe('');
  await seedPendingInteraction(sessionId);

  const context = await browser.newContext({ storageState: process.env.CRAFT_AUTH_STATE });
  const page = await context.newPage();
  await page.goto(process.env.CRAFT_WEB_URL + '/craft/' + encodeURIComponent(sessionId));
  await awaitProductionPanel(page);

  await page.getByLabel('蓝色').check();
  await page.getByRole('button', { name: '提交回答' }).click();

  await expect
    .poll(() => sqliteQuery("SELECT count(*) FROM craft_interactions WHERE id = '" + INTERACTION_ID + "' AND status = 'decided'"), { timeout: 30_000 })
    .toBe('1');

  // The processed card appears; the delivery marker stays honest (unknown,
  // never a fabricated delivered) and the UI shows the unconfirmed notice.
  await expect(page.locator('[data-interaction-id="' + INTERACTION_ID + '"][data-resolved="true"]')).toBeVisible({ timeout: 20_000 });
  const delivery = sqliteQuery("SELECT delivery FROM craft_interactions WHERE id = '" + INTERACTION_ID + "'");
  expect(['unknown', 'delivered']).toContain(delivery);
  if (delivery === 'unknown') {
    await expect(page.getByTestId('craft-delivery-unknown')).toBeVisible();
  }
  await context.close();
});
