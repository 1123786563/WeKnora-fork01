import { defineConfig } from '@playwright/test';

/**
 * Semantic flow browser tests (W02).
 *
 * These run against the REAL Go service (baseURL from SEMANTIC_E2E_BASE_URL;
 * default the local dev deployment). They are NOT part of unit CI: execute
 * via pnpm --filter @weknora/web exec playwright test --config
 * playwright.semantic.config.ts once the service stack is up.
 */
export default defineConfig({
  testDir: 'tests',
  timeout: 60_000,
  use: {
    baseURL: process.env.SEMANTIC_E2E_BASE_URL ?? 'http://127.0.0.1:38080',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'semantic-flow', use: { browserName: 'chromium' } }],
});
