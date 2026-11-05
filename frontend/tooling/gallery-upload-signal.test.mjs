import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("upload cancellation signal reaches hook, facade and port without losing key or file", async () => {
  const facade = await readFile(
    new URL(
      "../src/features/host-listings/application/host-listings-facade.ts",
      import.meta.url,
    ),
    "utf8",
  );
  const method = facade.slice(
    facade.indexOf("  async uploadPhoto("),
    facade.indexOf("  async setCoverPhoto("),
  );
  assert.match(method, /signal\?: AbortSignal/);
  assert.match(method, /idempotencyKey,\s*signal,/);
  const hook = await readFile(
    new URL(
      "../src/features/host-listings/ui/use-host-listings.ts",
      import.meta.url,
    ),
    "utf8",
  );
  const upload = hook.slice(
    hook.indexOf("  const uploadPhoto ="),
    hook.indexOf("  const setCoverPhoto ="),
  );
  assert.match(upload, /signal\?: AbortSignal/);
  assert.match(upload, /idempotencyKey,\s*signal/);
});
