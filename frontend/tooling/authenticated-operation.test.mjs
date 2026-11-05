import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { pathToFileURL } from "node:url";

const root = path.resolve(import.meta.dirname, "..");
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-auth-operation-"),
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

const errorsPath = await compile("src/lib/api/errors.ts", "api-errors.mjs");
await writeFile(
  path.join(scratch, "private-blob-urls.mjs"),
  "export const privateBlobUrls = { clear() {} };",
);
const authPath = await compile(
  "src/features/auth/application/auth-facade.ts",
  "auth-facade.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/errors", "./api-errors.mjs"],
    ["@/lib/api/private-blob-urls", "./private-blob-urls.mjs"],
  ],
);
const { AuthFacade } = await import(pathToFileURL(authPath));
const apiErrors = await import(pathToFileURL(errorsPath));
const accountId = "550e8400-e29b-41d4-a716-446655440000";

function authenticatedAuth() {
  const accessToken = "synthetic-memory-only-token";
  return new AuthFacade({
    login: async () => ({
      accessToken,
      session: {
        accountId,
        roles: [],
        capabilities: [],
        restrictedMode: false,
        expiresAt: new Date(Date.now() + 60_000).toISOString(),
      },
    }),
    currentSession: async () => ({
      accountId,
      roles: [],
      capabilities: [],
      restrictedMode: false,
    }),
    logout: async () => {},
  });
}

test("authenticated operations reject late results after logout", async () => {
  const auth = authenticatedAuth();
  await auth.login({ email: "test@example.invalid", password: "Synthetic1!" });
  let finish;
  let sessionSignal;
  const pending = auth.runAuthenticatedOperation((token, signal) => {
    assert.equal(token, "synthetic-memory-only-token");
    assert.equal(signal.aborted, false);
    sessionSignal = signal;
    return new Promise((resolve) => {
      finish = resolve;
    });
  });

  await auth.logout();
  assert.equal(sessionSignal.aborted, true);
  finish("private result from the previous session");

  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(auth.getSnapshot().status, "anonymous");
});

test("authenticated operations clear the session after a 401 or 403", async () => {
  const auth = authenticatedAuth();
  await auth.login({ email: "test@example.invalid", password: "Synthetic1!" });
  const error = new apiErrors.ApiError({
    status: 403,
    code: "forbidden",
    requestId: null,
    retryAfter: null,
    fields: [],
  });

  await assert.rejects(
    auth.runAuthenticatedOperation(async () => {
      throw error;
    }),
    (actual) => actual === error,
  );
  assert.equal(auth.getSnapshot().status, "anonymous");
  assert.equal(auth.getAccessToken(), null);
});

test("cookie restoration retries a transient error and deduplicates concurrent callers", async () => {
  let calls = 0;
  const auth = new AuthFacade({
    currentSession: async () => {
      calls++;
      if (calls === 1) throw new Error("synthetic transient outage");
      return {
        accountId,
        roles: [],
        capabilities: [],
        restrictedMode: false,
        expiresAt: new Date(Date.now() + 60_000).toISOString(),
      };
    },
    logout: async () => {},
  });
  await assert.rejects(auth.restoreSession());
  const first = auth.restoreSession();
  const second = auth.restoreSession();
  assert.equal(first, second);
  assert.equal(await first, true);
  assert.equal(calls, 2);
  assert.equal(auth.getAccessToken(), "bff-session");
  await auth.logout();
});

test("logout during cookie restoration clears the BFF cookie and fences the late result", async () => {
  let resolve;
  let logoutCalls = 0;
  const auth = new AuthFacade({
    currentSession: () =>
      new Promise((done) => {
        resolve = done;
      }),
    logout: async () => {
      logoutCalls++;
    },
  });
  const restoring = auth.restoreSession();
  await auth.logout();
  assert.equal(logoutCalls, 1);
  resolve({
    accountId,
    roles: [],
    capabilities: [],
    restrictedMode: false,
    expiresAt: new Date(Date.now() + 60_000).toISOString(),
  });
  assert.equal(await restoring, false);
  assert.equal(auth.getAccessToken(), null);
  assert.equal(await auth.restoreSession(), false);
});

test.after(async () => rm(scratch, { recursive: true, force: true }));
