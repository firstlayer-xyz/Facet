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
    // Use vite dev to serve the frontend. The earlier "build first" approach
    // ran tsc against src/, which requires the Wails-generated wailsjs/
    // bindings — and those are gitignored, only produced by `wails build`
    // locally. CI has no Wails, so tsc-driven builds fail there. vite dev
    // serves the same source with esbuild transformation, exercising the
    // same code paths the tests care about (Monaco, app logic, layout) with
    // no separate type-check step blocking the build.
    command: `npx vite --port ${PREVIEW_PORT} --strictPort`,
    cwd: '..',
    port: PREVIEW_PORT,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
  },
});
