import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const scratch = await mkdtemp(
  path.join(process.env.TMPDIR ?? os.tmpdir(), "espacigo-bff-auth-"),
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

await compile("src/lib/api/exact-json.ts", "exact-json.mjs");
await compile("src/lib/api/errors.ts", "api-errors.mjs");
await writeFile(
  path.join(scratch, "private-blob-urls.mjs"),
  `let clearedCount = 0;
export const privateBlobUrls = {
  clear() { clearedCount++; },
  get clearedCount() { return clearedCount; }
};`,
);

const adapterPath = await compile(
  "src/features/auth/infrastructure/auth-openapi-adapter.ts",
  "auth-adapter.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/errors", "./api-errors.mjs"],
    ["@/lib/api/exact-json", "./exact-json.mjs"],
  ],
);

const lifecyclePath = await compile(
  "src/features/auth/application/auth-lifecycle.ts",
  "auth-lifecycle.mjs",
  [['import "client-only";', ""]],
);

const facadePath = await compile(
  "src/features/auth/application/auth-facade.ts",
  "auth-facade.mjs",
  [
    ['import "client-only";', ""],
    ["@/lib/api/errors", "./api-errors.mjs"],
    ["@/lib/api/private-blob-urls", "./private-blob-urls.mjs"],
    ["./auth-lifecycle", "./auth-lifecycle.mjs"],
  ],
);

const { OpenApiAuthAdapter } = await import(pathToFileURL(adapterPath));
const { AuthFacade } = await import(pathToFileURL(facadePath));
const { retainAuthLifecycle } = await import(pathToFileURL(lifecyclePath));
const { ApiError } = await import(
  pathToFileURL(path.join(scratch, "api-errors.mjs"))
);
const { privateBlobUrls } = await import(
  pathToFileURL(path.join(scratch, "private-blob-urls.mjs"))
);

// Controlled fixture deadlines are fixed per run, never renewed by session checks.
const fixtureExpiry = new Date(Date.now() + 60 * 60 * 1000).toISOString();
const laterFixtureExpiry = new Date(
  Date.now() + 2 * 60 * 60 * 1000,
).toISOString();
const uuid = "550e8400-e29b-41d4-a716-446655440000";
const uuid2 = "660e8400-e29b-41d4-a716-446655440000";

test("OpenApiAuthAdapter login sends POST to BFF path with same-origin, no Authorization, and parses snake_case payload", async () => {
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ url, init });
    return new Response(
      JSON.stringify({
        account_id: uuid,
        roles: ["user"],
        capabilities: ["profile_read"],
        restricted_mode: false,
        expires_at: fixtureExpiry,
      }),
      { status: 200, headers: { "content-type": "application/json" } },
    );
  };

  const adapter = new OpenApiAuthAdapter();
  const controller = new AbortController();
  const session = await adapter.login(
    { email: "test@example.invalid", password: "secret123" },
    controller.signal,
  );

  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "/api/backend/auth/login");
  assert.equal(calls[0].init.method, "POST");
  assert.equal(calls[0].init.credentials, "same-origin");
  assert.equal(calls[0].init.cache, "no-store");
  assert.equal(calls[0].init.headers["Authorization"], undefined);
  assert.equal(calls[0].init.headers["Content-Type"], "application/json");
  assert.deepEqual(session, {
    accountId: uuid,
    roles: ["user"],
    capabilities: ["profile_read"],
    restrictedMode: false,
    expiresAt: fixtureExpiry,
  });
});

test("OpenApiAuthAdapter currentSession sends GET to /api/backend/auth/session without Authorization", async () => {
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ url, init });
    return new Response(
      JSON.stringify({
        account_id: uuid,
        roles: ["user", "admin"],
        capabilities: ["profile_read", "admin_read"],
        restricted_mode: true,
        expires_at: laterFixtureExpiry,
      }),
      { status: 200, headers: { "content-type": "application/json" } },
    );
  };

  const adapter = new OpenApiAuthAdapter();
  const session = await adapter.currentSession(new AbortController().signal);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "/api/backend/auth/session");
  assert.equal(calls[0].init.method, "GET");
  assert.equal(calls[0].init.credentials, "same-origin");
  assert.equal(calls[0].init.headers["Authorization"], undefined);
  assert.equal(session.accountId, uuid);
  assert.equal(session.restrictedMode, true);
});

