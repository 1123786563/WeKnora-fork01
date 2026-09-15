#!/usr/bin/env node
// Read-only public runtime evidence. No credentials, mutations, or mocks.
// Run with: pnpm --dir apps/web exec node artifacts/browser-evidence-20260915/collect-public-route-evidence.mjs
import { createRequire } from 'node:module';
import { mkdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

// Resolve through the web package so the locked workspace dependency is used
// even though this evidence script lives under artifacts/.
const require = createRequire(new URL('../../apps/web/package.json', import.meta.url));
const { chromium } = require('@playwright/test');

const outputDir = resolve(process.env.EVIDENCE_OUTPUT_DIR ?? 'artifacts/browser-evidence-20260915');
const viewport = { width: 1440, height: 900 };
const targets = [
  { name: 'vue', baseURL: process.env.VUE_URL ?? 'http://127.0.0.1:5180' },
  { name: 'react', baseURL: process.env.REACT_URL ?? 'http://127.0.0.1:5181' },
];
const publicRoutes = ['/login', '/register', '/onboarding/workspace'];
const protectedRoute = '/platform/apps';

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch();
const results = {
  capturedAt: new Date().toISOString(),
  viewport,
  locale: 'zh-CN',
  publicRoutes,
  protectedRoute,
  targets: [],
  evidenceBoundary: {
    included: [
      'public route HTTP/render reachability',
      'unauthenticated protected-route redirect',
      'authentication API response statuses observed by the browser',
      'login input computed style and fixed-viewport screenshots',
    ],
    excluded: [
      'authenticated user data',
      'permission variants',
      'real mutations',
      'full Vue-to-React parity',
      'Wails, iOS, Android',
    ],
  },
};

for (const target of targets) {
  const context = await browser.newContext({ viewport, locale: 'zh-CN' });
  const page = await context.newPage();
  const errors = [];
  const apiResponses = [];
  page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`));
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`console: ${message.text()}`);
  });
  page.on('response', (response) => {
    const url = response.url();
    if (url.includes('/api/v1/auth/')) {
      apiResponses.push({ method: response.request().method(), status: response.status(), url });
    }
  });

  const routeResults = [];
  for (const route of publicRoutes) {
    const requestedURL = `${target.baseURL}${route}`;
    const response = await page.goto(requestedURL, { waitUntil: 'networkidle', timeout: 20000 });
    await page.waitForTimeout(300);
    routeResults.push({
      route,
      requestedURL,
      responseStatus: response?.status() ?? null,
      finalURL: page.url(),
      heading: await page.locator('h1, h2, h3, h4').first().textContent().catch(() => null),
    });
    if (route === '/login') {
      const loginSnapshot = await page.evaluate(() => {
        const loginInput = document.querySelector('input[type="text"], input[type="email"], input:not([type])');
        const inputStyle = loginInput ? getComputedStyle(loginInput) : null;
        const rect = loginInput?.getBoundingClientRect();
        const style = (selector) => {
          const element = document.querySelector(selector);
          if (!element) return null;
          const computed = getComputedStyle(element);
          const box = element.getBoundingClientRect();
          return {
            selector,
            text: element.textContent?.trim().replace(/\\s+/g, ' ').slice(0, 160) ?? '',
            rect: { x: box.x, y: box.y, width: box.width, height: box.height },
            color: computed.color,
            backgroundColor: computed.backgroundColor,
            fontFamily: computed.fontFamily,
            fontSize: computed.fontSize,
            lineHeight: computed.lineHeight,
            border: computed.border,
            borderRadius: computed.borderRadius,
            padding: computed.padding,
          };
        };
        return {
          title: document.title,
          bodyText: document.body.innerText.replace(/\\s+/g, ' ').trim().slice(0, 1600),
          inputs: [...document.querySelectorAll('input')].map((input) => ({ type: input.type, name: input.name, placeholder: input.placeholder })),
          buttons: [...document.querySelectorAll('button')].map((button) => button.innerText.trim()).filter(Boolean).slice(0, 12),
          loginInputComputedStyle: inputStyle && rect ? {
            color: inputStyle.color,
            backgroundColor: inputStyle.backgroundColor,
            fontFamily: inputStyle.fontFamily,
            fontSize: inputStyle.fontSize,
            lineHeight: inputStyle.lineHeight,
            border: inputStyle.border,
            borderRadius: inputStyle.borderRadius,
            padding: inputStyle.padding,
            box: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
          } : null,
          styles: [style('h1, h2, h3, h4'), style('input'), style('button')].filter(Boolean),
        };
      });
      await page.screenshot({ path: resolve(outputDir, `${target.name}-login-${viewport.width}x${viewport.height}.png`), fullPage: false });
      routeResults[routeResults.length - 1].snapshot = loginSnapshot;
    }
  }

  const protectedURL = `${target.baseURL}${protectedRoute}`;
  const protectedResponse = await page.goto(protectedURL, { waitUntil: 'networkidle', timeout: 20000 });
  await page.waitForTimeout(300);
  results.targets.push({
    name: target.name,
    baseURL: target.baseURL,
    publicRoutes: routeResults,
    unauthenticatedProtectedRoute: {
      requestedURL: protectedURL,
      responseStatus: protectedResponse?.status() ?? null,
      finalURL: page.url(),
      redirectedToLogin: new URL(page.url()).pathname === '/login',
      visibleLoginHeading: await page.locator('h1, h2, h3, h4').filter({ hasText: /登录|Sign in/i }).count() > 0,
    },
    authenticationAPIResponses: apiResponses,
    browserErrors: errors,
  });
  await context.close();
}

await browser.close();
await writeFile(resolve(outputDir, 'results.json'), `${JSON.stringify(results, null, 2)}\n`);
console.log(JSON.stringify({ outputDir, capturedAt: results.capturedAt, targets: results.targets.map((target) => ({ name: target.name, routes: target.publicRoutes, protected: target.unauthenticatedProtectedRoute, authenticationAPIResponses: target.authenticationAPIResponses, browserErrors: target.browserErrors })) }, null, 2));
