import { defineConfig, devices } from "@playwright/test";

const fixturePort = 18089;
const appPort = 4208;

export default defineConfig({
  testDir: "./tests",
  testMatch: "public-rental-commercial-journey.spec.ts",
  fullyParallel: false,
  reporter: "list",
  outputDir: "test-results/public-rental-commercial",
  use: {
    baseURL: `http://127.0.0.1:${appPort}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: `PUBLIC_CALENDAR_FIXTURE_PORT=${fixturePort} node tooling/public-calendar-fixture.mjs`,
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