test("OpenApiAuthAdapter logout distinguishes 204 complete, 502 local cleared, and network failures", async () => {
  const adapter = new OpenApiAuthAdapter();

  // 204: both local and remote confirmed
  globalThis.fetch = async () => new Response(null, { status: 204 });
  const res204 = await adapter.logout(new AbortController().signal);
  assert.deepEqual(res204, { cookieCleared: true, revocationConfirmed: true });

  // 502 with header X-Espacigo-Session-Cleared: 1
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({ error: { code: "remote_logout_unconfirmed" } }),
      {
        status: 502,
        headers: {
          "content-type": "application/json",
          "x-espacigo-session-cleared": "1",
        },
      },
    );
  const res502 = await adapter.logout(new AbortController().signal);
  assert.deepEqual(res502, { cookieCleared: true, revocationConfirmed: false });

  // Network failure throws without claiming success
  globalThis.fetch = async () => {
    throw new TypeError("Failed to fetch");
  };
  await assert.rejects(adapter.logout(new AbortController().signal), TypeError);
});

test("AuthFacade initial snapshot is checking and server snapshot is checking", () => {
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  assert.equal(auth.getSnapshot().status, "checking");
  assert.equal(auth.getServerSnapshot().status, "checking");
});

test("AuthFacade checkSession coalesces concurrent calls into one port invocation", async () => {
  let portCalls = 0;
  let resolvePort;
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => {
      portCalls++;
      return new Promise((resolve) => {
        resolvePort = () =>
          resolve({
            accountId: uuid,
            roles: ["user"],
            capabilities: [],
            restrictedMode: false,
            expiresAt: fixtureExpiry,
          });
      });
    },
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  const promise1 = auth.checkSession();
  const promise2 = auth.checkSession();
  assert.equal(portCalls, 1);
  assert.equal(promise1, promise2);

  resolvePort();
  const [res1, res2] = await Promise.all([promise1, promise2]);
  assert.equal(res1, true);
  assert.equal(res2, true);
  assert.equal(auth.getSnapshot().status, "authenticated");
});

