import { defineConfig, devices } from '@playwright/test';

const PREVIEW_PORT = 4173;

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.test.ts',
  timeout: 20_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,        // App owns global state (clipboard override, etc.) — serialize for now
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    ...devices['Desktop Chrome'],
    baseURL: `http://localhost:${PREVIEW_PORT}`,
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
  webServer: {
    // Build first so dist/ exists, then serve the built bundle. We use the
    // production build (not vite dev) so what we test matches what Wails
    // embeds into the desktop binary.
    command: `npm run build && npx vite preview --port ${PREVIEW_PORT} --strictPort`,
    cwd: '..',
    port: PREVIEW_PORT,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
  },
});
