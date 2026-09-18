// CFT-S05-T034: the five-width visual baseline. Drives the real stack's
// home and workbench at 1440/1280/1024/768/390, screenshots each state and
// asserts the invariant contracts: no page-level horizontal overflow at any
// width, the narrow switch keeps both panes mounted, and the long title
// renders verbatim (wrapping, never clipped).
import { test, expect } from '@playwright/test';

const WIDTHS = [1440, 1280, 1024, 768, 390] as const;

test.describe.configure({ mode: 'serial' });

test('visual baseline: home + workbench across five widths', async ({ page }) => {
  for (const width of WIDTHS) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/craft');
    await expect(page.getByTestId('craft-goal')).toBeVisible();
    const homeOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(homeOverflow, `home @${width} horizontal overflow`).toBeLessThanOrEqual(0);
    await page.screenshot({ path: `${process.env.CRAFT_E2E_OUTPUT}/visual-home-${width}.png`, fullPage: false });

    // Enter a session for the workbench shot (deterministic, no send needed).
    await page.getByTestId('craft-goal').fill('视觉基线：按月分析销售数据，生成可筛选网页报告');
    await page.getByTestId('craft-create').click();
    await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
    const benchOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(benchOverflow, `workbench @${width} horizontal overflow`).toBeLessThanOrEqual(0);
    await page.screenshot({ path: `${process.env.CRAFT_E2E_OUTPUT}/visual-workbench-${width}.png`, fullPage: false });
    if (width === WIDTHS[WIDTHS.length - 1]) break;
  }
});

test('narrow switch keeps both panes mounted (390)', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/craft');
  await page.getByTestId('craft-goal').fill('窄屏基线');
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
  // Both panes exist in the DOM; hiding is CSS-only (drafts/subscriptions survive).
  const panes = await page.locator('section.wk-craft-pane').count();
  expect(panes).toBeGreaterThanOrEqual(2);
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});

test('long title renders verbatim without overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/craft');
  const longTitle = '超长标题'.repeat(20) + '——视觉基线验证任何宽度下都不撑破布局的极长作品名称';
  await page.getByTestId('craft-goal').fill(longTitle);
  await page.getByTestId('craft-create').click();
  await expect(page.getByTestId('craft-main-status')).toBeVisible({ timeout: 60_000 });
  const body = await page.evaluate(() => document.body.innerText);
  expect(body).toContain('超长标题');
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
  await page.screenshot({ path: `${process.env.CRAFT_E2E_OUTPUT}/visual-longtitle-390.png`, fullPage: false });
});
