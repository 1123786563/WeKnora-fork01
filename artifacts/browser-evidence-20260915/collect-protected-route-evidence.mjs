#!/usr/bin/env node
// Paired browser evidence for anonymous route guards and fixed viewports.
// No valid credentials, successful login, or business mutations are used.
import { createRequire } from 'node:module';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const require = createRequire(new URL('../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');

const outputDir = resolve(process.env.EVIDENCE_OUTPUT_DIR ?? 'artifacts/browser-evidence-20260915');
const viewports = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'laptop', width: 1024, height: 768 },
  { name: 'mobile', width: 390, height: 844 },
];
const targets = [
  { name: 'vue', baseURL: process.env.VUE_URL ?? 'http://127.0.0.1:5180' },
  { name: 'react', baseURL: process.env.REACT_URL ?? 'http://127.0.0.1:5181' },
];
const routes = [
  '/platform/apps',
  '/platform/apps?tab=connections',
  '/platform/apps/not-a-route',
  '/onboarding/workspace',
];

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch();
const results = {
  capturedAt: new Date().toISOString(),
  locale: 'zh-CN',
  viewports,
  routes,
  targets: [],
  evidenceBoundary: {
    included: ['anonymous route guard redirects', 'next query preservation', 'anonymous auth API error status', 'visible login error state', 'fixed viewport screenshots'],
    excluded: ['valid credentials', 'successful authentication', 'business mutations', 'authenticated tenant/permission states', 'Wails/iOS/Android'],
  },
};

for (const target of targets) {
  const targetResult = { name: target.name, baseURL: target.baseURL, viewports: [] };
  for (const viewport of viewports) {
    const context = await browser.newContext({ viewport, locale: 'zh-CN' });
    const page = await context.newPage();
    const browserErrors = [];
    const apiResponses = [];
    page.on('pageerror', (error) => browserErrors.push(`pageerror: ${error.message}`));
    page.on('console', (message) => { if (message.type() === 'error') browserErrors.push(`console: ${message.text()}`); });
    page.on('response', (response) => {
      if (response.url().includes('/api/v1/auth/')) {
        apiResponses.push({ method: response.request().method(), status: response.status(), url: response.url() });
      }
    });

    const routeResults = [];
    for (const route of routes) {
      const response = await page.goto(`${target.baseURL}${route}`, { waitUntil: 'networkidle', timeout: 20000 });
      await page.waitForTimeout(250);
      routeResults.push({
        route,
        responseStatus: response?.status() ?? null,
        finalURL: page.url(),
        heading: await page.locator('h1, h2, h3, h4').first().textContent().catch(() => null),
        bodyText: (await page.locator('body').innerText().catch(() => '')).replace(/\s+/g, ' ').trim().slice(0, 500),
      });
    }

    await page.goto(`${target.baseURL}/login`, { waitUntil: 'networkidle', timeout: 20000 });
    const email = page.locator('input[type="email"], input[placeholder*="邮箱"], input[type="text"]').first();
    const password = page.locator('input[type="password"]').first();
    const submit = page.getByRole('button', { name: /登录|sign in/i }).first();
    const loginError = { attempted: false, visible: false, text: null };
    if (await email.count() && await password.count() && await submit.count()) {
      loginError.attempted = true;
      await email.fill('anonymous-evidence@example.invalid');
      await password.fill('invalid-evidence-password');
      await submit.click();
      await page.waitForTimeout(700);
      const bodyText = (await page.locator('body').innerText()).replace(/\s+/g, ' ').trim();
      loginError.text = bodyText.slice(-500);
      loginError.visible = /失败|错误|invalid|failed|检查邮箱|密码/i.test(bodyText);
    }
    await page.screenshot({ path: resolve(outputDir, `${target.name}-${viewport.name}-protected-login.png`), fullPage: false });

    targetResult.viewports.push({ viewport, routes: routeResults, anonymousLoginError: loginError, authenticationAPIResponses: apiResponses, browserErrors });
    await context.close();
  }
  results.targets.push(targetResult);
}

await browser.close();
await writeFile(resolve(outputDir, 'protected-route-results.json'), `${JSON.stringify(results, null, 2)}\n`);
console.log(JSON.stringify({ capturedAt: results.capturedAt, viewports, targets: results.targets.map((target) => ({ name: target.name, viewports: target.viewports.map((item) => ({ viewport: item.viewport, routes: item.routes, anonymousLoginError: item.anonymousLoginError, authenticationAPIResponses: item.authenticationAPIResponses, browserErrors: item.browserErrors })) })) }, null, 2));
