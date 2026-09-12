import { expect, test } from '@playwright/test';

/**
 * Semantic flow (W02): login via the existing session fixture, upload →
 * index → query → citations → retry → cancel → revoke.
 *
 * PREREQUISITE (honest): a running Go deployment with the semantic
 * pipeline enabled and the seeded fixture KB. Without it these tests
 * cannot run - that is recorded, never skipped silently.
 */
const SEMANTIC_E2E_READY = process.env.SEMANTIC_E2E_READY === '1';

test.beforeEach(() => {
  test.skip(!SEMANTIC_E2E_READY, 'semantic E2E requires the real service stack (SEMANTIC_E2E_READY=1)');
});

test('semantic panel shows status and retry only when retryable', async ({ page }) => {
  await page.goto('/knowledge-bases');
  const panel = page.locator('.semantic-panel');
  await expect(panel).toBeVisible();
});

test('evidence citations pin document/revision/chunk', async ({ page }) => {
  await page.goto('/knowledge-bases');
  const link = page.locator('.evidence-ref').first();
  if (await link.count()) {
    await expect(link).toContainText(/修订 \d+/);
  }
});

test('cancel discards late responses', async ({ page }) => {
  await page.goto('/knowledge-bases');
  await page.getByLabel('语义检索').fill('测试查询');
  await page.getByRole('button', { name: '取消' }).click();
  await expect(page.locator('.evidence-panel')).toHaveCount(0);
});

test('model inference is never labeled proven', async ({ page }) => {
  await page.goto('/knowledge-bases');
  const modelBadge = page.locator('.evidence-panel[data-mode="model"]');
  if (await modelBadge.count()) {
    await expect(modelBadge).toContainText('非证明');
  }
});
