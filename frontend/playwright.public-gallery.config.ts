import { defineConfig, devices } from "@playwright/test";

const appPort = Number(process.env.PUBLIC_GALLERY_PORT ?? 4208);
const fixturePort = Number(process.env.PUBLIC_GALLERY_FIXTURE_PORT ?? 18089);

export default defineConfig({
  testDir: "./tests",
  testMatch: ["public-gallery-media.spec.ts"],
  fullyParallel: false,
  reporter: "list",
  outputDir: "test-results/public-gallery",
  use: {
    baseURL: `http://127.0.0.1:${appPort}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: `PUBLIC_GALLERY_FIXTURE_PORT=${fixturePort} node tooling/public-gallery-fixture.mjs`,
      url: `http://127.0.0.1:${fixturePort}/healthz`,
      reuseExistingServer: false,
      timeout: 10000,
    },
    {
      command: `API_BACKEND_BASE_URL=http://127.0.0.1:${fixturePort}/api/v1 HOSTNAME=127.0.0.1 PORT=${appPort} pnpm start:local`,
      url: `http://127.0.0.1:${appPort}/api/health`,
      reuseExistingServer: false,
      timeout: 60000,
    },
  ],
});
