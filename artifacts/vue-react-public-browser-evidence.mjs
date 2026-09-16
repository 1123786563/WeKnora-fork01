#!/usr/bin/env node
// Read-only public browser evidence. No credentials, API mutations, or mocks.
import { chromium } from 'playwright-core';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

const outputDir = resolve(process.argv[2] ?? 'artifacts/browser-evidence-20260915');
const viewport = { width: 1355, height: 776 };
const targets = [
  { name: 'vue', baseURL: process.env.VUE_URL ?? 'http://127.0.0.1:5180' },
  { name: 'react', baseURL: process.env.REACT_URL ?? 'http://127.0.0.1:5181' },
];

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch();
const results = {
  capturedAt: new Date().toISOString(),
  viewport,
  locale: 'zh-CN',
  targets: [],
  evidenceBoundary: {
    included: ['public login render', 'unauthenticated protected-route redirect', 'DOM text and computed styles', 'viewport screenshot'],
    excluded: ['authenticated user data', 'permission variants', 'real mutations', 'Wails', 'iOS', 'Android'],
  },
};

for (const target of targets) {
  const context = await browser.newContext({ viewport, locale: 'zh-CN' });
  const page = await context.newPage();
  const errors = [];
  const failedResponses = [];
  page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`));
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`console: ${message.text()}`);
  });
  page.on('response', (response) => {
    if (response.status() >= 400) failedResponses.push({ status: response.status(), url: response.url() });
  });

  const loginURL = `${target.baseURL}/login`;
  const protectedURL = `${target.baseURL}/platform/apps`;
  const loginResponse = await page.goto(loginURL, { waitUntil: 'networkidle', timeout: 20000 });
  await page.waitForTimeout(500);
  const loginSnapshot = await page.evaluate(() => {
    const first = (selector) => document.querySelector(selector);
    const style = (selector) => {
      const element = first(selector);
      if (!element) return null;
      const computed = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return {
        selector,
        text: element.textContent?.trim().replace(/\s+/g, ' ').slice(0, 160) ?? '',
        rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
        color: computed.color,
        backgroundColor: computed.backgroundColor,
        fontFamily: computed.fontFamily,
        fontSize: computed.fontSize,
        lineHeight: computed.lineHeight,
        borderRadius: computed.borderRadius,
      };
    };
    return {
      title: document.title,
      bodyText: document.body.innerText.replace(/\s+/g, ' ').trim().slice(0, 1200),
      inputs: [...document.querySelectorAll('input')].map((input) => ({ type: input.type, name: input.name, placeholder: input.placeholder })),
      buttons: [...document.querySelectorAll('button')].map((button) => button.innerText.trim()).filter(Boolean).slice(0, 12),
      styles: [style('h1, h2, h3, h4'), style('input'), style('button')].filter(Boolean),
    };
  });
  const screenshot = `${target.name}-login-public.png`;
  await page.screenshot({ path: resolve(outputDir, screenshot), fullPage: false });

  const protectedResponse = await page.goto(protectedURL, { waitUntil: 'networkidle', timeout: 20000 });
  await page.waitForTimeout(500);
  const protectedSnapshot = {
    requestedURL: protectedURL,
    responseStatus: protectedResponse?.status() ?? null,
    finalURL: page.url(),
    redirectedToLogin: new URL(page.url()).pathname === '/login',
    visibleLoginHeading: await page.locator('h1, h2, h3, h4').filter({ hasText: /登录|Sign in/i }).count() > 0,
  };

  results.targets.push({
    name: target.name,
    baseURL: target.baseURL,
    login: { requestedURL: loginURL, responseStatus: loginResponse?.status() ?? null, finalURL: loginURL, snapshot: loginSnapshot, screenshot },
    unauthenticatedProtectedRoute: protectedSnapshot,
    browserErrors: errors,
    failedResponses,
  });
  await context.close();
}

await browser.close();
await import('node:fs/promises').then(({ writeFile }) => writeFile(resolve(outputDir, 'results.json'), `${JSON.stringify(results, null, 2)}\n`));
console.log(JSON.stringify({ outputDir, targets: results.targets.map(({ name, login, unauthenticatedProtectedRoute, browserErrors }) => ({ name, loginStatus: login.responseStatus, protected: unauthenticatedProtectedRoute, browserErrors })) }, null, 2));
