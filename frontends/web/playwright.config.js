import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  // The journey suite runs under its own config against a fresh server
  // (playwright.journey.config.js): it needs the bootstrap operator, which
  // is the first signup on the instance.
  testIgnore: '**/journey/**',
  timeout: 30_000,
  retries: 0,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:18443',
    headless: true,
  },
  webServer: {
    command:
      'npm run build && cd ../.. && mkdir -p frontends/web/serving/dist && cp frontends/web/src/index.html frontends/web/serving/dist/index.html && cp frontends/web/dist/bundle.js frontends/web/serving/dist/bundle.js && go run ./frontends/web',
    url: 'http://127.0.0.1:18443/debug/capabilities',
    timeout: 60_000,
    reuseExistingServer: false,
  },
});