test("AuthFacade checkSession does NOT abort an active in-flight login on remount", async () => {
  let resolveLogin;
  const auth = new AuthFacade({
    login: async (_input, signal) => {
      return new Promise((resolve, reject) => {
        signal.addEventListener("abort", () =>
          reject(new DOMException("Aborted", "AbortError")),
        );
        resolveLogin = () =>
          resolve({
            accountId: uuid,
            roles: ["user"],
            capabilities: [],
            restrictedMode: false,
            expiresAt: fixtureExpiry,
          });
      });
    },
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  const loginPromise = auth.login({
    email: "test@example.invalid",
    password: "secret123",
  });
  // Simulate component remount triggering checkSession concurrently
  const checkPromise = auth.checkSession();

  resolveLogin();
  const loginResult = await loginPromise;
  assert.equal(loginResult, true);
  assert.equal(auth.getSnapshot().status, "authenticated");
  await checkPromise;
});

test("AuthFacade account change A to B purges private blobs and updates identity", async () => {
  let currentAccountId = uuid;
  const auth = new AuthFacade({
    login: async () => ({
      accountId: currentAccountId,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: currentAccountId,
      roles: ["user"],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  await auth.checkSession();
  assert.equal(auth.getSnapshot().accountId, uuid);

  // Now backend returns account B
  currentAccountId = uuid2;
  const blobBefore = privateBlobUrls.clearedCount;
  await auth.checkSession();
  assert.equal(auth.getSnapshot().accountId, uuid2);
  assert.ok(
    privateBlobUrls.clearedCount > blobBefore,
    "blobs should be purged on account change",
  );
});

test("AuthFacade discards late stale responses from earlier generations", async () => {
  let finishFirstSession;
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid2,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => {
      return new Promise((resolve) => {
        finishFirstSession = () =>
          resolve({
            accountId: uuid,
            roles: ["user"],
            capabilities: [],
            restrictedMode: false,
            expiresAt: fixtureExpiry,
          });
      });
    },
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  const firstCheck = auth.checkSession();
  // User logs in to account B before first check resolves
  await auth.login({ email: "b@example.invalid", password: "secret" });
  assert.equal(auth.getSnapshot().accountId, uuid2);

  // Now the stale first check resolves
  finishFirstSession();
  const checkResult = await firstCheck;
  assert.equal(checkResult, false);
  assert.equal(
    auth.getSnapshot().accountId,
    uuid2,
    "should preserve newer session and discard stale response",
  );
});

test("AuthFacade 401 in runAuthenticated clears session state to anonymous", async () => {
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  await auth.login({ email: "a@example.invalid", password: "secret" });
  assert.equal(auth.getSnapshot().status, "authenticated");

  await assert.rejects(
    auth.runAuthenticated(async () => ({
      response: new Response(
        JSON.stringify({ error: { code: "unauthorized" } }),
        { status: 401 },
      ),
      value: null,
    })),
    (err) => err instanceof ApiError && err.status === 401,
  );
  assert.equal(auth.getSnapshot().status, "anonymous");
});

test("AuthFacade 403 on operation purges private blobs and revalidates without clearing valid session", async () => {
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => {
      return {
        accountId: uuid,
        roles: ["user"],
        capabilities: [],
        restrictedMode: false,
        expiresAt: fixtureExpiry,
      };
    },
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  await auth.login({ email: "a@example.invalid", password: "secret" });
  assert.equal(auth.getSnapshot().status, "authenticated");

  const blobCountBefore = privateBlobUrls.clearedCount;
  await assert.rejects(
    auth.runAuthenticated(async () => ({
      response: new Response(
        JSON.stringify({ error: { code: "forbidden_action" } }),
        { status: 403 },
      ),
      value: null,
    })),
    (err) => err instanceof ApiError && err.status === 403,
  );

  // Private blobs were purged
  assert.ok(privateBlobUrls.clearedCount > blobCountBefore);
  // Session is NOT marked expired/anonymous because 403 was an operation permission denial
  assert.equal(auth.getSnapshot().status, "authenticated");
});

test("AuthFacade 503 / network error does NOT convert into invalid credential", async () => {
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => {
      throw new ApiError({
        status: 503,
        code: "service_unavailable",
        requestId: null,
        retryAfter: null,
        fields: [],
      });
    },
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  await auth.login({ email: "a@example.invalid", password: "secret" });
  assert.equal(auth.getSnapshot().status, "authenticated");

  // checkSession fails with 503
  await assert.rejects(
    auth.checkSession(),
    (err) => err instanceof ApiError && err.status === 503,
  );
  // Session status remains authenticated (503 is not an invalid credential)
  assert.equal(auth.getSnapshot().status, "authenticated");
});

test("AuthFacade logout sets espacigo_logout_pending and clears it when cookie is cleared", async () => {
  const storage = new Map();
  globalThis.window = {
    localStorage: {
      getItem: (k) => storage.get(k) ?? null,
      setItem: (k, v) => storage.set(k, String(v)),
      removeItem: (k) => storage.delete(k),
    },
    sessionStorage: {
      getItem: () => null,
      removeItem: () => {},
    },
  };

  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => {
      // During logout, espacigo_logout_pending is '1'
      assert.equal(storage.get("espacigo_logout_pending"), "1");
      return { cookieCleared: true, revocationConfirmed: true };
    },
  });

  await auth.login({ email: "a@example.invalid", password: "secret" });
  await auth.logout();
  assert.equal(auth.getSnapshot().status, "anonymous");
  // Once confirmed cleared, marker is removed
  assert.equal(storage.get("espacigo_logout_pending"), undefined);
});

test("AuthFacade pending logout marker prevents auto-restoring session on reopen until BFF cleans", async () => {
  const storage = new Map([["espacigo_logout_pending", "1"]]);
  globalThis.window = {
    localStorage: {
      getItem: (k) => storage.get(k) ?? null,
      setItem: (k, v) => storage.set(k, String(v)),
      removeItem: (k) => storage.delete(k),
    },
    sessionStorage: {
      getItem: () => null,
      removeItem: () => {},
    },
  };

  let currentSessionCalled = false;
  let logoutCalled = false;
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => {
      currentSessionCalled = true;
      return {
        accountId: uuid,
        roles: ["user"],
        capabilities: [],
        restrictedMode: false,
        expiresAt: fixtureExpiry,
      };
    },
    logout: async () => {
      logoutCalled = true;
      return { cookieCleared: true, revocationConfirmed: true };
    },
  });

  // Check session with pending logout marker
  const restored = await auth.checkSession();
  assert.equal(restored, false);
  assert.equal(auth.getSnapshot().status, "anonymous");
  assert.equal(
    currentSessionCalled,
    false,
    "currentSession must NOT be called when logout is pending",
  );
  assert.equal(
    logoutCalled,
    true,
    "BFF logout should be triggered to clean up",
  );
  assert.equal(
    storage.get("espacigo_logout_pending"),
    undefined,
    "marker cleared after successful cleanup",
  );
});

test("AuthFacade logout throws remote_logout_unconfirmed on 502 with local cookie cleared", async () => {
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({
      cookieCleared: true,
      revocationConfirmed: false,
    }),
  });

  await auth.login({ email: "a@example.invalid", password: "secret" });
  await assert.rejects(
    auth.logout(),
    (err) =>
      err instanceof ApiError &&
      err.status === 502 &&
      err.code === "remote_logout_unconfirmed",
  );
  // Local session was cleared immediately
  assert.equal(auth.getSnapshot().status, "anonymous");
});

