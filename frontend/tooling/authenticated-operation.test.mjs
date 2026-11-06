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
  `let clears = 0;
export const privateBlobUrls = { clear() { clears++; }, get clears() { return clears; } };`,
);
const lifecyclePath = await compile(
  "src/features/auth/application/auth-lifecycle.ts",
  "auth-lifecycle.mjs",
  [['import "client-only";', ""]],
);
const authPath = await compile(
  "src/features/auth/application/auth-facade.ts",
  "auth-facade.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/errors", "./api-errors.mjs"],
    ["@/lib/api/private-blob-urls", "./private-blob-urls.mjs"],
    ["./auth-lifecycle", "./auth-lifecycle.mjs"],
  ],
);
const { AuthFacade } = await import(pathToFileURL(authPath));
const { ApiError } = await import(pathToFileURL(errorsPath));
const { privateBlobUrls } = await import(
  pathToFileURL(path.join(scratch, "private-blob-urls.mjs"))
);
const { retainAuthLifecycle } = await import(pathToFileURL(lifecyclePath));
const accountId = "550e8400-e29b-41d4-a716-446655440000";
const fixtureExpiry = new Date(Date.now() + 60_000).toISOString();

function publicSession(overrides = {}) {
  return {
    accountId,
    roles: [],
    capabilities: [],
    restrictedMode: false,
    expiresAt: fixtureExpiry,
    ...overrides,
  };
}

function authenticatedAuth(port = {}) {
  return new AuthFacade({
    login: async () => publicSession(),
    currentSession: async () => publicSession(),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
    ...port,
  });
}

test("authenticated operations reject late results after logout", async () => {
  const auth = authenticatedAuth();
  await auth.login({ email: "test@example.invalid", password: "Synthetic1!" });
  let finish;
  let sessionSignal;
  const scope = auth.getAccessToken();
  assert.match(scope, /^bff-session:/);
  assert.doesNotMatch(scope, /Bearer|synthetic-memory-only-token/);
  const pending = auth.runAuthenticatedOperation((operationScope, signal) => {
    assert.equal(operationScope, scope);
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

test("401 clears session; 403 purges blobs and revalidates without forcing logout", async () => {
  const auth = authenticatedAuth();
  await auth.login({ email: "test@example.invalid", password: "Synthetic1!" });
  const unauthorized = new ApiError({
    status: 401,
    code: "unauthorized",
    requestId: null,
    retryAfter: null,
    fields: [],
  });
  await assert.rejects(
    auth.runAuthenticatedOperation(async () => {
      throw unauthorized;
    }),
    (actual) => actual === unauthorized,
  );
  assert.equal(auth.getSnapshot().status, "anonymous");

  let sessionChecks = 0;
  const forbiddenAuth = authenticatedAuth({
    currentSession: async () => {
      sessionChecks++;
      return publicSession();
    },
  });
  await forbiddenAuth.login({
    email: "test@example.invalid",
    password: "Synthetic1!",
  });
  const beforeClears = privateBlobUrls.clears;
  const forbidden = new ApiError({
    status: 403,
    code: "forbidden",
    requestId: null,
    retryAfter: null,
    fields: [],
  });
  await assert.rejects(
    forbiddenAuth.runAuthenticatedOperation(async () => {
      throw forbidden;
    }),
    (actual) => actual === forbidden,
  );
  assert.ok(privateBlobUrls.clears > beforeClears);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(sessionChecks, 1);
  assert.equal(forbiddenAuth.getSnapshot().status, "authenticated");

  const deniedBySession = authenticatedAuth({
    currentSession: async () => {
      throw new ApiError({
        status: 401,
        code: "unauthorized",
        requestId: null,
        retryAfter: null,
        fields: [],
      });
    },
  });
  await deniedBySession.login({
    email: "test@example.invalid",
    password: "Synthetic1!",
  });
  await deniedBySession
    .runAuthenticatedOperation(async () => {
      throw forbidden;
    })
    .catch(() => undefined);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(deniedBySession.getSnapshot().status, "anonymous");
});

test("checkSession retries a transient error and deduplicates concurrent callers", async () => {
  let calls = 0;
  const auth = authenticatedAuth({
    currentSession: async () => {
      calls++;
      if (calls === 1) throw new Error("synthetic transient outage");
      return publicSession();
    },
  });
  await assert.rejects(auth.checkSession());
  const first = auth.checkSession();
  const second = auth.checkSession();
  assert.equal(first, second);
  assert.equal(await first, true);
  assert.equal(calls, 2);
  assert.match(auth.getAccessToken(), /^bff-session:/);
  await auth.logout();
});

test("logout during session check clears the cookie and fences the late result", async () => {
  let resolve;
  let currentSessionCalls = 0;
  let logoutCalls = 0;
  const auth = authenticatedAuth({
    currentSession: () => {
      currentSessionCalls++;
      if (currentSessionCalls > 1) return Promise.resolve(publicSession());
      return new Promise((done) => {
        resolve = done;
      });
    },
    logout: async () => {
      logoutCalls++;
      return { cookieCleared: true, revocationConfirmed: true };
    },
  });
  const checking = auth.checkSession();
  await auth.logout();
  assert.equal(logoutCalls, 1);
  resolve(publicSession());
  assert.equal(await checking, false);
  assert.equal(auth.getAccessToken(), null);
});

test.after(async () => rm(scratch, { recursive: true, force: true }));
