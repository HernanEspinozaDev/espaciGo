import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-reg-result-"),
);

async function compile(sourcePath, outputName, replacements = []) {
  let source = await readFile(path.join(root, sourcePath), "utf8");
  for (const [before, after] of replacements)
    source = source.replaceAll(before, after);
  const output = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
  const destination = path.join(scratch, outputName);
  await writeFile(destination, output);
  return destination;
}

const modulePath = await compile(
  "src/features/auth/application/registration-result.ts",
  "registration-result.mjs",
);

// Setup client environment for testing client-side storage
globalThis.window = {};

const {
  setRegistrationResult,
  getRegistrationResult,
  clearRegistrationResult,
  scheduleClearRegistrationResult,
  cancelScheduledClear,
  subscribeRegistrationResult,
  getRegistrationResultServerSnapshot,
} = await import(pathToFileURL(modulePath).href);

test.after(async () => {
  delete globalThis.window;
  await rm(scratch, { recursive: true, force: true });
});

test("registration-result: initially null", () => {
  clearRegistrationResult();
  assert.equal(getRegistrationResult(), null);
});

test("registration-result: stores and retrieves status 201 without extra fields", () => {
  setRegistrationResult({
    email: "usuario.valido@espacigo.com",
    status: 201,
  });
  const result = getRegistrationResult();
  assert.deepEqual(result, {
    email: "usuario.valido@espacigo.com",
    status: 201,
  });
  assert.equal(Object.keys(result).sort().join(","), "email,status");
});

test("registration-result: stores and retrieves status 202", () => {
  setRegistrationResult({
    email: "usuario.pendiente@espacigo.com",
    status: 202,
  });
  const result = getRegistrationResult();
  assert.deepEqual(result, {
    email: "usuario.pendiente@espacigo.com",
    status: 202,
  });
});

test("registration-result: returns referentially stable snapshot across multiple calls", () => {
  setRegistrationResult({
    email: "estable@espacigo.com",
    status: 201,
  });
  const snap1 = getRegistrationResult();
  const snap2 = getRegistrationResult();
  assert.equal(
    snap1,
    snap2,
    "Snapshot must maintain reference equality for useSyncExternalStore",
  );
});

test("registration-result: SSR neutral server snapshot is always null", () => {
  assert.equal(typeof getRegistrationResultServerSnapshot, "function");
  assert.equal(getRegistrationResultServerSnapshot(), null);
});

test("registration-result: subscribers are notified on set and clear", () => {
  let notifications = 0;
  const unsubscribe = subscribeRegistrationResult(() => {
    notifications++;
  });

  setRegistrationResult({
    email: "notify@espacigo.com",
    status: 201,
  });
  assert.equal(notifications, 1);

  clearRegistrationResult();
  assert.equal(notifications, 2);

  unsubscribe();
  setRegistrationResult({
    email: "silent@espacigo.com",
    status: 202,
  });
  assert.equal(notifications, 2);
  clearRegistrationResult();
});

test("registration-result: clearRegistrationResult immediately purges memory", () => {
  setRegistrationResult({
    email: "purge@espacigo.com",
    status: 201,
  });
  assert.notEqual(getRegistrationResult(), null);
  clearRegistrationResult();
  assert.equal(getRegistrationResult(), null);
});

test("registration-result: scheduled clear defaults to microtask without 1500ms delay", async () => {
  setRegistrationResult({
    email: "microtask@espacigo.com",
    status: 201,
  });
  scheduleClearRegistrationResult();
  // Microtask or next tick should wipe immediately
  await new Promise((resolve) => setTimeout(resolve, 5));
  assert.equal(getRegistrationResult(), null);
});

test("registration-result: scheduled microtask clear can be cancelled by StrictMode remount", async () => {
  setRegistrationResult({
    email: "strictmode-remount@espacigo.com",
    status: 201,
  });
  scheduleClearRegistrationResult();
  cancelScheduledClear();
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.deepEqual(getRegistrationResult(), {
    email: "strictmode-remount@espacigo.com",
    status: 201,
  });
  clearRegistrationResult();
});

test("registration-result: new registration response cannot be wiped by older scheduled cleanup", async () => {
  // Response 1
  setRegistrationResult({
    email: "first@espacigo.com",
    status: 201,
  });
  // Schedule clear for first response with delay
  scheduleClearRegistrationResult(30);

  // Response 2 arrives slightly later (new response intent)
  await new Promise((resolve) => setTimeout(resolve, 10));
  setRegistrationResult({
    email: "second@espacigo.com",
    status: 202,
  });

  // Wait for the old 30ms timer to pass
  await new Promise((resolve) => setTimeout(resolve, 35));

  // The new response must NOT have been wiped by the first cleanup
  assert.deepEqual(getRegistrationResult(), {
    email: "second@espacigo.com",
    status: 202,
  });
  clearRegistrationResult();
});

test("registration-result: SSR neutral when window is undefined", () => {
  const originalWindow = globalThis.window;
  try {
    delete globalThis.window;
    assert.equal(typeof window, "undefined");
    // SSR set does not leak into memory
    setRegistrationResult({
      email: "leak@espacigo.com",
      status: 201,
    });
    assert.equal(getRegistrationResult(), null);
  } finally {
    globalThis.window = originalWindow;
  }
});
