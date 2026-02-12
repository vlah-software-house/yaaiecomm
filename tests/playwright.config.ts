import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? 'github' : 'html',
  timeout: 30_000,

  use: {
    baseURL: process.env.ADMIN_URL || 'http://localhost:8081',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  /* Start the Go server before running tests (CI only — dev should start manually) */
  ...(process.env.CI
    ? {
        webServer: {
          command: 'cd ../api && go run ./cmd/server/',
          port: 8081,
          timeout: 30_000,
          reuseExistingServer: false,
        },
      }
    : {}),
});
