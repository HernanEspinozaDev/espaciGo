import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "wizard-public-native.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 120_000,
  outputDir:
    "../.local/testing/wizard-improvements/wizard-public-native-output",
  reporter: "list",
  use: {
    baseURL: "https://127.0.0.1:4206",
    ignoreHTTPSErrors: true,
    trace: "off",
    video: "off",
    screenshot: "off",
  },
});
