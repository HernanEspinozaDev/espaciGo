import { defineConfig, devices } from "@playwright/test";

const baseURL = "https://127.0.0.1:4206";

export default defineConfig({
  testDir: "./tests",
  testMatch: "wizard-editor-native-visual.spec.ts",
  fullyParallel: false,
  reporter: "list",
  timeout: 90_000,
  expect: { timeout: 10_000 },
  outputDir: "../.local/testing/wizard-improvements/playwright-visual-output",
  use: {
    baseURL,
    ignoreHTTPSErrors: true,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  projects: [
    { name: "chromium-native-visual", use: { ...devices["Desktop Chrome"] } },
  ],
});
