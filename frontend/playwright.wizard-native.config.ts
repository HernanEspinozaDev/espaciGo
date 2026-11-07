import { defineConfig, devices } from "@playwright/test";

const baseURL = "https://127.0.0.1:4206";

export default defineConfig({
  testDir: "./tests",
  testMatch: [
    "wizard-improvements-native.spec.ts",
    "wizard-editor-native.spec.ts",
  ],
  fullyParallel: false,
  reporter: "list",
  timeout: 90_000,
  expect: { timeout: 10_000 },
  outputDir: "../.local/testing/wizard-improvements/playwright-output",
  use: {
    baseURL,
    ignoreHTTPSErrors: true,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  projects: [
    { name: "chromium-native-local", use: { ...devices["Desktop Chrome"] } },
  ],
});
