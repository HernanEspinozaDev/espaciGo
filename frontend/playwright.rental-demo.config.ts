import path from "node:path";
import { defineConfig, devices } from "@playwright/test";

const frontendRoot = process.cwd();
const repoRoot = path.resolve(frontendRoot, "..");
const uiPort = Number(process.env.RENTAL_DEMO_UI_PORT ?? 4205);
const apiPort = Number(process.env.RENTAL_DEMO_API_PORT ?? 18117);
for (const [name, port] of [
  ["RENTAL_DEMO_UI_PORT", uiPort],
  ["RENTAL_DEMO_API_PORT", apiPort],
] as const) {
  if (!Number.isInteger(port) || port < 1024 || port > 65535)
    throw new Error(`${name} must be an unprivileged TCP port`);
}

const baseURL = `http://127.0.0.1:${uiPort}`;
const apiURL = `http://127.0.0.1:${apiPort}`;
const quote = (value: string) => `'${value.replaceAll("'", "'\\''")}'`;
const isolatedEnv = [
  "env -i",
  `PATH=${quote(process.env.PATH ?? "")}`,
  `HOME=${quote(process.env.HOME ?? "")}`,
  `TMPDIR=${quote(process.env.TMPDIR ?? "/home/nandev/.hermes/cache/scratch")}`,
].join(" ");

export default defineConfig({
  testDir: "./tests",
  testMatch: "rental-demo-visible.spec.ts",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  timeout: 45_000,
  expect: { timeout: 8_000 },
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: `${isolatedEnv} GOPROXY=off GOTOOLCHAIN=local go run ./cmd/rentaldemo -addr 127.0.0.1:${apiPort} -now 2030-01-15T12:00:00Z`,
      cwd: repoRoot,
      url: `${apiURL}/api/v1/local/rental-demo/spaces`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      command: `${isolatedEnv} NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=${uiPort} RENTAL_DEMO_ENABLED=1 RENTAL_DEMO_BASE_URL=${quote(apiURL)} pnpm start:local`,
      cwd: frontendRoot,
      url: `${baseURL}/demo/arriendo?mode=public&month=2030-01`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});
