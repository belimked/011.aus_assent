import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  testMatch: '**/*.spec.js',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  workers: 1, // Shared seeded database; each spec creates its own uniquely named records.
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  outputDir: 'test-results',
  use: {
    baseURL: 'http://127.0.0.1:8083',
    browserName: 'chromium',
    screenshot: 'retain-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
  webServer: {
    command: 'bash scripts/start-server.sh',
    url: 'http://127.0.0.1:8083/api/v1/health',
    reuseExistingServer: false,
    timeout: 180_000,
  },
});
