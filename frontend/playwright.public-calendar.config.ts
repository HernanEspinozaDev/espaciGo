import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "public-weekly-calendar.spec.ts",
  fullyParallel: false,
  reporter: "list",
  outputDir: "test-results/public-calendar",
  use: { baseURL: "http://127.0.0.1:4207", trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      command: "node tooling/public-calendar-fixture.mjs",
      url: "http://127.0.0.1:18088/healthz",
      reuseExistingServer: false,
      timeout: 10000,
    },
    {
      command:
        "API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 HOSTNAME=127.0.0.1 PORT=4207 pnpm start:local",
      url: "http://127.0.0.1:4207/api/health",
      reuseExistingServer: false,
      timeout: 60000,
    },
  ],
});
