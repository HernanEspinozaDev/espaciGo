import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const adapter = await readFile(
  path.join(
    root,
    "src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts",
  ),
  "utf8",
);

test("host photo variant uses typed API client blob GET with abort signal", () => {
  const variant = adapter.slice(adapter.indexOf("async getPhotoVariant("));
  assert.match(variant, /variant:\s*"thumbnail"\s*\|\s*"display"/);
  assert.match(variant, /parseAs:\s*"blob"/);
  assert.match(variant, /signal/);
  assert.doesNotMatch(variant, /Authorization\s*:/);
  assert.doesNotMatch(variant, /credentials\s*:/);
});

test("createDraft does not send unsupported Idempotency-Key", () => {
  const start = adapter.indexOf("async createDraft(");
  const end = adapter.indexOf("async updateDraft(", start);
  assert.ok(start >= 0 && end > start);
  assert.doesNotMatch(adapter.slice(start, end), /Idempotency-Key/);
});
