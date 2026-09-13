// W06 craft browser-acceptance Playwright config.
//
// Consumed entirely from the environment the stack harness exports:
//   CRAFT_WEB_URL      the web origin under test (vite dev server)
//   CRAFT_AUTH_STATE   the owner storageState produced by a real login
// Screenshots/traces land under CRAFT_E2E_OUTPUT (a temp dir outside the
// repository — see docs/testing/craft/web-acceptance.md for their location).
import { defineConfig } from '@playwright/test';
import { tmpdir } from 'node:os';

export default defineConfig({
  testDir: './e2e',
  testMatch: 'craft-*.spec.ts',
  workers: 1,
  retries: 0,
  use: {
    acceptDownloads: true,
    baseURL: process.env.CRAFT_WEB_URL ?? 'http://localhost:5173',
    storageState: process.env.CRAFT_AUTH_STATE,
    // The controlled preview origin serves a locally self-signed certificate.
    ignoreHTTPSErrors: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  timeout: 240000,
  outputDir: process.env.CRAFT_E2E_OUTPUT ?? `${tmpdir()}/craft-w06-artifacts`,
  reporter: [['list']],
});
