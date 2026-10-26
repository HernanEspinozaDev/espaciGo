import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const frontendRoot = fileURLToPath(new URL("../", import.meta.url));
const outputPath = path.join(
  frontendRoot,
  "src/lib/api/generated/openapi.d.ts",
);
const generatorPath = path.join(frontendRoot, "tooling/api-generate.mjs");
const inventoryPath = path.join(
  frontendRoot,
  "tooling/generate-api-inventory.mjs",
);
const allowlistGeneratorPath = path.join(
  frontendRoot,
  "tooling/generate-api-allowlist.mjs",
);
const allowlistPath = path.join(
  frontendRoot,
  "src/lib/api/generated/gateway-operations.ts",
);
const temporaryDirectory = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-api-check-"),
);

function runNode(scriptPath, args = []) {
  const result = spawnSync(process.execPath, [scriptPath, ...args], {
    cwd: frontendRoot,
    encoding: "utf8",
    env: process.env,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      [result.stdout, result.stderr].filter(Boolean).join("\n") ||
        `Command failed: ${path.basename(scriptPath)}`,
    );
  }
  return result.stdout.trim();
}

try {
  const generatedPath = path.join(temporaryDirectory, "openapi.d.ts");
  const generatedAllowlistPath = path.join(
    temporaryDirectory,
    "gateway-operations.ts",
  );
  runNode(generatorPath, ["--output", generatedPath]);
  runNode(allowlistGeneratorPath, ["--output", generatedAllowlistPath]);
  const [
    committedOutput,
    generatedOutput,
    committedAllowlist,
    generatedAllowlist,
  ] = await Promise.all([
    readFile(outputPath, "utf8"),
    readFile(generatedPath, "utf8"),
    readFile(allowlistPath, "utf8"),
    readFile(generatedAllowlistPath, "utf8"),
  ]);
  assert.equal(
    committedOutput,
    generatedOutput,
    "Generated OpenAPI client is stale; run pnpm api:generate",
  );
  assert.equal(
    committedAllowlist,
    generatedAllowlist,
    "Generated gateway allowlist is stale; run pnpm api:generate",
  );
  const inventoryStatus = runNode(inventoryPath, ["--check"]);
  console.log(
    "Generated OpenAPI types, gateway allowlist, and operation inventory are current.",
  );
  console.log(inventoryStatus);
} finally {
  await rm(temporaryDirectory, { recursive: true, force: true });
}
