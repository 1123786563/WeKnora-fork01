#!/usr/bin/env node
// Read-only anonymous browser evidence. No credentials, mutations, or mocks.
import { createRequire } from 'node:module';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const require = createRequire(new URL('../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');

const outputDir = resolve(process.env.EVIDENCE_OUTPUT_DIR ?? 'artifacts/browser-evidence-20260916');
const viewport = { width: 1440, height: 900 };
const targets = [
  { name: 'vue', baseURL: process.env.VUE_URL ?? 'http://127.0.0.1:5180' },
  { name: 'react', baseURL: process.env.REACT_URL ?? 'http://127.0.0.1:5181' },
];
const routes = ['/login', '/register', '/platform/apps', '/platform/settings'];

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch();
const results = {
  capturedAt: new Date().toISOString(),
  viewport,
  locale: 'zh-CN',
  mode: 'fresh anonymous browser contexts',
  targets: [],
  evidenceBoundary: {
    included: [
      'login and registration public route renderability',
      'anonymous protected-route redirects for /platform/apps and settings Portal',
      'authentication API response statuses observed by the browser',
      'fixed viewport screenshots',
    ],
    excluded: [
      'valid credentials and authenticated business data',
      'real registration or other business mutations',
      'tenant/permission variants',
      'Wails, iOS, Android',
    ],
  },
};

for (const target of targets) {
  const context = await browser.newContext({ viewport, locale: 'zh-CN' });
  const page = await context.newPage();
  const browserErrors = [];
  const authResponses = [];
  page.on('pageerror', (error) => browserErrors.push(`pageerror: ${error.message}`));
  page.on('console', (message) => {
    if (message.type() === 'error') browserErrors.push(`console: ${message.text()}`);
  });
  page.on('response', (response) => {
    if (response.url().includes('/api/v1/auth/')) {
      authResponses.push({ method: response.request().method(), status: response.status(), url: response.url() });
    }
  });

  const routeResults = [];
  for (const route of routes) {
    const response = await page.goto(`${target.baseURL}${route}`, { waitUntil: 'networkidle', timeout: 20000 });
    await page.waitForTimeout(400);
    const snapshot = await page.evaluate(() => ({
      title: document.title,
      heading: document.querySelector('h1,h2,h3,h4')?.textContent?.trim() ?? null,
      bodyText: document.body.innerText.replace(/\s+/g, ' ').trim().slice(0, 900),
      inputs: [...document.querySelectorAll('input')].map((input) => ({ type: input.type, name: input.name, placeholder: input.placeholder })),
      buttons: [...document.querySelectorAll('button')].map((button) => button.innerText.trim()).filter(Boolean).slice(0, 12),
      dialogs: document.querySelectorAll('[role="dialog"]').length,
    }));
    const safeName = route.replaceAll('/', '-').replace(/^-/, 'root').replaceAll('?', '_').replaceAll('&', '_').replaceAll('=', '-');
    const screenshot = `${target.name}-${safeName}-${viewport.width}x${viewport.height}.png`;
    await page.screenshot({ path: resolve(outputDir, screenshot), fullPage: false });
    routeResults.push({
      route,
      responseStatus: response?.status() ?? null,
      finalURL: page.url(),
      redirectedToLogin: new URL(page.url()).pathname === '/login',
      snapshot,
      screenshot,
    });
  }

  results.targets.push({ name: target.name, baseURL: target.baseURL, routes: routeResults, authResponses, browserErrors });
  await context.close();
}

await browser.close();
await writeFile(resolve(outputDir, 'anonymous-auth-results.json'), `${JSON.stringify(results, null, 2)}\n`);
console.log(JSON.stringify({ outputDir, capturedAt: results.capturedAt, targets: results.targets }, null, 2));
