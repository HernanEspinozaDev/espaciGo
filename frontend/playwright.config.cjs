const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.cjs',
  fullyParallel: false,
  workers: 1,
  timeout: 90_000,
  outputDir: process.env.FE14_RESULTS_DIR || '/run/fe14/results/playwright',
  expect: { timeout: 15_000 },
  reporter: [['list']],
  use: {
    baseURL: process.env.FRONTEND_BASE_URL || 'http://angular-frontend:4000',
    browserName: 'chromium',
    headless: true,
    acceptDownloads: true,
    trace: 'retain-on-failure',
  },
});
