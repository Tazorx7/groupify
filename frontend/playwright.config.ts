import { defineConfig, devices } from '@playwright/test'

/**
 * The suite runs against a real server — the built SPA served by the Go API,
 * backed by Postgres — because the things that broke in development were
 * integration problems, not rendering problems: a 401 loop, a cookie the
 * browser refused to send back, a redirect that fought the router.
 *
 * `scripts/e2e.sh` starts that stack on a throwaway database. Set
 * E2E_BASE_URL to point somewhere else (the Vite dev server, for instance).
 */
const baseURL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:8090'

export default defineConfig({
  testDir: './e2e',
  // The tests share one database and change global state (the registration
  // period), so they are ordered and serialised rather than raced.
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 30_000,
  expect: { timeout: 10_000 },
  reporter: process.env.CI ? [['github'], ['list']] : [['list']],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    video: 'off',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
