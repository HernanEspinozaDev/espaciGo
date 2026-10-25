import { defineConfig, devices } from "@playwright/test";

const testPort = Number(process.env.PLAYWRIGHT_PORT ?? 4203);
const baseURL = `http://127.0.0.1:${testPort}`;

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  reporter: "list",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium-local",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: `PORT=${testPort} pnpm start:local`,
    url: baseURL,
    reuseExistingServer:
      (process.env.PHASE3_NATIVE_API_E2E === "1" ||
        process.env.PHASE5_NATIVE_API_E2E === "1" ||
        process.env.PHASE6_NATIVE_API_E2E === "1" ||
        process.env.PHASE7_NATIVE_API_E2E === "1" ||
        process.env.PHASE8_NATIVE_API_E2E === "1") &&
      process.env.PLAYWRIGHT_REUSE_SERVER === "1",
    timeout: 60_000,
    stdout: "pipe",
    stderr: "pipe",
  },
});
