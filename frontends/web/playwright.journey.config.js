import { defineConfig } from '@playwright/test';

// The journey suite runs against a fresh instance of its own. The journey's
// first signup is the bootstrap operator (core/internal/auth/auth.go — the
// first user becomes operator), which creates the provider and sink slots
// through the admin API before any UI signup. Sharing one server with the
// smoke suite would make "who is the operator" depend on test file
// ordering, so the journey boots its own server on the same port; the two
// configs run one after the other (package.json's test:e2e), and each
// server is in-memory, so each run starts clean.
export default defineConfig({
  testDir: './e2e',
  testMatch: 'journey/**/*.spec.js',
  timeout: 60_000,
  retries: 0,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:18443',
    headless: true,
  },
  webServer: {
    // JOURNEY_SEED_PASSWORD is the operator's seed account's password. It is
    // a valid, non-empty value so the seed logs in and syncs cleanly — the
    // fake serves it an empty library, so it never feeds the journey's
    // library while still satisfying the adapter's "at least one account"
    // requirement. A seed that failed to log in would trip the shared
    // per-provider pacing governor, taking the linked account down with it.
    command:
      'npm run build && cd ../.. && mkdir -p frontends/web/serving/dist && cp frontends/web/src/index.html frontends/web/serving/dist/index.html && cp frontends/web/dist/bundle.js frontends/web/serving/dist/bundle.js && JOURNEY_SEED_PASSWORD=seed-password go run ./frontends/web',
    url: 'http://127.0.0.1:18443/debug/capabilities',
    timeout: 60_000,
    reuseExistingServer: false,
  },
});
