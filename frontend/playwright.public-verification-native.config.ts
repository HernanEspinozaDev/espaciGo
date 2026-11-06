import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "phase-public-verification-native.spec.ts",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  timeout: 180_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: "https://127.0.0.1:4203",
    ignoreHTTPSErrors: true,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  projects: [{ name: "chromium-local", use: { ...devices["Desktop Chrome"] } }],
});