test("AuthFacade handlePeerLogout immediately purges local session and blobs", async () => {
  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: ["user"],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  await auth.login({ email: "a@example.invalid", password: "secret" });
  assert.equal(auth.getSnapshot().status, "authenticated");

  const blobsBefore = privateBlobUrls.clearedCount;
  auth.handlePeerLogout();
  assert.equal(auth.getSnapshot().status, "anonymous");
  assert.ok(privateBlobUrls.clearedCount > blobsBefore);
});

test("retainAuthLifecycle manages refcount and registers event listeners and channel cleanup", () => {
  const listeners = new Map();
  globalThis.window = {
    addEventListener: (type, fn) => listeners.set(type, fn),
    removeEventListener: (type) => listeners.delete(type),
  };
  globalThis.document = {
    addEventListener: (type, fn) => listeners.set(type, fn),
    removeEventListener: (type) => listeners.delete(type),
    visibilityState: "visible",
  };
  let channelClosed = false;
  globalThis.BroadcastChannel = class {
    constructor(name) {
      this.name = name;
    }
    close() {
      channelClosed = true;
    }
    postMessage() {}
  };

  const auth = new AuthFacade({
    login: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    currentSession: async () => ({
      accountId: uuid,
      roles: [],
      capabilities: [],
      restrictedMode: false,
      expiresAt: fixtureExpiry,
    }),
    logout: async () => ({ cookieCleared: true, revocationConfirmed: true }),
  });

  const release1 = retainAuthLifecycle(auth);
  assert.ok(listeners.has("focus"));
  assert.ok(listeners.has("visibilitychange"));
  assert.ok(listeners.has("pageshow"));

  // Second subscriber increments refcount
  const release2 = retainAuthLifecycle(auth);
  release1();
  // Still active because release2 hasn't been called
  assert.ok(listeners.has("focus"));

  // Final release cleans up everything
  release2();
  assert.equal(listeners.has("focus"), false);
  assert.equal(listeners.has("visibilitychange"), false);
  assert.equal(listeners.has("pageshow"), false);
  assert.equal(channelClosed, true);
});

test.after(async () => rm(scratch, { recursive: true, force: true }));
